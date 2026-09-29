package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := fs.ReadFile(surveyFiles, schemaFile)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestShippedSurveysMatchSchema is the CI check that every definition in
// surveys/, and the test definitions here, fit schema.json, so a client that
// trusts the schema can read what copilot-survey serves.
func TestShippedSurveysMatchSchema(t *testing.T) {
	s := compileSchema(t)
	names, _ := fs.Glob(surveyFiles, "surveys/*.json")
	var docs []string
	for _, name := range names {
		if name != schemaFile {
			raw, _ := fs.ReadFile(surveyFiles, name)
			docs = append(docs, string(raw))
		}
	}
	var tests []json.RawMessage
	if err := json.Unmarshal([]byte(testSurveys), &tests); err != nil {
		t.Fatal(err)
	}
	for _, raw := range tests {
		docs = append(docs, string(raw))
	}
	for _, raw := range docs {
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(inst); err != nil {
			t.Errorf("%.60s: %v", raw, err)
		}
	}
	if !slices.ContainsFunc(names, func(n string) bool { return n == "surveys/dev-e2e-test.json" }) {
		t.Fatal("no shipped definitions found")
	}
}

// TestSchemaMatchesStructs: schema.json names exactly the fields
// copilot-survey decodes (it refuses any other), so the two cannot drift.
func TestSchemaMatchesStructs(t *testing.T) {
	var doc struct {
		Version    int `json:"schema_version"`
		Properties map[string]any
		Defs       struct {
			Question struct{ Properties map[string]any } `json:"question"`
		} `json:"$defs"`
	}
	raw, _ := fs.ReadFile(surveyFiles, schemaFile)
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Version < 1 {
		t.Fatalf("schema_version missing: %v", err)
	}
	for _, c := range []struct {
		typ   reflect.Type
		props map[string]any
	}{
		{reflect.TypeFor[survey](), doc.Properties},
		{reflect.TypeFor[question](), doc.Defs.Question.Properties},
	} {
		var tags []string
		for f := range c.typ.Fields() {
			tags = append(tags, strings.Split(f.Tag.Get("json"), ",")[0])
		}
		var props []string
		for p := range c.props {
			props = append(props, p)
		}
		slices.Sort(tags)
		slices.Sort(props)
		if !slices.Equal(tags, props) {
			t.Errorf("%s: fields %v, schema %v", c.typ.Name(), tags, props)
		}
	}
}

func TestSchemaIsPublic(t *testing.T) {
	h, _ := testRouter(t)
	rec := do(h, "GET", "/api/v1/surveys/schema", "", "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/schema+json" || !strings.Contains(rec.Body.String(), `"schema_version"`) {
		t.Fatalf("%d %s", rec.Code, rec.Header())
	}
}
