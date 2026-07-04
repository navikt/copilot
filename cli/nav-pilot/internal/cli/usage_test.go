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
	cases := map[int64]string{
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
	u := &usageResponse{
		UserLogin:           "starefossen",
		TotalAcceptances:    210,
		TotalInteractions:   320,
		TotalGenerations:    350,
		TotalLinesSuggested: 4500,
		TotalLinesAccepted:  3800,
		ActiveDays:          18,
		DaysInPeriod:        30,
		CLITotalRequests:    42,
		CLISessions:         12,
		TopModels: []usageModel{
			{Model: "gpt-5", Interactions: 200},
			{Model: "claude-sonnet-5", Interactions: 120},
		},
		Teams: []string{"team-a", "team-b"},
	}

	out := formatUsageTerminal(u)
	if !strings.Contains(out, "4 500") || !strings.Contains(out, "3 800") {
		t.Errorf("expected formatted line-count numbers in output: %s", out)
	}
	if !strings.Contains(out, "starefossen") {
		t.Errorf("expected username in output: %s", out)
	}
	if !strings.Contains(out, "gpt-5") || !strings.Contains(out, "claude-sonnet-5") {
		t.Errorf("expected top models in output: %s", out)
	}
	if !strings.Contains(out, "team-a") {
		t.Errorf("expected teams in output: %s", out)
	}
}

func TestAcceptanceRate(t *testing.T) {
	u := &usageResponse{TotalAcceptances: 50, TotalGenerations: 200}
	if got := u.acceptanceRate(); got != 25 {
		t.Errorf("acceptanceRate() = %v, want 25", got)
	}

	zero := &usageResponse{}
	if got := zero.acceptanceRate(); got != 0 {
		t.Errorf("acceptanceRate() with no generations = %v, want 0", got)
	}
}

func TestFormatUsageTmux(t *testing.T) {
	u := &usageResponse{TotalAcceptances: 36, TotalGenerations: 100}
	if got := formatUsageTmux(u); got != "Copilot 36%" {
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
		_, _ = w.Write([]byte(`{"user_login":"starefossen","total_acceptances":100,"total_generations":400,"active_days":5}`))
	}))
	defer server.Close()

	usage, err := fetchUsage(context.Background(), server.URL, "tok")
	if err != nil {
		t.Fatalf("fetchUsage: %v", err)
	}
	if usage.UserLogin != "starefossen" || usage.TotalAcceptances != 100 {
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
		_, _ = w.Write([]byte(`{"user_login":"starefossen","total_acceptances":100,"total_generations":400,"active_days":5}`))
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
