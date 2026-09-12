package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubClientResolveUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"hans","name":"Hans","id":1}`))
	}))
	defer srv.Close()

	gh := newGitHubClient()
	gh.baseURL = srv.URL

	user, err := gh.resolveUser(t.Context(), "good-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Login != "hans" {
		t.Fatalf("expected login hans, got %s", user.Login)
	}

	if _, err := gh.resolveUser(t.Context(), "bad-token"); err == nil {
		t.Fatal("expected error for bad token")
	}
}

func TestGitHubClientIsOrgMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/navikt/members/hans":
			w.WriteHeader(http.StatusNoContent)
		case "/orgs/navikt/members/outsider":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	gh := newGitHubClient()
	gh.baseURL = srv.URL

	member, err := gh.isOrgMember(t.Context(), "token", "navikt", "hans")
	if err != nil || !member {
		t.Fatalf("expected hans to be a member, got member=%v err=%v", member, err)
	}

	member, err = gh.isOrgMember(t.Context(), "token", "navikt", "outsider")
	if err != nil || member {
		t.Fatalf("expected outsider to not be a member, got member=%v err=%v", member, err)
	}
}
