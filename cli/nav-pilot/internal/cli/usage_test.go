package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestFormatNorwegianNumber(t *testing.T) {
	cases := map[int]string{
		0:       "0",
		42:      "42",
		999:     "999",
		1000:    "1 000",
		151354:  "151 354",
		-1234:   "-1 234",
		1000000: "1 000 000",
	}
	for in, want := range cases {
		if got := formatNorwegianNumber(in); got != want {
			t.Errorf("formatNorwegianNumber(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestProgressBar(t *testing.T) {
	if got := progressBar(0, 10); got != strings.Repeat("░", 10) {
		t.Errorf("progressBar(0,10) = %q", got)
	}
	if got := progressBar(100, 10); got != strings.Repeat("█", 10) {
		t.Errorf("progressBar(100,10) = %q", got)
	}
	if got := progressBar(50, 10); got != strings.Repeat("█", 5)+strings.Repeat("░", 5) {
		t.Errorf("progressBar(50,10) = %q", got)
	}
	// Out-of-range values should clamp rather than panic.
	if got := progressBar(150, 10); got != strings.Repeat("█", 10) {
		t.Errorf("progressBar(150,10) should clamp: got %q", got)
	}
	if got := progressBar(-10, 10); got != strings.Repeat("░", 10) {
		t.Errorf("progressBar(-10,10) should clamp: got %q", got)
	}
}

func TestCapitalize(t *testing.T) {
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize(\"\") = %q", got)
	}
	if got := capitalize("active"); got != "Active" {
		t.Errorf("capitalize(active) = %q", got)
	}
}

func TestFormatUsageTerminalDoesNotPanic(t *testing.T) {
	u := &usageResponse{Period: "November 2025"}
	u.Credits.Used = 4500
	u.Credits.Limit = 10000
	u.Credits.Percentage = 45
	u.Interactions.Total = 320
	u.Interactions.Accepted = 210
	u.Interactions.AcceptanceRate = 65.6
	u.ActiveDays = 18
	u.Subscription.Status = "active"

	out := formatUsageTerminal(u)
	if !strings.Contains(out, "4 500") || !strings.Contains(out, "10 000") {
		t.Errorf("expected formatted credit numbers in output: %s", out)
	}
	if !strings.Contains(out, "45%") {
		t.Errorf("expected percentage in output: %s", out)
	}
}

func TestFormatUsageTmux(t *testing.T) {
	u := &usageResponse{}
	u.Credits.Percentage = 72
	if got := formatUsageTmux(u); got != "Copilot 72%" {
		t.Errorf("formatUsageTmux = %q", got)
	}
}

func TestCopilotCLIURL(t *testing.T) {
	if got := copilotCLIURL(); got != defaultCopilotCLIURL {
		t.Errorf("copilotCLIURL() default = %q, want %q", got, defaultCopilotCLIURL)
	}

	t.Setenv("NAV_PILOT_COPILOT_CLI_URL", "https://example.test")
	if got := copilotCLIURL(); got != "https://example.test" {
		t.Errorf("copilotCLIURL() override = %q", got)
	}
}

func TestFetchUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/usage" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"starefossen","period":"November 2025","credits":{"used":100,"limit":1000,"percentage":10}}`))
	}))
	defer server.Close()

	usage, err := fetchUsage(context.Background(), server.URL, "tok")
	if err != nil {
		t.Fatalf("fetchUsage: %v", err)
	}
	if usage.Username != "starefossen" || usage.Credits.Used != 100 {
		t.Fatalf("unexpected usage: %+v", usage)
	}

	if _, err := fetchUsage(context.Background(), server.URL, "wrong"); err == nil {
		t.Fatal("expected error for unauthorized token")
	}
}

func TestFetchUsageServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	if _, err := fetchUsage(context.Background(), server.URL, "tok"); err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestCmdUsageNotLoggedIn(t *testing.T) {
	keyring.MockInit()
	if err := cmdUsage(false, false); err == nil {
		t.Fatal("expected error when not logged in")
	}
}

func TestCmdUsageLoggedIn(t *testing.T) {
	keyring.MockInit()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"starefossen","period":"November 2025","credits":{"used":100,"limit":1000,"percentage":10}}`))
	}))
	defer server.Close()

	if err := saveToken(storedToken{AccessToken: "tok"}); err != nil {
		t.Fatalf("saveToken: %v", err)
	}
	t.Setenv("NAV_PILOT_COPILOT_CLI_URL", server.URL)

	if err := cmdUsage(false, false); err != nil {
		t.Fatalf("cmdUsage (text): %v", err)
	}
	if err := cmdUsage(true, false); err != nil {
		t.Fatalf("cmdUsage (json): %v", err)
	}
	if err := cmdUsage(false, true); err != nil {
		t.Fatalf("cmdUsage (tmux): %v", err)
	}
}
