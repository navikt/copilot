package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCheckOrgMembership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/members/member-user"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/members/non-member"):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	orig := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = orig }()

	member, err := checkOrgMembership(t.Context(), "tok", "navikt", "member-user")
	if err != nil || !member {
		t.Fatalf("expected member=true, got %v err=%v", member, err)
	}

	member, err = checkOrgMembership(t.Context(), "tok", "navikt", "non-member")
	if err != nil || member {
		t.Fatalf("expected member=false, got %v err=%v", member, err)
	}

	if _, err := checkOrgMembership(t.Context(), "tok", "navikt", "boom"); err == nil {
		t.Fatal("expected error for unexpected status code")
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
	keyring.MockInit()
	if err := saveToken(storedToken{AccessToken: "x"}); err != nil {
		t.Fatalf("saveToken: %v", err)
	}
	if err := cmdAuthLogout(); err != nil {
		t.Fatalf("cmdAuthLogout: %v", err)
	}
	if _, err := loadToken(); err == nil {
		t.Fatal("expected token to be removed")
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
			w.WriteHeader(http.StatusNoContent) // org membership: member
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
	member := true
	s := authStatus{LoggedIn: true, Login: "starefossen", OrgMember: &member}
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
	if out.OrgMember == nil || !*out.OrgMember {
		t.Fatalf("expected org_member true after roundtrip, got %+v", out.OrgMember)
	}
}

// TestAuthStatusJSONOrgMemberFalseVsUnknown ensures an explicit non-member
// (false) is serialized while an unknown membership (nil pointer, e.g. the
// org check failed) is omitted from the JSON output.
func TestAuthStatusJSONOrgMemberFalseVsUnknown(t *testing.T) {
	notMember := false
	data, err := json.Marshal(authStatus{LoggedIn: true, OrgMember: &notMember})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"org_member":false`) {
		t.Errorf("expected explicit org_member false in %s", data)
	}

	data, err = json.Marshal(authStatus{LoggedIn: true, OrgCheckError: "boom"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "org_member") {
		t.Errorf("expected org_member omitted when unknown, got %s", data)
	}
}
