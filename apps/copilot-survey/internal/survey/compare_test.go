package survey

import (
	"testing"
)

// Two waves. Spring showed the statements to everyone; autumn skips them
// for "none". "slow" changed version between the waves.
const compareSurveys = `[
{"id":"spring","title":"Vår","starts":"2026-03-01","ends":"2026-03-31","questions":[
 {"id":"tools","version":1,"type":"multi","text":"Tools?","options":["a","none"]},
 {"id":"grid","version":1,"type":"matrix","text":"Agree?","min":1,"max":5,
  "items":[{"id":"fast","version":1,"text":"Fast."},{"id":"slow","version":1,"text":"Slow."}]},
 {"id":"note","version":1,"type":"text","text":"?","max_length":9}]},
{"id":"autumn","title":"Høst","starts":"2026-10-01","ends":"2026-10-31","questions":[
 {"id":"tools","version":1,"type":"multi","text":"Tools?","options":["a","none"]},
 {"id":"grid","version":1,"type":"matrix","text":"Agree?","min":1,"max":5,"skip_if":{"question":"tools","answer":"none"},
  "items":[{"id":"fast","version":1,"text":"Fast."},{"id":"slow","version":2,"text":"Slow, reworded."}]},
 {"id":"note","version":1,"type":"text","text":"?","max_length":9}]}]`

// waveRows: spring 4 users (fast 5,5,4,2) + 2 non-users (fast 1);
// autumn 5 users (fast 3). Spring from Forms, autumn 3 web + 2 copilot.
func waveRows() map[string][]StoredAnswer {
	out := map[string][]StoredAnswer{}
	for _, f := range []string{"5", "5", "4", "2"} {
		r := row(`{"tools":["a"],"fast":`+f+`,"slow":3,"note":"x"}`, `{"tools":1,"fast":1,"slow":1,"note":1}`)
		r.Client, r.Context = "forms", map[string]any{"client": "forms"}
		out["spring"] = append(out["spring"], r)
	}
	for range 2 {
		r := row(`{"tools":["none"],"fast":1,"slow":1}`, `{"tools":1,"fast":1,"slow":1}`)
		r.Client, r.Context = "forms", map[string]any{"client": "forms"}
		out["spring"] = append(out["spring"], r)
	}
	for i := range 5 {
		r := row(`{"tools":["a"],"fast":3,"slow":4}`, `{"tools":1,"fast":1,"slow":2}`)
		r.Client = map[bool]string{true: "web", false: "copilot"}[i < 3]
		r.Context = map[string]any{"client": r.Client}
		out["autumn"] = append(out["autumn"], r)
	}
	return out
}

// Two skip rules that overlap: the first respondent matches both, so the
// set skips 1 of 6, not 2. «Vet ikke» (unsure) is answered by the skipped
// respondent and one kept, so it is subtracted like any option and stays
// out of the mean. Rule sets are one rule per question until
// skip_if becomes a list (#2267), so the set is built here.
const unionSurvey = `[{"id":"union","title":"U","starts":"2026-01-01","ends":"2026-01-31","questions":[
 {"id":"a","version":1,"type":"choice","text":"A?","options":["x","y"]},
 {"id":"b","version":1,"type":"choice","text":"B?","options":["p","q"]},
 {"id":"fast","version":1,"type":"scale","text":"Fast?","min":1,"max":5,"unsure":"Vet ikke"}]}]`

func unionFixture(t *testing.T) (Survey, []StoredAnswer, string, map[string]SkipSet) {
	t.Helper()
	defs, err := loadSurveys([]byte(unionSurvey))
	if err != nil {
		t.Fatal(err)
	}
	rows := []StoredAnswer{row(`{"a":"x","b":"p","fast":"Vet ikke"}`, `{"a":1,"b":1,"fast":1}`)}
	for _, f := range []string{"2", "3", "4", "5", `"Vet ikke"`} {
		rows = append(rows, row(`{"a":"y","b":"q","fast":`+f+`}`, `{"a":1,"b":1,"fast":1}`))
	}
	rules := []SkipRule{{"b", "p"}, {"a", "x"}, {"b", "p"}}
	id := SkipSetID(rules)
	if id != SkipSetID([]SkipRule{{"a", "x"}, {"b", "p"}}) {
		t.Fatalf("skipSetID not canonical: %s", id)
	}
	return defs[0], rows, id, map[string]SkipSet{id: {Rules: rules, Keys: []string{"fast"}}}
}

func keptFast(t *testing.T, tl WaveTally, id string) ([]VersionResult, int) {
	t.Helper()
	kept, excluded := Without(tl.Keys["fast"], tl.Skipped[id]["fast"])
	return VersionsFrom(Question{ID: "fast", Type: "scale", Version: 1, Min: 1, Max: 5, Unsure: "Vet ikke"}, kept), excluded
}

func TestSkipSetUnion(t *testing.T) {
	s, rows, id, sets := unionFixture(t)
	vs, excluded := keptFast(t, RowsTally(s, rows, sets), id)
	o := vs[0].Options
	if excluded != 1 || len(vs) != 1 || *vs[0].N != 5 || len(o) != 6 || *o[0].Count != 0 || o[5].Value != "Vet ikke" || *o[5].Count != 1 {
		t.Fatalf("excluded %d, kept %+v", excluded, vs)
	}
	if mean, agree := scaleStats(vs[0], 5); *mean != 3.5 || *agree != 0.5 {
		t.Fatalf("unsure in mean or agree: %v %v", *mean, *agree)
	}
}
