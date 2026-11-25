package api

import (
	"strings"
	"testing"
)

func TestSchemaIsPublic(t *testing.T) {
	h, _ := testRouter(t)
	rec := do(h, "GET", "/api/v1/surveys/schema", "", "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/schema+json" || !strings.Contains(rec.Body.String(), `"schema_version"`) {
		t.Fatalf("%d %s", rec.Code, rec.Header())
	}
}
