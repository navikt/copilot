package artifacts

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestParseVersionTimestamp(t *testing.T) {
	tests := []struct {
		ts      string
		wantOK  bool
		wantUTC time.Time
	}{
		{"2026.04.13-170138", true, time.Date(2026, 4, 13, 17, 1, 38, 0, time.UTC)},
		{"2026.01.01-000000", true, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"2026.12.31-235959", true, time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
		// invalid: no dash
		{"20260413170138", false, time.Time{}},
		// invalid: wrong date segments
		{"2026.04-170138", false, time.Time{}},
		// invalid: time too short
		{"2026.04.13-1701", false, time.Time{}},
		// invalid: non-numeric year
		{"YYYY.04.13-170138", false, time.Time{}},
		// invalid: non-numeric month
		{"2026.MM.13-170138", false, time.Time{}},
		// invalid: non-numeric day
		{"2026.04.DD-170138", false, time.Time{}},
		// invalid: non-numeric hour
		{"2026.04.13-HH0138", false, time.Time{}},
		// invalid: non-numeric minute
		{"2026.04.13-17MM38", false, time.Time{}},
		// invalid: non-numeric second
		{"2026.04.13-1701SS", false, time.Time{}},
	}
	for _, tt := range tests {
		got, ok := ParseVersionTimestamp(tt.ts)
		if ok != tt.wantOK {
			t.Errorf("ParseVersionTimestamp(%q) ok = %v, want %v", tt.ts, ok, tt.wantOK)
			continue
		}
		if ok && !got.Equal(tt.wantUTC) {
			t.Errorf("ParseVersionTimestamp(%q) = %v, want %v", tt.ts, got, tt.wantUTC)
		}
	}
}

func TestVersionSkewDays(t *testing.T) {
	tests := []struct {
		latest, installed string
		wantDays          int64
		wantOK            bool
	}{
		// same day
		{"2026.04.13-170138", "2026.04.13-170138", 0, true},
		// 1 day newer
		{"2026.04.14-170138", "2026.04.13-170138", 1, true},
		// 7 days newer
		{"2026.04.20-000000", "2026.04.13-000000", 7, true},
		// installed newer than latest -> 0 days (negative clamped)
		{"2026.04.13-170138", "2026.04.20-170138", 0, true},
		// invalid latest
		{"notaversion", "2026.04.13-170138", 0, false},
		// invalid installed
		{"2026.04.13-170138", "notaversion", 0, false},
	}
	for _, tt := range tests {
		days, ok := VersionSkewDays(tt.latest, tt.installed)
		if ok != tt.wantOK {
			t.Errorf("VersionSkewDays(%q, %q) ok = %v, want %v", tt.latest, tt.installed, ok, tt.wantOK)
			continue
		}
		if ok && days != tt.wantDays {
			t.Errorf("VersionSkewDays(%q, %q) = %d, want %d", tt.latest, tt.installed, days, tt.wantDays)
		}
	}
}

func TestAssessFromLatest(t *testing.T) {
	tests := []struct {
		name              string
		installed, latest string
		fallback          string
		wantResult        string
		wantUpToDate      bool
	}{
		{
			name:         "empty latest returns lookup_failed",
			installed:    "2026.04.13-170138-abc",
			latest:       "",
			wantResult:   "lookup_failed",
			wantUpToDate: false,
		},
		{
			name:         "empty latest with fallback returns fallback",
			installed:    "2026.04.13-170138-abc",
			latest:       "",
			fallback:     "cooldown",
			wantResult:   "cooldown",
			wantUpToDate: false,
		},
		{
			name:         "up_to_date when latest == installed",
			installed:    "2026.04.13-170138-abc",
			latest:       "2026.04.13-170138-abc",
			wantResult:   "up_to_date",
			wantUpToDate: true,
		},
		{
			name:         "stale when latest is 30 days newer",
			installed:    "2026.03.01-000000-old",
			latest:       "2026.04.13-000000-new",
			wantResult:   "stale",
			wantUpToDate: false,
		},
		{
			name:         "fallback overrides result",
			installed:    "2026.04.13-170138-abc",
			latest:       "2026.04.13-170138-abc",
			fallback:     "cooldown",
			wantResult:   "cooldown",
			wantUpToDate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := AssessFromLatest(tt.installed, tt.latest, tt.fallback)
			if a.Result != tt.wantResult {
				t.Errorf("Result = %q, want %q", a.Result, tt.wantResult)
			}
			if a.UpToDate != tt.wantUpToDate {
				t.Errorf("UpToDate = %v, want %v", a.UpToDate, tt.wantUpToDate)
			}
		})
	}
}

func TestWriteStateAndReadBack(t *testing.T) {
	tmp := t.TempDir()
	scope := domain.ScopeRepo(tmp)
	state := &domain.StateFile{
		Collection: "all",
		Version:    "2026.04.13-170138-abc",
		Scope:      "repo",
		Files: []domain.InstalledFile{
			{Path: ".github/agents/nav-pilot.agent.md", Hash: "abc123"},
		},
	}
	if err := WriteScopedState(scope, state); err != nil {
		t.Fatalf("WriteScopedState: %v", err)
	}
	got, err := ReadScopedState(scope)
	if err != nil {
		t.Fatalf("ReadScopedState: %v", err)
	}
	if got.Version != "2026.04.13-170138-abc" {
		t.Errorf("Version = %q, want 2026.04.13-170138-abc", got.Version)
	}
	if len(got.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(got.Files))
	}
	if got.Files[0].Path != ".github/agents/nav-pilot.agent.md" {
		t.Errorf("Files[0].Path = %q", got.Files[0].Path)
	}
}

// --- outputJSON (in export.go) ---

func TestOutputJSON_Artifacts(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	type payload struct {
		Key string `json:"key"`
	}
	if err := outputJSON(payload{Key: "value"}); err != nil {
		w.Close()
		os.Stdout = origStdout
		t.Fatalf("outputJSON = %v", err)
	}
	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 512)
	n, _ := r.Read(buf)
	r.Close()
	if !strings.Contains(string(buf[:n]), "value") {
		t.Errorf("outputJSON output = %q, want to contain 'value'", buf[:n])
	}
}
