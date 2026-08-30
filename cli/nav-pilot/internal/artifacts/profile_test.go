package artifacts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const validProfile = `{
  "profileVersion": "1",
  "defaultModels": {"opencode": "github-copilot/claude-sonnet-4.6"}
}`

// setProfileFetch installs a FetchProfile for the duration of one test.
func setProfileFetch(t *testing.T, fn func() ([]byte, error)) {
	t.Helper()
	orig := FetchProfile
	FetchProfile = fn
	t.Cleanup(func() { FetchProfile = orig })
}

// okFetch is a release lookup that always succeeds, so AssessStaleness reaches
// the profile refresh.
func okFetch() (string, string, error) {
	return "2026.04.13-170138-abc1234", "nav-pilot/2026.04.13-170138-abc1234", nil
}

func TestParseProfile(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr bool
		want    string // expected opencode default, checked when wantErr is false
	}{
		{
			name: "valid",
			doc:  validProfile,
			want: "github-copilot/claude-sonnet-4.6",
		},
		{
			name: "unknown fields are tolerated so a newer profile stays readable",
			doc:  `{"profileVersion":"1","note":"hi","killSwitch":{"from":"2027"},"defaultModels":{"opencode":"github-copilot/auto"}}`,
			want: "github-copilot/auto",
		},
		{
			name:    "not json",
			doc:     `{"profileVersion": `,
			wantErr: true,
		},
		{
			name:    "html error page",
			doc:     "<!doctype html><title>404</title>",
			wantErr: true,
		},
		{
			name:    "unknown contract version",
			doc:     `{"profileVersion":"2","defaultModels":{"opencode":"github-copilot/auto"}}`,
			wantErr: true,
		},
		{
			name:    "missing defaultModels",
			doc:     `{"profileVersion":"1"}`,
			wantErr: true,
		},
		{
			name:    "empty defaultModels",
			doc:     `{"profileVersion":"1","defaultModels":{}}`,
			wantErr: true,
		},
		{
			name:    "model id with a shell metacharacter",
			doc:     `{"profileVersion":"1","defaultModels":{"opencode":"github-copilot/auto; rm -rf /"}}`,
			wantErr: true,
		},
		{
			name:    "model is not a string",
			doc:     `{"profileVersion":"1","defaultModels":{"opencode":42}}`,
			wantErr: true,
		},
		{
			name:    "client id is not an identifier",
			doc:     `{"profileVersion":"1","defaultModels":{"OpenCode!":"github-copilot/auto"}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseProfile([]byte(tt.doc))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseProfile(%s) = %+v, want an error", tt.doc, p)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseProfile(%s) failed: %v", tt.doc, err)
			}
			if got := p.DefaultModels["opencode"]; got != tt.want {
				t.Errorf("defaultModels.opencode = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPublishedProfileIsValid keeps the file people actually edit honest. It is
// the only guard between a typo in nav-pilot-profile.json and every binary in
// Nav silently ignoring the profile.
func TestPublishedProfileIsValid(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "nav-pilot-profile.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the published profile: %v", err)
	}
	p, err := ParseProfile(data)
	if err != nil {
		t.Fatalf("the published profile does not validate: %v", err)
	}
	if p.DefaultModels["opencode"] == "" {
		t.Error("the published profile declares no opencode default")
	}
}

// TestProfileRefresh covers what a launch sees: the profile is fetched in the
// release check's budget, and every way that can fail leaves the launch on the
// last known good value or on nothing at all.
func TestProfileRefresh(t *testing.T) {
	tests := []struct {
		name    string
		warm    string // opencode model already cached, "" for a cold cache
		fetch   func() ([]byte, error)
		release func() (string, string, error)
		want    string
	}{
		{
			name:    "profile applied",
			fetch:   func() ([]byte, error) { return []byte(validProfile), nil },
			release: okFetch,
			want:    "github-copilot/claude-sonnet-4.6",
		},
		{
			name:    "profile absent: no fetch function at all",
			fetch:   nil,
			release: okFetch,
			want:    "",
		},
		{
			name:    "profile absent: warm cache is not thrown away",
			warm:    "github-copilot/gpt-5.3",
			fetch:   nil,
			release: okFetch,
			want:    "github-copilot/gpt-5.3",
		},
		{
			name:    "profile malformed, cold cache",
			fetch:   func() ([]byte, error) { return []byte("not json"), nil },
			release: okFetch,
			want:    "",
		},
		{
			name:    "profile malformed, warm cache keeps the last known good",
			warm:    "github-copilot/gpt-5.3",
			fetch:   func() ([]byte, error) { return []byte("not json"), nil },
			release: okFetch,
			want:    "github-copilot/gpt-5.3",
		},
		{
			name:    "profile schema-invalid, warm cache keeps the last known good",
			warm:    "github-copilot/gpt-5.3",
			fetch:   func() ([]byte, error) { return []byte(`{"profileVersion":"9"}`), nil },
			release: okFetch,
			want:    "github-copilot/gpt-5.3",
		},
		{
			name:    "profile fetch fails, warm cache keeps the last known good",
			warm:    "github-copilot/gpt-5.3",
			fetch:   func() ([]byte, error) { return nil, errors.New("dial tcp: no route to host") },
			release: okFetch,
			want:    "github-copilot/gpt-5.3",
		},
		{
			name:    "profile fetch fails, cold cache stays cold",
			fetch:   func() ([]byte, error) { return nil, errors.New("dial tcp: no route to host") },
			release: okFetch,
			want:    "",
		},
		{
			name: "offline: release lookup fails, warm cache survives",
			warm: "github-copilot/gpt-5.3",
			fetch: func() ([]byte, error) {
				t.Fatal("the profile must not be fetched after a failed release lookup")
				return nil, nil
			},
			release: func() (string, string, error) { return "", "", errors.New("no network") },
			want:    "github-copilot/gpt-5.3",
		},
		{
			name: "offline: release lookup fails, cold cache stays cold",
			fetch: func() ([]byte, error) {
				t.Fatal("the profile must not be fetched after a failed release lookup")
				return nil, nil
			},
			release: func() (string, string, error) { return "", "", errors.New("no network") },
			want:    "",
		},
		{
			name:    "a profile that drops a client returns it to the compiled-in default",
			warm:    "github-copilot/gpt-5.3",
			fetch:   func() ([]byte, error) { return []byte(`{"profileVersion":"1","defaultModels":{"pi":"nav/pi"}}`), nil },
			release: okFetch,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestCache(t)
			setProfileFetch(t, tt.fetch)
			if tt.warm != "" {
				WriteCache(&StalenessCache{DefaultModels: map[string]string{"opencode": tt.warm}})
			}

			AssessStaleness("2026.01.01-080000-old1234", tt.release)

			if got := ProfileDefaultModel("opencode"); got != tt.want {
				t.Errorf("ProfileDefaultModel(opencode) = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestProfileNotRefetchedWithinTTL pins the "same budget" claim: the profile
// costs one request per release check, not one per launch.
func TestProfileNotRefetchedWithinTTL(t *testing.T) {
	setupTestCache(t)
	fetches := 0
	setProfileFetch(t, func() ([]byte, error) {
		fetches++
		return []byte(validProfile), nil
	})

	for range 3 {
		AssessStaleness("2026.01.01-080000-old1234", okFetch)
	}

	if fetches != 1 {
		t.Errorf("profile fetched %d times across 3 launches, want 1", fetches)
	}
	if got := ProfileDefaultModel("opencode"); got != "github-copilot/claude-sonnet-4.6" {
		t.Errorf("ProfileDefaultModel(opencode) = %q, want the fetched value", got)
	}
}

// TestProfileSurvivesFailureStamp guards the interaction the failure path is
// easiest to get wrong: a later offline launch rewrites the cache, and must not
// drop the profile while it stamps the failure.
func TestProfileSurvivesFailureStamp(t *testing.T) {
	setupTestCache(t)
	WriteCache(&StalenessCache{
		LastChecked:   time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
		LatestVersion: "2026.05.01-120000-new1234",
		DefaultModels: map[string]string{"opencode": "github-copilot/gpt-5.3"},
	})
	setProfileFetch(t, func() ([]byte, error) { return []byte(validProfile), nil })

	AssessStaleness("2026.01.01-080000-old1234", func() (string, string, error) {
		return "", "", errors.New("no network")
	})

	c := ReadCache()
	if c.LastFailed == "" {
		t.Error("a failed lookup did not stamp last_failed")
	}
	if got := c.DefaultModels["opencode"]; got != "github-copilot/gpt-5.3" {
		t.Errorf("default_models after a failed lookup = %q, want the previous value", got)
	}
}

func TestProfileDefaultModelWithoutCache(t *testing.T) {
	setupTestCache(t)
	if got := ProfileDefaultModel("opencode"); got != "" {
		t.Errorf("ProfileDefaultModel with no cache file = %q, want \"\"", got)
	}
}
