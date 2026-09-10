package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

var (
	shaA = strings.Repeat("a", 40)
	shaB = strings.Repeat("b", 40)
	shaC = strings.Repeat("c", 40)
)

type fakeRelease struct {
	tag                      string
	draft, prerelease, mutab bool
	asset                    string // metadata body; empty = no metadata asset
}

// fakeGitHub serves the four endpoints discovery reads. compare maps
// "base...head" to a status; a missing key is "ahead" when head is the default
// branch (the SHA is on main) and 404 otherwise.
type fakeGitHub struct {
	releases []fakeRelease
	compare  map[string]string
	status   int // non-zero: the releases list answers with this
}

func (f *fakeGitHub) serve(t *testing.T) {
	t.Helper()
	const repo = "/repos/navikt/grillmester"
	mux := http.NewServeMux()
	mux.HandleFunc(repo+"/releases", func(w http.ResponseWriter, r *http.Request) {
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		if r.URL.Query().Get("per_page") != "100" {
			t.Errorf("releases requested with per_page=%q, want 100", r.URL.Query().Get("per_page"))
		}
		list := []map[string]any{}
		for i, rel := range f.releases {
			assets := []map[string]string{{"name": "grillmester.tar.gz", "url": "http://" + r.Host + "/assets/none"}}
			if rel.asset != "" {
				assets = append(assets, map[string]string{"name": pakkeReleaseAsset, "url": fmt.Sprintf("http://%s/assets/%d", r.Host, i)})
			}
			list = append(list, map[string]any{
				"tag_name": rel.tag, "draft": rel.draft, "prerelease": rel.prerelease,
				"immutable": !rel.mutab, "assets": assets,
			})
		}
		_ = json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Errorf("asset requested with Accept %q", r.Header.Get("Accept"))
		}
		i, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/assets/"))
		if err != nil {
			t.Errorf("discovery downloaded an asset that is not the metadata: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, f.releases[i].asset)
	})
	mux.HandleFunc(repo, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"default_branch":"main"}`)
	})
	mux.HandleFunc(repo+"/compare/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, repo+"/compare/")
		st, ok := f.compare[key]
		if !ok && strings.HasSuffix(key, "...main") {
			st, ok = "ahead", true
		}
		if !ok || st == "404" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprintf(w, `{"status":%q}`, st)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	orig := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = orig })
	t.Setenv("GITHUB_TOKEN", "")
}

func meta(name, version, sha string) string {
	return fmt.Sprintf(`{"schemaVersion":1,"name":%q,"version":%q,"sourceSha":%q}`, name, version, sha)
}

func TestDiscoverPakkeRelease(t *testing.T) {
	good := func(tag, version, sha string) fakeRelease {
		return fakeRelease{tag: tag, asset: meta("grillmester", version, sha)}
	}
	cases := []struct {
		name      string
		gh        fakeGitHub
		installed string
		want      releaseOutcome
		wantRel   pakkeRelease
		wantErr   string
	}{
		{name: "no releases", want: releaseNoMetadata},
		{name: "releases without metadata", gh: fakeGitHub{releases: []fakeRelease{{tag: "v0.4.0"}}}, want: releaseNoMetadata},
		{name: "draft ignored", gh: fakeGitHub{releases: []fakeRelease{{tag: "v1.0.0", draft: true, asset: meta("grillmester", "1.0.0", shaA)}}}, want: releaseNoMetadata},
		{name: "prerelease ignored", gh: fakeGitHub{releases: []fakeRelease{{tag: "v1.0.0", prerelease: true, asset: meta("grillmester", "1.0.0", shaA)}}}, want: releaseNoMetadata},
		{name: "mutable release ignored", gh: fakeGitHub{releases: []fakeRelease{{tag: "v1.0.0", mutab: true, asset: meta("grillmester", "1.0.0", shaA)}}}, want: releaseNoMetadata},
		{name: "another package in the same repo", gh: fakeGitHub{releases: []fakeRelease{{tag: "other/v1.0.0", asset: meta("other", "1.0.0", shaA)}}}, want: releaseNoMetadata},
		{
			name: "highest version wins, not newest release",
			gh: fakeGitHub{releases: []fakeRelease{
				good("v0.9.0", "0.9.0", shaA), good("grillmester/v0.10.0", "0.10.0", shaB), good("v0.4.0", "0.4.0", shaC),
			}},
			want: releaseCandidate, wantRel: pakkeRelease{Version: "0.10.0", SHA: shaB, Tag: "grillmester/v0.10.0"},
		},
		{
			name:    "unsupported schemaVersion",
			gh:      fakeGitHub{releases: []fakeRelease{{tag: "v1.0.0", asset: `{"schemaVersion":2,"name":"grillmester"}`}}},
			wantErr: "unsupported schemaVersion 2",
		},
		{
			name:    "unknown field",
			gh:      fakeGitHub{releases: []fakeRelease{{tag: "v1.0.0", asset: `{"schemaVersion":1,"name":"grillmester","version":"1.0.0","sourceSha":"` + shaA + `","url":"https://evil"}`}}},
			wantErr: "unknown field",
		},
		{
			name:    "trailing data",
			gh:      fakeGitHub{releases: []fakeRelease{{tag: "v1.0.0", asset: meta("grillmester", "1.0.0", shaA) + `{}`}}},
			wantErr: "not valid JSON",
		},
		{
			name:    "oversized asset",
			gh:      fakeGitHub{releases: []fakeRelease{{tag: "v1.0.0", asset: meta("grillmester", "1.0.0", shaA) + strings.Repeat(" ", pakkeReleaseAssetMax)}}},
			wantErr: "larger than",
		},
		{name: "prerelease version", gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0-rc.1", "1.0.0-rc.1", shaA)}}, wantErr: "not MAJOR.MINOR.PATCH"},
		{name: "v in version", gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0", "v1.0.0", shaA)}}, wantErr: "not MAJOR.MINOR.PATCH"},
		{name: "leading zero", gh: fakeGitHub{releases: []fakeRelease{good("v01.0.0", "01.0.0", shaA)}}, wantErr: "not MAJOR.MINOR.PATCH"},
		{name: "tag does not bind", gh: fakeGitHub{releases: []fakeRelease{good("v1.0.1", "1.0.0", shaA)}}, wantErr: "tag does not bind"},
		{name: "short sha", gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", "abc1234")}}, wantErr: "not a full lowercase commit SHA"},
		{name: "uppercase sha", gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", strings.Repeat("A", 40))}}, wantErr: "not a full lowercase commit SHA"},
		{
			name: "a broken newer release does not block a valid older one",
			gh:   fakeGitHub{releases: []fakeRelease{good("v2.0.0", "2.0.0", "nope"), good("v1.0.0", "1.0.0", shaA)}},
			want: releaseCandidate, wantRel: pakkeRelease{Version: "1.0.0", SHA: shaA, Tag: "v1.0.0"},
		},
		{
			name:    "same version, different sha",
			gh:      fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA), good("grillmester/1.0.0", "1.0.0", shaB)}},
			wantErr: "published twice",
		},
		{
			name:    "sha not on the default branch",
			gh:      fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}, compare: map[string]string{shaA + "...main": "diverged"}},
			wantErr: "not on navikt/grillmester's default branch",
		},
		{
			name:    "sha unknown to the repo (fork-only commit, or no common history)",
			gh:      fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}, compare: map[string]string{shaA + "...main": "404"}},
			wantErr: "checking that",
		},
		{
			name:    "sha ahead of the default branch",
			gh:      fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}, compare: map[string]string{shaA + "...main": "behind"}},
			wantErr: "not on navikt/grillmester's default branch",
		},
		{name: "releases list fails", gh: fakeGitHub{status: http.StatusForbidden}, wantErr: "returned 403"},
		{name: "installed is the release", gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}}, installed: shaA, want: releaseUpToDate, wantRel: pakkeRelease{Version: "1.0.0", SHA: shaA, Tag: "v1.0.0"}},
		{
			name: "installed is older", installed: shaC, want: releaseCandidate, wantRel: pakkeRelease{Version: "1.0.0", SHA: shaA, Tag: "v1.0.0"},
			gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}, compare: map[string]string{shaC + "..." + shaA: "ahead"}},
		},
		{
			name: "installed is newer", installed: shaC, want: releaseNotOffered, wantRel: pakkeRelease{Version: "1.0.0", SHA: shaA, Tag: "v1.0.0"},
			gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}, compare: map[string]string{shaC + "..." + shaA: "behind"}},
		},
		{
			name: "installed diverged", installed: shaC, want: releaseNotOffered, wantRel: pakkeRelease{Version: "1.0.0", SHA: shaA, Tag: "v1.0.0"},
			gh: fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}, compare: map[string]string{shaC + "..." + shaA: "diverged"}},
		},
		{
			name: "installed unknown to the repo", installed: shaC,
			gh:      fakeGitHub{releases: []fakeRelease{good("v1.0.0", "1.0.0", shaA)}},
			wantErr: "comparing installed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gh := tc.gh
			gh.serve(t)
			got, rel, err := discoverPakkeReleaseHTTP(context.Background(), "navikt/grillmester", "grillmester", tc.installed)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q (outcome %d, %+v)", err, tc.wantErr, got, rel)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != tc.want || rel != tc.wantRel {
				t.Errorf("= (%d, %+v), want (%d, %+v)", got, rel, tc.want, tc.wantRel)
			}
		})
	}
}
