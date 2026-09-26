package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
)

// The versions fake-gh deals in: the one nav-pilot claims to be, and the
// release it finds. Twenty-three days apart, so a terminal gets the prompt.
const (
	fakeGHCurrent = "2026.09.01-120000-aaaaaaa"
	fakeGHLatest  = "2026.09.24-120000-bbbbbbb"
)

// fakeGHBinary is the "release" fake-gh serves: a script, so a journey can
// tell an upgraded nav-pilot from the old one, and a re-exec prints one JSON
// document on stdout.
const fakeGHBinary = "#!/bin/sh\necho '{\"upgraded\": \"" + fakeGHLatest + "\"}'\n"

// fake-gh [ok|badsum] serves the GitHub releases API and the release
// downloads nav-pilot upgrades from, on 127.0.0.1, and copies nav-pilot into
// $WORK/bin (first on PATH) so an upgrade replaces that copy and never the
// binary every other journey runs. nav-pilot claims to be fakeGHCurrent and
// finds fakeGHLatest. badsum serves a SHA256SUMS that matches nothing.
//
// The binary reads NAV_PILOT_E2E_GITHUB and NAV_PILOT_E2E_VERSION only because
// the suite builds it with the e2e seams on (see binary()); a release build
// ignores both.
func cmdFakeGH(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) > 1 {
		ts.Fatalf("usage: fake-gh [ok|badsum]")
	}
	mode := "ok"
	if len(args) == 1 {
		mode = args[0]
	}
	if mode != "ok" && mode != "badsum" {
		ts.Fatalf("fake-gh: unknown mode %q", mode)
	}
	asset := fmt.Sprintf("nav-pilot-%s-%s", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256([]byte(fakeGHBinary))
	sums := hex.EncodeToString(sum[:]) + "  " + asset + "\n"
	if mode == "badsum" {
		sums = strings.Repeat("0", 64) + "  " + asset + "\n"
	}
	dl := "/download/nav-pilot/" + fakeGHLatest + "/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/navikt/copilot/releases":
			fmt.Fprintf(w, `[{"tag_name": "nav-pilot/%s"}]`, fakeGHLatest)
		case "/repos/navikt/cplt/releases":
			fmt.Fprint(w, `[{"tag_name": "2026.09.24-192459-38642b4"}]`)
		case dl + asset:
			fmt.Fprint(w, fakeGHBinary)
		case dl + "SHA256SUMS":
			fmt.Fprint(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	ts.Defer(srv.Close)

	bin := ts.MkAbs("bin")
	ts.Check(os.MkdirAll(bin, 0o755))
	data, err := os.ReadFile(binPath)
	ts.Check(err)
	ts.Check(os.WriteFile(filepath.Join(bin, "nav-pilot"), data, 0o755))
	ts.Setenv("PATH", bin+string(os.PathListSeparator)+ts.Getenv("PATH"))
	ts.Setenv("NAV_PILOT_E2E_GITHUB", srv.URL)
	ts.Setenv("NAV_PILOT_E2E_VERSION", fakeGHCurrent)
}
