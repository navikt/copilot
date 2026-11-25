package survey

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

const testSurveys = `[{"id":"q4-2026","title":"Q4","active":true,"starts":"2026-10-01","ends":"2026-11-30","questions":[
 {"id":"overall","version":1,"type":"scale","text":"How useful?","min":1,"max":5,"required":true},
 {"id":"client","version":1,"type":"choice","text":"Which client?","options":["copilot","opencode"]},
 {"id":"tools","version":1,"type":"multi","text":"Which tools?","options":["a","b","c"],"max_choices":2},
 {"id":"why","version":1,"type":"choice","text":"Why not copilot?","options":["habit","other"],"skip_if":{"question":"client","answer":"copilot"}},
 {"id":"editor","version":1,"type":"multi","text":"Which editors?","options":["vim","vscode"],"other":"Annet","max_length":10},
 {"id":"grid","version":1,"type":"matrix","text":"How much do you agree?","min":1,"max":5,"unsure":"Vet ikke","skip_if":{"question":"client","answer":"opencode"},
  "items":[{"id":"fast","version":2,"text":"It is fast."},{"id":"safe","version":1,"text":"It is safe.","reverse":true}]},
 {"id":"comment","version":1,"type":"text","text":"Anything else?","max_length":20}]},
 {"id":"old","title":"Old","active":true,"starts":"2025-01-01","ends":"2025-01-31","questions":[{"id":"a","version":1,"type":"scale","text":"?","min":1,"max":3}]}]`

func TestParticipantHash(t *testing.T) {
	k1 := []byte("0123456789abcdef0123456789abcdef")
	k2 := []byte("another key another key another k")
	if ParticipantHash(k1, "Ola.Nordmann@nav.no ") != ParticipantHash(k1, "ola.nordmann@NAV.no") {
		t.Fatal("e-mail is not normalised")
	}
	if ParticipantHash(k1, "a@nav.no") == ParticipantHash(k2, "a@nav.no") {
		t.Fatal("hash does not depend on the survey key")
	}
}

func TestLoadSurveysRejectsBadDefinitions(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown field":        `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}],"x":1}]`,
		"ends first":           `[{"id":"a","title":"t","starts":"2026-01-02","ends":"2026-01-01","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"bad type":             `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"essay","text":"?"}]}]`,
		"text no limit":        `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?"}]}]`,
		"one option":           `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x"]}]}]`,
		"duplicate id":         `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]},{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"no version":           `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","type":"scale","text":"?","min":1,"max":5}]}]`,
		"labels count":         `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"labels":["a","b"]}]}]`,
		"skip_if later":        `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":{"question":"r","answer":"x"}},{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"]}]}]`,
		"skip_if object extra": `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"]},{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":{"question":"r","answer":"x","x":1}}]}]`,
		"skip_if list extra":   `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"]},{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":[{"question":"r","answer":"x","x":1}]}]}]`,
		"skip_if list empty":   `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"]},{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":[]}]}]`,
		"skip_if list bad":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"]},{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":[{"question":"r","answer":"x"},{"question":"r","answer":"z"}]}]}]`,
		"skip_if scale":        `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"scale","text":"?","min":1,"max":5,"options":["x"]},{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":{"question":"r","answer":"x"}}]}]`,
		"exclusive choice":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"],"exclusive":"x"}]}]`,
		"exclusive unknown":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"multi","text":"?","options":["x","y"],"exclusive":"z"}]}]`,
		"stray ]":              `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]]`,
		"bad nudge":            `[{"id":"a","title":"t","nudge":"loud","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"two texts":            `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?","max_length":5},{"id":"r","version":1,"type":"text","text":"?","max_length":5}]}]`,
		"other on scale":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"other":"Annet","max_length":5}]}]`,
		"other is option":      `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","Annet"],"other":"Annet","max_length":5}]}]`,
		"other no limit":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet"}]}]`,
		"other too long":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":201}]}]`,
		"four others":          `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"a","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5},{"id":"b","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5},{"id":"c","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5},{"id":"d","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5}]}]`,
		"limit no other":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","y"],"max_length":5}]}]`,
		"matrix one item":      `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"i","version":1,"text":"?"}]}]}]`,
		"matrix item id":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"q","version":1,"text":"?"},{"id":"i","version":1,"text":"?"}]}]}]`,
		"matrix item twice":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"i","version":1,"type":"scale","text":"?","min":1,"max":5},{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"i","version":1,"text":"?"},{"id":"j","version":1,"text":"?"}]}]}]`,
		"matrix no version":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"i","text":"?"},{"id":"j","version":1,"text":"?"}]}]}]`,
		"items on scale":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"items":[{"id":"i","version":1,"text":"?"},{"id":"j","version":1,"text":"?"}]}]}]`,
		"id with slash":        `[{"id":"a/b","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"unsure on choice":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"],"unsure":"Vet ikke"}]}]`,
		"unsure numeric":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"unsure":"0"}]}]`,
	} {
		if _, err := loadSurveys([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSurveyKeysOnlyForOpenSurveys(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	testKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	t.Setenv("SURVEY_KEY_Q4_2026", testKey)
	t.Setenv("SURVEY_KEY_OLD", testKey)
	keys := Keys(defs, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if len(keys["q4-2026"]) != 32 || keys["old"] != nil {
		t.Fatalf("keys: %v", keys)
	}
}

// TestShippedSurveys is the CI check of the definitions in surveys/.
func TestShippedSurveys(t *testing.T) {
	if _, err := LoadSurveyDir(SurveyFiles); err != nil {
		t.Fatal(err)
	}
}

// TestDuplicateKeysRefused: encoding/json merges a duplicate "questions" into
// the next one, so a min could change unseen by check-frozen-surveys.sh.
func TestDuplicateKeysRefused(t *testing.T) {
	q := `{"id":"a","version":1,"type":"scale","text":"?","min":0,"max":4}`
	head := `{"id":"w","title":"T","starts":"2000-01-01","ends":"2000-02-01",`
	for name, def := range map[string]string{
		"top level":     head + `"questions":[{"min":2}],"questions":[` + q + `]}`,
		"other case":    head + `"Questions":[{"min":2}],"questions":[` + q + `]}`,
		"in a question": head + `"questions":[{"min":2,` + q[1:] + `]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadSurveys([]byte("[" + def + "]")); err == nil || !strings.Contains(err.Error(), "duplicate key") {
				t.Fatalf("err = %v, want duplicate key", err)
			}
		})
	}
	if _, err := loadSurveys([]byte("[" + head + `"questions":[` + q + `,{"id":"b","version":1,"type":"scale","text":"?","min":0,"max":4}]}]`)); err != nil {
		t.Fatalf("same keys in sibling objects: %v", err)
	}
}

