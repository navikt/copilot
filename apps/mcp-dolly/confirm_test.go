package main

import (
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func previewToken(t *testing.T, out map[string]any) string {
	t.Helper()
	if _, ok := out["result"]; !ok {
		return ""
	}
	txt, isErr := extractToolText(t, out)
	if isErr {
		return ""
	}
	var p struct {
		Token string `json:"confirmationToken"`
	}
	if json.Unmarshal([]byte(txt), &p) != nil {
		return ""
	}
	return p.Token
}

// callOrderConfirmed runs both steps. If step 1 yields no token (validation
// error, disabled tool) it returns step 1's output.
func callOrderConfirmed(t *testing.T, srv *server, args map[string]any) (any, map[string]any) {
	t.Helper()
	_, out := callToolModern(t, srv.routes(), "in", "create_order", args)
	tok := previewToken(t, out)
	if tok == "" {
		return nil, out
	}
	a := maps.Clone(args)
	a["confirmationToken"] = tok
	return callToolModern(t, srv.routes(), "in", "create_order", a)
}

var twoStepArgs = map[string]any{"gruppeId": 42, "antall": 2, "environments": []string{"q1"}, "kjoenn": "MANN"}

func twoStepServer(t *testing.T) (*server, func() int, func(time.Duration)) {
	t.Helper()
	var mu sync.Mutex
	posts := 0
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/bruker/current":
			writeJSON(w, http.StatusOK, map[string]any{"brukerId": "o"})
		case r.URL.Path == "/api/v1/gruppe/42/page/0":
			writeJSON(w, http.StatusOK, map[string]any{"id": 42, "opprettetAv": map[string]any{"brukerId": "o"}})
		case r.Method == http.MethodPost:
			mu.Lock()
			posts++
			mu.Unlock()
			writeJSON(w, http.StatusCreated, map[string]any{"id": 1})
		default:
			http.NotFound(w, r)
		}
	})
	srv.cfg.EnableCreateOrder = true
	srv.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return posts },
		func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
}

func TestCreateOrderTwoStep(t *testing.T) {
	h := func(srv *server) http.Handler { return srv.routes() }
	call := func(srv *server, args map[string]any) map[string]any {
		_, out := callToolModern(t, h(srv), "in", "create_order", args)
		return out
	}
	withTok := func(tok string, over map[string]any) map[string]any {
		a := maps.Clone(twoStepArgs)
		maps.Copy(a, over)
		a["confirmationToken"] = tok
		return a
	}
	wantReject := func(t *testing.T, out map[string]any, posts func() int) {
		t.Helper()
		txt, isErr := extractToolText(t, out)
		if !isErr || !strings.Contains(txt, "confirmation token") {
			t.Fatalf("want confirmation error, got isErr=%v %q", isErr, txt)
		}
		if posts() != 0 {
			t.Fatal("order reached Dolly")
		}
	}

	t.Run("preview orders nothing and charges no quota", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		out := call(srv, twoStepArgs)
		txt, isErr := extractToolText(t, out)
		if isErr || previewToken(t, out) == "" {
			t.Fatalf("no preview: %q", txt)
		}
		for _, want := range []string{`"gruppeId":42`, `"antall":2`, `"q1"`, `"MANN"`, `"quotaRemaining":20`, `"defaults":"Each person is created with FNR`, `folkeregister personstatus=bosatt`} {
			if !strings.Contains(txt, want) {
				t.Errorf("preview missing %s: %s", want, txt)
			}
		}
		if posts() != 0 || srv.quota.remaining(srv.quota.hashUser("Z123456"), srv.now()) != dailyPersonQuota {
			t.Fatal("preview ordered or charged quota")
		}
	})

	t.Run("confirmed call orders once and charges quota", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		tok := previewToken(t, call(srv, twoStepArgs))
		if txt, isErr := extractToolText(t, call(srv, withTok(tok, nil))); isErr {
			t.Fatalf("confirm failed: %s", txt)
		}
		if posts() != 1 {
			t.Fatalf("posts = %d, want 1", posts())
		}
	})

	t.Run("changed params rejected", func(t *testing.T) {
		for _, over := range []map[string]any{{"antall": 3}, {"gruppeId": 43}, {"environments": []string{"q2"}}, {"kjoenn": "KVINNE"}, {"minAlder": 20}} {
			srv, posts, _ := twoStepServer(t)
			tok := previewToken(t, call(srv, twoStepArgs))
			wantReject(t, call(srv, withTok(tok, over)), posts)
		}
	})

	t.Run("other user rejected", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		tok := previewToken(t, call(srv, twoStepArgs))
		// A different user must not be able to spend Z123456's token.
		if got := srv.confirm.consume(tok, srv.quota.hashUser("Z999999"), paramsHash(rawFields(t, withTok(tok, nil))), srv.now()); got == nil {
			t.Fatal("token accepted for another user")
		}
		if got := srv.confirm.consume(tok, srv.quota.hashUser("Z123456"), paramsHash(rawFields(t, twoStepArgs)), srv.now()); got != nil {
			t.Fatalf("token rejected for the right user: %v", got)
		}
		if posts() != 0 {
			t.Fatal("order reached Dolly")
		}
	})

	t.Run("expired rejected", func(t *testing.T) {
		srv, posts, advance := twoStepServer(t)
		tok := previewToken(t, call(srv, twoStepArgs))
		advance(confirmTTL + time.Second)
		wantReject(t, call(srv, withTok(tok, nil)), posts)
	})

	t.Run("reused rejected", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		tok := previewToken(t, call(srv, twoStepArgs))
		call(srv, withTok(tok, nil))
		out := call(srv, withTok(tok, nil))
		if txt, isErr := extractToolText(t, out); !isErr || !strings.Contains(txt, "confirmation token") || posts() != 1 {
			t.Fatalf("reuse not rejected: isErr=%v %q posts=%d", isErr, txt, posts())
		}
	})

	t.Run("reused after 20 uncharged confirms rejected", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		srv.quota.reserve(srv.quota.hashUser("Z123456"), dailyPersonQuota-4, srv.now())

		firstTok := previewToken(t, call(srv, twoStepArgs))
		if txt, isErr := extractToolText(t, call(srv, withTok(firstTok, nil))); isErr {
			t.Fatalf("first confirm failed: %s", txt)
		}

		// 20 confirms for antall=5 fail on the 2-person remaining quota without charging it.
		overArgs := map[string]any{"gruppeId": 42, "antall": 5, "environments": []string{"q1"}, "kjoenn": "MANN"}
		for i := range maxSeenPerUser {
			tok := previewToken(t, call(srv, overArgs))
			txt, isErr := extractToolText(t, call(srv, withTok(tok, map[string]any{"antall": 5})))
			if !isErr {
				t.Fatalf("uncharged confirm %d succeeded: %s", i+1, txt)
			}
			if i == maxSeenPerUser-1 && !strings.Contains(txt, "confirmation token") {
				t.Errorf("20th uncharged confirm should fail closed on full seen set, got %q", txt)
			}
		}

		out := call(srv, withTok(firstTok, nil))
		if txt, isErr := extractToolText(t, out); !isErr || !strings.Contains(txt, "confirmation token") || posts() != 1 {
			t.Fatalf("oldest token replayed after 20 uncharged confirms: isErr=%v %q posts=%d", isErr, txt, posts())
		}
	})

	t.Run("tampered and malformed tokens rejected", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		tok := previewToken(t, call(srv, twoStepArgs))
		i := strings.LastIndex(tok, ".")
		exp, sig := tok[:i], tok[i+1:]
		for _, bad := range []string{"", "x", exp + ".", "9999999999.00." + sig, exp + "." + strings.Repeat("0", 64)} {
			if bad == tok {
				continue
			}
			wantReject(t, call(srv, withTok(bad, nil)), posts)
		}
	})
}

