package api

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/contract from the handlers")

// contractDir holds the answers my-copilot parses with src/lib/fleet-control.ts
// (#3087). A field change here fails both this test and my-copilot's.
const contractDir = "../../testdata/contract"

var rfc3339 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)

// normalize replaces values that change per run: times, uid and resourceVersion.
func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if k == "resourceVersion" || k == "uid" {
				x[k] = "1"
				continue
			}
			x[k] = normalize(e)
		}
	case []any:
		for i, e := range x {
			x[i] = normalize(e)
		}
	case string:
		if rfc3339.MatchString(x) {
			return "2026-01-01T00:00:00Z"
		}
	}
	return v
}

func golden(t *testing.T, name, body string) {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("%s: %v: %s", name, err, body)
	}
	got, _ := json.MarshalIndent(normalize(v), "", "  ")
	got = append(got, '\n')
	path := filepath.Join(contractDir, name)
	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // fixed test path
	if err != nil {
		t.Fatalf("%v; run go test ./internal/api -run TestRealPostgresContract -update", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s drifted from the handler; my-copilot reads it. If intended, run with -update and fix src/lib/fleet-control.ts.\ngot:\n%s", name, got)
	}
}

func TestRealPostgresContract(t *testing.T) {
	e := newApisEnv(t)
	base := nsBase + "team-a/mcpservers"
	m := decodeServer(t, e.want(201, "POST", base, person("A111111"), serverJSON("team-a", "svc", "", "search", "fetch")))
	golden(t, "mcpserver-pending.json", e.want(200, "GET", base+"/svc", person("A111111"), ""))
	golden(t, "mcpserver-published.json", e.want(200, "POST", base+"/svc/approval", person("P999999"), approval(1, m.Status.Pending.ContentHash)))
	golden(t, "mcpserver-list.json", e.want(200, "GET", base, person("A111111"), ""))
	_, conflict := e.do("POST", base, person("A111111"), serverJSON("team-a", "svc", "", "search"))
	golden(t, "status-already-exists.json", conflict)

	e.publish("team-b", "kodeverk", "B444444", "lookup")
	pbase := nsBase + "aksel-studio/agentprofiles"
	p := decodeProfile(t, e.want(201, "POST", pbase, person("A111111"), profileJSON("aksel-studio", "assistant", "", "team-b/kodeverk=lookup")))
	golden(t, "agentprofile-pending.json", e.want(200, "GET", pbase+"/assistant", person("A555555"), ""))
	e.want(200, "POST", pbase+"/assistant/approval", person("B444444"), grantBody(1, "team-b/kodeverk", p.Status.Grants[0].ContentHash))
	golden(t, "agentprofile-ready.json", e.want(200, "GET", pbase+"/assistant", person("A555555"), ""))
	golden(t, "agentprofile-list.json", e.want(200, "GET", pbase, person("A555555"), ""))
}
