package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// #830: a test in this package installed an agentpakke and, three calls down,
// asked api.github.com for the releases of a repo that does not exist. Nothing
// failed — the lookup 404s, the install falls back, the assertions still hold —
// so it went unnoticed until a hand-rolled overlay caught it. The suite was
// quietly hostage to GitHub being up, to an anonymous rate limit shared by
// every CI job, and it failed for an offline developer for reasons that had
// nothing to do with their change.
//
// nettvakt makes that impossible to miss instead of impossible to see: every
// request leaving the test binary for a host that is not loopback panics,
// naming the URL and the test in the stack. Loopback stays open, because an
// httptest server is how this package already stubs HTTP — the guard draws the
// line at the real internet, not at the net/http package.
//
// Transport level rather than at each lookup hook on purpose: a hook guard
// covers the one door it was written for, and #830 was precisely a door nobody
// had thought to cover. Everything here ends up in an http.RoundTripper, so
// that is where one guard covers all of them.
type nettvakt struct{ ekte http.RoundTripper }

func (g nettvakt) RoundTrip(req *http.Request) (*http.Response, error) {
	if host := req.URL.Hostname(); host == "localhost" {
		return g.ekte.RoundTrip(req)
	} else if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return g.ekte.RoundTrip(req)
	}
	panic(fmt.Sprintf("nettvakt: this test reached the network: %s %s\n"+
		"Stub the lookup (stubRelease/releaseSyncSource), assert it stays put (utenNett),\n"+
		"serve it from an httptest server, or call medNett(t) if the test really needs the internet.",
		req.Method, req.URL))
}

var errOfflineForTests = errors.New("offline: the test binary starts without base freshness lookups")

// vakt is the guard every client in the binary is pointed at, kept so medNett
// can hand back the transport it replaced.
var vakt = nettvakt{ekte: http.DefaultTransport}

func TestMain(m *testing.M) {
	// Both doors: http.DefaultTransport covers every client built without a
	// transport of its own, httpClient is the one client that brings one. A
	// third client that constructs its own transport would slip past both, so
	// don't — take httpClient.Transport, the way assessStaleness does.
	http.DefaultTransport = vakt
	httpClient.Transport = vakt
	// Release checks in the foreground: tests swap releasesAPI and httpClient.
	artifacts.RefreshInBackground = false
	// A fake cplt is a shell script, and under load its first exec alone
	// can take longer than the 2 s a real cplt gets (#1335). No test here
	// relies on that deadline firing.
	cpltCommandTimeout = 30 * time.Second
	// Every sync of a composed pakke and every doctor run on another source
	// asks GitHub whether the reused base is behind (#1368). Offline by
	// default, which the check answers with silence; realBaseFreshness puts
	// the HTTP lookups back for the tests about it.
	lookupBaseLag = func(context.Context, string, string, string) (*baseLag, error) { return nil, errOfflineForTests }
	githubFileJSON = func(context.Context, string, string, string, any) error { return errOfflineForTests }
	githubFile = func(context.Context, string, string, string) ([]byte, error) { return nil, errOfflineForTests }
	lookupPakkeUpdate = func(context.Context, string, string, string) (*pakkeRelease, error) { return nil, errOfflineForTests }
	os.Exit(testhome.Run(m))
}

// medNett opts one test back onto the real network. Only a test whose whole
// point is crossing that boundary should call it, and it must be able to skip
// itself when the network is not there.
func medNett(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		http.DefaultTransport = vakt
		httpClient.Transport = vakt
	})
	http.DefaultTransport = vakt.ekte
	httpClient.Transport = vakt.ekte
}
