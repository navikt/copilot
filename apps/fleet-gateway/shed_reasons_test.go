package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

type shedFakeStore struct {
	SessionStore
	putErrForKind map[string]error
	putEvictErr   error
}

func (s *shedFakeStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	if err, ok := s.putErrForKind[kind]; ok {
		return err
	}
	return s.SessionStore.Put(ctx, kind, key, val, ttl)
}

func (s *shedFakeStore) PutEvict(ctx context.Context, kind, key string, val []byte, ttl time.Duration) (bool, error) {
	if s.putEvictErr != nil {
		return false, s.putEvictErr
	}
	if err, ok := s.putErrForKind[kind]; ok {
		return false, err
	}
	return s.SessionStore.PutEvict(ctx, kind, key, val, ttl)
}

func assertShed503(t *testing.T, wantReason, otherReason string, fn func() int) {
	t.Helper()
	wantCounter := httpSeries("503", wantReason)
	otherCounter := httpSeries("503", otherReason)
	beforeWant := testutil.ToFloat64(wantCounter)
	beforeOther := testutil.ToFloat64(otherCounter)

	code := fn()
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
	if got := testutil.ToFloat64(wantCounter); got != beforeWant+1 {
		t.Errorf("HTTPRequests(503, %q) = %v, want %v", wantReason, got, beforeWant+1)
	}
	if got := testutil.ToFloat64(otherCounter); got != beforeOther {
		t.Errorf("HTTPRequests(503, %q) = %v, want %v", otherReason, got, beforeOther)
	}
}

func TestShedReasons_StoreFull(t *testing.T) {
	const verifier = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	t.Run("dcr_register", func(t *testing.T) {
		te := newTestEnv(t)
		enableDCR(te, 10, 10)
		te.gw.store = &shedFakeStore{SessionStore: te.gw.store, putEvictErr: errStoreFull}

		assertShed503(t, "store_full", "", func() int {
			code, _ := postRegister(te, opencodeRegistration(), "10.0.0.1")
			return code
		})
	})

	t.Run("authorize_pending_entra", func(t *testing.T) {
		te := newTestEnv(t)
		te.gw.store = &shedFakeStore{
			SessionStore:  te.gw.store,
			putErrForKind: map[string]error{kindPendingEntra: errStoreFull},
		}

		assertShed503(t, "store_full", "", func() int {
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil))
			return rec.Code
		})
	})

	t.Run("callback_pending_consent", func(t *testing.T) {
		te := newTestEnv(t)
		entraState, stateCookie := te.startLogin(verifier)
		te.gw.store = &shedFakeStore{
			SessionStore:  te.gw.store,
			putErrForKind: map[string]error{kindPendingConsent: errStoreFull},
		}

		assertShed503(t, "store_full", "", func() int {
			resp := te.callback(entraState, stateCookie)
			_ = resp.Body.Close()
			return resp.StatusCode
		})
	})

	t.Run("callback_issue_code", func(t *testing.T) {
		te := newTestEnv(t)
		_ = te.obtainAuthCode(verifier) // remembers consent so the next callback issues a code directly
		entraState, stateCookie := te.startLogin(verifier)
		te.gw.store = &shedFakeStore{
			SessionStore:  te.gw.store,
			putErrForKind: map[string]error{kindCode: errStoreFull},
		}

		assertShed503(t, "store_full", "", func() int {
			resp := te.callback(entraState, stateCookie)
			_ = resp.Body.Close()
			return resp.StatusCode
		})
	})

	t.Run("token_authorization_code", func(t *testing.T) {
		te := newTestEnv(t)
		code := te.obtainAuthCode(verifier)
		te.gw.store = &shedFakeStore{
			SessionStore:  te.gw.store,
			putErrForKind: map[string]error{kindSession: errStoreFull},
		}

		assertShed503(t, "store_full", "", func() int {
			status, _ := te.postToken(url.Values{
				"grant_type":    {"authorization_code"},
				"code":          {code},
				"client_id":     {te.clientID},
				"redirect_uri":  {te.redirectURI},
				"resource":      {te.resource},
				"code_verifier": {verifier},
			})
			return status
		})
	})

	t.Run("token_refresh_token", func(t *testing.T) {
		te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
		_, rt, _ := te.obtainTokensForResource(te.resource)
		te.gw.store = &shedFakeStore{
			SessionStore:  te.gw.store,
			putErrForKind: map[string]error{kindSession: errStoreFull},
		}

		assertShed503(t, "store_full", "", func() int {
			status, _ := te.postToken(refreshForm(te, rt))
			return status
		})
	})
}

func TestShedReasons_RefreshInFlight(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	te.gw.refreshWait = 50 * time.Millisecond
	_, rt, _ := te.obtainTokensForResource(te.resource)

	entered, release := make(chan struct{}), make(chan struct{})
	te.mu.Lock()
	te.entraRefreshHook = func() {
		close(entered)
		<-release
	}
	te.mu.Unlock()

	firstDone := make(chan int, 1)
	go func() {
		status, _ := te.postToken(refreshForm(te, rt))
		firstDone <- status
	}()
	<-entered

	assertShed503(t, "refresh_in_flight", "", func() int {
		status, _ := te.postToken(refreshForm(te, rt))
		return status
	})

	close(release)
	if status := <-firstDone; status != http.StatusOK {
		t.Fatalf("in-flight refresh status = %d, want 200", status)
	}
}

func TestShedReasons_GenericStoreErrorUnlabelled(t *testing.T) {
	const verifier = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	te := newTestEnv(t)
	code := te.obtainAuthCode(verifier)
	te.gw.store = &shedFakeStore{
		SessionStore:  te.gw.store,
		putErrForKind: map[string]error{kindSession: errors.New("db connection refused")},
	}

	assertShed503(t, "", "store_full", func() int {
		status, body := te.postToken(url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {te.clientID},
			"redirect_uri":  {te.redirectURI},
			"resource":      {te.resource},
			"code_verifier": {verifier},
		})
		if body["error_description"] != "session store unavailable" {
			t.Errorf("error_description = %v, want %q", body["error_description"], "session store unavailable")
		}
		return status
	})
}
