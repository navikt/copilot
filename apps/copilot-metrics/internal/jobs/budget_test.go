package jobs

import (
	"net/http"
)

// rewriteHostTransport rewrites outgoing requests to hit a test server instead of GitHub.
type rewriteHostTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = t.target[len("http://"):]
	if t.base != nil {
		return t.base.RoundTrip(clone)
	}
	return http.DefaultTransport.RoundTrip(clone)
}
