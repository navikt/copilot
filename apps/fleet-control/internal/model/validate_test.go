package model

import (
	"strings"
	"testing"
)

func TestValidateRoute(t *testing.T) {
	ok := Route{Path: "/mcp/team-a/svc", Target: "dev-gcp:team-a:app", UpstreamURL: "http://app.team-a/mcp", Tools: []string{"a", "b.c"}}
	cases := map[string]struct {
		edit func(*Route)
		want string // "" means valid
	}{
		"ok":                {func(*Route) {}, ""},
		"svc.cluster.local": {func(r *Route) { r.UpstreamURL = "https://app.team-a.svc.cluster.local/mcp" }, ""},
		"trailing root dot": {func(r *Route) { r.UpstreamURL = "http://app.team-a./" }, ""},
		"bad path":          {func(r *Route) { r.Path = "/mcp/team-a/../x" }, "path must be"},
		"path not /mcp":     {func(r *Route) { r.Path = "/x/team-a/svc" }, "path must be"},
		"target parts":      {func(r *Route) { r.Target = "dev-gcp:team-a" }, "must be <cluster>"},
		"target other ns":   {func(r *Route) { r.Target = "dev-gcp:team-b:app" }, "must be <cluster>"},
		"target bad app":    {func(r *Route) { r.Target = "dev-gcp:team-a:App" }, "must be <cluster>"},
		// Former difference: the seed accepted any non-empty cluster.
		"target bad cluster":   {func(r *Route) { r.Target = "Dev_gcp:team-a:app" }, "must be <cluster>"},
		"target other cluster": {func(r *Route) { r.Target = "prod-gcp:team-a:app" }, "must be in cluster"},
		"scheme":               {func(r *Route) { r.UpstreamURL = "ftp://app.team-a/" }, "http(s) without userinfo"},
		"userinfo":             {func(r *Route) { r.UpstreamURL = "http://u:p@app.team-a/" }, "http(s) without userinfo"},
		"empty host":           {func(r *Route) { r.UpstreamURL = "http:///x" }, "invalid host"},
		"ip":                   {func(r *Route) { r.UpstreamURL = "http://10.0.0.1/" }, "IP addresses"},
		"short ip":             {func(r *Route) { r.UpstreamURL = "http://127.1/" }, "IP addresses"},
		"punycode":             {func(r *Route) { r.UpstreamURL = "http://xn--nv-fia.team-a/" }, "punycode"},
		"other namespace":      {func(r *Route) { r.UpstreamURL = "http://app.team-b/" }, "host must be"},
		// Former differences: the seed accepted another app's service DNS and
		// Nav ingress hosts over https, and query strings and fragments.
		"other app":   {func(r *Route) { r.UpstreamURL = "http://other.team-a/" }, "host must be"},
		"ingress":     {func(r *Route) { r.UpstreamURL = "https://app.intern.nav.no/" }, "host must be"},
		"query":       {func(r *Route) { r.UpstreamURL = "http://app.team-a/?x=1" }, "query strings"},
		"empty query": {func(r *Route) { r.UpstreamURL = "http://app.team-a/?" }, "query strings"},
		"fragment":    {func(r *Route) { r.UpstreamURL = "http://app.team-a/#x" }, "query strings"},
		"no tools":    {func(r *Route) { r.Tools = nil }, "tools must not be empty"},
		"bad tool":    {func(r *Route) { r.Tools = []string{"a b"} }, "invalid tool name"},
		"star tool":   {func(r *Route) { r.Tools = []string{"*"} }, "invalid tool name"},
		"long tool":   {func(r *Route) { r.Tools = []string{strings.Repeat("a", 129)} }, "invalid tool name"},
		"dup tool":    {func(r *Route) { r.Tools = []string{"a", "a"} }, "duplicate tool"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := ok
			r.Tools = append([]string(nil), ok.Tools...)
			c.edit(&r)
			err := ValidateRoute("dev-gcp", r)
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("want valid, got %v", err)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
	if err := ValidateRoute("", ok); err == nil {
		t.Error("an unset cluster must refuse every target")
	}
}