// TestActivateInDevOnly: SURVEY_ACTIVE_IDS opens a survey in dev-gcp and
// nowhere else, and the shipped dummy is inactive on its own.
func TestActivateInDevOnly(t *testing.T) {
	defs, err := LoadSurveyDir(SurveyFiles)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(defs, func(s Survey) bool { return s.ID == "dev-e2e-test" })
	if i < 0 || defs[i].Active {
		t.Fatal("dev-e2e-test must ship, inactive")
	}
	for _, cluster := range []string{"prod-gcp", "local", ""} {
		ActivateInDev(defs, cluster, "dev-e2e-test")
		if defs[i].Active {
			t.Fatalf("activated in %q", cluster)
		}
	}
	ActivateInDev(defs, "dev-gcp", " dev-e2e-test ,nope")
	if !defs[i].Active {
		t.Fatal("not activated in dev-gcp")
	}
}

func TestContextKeepsOnlyCoarseVersion(t *testing.T) {
	for in, want := range map[string]string{"2026.09.24-120000-abc1234": "2026.09", "v1.2.3": "1.2", "dev": "dev", "web": "web"} {
		c := TechContext{Version: in, OS: "linux", Client: "pi"}
		if err := c.Validate(); err != nil || c.Version != want {
			t.Errorf("%s: got %q, %v; want %q", in, c.Version, err, want)
		}
	}
}

func TestOtherCountsTowardMaxChoices(t *testing.T) {
	if _, err := loadSurveys([]byte(`[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"multi","text":"?","options":["x","y"],"max_choices":3,"other":"Annet","max_length":5}]}]`)); err != nil {
		t.Fatal(err)
	}
}

