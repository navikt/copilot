package store

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
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
func waveRows() map[string][]survey.StoredAnswer {
	out := map[string][]survey.StoredAnswer{}
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

func unionFixture(t *testing.T) (survey.Survey, []survey.StoredAnswer, string, map[string]survey.SkipSet) {
	t.Helper()
	defs, err := loadSurveys([]byte(unionSurvey))
	if err != nil {
		t.Fatal(err)
	}
	rows := []survey.StoredAnswer{row(`{"a":"x","b":"p","fast":"Vet ikke"}`, `{"a":1,"b":1,"fast":1}`)}
	for _, f := range []string{"2", "3", "4", "5", `"Vet ikke"`} {
		rows = append(rows, row(`{"a":"y","b":"q","fast":`+f+`}`, `{"a":1,"b":1,"fast":1}`))
	}
	rules := []survey.SkipRule{{Question: "b", Answer: "p"}, {Question: "a", Answer: "x"}, {Question: "b", Answer: "p"}}
	id := survey.SkipSetID(rules)
	if id != survey.SkipSetID([]survey.SkipRule{{Question: "a", Answer: "x"}, {Question: "b", Answer: "p"}}) {
		t.Fatalf("skipSetID not canonical: %s", id)
	}
	return defs[0], rows, id, map[string]survey.SkipSet{id: {Rules: rules, Keys: []string{"fast"}}}
}

func keptFast(t *testing.T, tl survey.WaveTally, id string) ([]survey.VersionResult, int) {
	t.Helper()
	kept, excluded := survey.Without(tl.Keys["fast"], tl.Skipped[id]["fast"])
	return survey.VersionsFrom(survey.Question{ID: "fast", Type: "scale", Version: 1, Min: 1, Max: 5, Unsure: "Vet ikke"}, kept), excluded
}

// TestSkipSetUnionStored is TestSkipSetUnion through survey_skip_results,
// before and after the rows are deleted (COPILOT_SURVEY_TEST_DB_URL).
func TestSkipSetUnionStored(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	ctx := t.Context()
	st, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Pool.Close()
	_, _ = st.Pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	s, rows, id, sets := unionFixture(t)
	for _, r := range rows {
		aj, _ := json.Marshal(r.Answers)
		vj, _ := json.Marshal(r.QuestionVersions)
		if _, err = st.Pool.Exec(ctx, `INSERT INTO survey_answers VALUES ('union', $1, $2, '{"client":"web"}', '2027-01-01')`, aj, vj); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	if _, err = finalize(ctx, st.Pool, []survey.Survey{s}, s, now, true); err != nil {
		t.Fatal(err)
	}
	if err = pgx.BeginFunc(ctx, st.Pool, func(tx pgx.Tx) error { return writeSkipped(ctx, tx, sets, s, rows) }); err != nil {
		t.Fatal(err)
	}
	wantV, wantX := keptFast(t, survey.RowsTally(s, rows, sets), id)
	_, _ = st.Pool.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id = 'union'`)
	stored, err := st.FinalCompare(ctx, "union")
	if err != nil || stored == nil {
		t.Fatal(stored, err)
	}
	gotV, gotX := keptFast(t, *stored, id)
	if gotX != wantX || !reflect.DeepEqual(gotV, wantV) || *gotV[0].N != 5 || *gotV[0].Options[5].Count != 1 {
		t.Fatalf("stored: excluded %d kept %+v, want %d %+v", gotX, gotV, wantX, wantV)
	}
}