func rawFields(t *testing.T, args map[string]any) map[string]json.RawMessage {
	t.Helper()
	b, _ := json.Marshal(args)
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestConfirmHardening(t *testing.T) {
	call := func(srv *server, args map[string]any) map[string]any {
		_, out := callToolModern(t, srv.routes(), "in", "create_order", args)
		return out
	}

	t.Run("environment order does not matter", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		a := map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1", "q2"}}
		tok := previewToken(t, call(srv, a))
		b := map[string]any{"environments": []string{"q2", "q1"}, "antall": 1, "gruppeId": 42, "confirmationToken": tok}
		if txt, isErr := extractToolText(t, call(srv, b)); isErr || posts() != 1 {
			t.Fatalf("reordered params rejected: %q posts=%d", txt, posts())
		}
	})

	t.Run("preview rejects unowned group without token", func(t *testing.T) {
		srv, posts, _ := twoStepServer(t)
		out := call(srv, map[string]any{"gruppeId": 43, "antall": 1, "environments": []string{"q1"}})
		if previewToken(t, out) != "" || posts() != 0 {
			t.Fatalf("token issued for unowned group: %+v", out)
		}
	})

	t.Run("one user cannot block another", func(t *testing.T) {
		c := newOrderConfirm(testKeys().confirm)
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		for range maxSeenPerUser {
			tok, _ := c.issue("attacker", "p", now)
			if err := c.consume(tok, "attacker", "p", now); err != nil {
				t.Fatalf("attacker consume: %v", err)
			}
		}
		extraTok, _ := c.issue("attacker", "p", now)
		if err := c.consume(extraTok, "attacker", "p", now); err == nil {
			t.Fatal("attacker token accepted when set is full")
		}
		if n := len(c.seen["attacker"]); n != maxSeenPerUser {
			t.Fatalf("attacker set = %d, want %d", n, maxSeenPerUser)
		}
		tok, _ := c.issue("victim", "p", now)
		if err := c.consume(tok, "victim", "p", now); err != nil {
			t.Fatalf("victim blocked: %v", err)
		}
		if err := c.consume(tok, "victim", "p", now); err == nil {
			t.Fatal("victim token reusable")
		}
	})
}