// A required matrix with skip_if is required unless skipped: skipped, it
// takes no item; asked, it takes every item.
func TestRequiredMatrixSkippedWhole(t *testing.T) {
	defs, err := loadSurveys([]byte(strings.Replace(testSurveys, `"type":"matrix","text":"How much do you agree?",`, `"type":"matrix","text":"How much do you agree?","required":true,`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		answers string
		ok      bool
	}{
		"skipped, no items":  {`{"overall":3,"client":"opencode"}`, true},
		"skipped, one item":  {`{"overall":3,"client":"opencode","fast":4}`, false},
		"asked, no items":    {`{"overall":3,"client":"copilot"}`, false},
		"asked, one item":    {`{"overall":3,"client":"copilot","fast":4}`, false},
		"asked, every item":  {`{"overall":3,"client":"copilot","fast":4,"safe":2}`, true},
		"unanswered skip_if": {`{"overall":3}`, false},
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tc.answers), &raw); err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateAnswers(defs[0], raw); (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// «Vet ikke» is taken as its label on a matrix item, and nothing else
// non-numeric is.
func TestUnsure(t *testing.T) {
	defs, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	for answers, ok := range map[string]bool{
		`{"overall":3,"client":"copilot","fast":"Vet ikke","safe":2}`: true,
		`{"overall":3,"client":"copilot","fast":"vet ikke"}`:          false,
		`{"overall":"Vet ikke"}`:                                      false,
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(answers), &raw); err != nil {
			t.Fatal(err)
		}
		out, err := ValidateAnswers(defs[0], raw)
		if (err == nil) != ok {
			t.Errorf("%s: %v", answers, err)
		}
		if ok && out["fast"] != "Vet ikke" {
			t.Errorf("stored %v", out["fast"])
		}
	}
	two, four := 2, 4
	mean, agree := scaleStats(VersionResult{Options: []OptionCount{{Value: "5", Count: &two}, {Value: "Vet ikke", Count: &four}}}, 5)
	if *mean != 5 || *agree != 1 {
		t.Errorf("mean %v agree %v", *mean, *agree)
	}
}

// skip_if as a list skips when any rule matches, and one rule still
// marshals as an object for clients that predate lists.
func TestSkipIfList(t *testing.T) {
	defs, err := loadSurveys([]byte(strings.Replace(testSurveys, `"skip_if":{"question":"client","answer":"opencode"}`,
		`"skip_if":[{"question":"client","answer":"opencode"},{"question":"tools","answer":"c"}]`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	for answers, ok := range map[string]bool{
		`{"overall":3,"client":"opencode"}`:                        true,
		`{"overall":3,"client":"copilot","tools":["c"]}`:           true,
		`{"overall":3,"client":"copilot","tools":["c"],"fast":4}`:  false,
		`{"overall":3,"client":"copilot","tools":["a"],"fast":4}`:  true,
		`{"overall":3,"client":"opencode","tools":["a"],"safe":2}`: false,
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(answers), &raw); err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateAnswers(defs[0], raw); (err == nil) != ok {
			t.Errorf("%s: %v", answers, err)
		}
	}
	one, _ := json.Marshal(defs[0].Questions[3].SkipIf)
	two, _ := json.Marshal(defs[0].Questions[5].SkipIf)
	if string(one) != `{"question":"client","answer":"copilot"}` || !strings.HasPrefix(string(two), "[") {
		t.Errorf("marshal: %s %s", one, two)
	}
}

// An exclusive multi option must be picked alone.
func TestExclusive(t *testing.T) {
	defs, err := loadSurveys([]byte(strings.Replace(testSurveys, `"max_choices":2}`, `"max_choices":2,"exclusive":"c"}`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	for answers, ok := range map[string]bool{
		`{"overall":3,"tools":["c"]}`:     true,
		`{"overall":3,"tools":["a","b"]}`: true,
		`{"overall":3,"tools":["a","c"]}`: false,
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(answers), &raw); err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateAnswers(defs[0], raw); (err == nil) != ok {
			t.Errorf("%s: %v", answers, err)
		}
	}
}

func TestForCluster(t *testing.T) {
	defs := func() []Survey {
		return []Survey{{ID: "real"}, {ID: "dev-only", Environments: []string{"dev-gcp"}}}
	}
	for cluster, want := range map[string]int{"prod-gcp": 1, "dev-gcp": 2, "": 2} {
		if got := ForCluster(defs(), cluster); len(got) != want || got[0].ID != "real" {
			t.Errorf("%q: %v", cluster, got)
		}
	}
}

func loadSurveys(raw []byte) ([]Survey, error) {
	var surveys []Survey
	if err := DecodeStrict(raw, &surveys); err != nil {
		return nil, err
	}
	return surveys, ValidateSurveys(surveys)
}
