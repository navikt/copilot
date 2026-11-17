package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestCodebooksMatchSchema checks every free-text codebook in
// surveys/codebooks/ (#2239) against its schema, and what the schema cannot
// say: unique category ids, one catch-all per field, every category in use, mutual merge_with pairs,
// and fields that exist in the series.
func TestCodebooksMatchSchema(t *testing.T) {
	raw, err := fs.ReadFile(surveyFiles, "surveys/codebooks/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource("codebook.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("codebook.json")
	if err != nil {
		t.Fatal(err)
	}
	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		t.Fatal(err)
	}
	names, _ := fs.Glob(surveyFiles, "surveys/codebooks/*.json")
	n := 0
	for _, name := range names {
		if path.Base(name) == "schema.json" {
			continue
		}
		n++
		raw, _ := fs.ReadFile(surveyFiles, name)
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := s.Validate(inst); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		var cb struct {
			Series string
			Fields []struct {
				ID         string
				Categories []string
			}
			Categories []struct {
				ID, Kind  string
				MergeWith string `json:"merge_with"`
			}
		}
		if err := json.Unmarshal(raw, &cb); err != nil {
			t.Fatal(err)
		}
		if path.Base(name) != cb.Series+".json" {
			t.Errorf("%s: must be named after the series, %s.json", name, cb.Series)
		}
		kind, used := map[string]string{}, map[string]bool{}
		for _, cat := range cb.Categories {
			if kind[cat.ID] != "" {
				t.Errorf("%s: category %s twice", name, cat.ID)
			}
			kind[cat.ID] = cat.Kind
		}
		for _, f := range cb.Fields {
			if !seriesHasFreetext(defs, cb.Series, f.ID) {
				t.Errorf("%s: no survey in series %s has free-text field %s", name, cb.Series, f.ID)
			}
			catchAll := 0
			for _, id := range f.Categories {
				used[id] = true
				switch kind[id] {
				case "":
					t.Errorf("%s: field %s: unknown category %s", name, f.ID, id)
				case "annet":
					catchAll++
				}
			}
			if catchAll != 1 {
				t.Errorf("%s: field %s: %d categories of kind annet, want 1", name, f.ID, catchAll)
			}
		}
		partner := map[string]string{}
		for _, cat := range cb.Categories {
			partner[cat.ID] = cat.MergeWith
		}
		for id, p := range partner {
			if p != "" && (p == id || partner[p] != id) {
				t.Errorf("%s: category %s: merge_with %s is not a mutual pair", name, id, p)
			}
		}
		for id := range kind {
			if !used[id] {
				t.Errorf("%s: category %s is in no field", name, id)
			}
		}
	}
	if n == 0 {
		t.Fatal("no codebooks found")
	}
}

func seriesHasFreetext(defs []survey, series, field string) bool {
	id, other := strings.CutSuffix(field, ".other")
	for _, d := range defs {
		if d.Series != series {
			continue
		}
		for _, q := range d.Questions {
			if q.ID == id && (other && q.Other != "" || !other && q.Type == "text") {
				return true
			}
		}
	}
	return false
}
