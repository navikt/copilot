package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestFetchGitHubUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"starefossen","name":"Hans Kristian"}`))
	}))
	defer server.Close()

	orig := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = orig }()

	user, err := fetchGitHubUser(t.Context(), "good-token")
	if err != nil {
		t.Fatalf("fetchGitHubUser: %v", err)
	}
	if user.Login != "starefossen" || user.Name != "Hans Kristian" {
		t.Fatalf("unexpected user: %+v", user)
	}

	if _, err := fetchGitHubUser(t.Context(), "bad-token"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestFetchGitHubUserServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	orig := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = orig }()

	if _, err := fetchGitHubUser(t.Context(), "any"); err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestCmdAuthDispatch(t *testing.T) {
	if err := cmdAuth(nil, false); err == nil {
		t.Fatal("expected error when no subcommand given")
	}
	if err := cmdAuth([]string{"bogus"}, false); err == nil {
		t.Fatal("expected error for unknown subcommand")
	}

	keyring.MockInit()
	if err := cmdAuth([]string{"logout"}, false); err != nil {
		t.Fatalf("auth logout via dispatcher: %v", err)
	}
}

func TestCmdAuthLogout(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusUnauthorized, http.StatusBadGateway, http.StatusNotFound} {
		keyring.MockInit()
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/revoke" {
				got = r.Header.Get("Authorization")
			}
			w.WriteHeader(status)
		}))
		t.Setenv("NAV_PILOT_COPILOT_CLI_URL", srv.URL)
		if err := saveToken(storedToken{AccessToken: "x"}); err != nil {
			t.Fatalf("saveToken: %v", err)
		}
		if err := cmdAuthLogout(); err != nil {
			t.Fatalf("%d: cmdAuthLogout: %v", status, err)
		}
		srv.Close()
		if got != "Bearer x" {
			t.Fatalf("%d: revoke sent Authorization %q", status, got)
		}
		if _, err := loadToken(); err == nil {
			t.Fatalf("%d: token not removed", status)
		}
	}
}

// Unreachable copilot-cli: the token is still removed.
func TestCmdAuthLogoutOffline(t *testing.T) {
	keyring.MockInit()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	t.Setenv("NAV_PILOT_COPILOT_CLI_URL", srv.URL)
	if err := saveToken(storedToken{AccessToken: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdAuthLogout(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadToken(); err == nil {
		t.Fatal("token not removed")
	}
}

func TestCmdAuthStatusNotLoggedIn(t *testing.T) {
	keyring.MockInit()
	if err := cmdAuthStatus(false); err != nil {
		t.Fatalf("cmdAuthStatus (text): %v", err)
	}
	if err := cmdAuthStatus(true); err != nil {
		t.Fatalf("cmdAuthStatus (json): %v", err)
	}
}

func TestCmdAuthStatusLoggedIn(t *testing.T) {
	keyring.MockInit()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"login":"starefossen","name":"Hans Kristian"}`))
		default:
			// The App has no permissions: nothing but /user may be asked.
			t.Errorf("unexpected GitHub call %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()

	orig := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = orig }()

	if err := saveToken(storedToken{AccessToken: "tok", Login: "starefossen"}); err != nil {
		t.Fatalf("saveToken: %v", err)
	}

	if err := cmdAuthStatus(false); err != nil {
		t.Fatalf("cmdAuthStatus (text): %v", err)
	}
	if err := cmdAuthStatus(true); err != nil {
		t.Fatalf("cmdAuthStatus (json): %v", err)
	}
}

func TestCmdAuthStatusInvalidToken(t *testing.T) {
	keyring.MockInit()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	orig := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = orig }()

	if err := saveToken(storedToken{AccessToken: "stale"}); err != nil {
		t.Fatalf("saveToken: %v", err)
	}

	if err := cmdAuthStatus(true); err != nil {
		t.Fatalf("cmdAuthStatus should not error, just report invalid: %v", err)
	}
}

func TestPrintAuthStatusJSON(t *testing.T) {
	if err := printAuthStatusJSON(authStatus{LoggedIn: true, Login: "x"}); err != nil {
		t.Fatalf("printAuthStatusJSON: %v", err)
	}
}

func TestAuthStatusJSONRoundtrip(t *testing.T) {
	s := authStatus{LoggedIn: true, Login: "starefossen"}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out authStatus
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Login != s.Login {
		t.Fatalf("roundtrip mismatch: %+v", out)
	}
}
