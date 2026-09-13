package cli

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
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
	os.Exit(m.Run())
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
