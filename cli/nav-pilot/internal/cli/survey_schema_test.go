package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestSurveyTypesFollowSchema: nav-pilot reads every field of copilot-survey's
// definition schema except those it has no use for, and no field the schema
// lacks. A new field in the schema fails here until nav-pilot renders it or
// lists it as ignored.
func TestSurveyTypesFollowSchema(t *testing.T) {
	raw, err := os.ReadFile("../../../../apps/copilot-survey/surveys/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Properties map[string]any
		Defs       struct {
			Question struct{ Properties map[string]any } `json:"question"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	// Not needed to render or send: the server serves only active surveys,
	// and the rest is for analysis.
	ignored := []string{"series", "active", "version", "construct", "reverse"}
	for _, c := range []struct {
		typ   reflect.Type
		props map[string]any
	}{
		{reflect.TypeFor[surveyDef](), doc.Properties},
		{reflect.TypeFor[surveyQuestion](), doc.Defs.Question.Properties},
	} {
		var tags []string
		for f := range c.typ.Fields() {
			tags = append(tags, strings.Split(f.Tag.Get("json"), ",")[0])
		}
		for _, tag := range tags {
			if _, ok := c.props[tag]; !ok {
				t.Errorf("%s.%s is not in schema.json", c.typ.Name(), tag)
			}
		}
		for p := range c.props {
			if !slices.Contains(tags, p) && !slices.Contains(ignored, p) {
				t.Errorf("%s: schema.json field %q is neither read nor ignored", c.typ.Name(), p)
			}
		}
	}
}
