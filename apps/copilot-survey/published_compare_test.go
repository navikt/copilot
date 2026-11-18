package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPublishedCompare(t *testing.T) {
	data := waveRows()
	api := compareAPI(t, func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil })
	api.now = func() time.Time { return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC) }
	wave := func(id, starts, ends string, published bool, rows []storedAnswer) {
		s := api.surveys[0]
		s.ID, s.Starts, s.Ends, s.Published = id, starts, ends, published
		api.surveys, data[id] = append(api.surveys, s), rows
	}
	for i := range api.surveys {
		api.surveys[i].Series, api.surveys[i].Published = "dev", true
	}
	wave("small", "2026-05-01", "2026-05-31", true, data["spring"][:4])
	wave("hidden", "2026-06-01", "2026-06-30", false, data["spring"])
	wave("open", "2026-11-01", "2026-11-30", true, data["spring"])
	api.surveys[len(api.surveys)-1].Active = true
	h := makeRouter(testAuthenticator(t), api)
	const path = "/api/v1/surveys/published/compare?series="

	for tok, want := range map[string]int{"": 401, "cli:hans": 401, cliAdmin: 401, webToken: 200, adminToken: 200} {
		if rec := do(h, "GET", path+"dev", tok, ""); rec.Code != want {
			t.Errorf("token %q: %d, want %d", tok, rec.Code, want)
		}
	}

	get := func() compareResult {
		t.Helper()
		rec := do(h, "GET", path+"dev", webToken, "")
		var got compareResult
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		return got
	}
	// Small, unpublished and open waves are left out; no channels.
	got := get()
	if len(got.Surveys) != 2 || got.Surveys[0].ID != "spring" || got.Surveys[1].ID != "autumn" {
		t.Fatalf("waves %+v", got.Surveys)
	}
	for _, s := range got.Surveys {
		if s.Channels != nil {
			t.Errorf("channels published: %+v", s)
		}
	}
	// Each wave counts its own answers: spring keeps its 2 non-users on
	// "fast" (n 6), as its /results does. Admin drops them (TestCompare).
	fast := got.Questions[1].Items[0]
	if s := fast.Waves[0]; s.Excluded != nil || *s.Versions[0].N != 6 || fast.P == nil || fast.PHolm == nil {
		t.Errorf("spring fast: %+v %+v", s, fast.significance)
	}
	body := do(h, "GET", path+"dev", webToken, "").Body.String()
	if strings.Contains(body, `"excluded"`) {
		t.Errorf("excluded published: %s", body)
	}
	// Later waves' skip_if does not reach back, and the definition is not changed.
	rule := api.surveys[1].Questions[1].SkipIf
	api.surveys[1].Questions[1].SkipIf = nil
	if other := do(h, "GET", path+"dev", webToken, "").Body.String(); other != body {
		t.Errorf("depends on autumn's skip_if:\n%s\n%s", body, other)
	}
	if api.surveys[1].Questions[1].SkipIf = rule; rule == nil {
		t.Fatal("fixture lost its skip_if")
	}
	if do(h, "GET", path+"dev", webToken, "").Body.String() != body || !reflect.DeepEqual(api.surveys[1].Questions[1].SkipIf, rule) {
		t.Error("published compare changed the definition")
	}

	// At most the last four, oldest first.
	for _, m := range []string{"01", "02", "03"} {
		wave("w"+m, "2025-"+m+"-01", "2025-"+m+"-28", true, data["spring"])
	}
	var ids []string
	for _, s := range get().Surveys {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, ",") != "w02,w03,spring,autumn" {
		t.Errorf("last four: %v", ids)
	}

	rec := do(h, "GET", path+"dev&format=csv", webToken, "")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "sammenligning-dev.csv") ||
		strings.Contains(rec.Body.String(), "\nchannel,") || !strings.Contains(rec.Body.String(), "\nanswer,autumn,") {
		t.Errorf("csv %d %v %s", rec.Code, rec.Header(), rec.Body)
	}

	// Unknown or empty series, and a series with one wave left, are 404.
	api.surveys[1].Series = "other"
	for _, q := range []string{"nope", "", "other"} {
		if rec := do(h, "GET", path+q, webToken, ""); rec.Code != 404 {
			t.Errorf("series=%q: %d, want 404", q, rec.Code)
		}
	}
}

func TestMergeSmallChannels(t *testing.T) {
	got := mergeSmallChannels(map[string]int{"web": 5, "copilot": 2, "opencode": 1})
	if len(got) != 2 || got["web"] != 5 || got["annet"] != 3 {
		t.Errorf("got %v", got)
	}
}

// publishedSpring builds spring from groups of {count, tools answer, fast
// version, 0 for no answer}; autumn has 5 users and skips "grid" for "none". It returns the
// published comparison's "fast" item and spring's published /results for it.
func publishedSpring(t *testing.T, groups ...[3]any) (compareItem, []versionResult) {
	t.Helper()
	data := waveRows()
	data["spring"] = nil
	for _, g := range groups {
		for range g[0].(int) {
			r := row(`{"tools":["`+g[1].(string)+`"],"fast":5,"slow":3}`, fmt.Sprintf(`{"tools":1,"fast":%d,"slow":1}`, g[2].(int)))
			if g[2].(int) == 0 { // did not answer "fast"
				delete(r.Answers, "fast")
			}
			data["spring"] = append(data["spring"], r)
		}
	}
	api := compareAPI(t, func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil })
	api.now = func() time.Time { return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC) }
	for i := range api.surveys {
		api.surveys[i].Series, api.surveys[i].Published = "dev", true
	}
	h := makeRouter(testAuthenticator(t), api)
	var cmp compareResult
	var res surveyResults
	if rec := do(h, "GET", "/api/v1/surveys/published/compare?series=dev", webToken, ""); rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &cmp) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/v1/surveys/spring/results", webToken, ""); rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return cmp.Questions[1].Items[0], res.Questions[1].Items[0].Versions
}

// sameAsResults: every version of a wave has /results' n and counts, or is
// hidden in both, so subtracting one from the other leaves nothing.
func sameAsResults(t *testing.T, w waveCounts, res []versionResult) {
	t.Helper()
	if len(w.Versions) != len(res) || w.Excluded != nil {
		t.Fatalf("versions %+v vs %+v", w, res)
	}
	for i, v := range w.Versions {
		r := res[i]
		if v.Version != r.Version || v.Hidden != r.Hidden || (v.N == nil) != (r.N == nil) || (v.N != nil && *v.N != *r.N) {
			t.Errorf("version %d: %+v vs %+v", v.Version, v.versionResult, r)
		}
		if v.Hidden && (v.Options != nil || v.Mean != nil || v.Agree != nil) {
			t.Errorf("hidden version %d has counts: %+v", v.Version, v)
		}
		counts := map[string]int{}
		for _, o := range r.Options {
			counts[o.Value] = *o.Count
		}
		for _, o := range v.Options {
			if *o.Count != counts[o.Value] {
				t.Errorf("version %d option %s: %d vs %d", v.Version, o.Value, *o.Count, counts[o.Value])
			}
		}
	}
}

// Astra's scenarios (#2271): with autumn's skip_if applied backwards, the
// difference to spring's /results showed 1 + 4 dropped non-users, or the one
// user kept out of 6. Each wave now has its own counts, equal to /results.
func TestPublishedCompareEqualsResults(t *testing.T) {
	// 20 answers: version 1 has 7 users and 1 non-user, version 2 8 and 4.
	fast, res := publishedSpring(t, [3]any{7, "a", 1}, [3]any{1, "none", 1}, [3]any{8, "a", 2}, [3]any{4, "none", 2})
	sameAsResults(t, fast.Waves[0], res)
	if *fast.Waves[0].Versions[0].N != 8 || *fast.Waves[0].Versions[1].N != 12 {
		t.Errorf("own counts: %+v", fast.Waves[0])
	}
	// 6 answers, 5 of them non-users: all 6 count, as in /results.
	fast, res = publishedSpring(t, [3]any{1, "a", 1}, [3]any{5, "none", 1})
	sameAsResults(t, fast.Waves[0], res)
	if *fast.Waves[0].Versions[0].N != 6 || fast.P == nil {
		t.Errorf("own counts: %+v", fast)
	}
	// A version under 5 is hidden in both.
	fast, res = publishedSpring(t, [3]any{7, "a", 1}, [3]any{3, "a", 2})
	sameAsResults(t, fast.Waves[0], res)
	if !fast.Waves[0].Versions[1].Hidden || fast.Waves[0].Versions[0].Hidden {
		t.Errorf("small version: %+v", fast.Waves[0])
	}
	// Only 3 answered "fast" in spring: hidden, and no test on 3 people.
	fast, res = publishedSpring(t, [3]any{3, "a", 1}, [3]any{3, "a", 0})
	sameAsResults(t, fast.Waves[0], res)
	if !fast.Waves[0].Versions[0].Hidden || fast.P != nil || fast.PHolm != nil {
		t.Errorf("small wave tested: %+v %+v", fast.Waves[0], fast.significance)
	}
}

// Published /results hides a version under 5 respondents; admin shows it.
func TestPublishedResultsSmallVersion(t *testing.T) {
	var rows []storedAnswer
	for i := range 6 {
		rows = append(rows, row(`{"a":2}`, fmt.Sprintf(`{"a":%d}`, 1+i/5))) // 5 on version 1, 1 on version 2
	}
	api := adminAPI(t, rows)
	for i := range api.surveys {
		api.surveys[i].Published = true
	}
	h := makeRouter(testAuthenticator(t), api)
	scaleA := func(token, path string) []versionResult {
		t.Helper()
		var res surveyResults
		if json.Unmarshal(do(h, "GET", path, token, "").Body.Bytes(), &res) != nil {
			t.Fatal(path)
		}
		for _, q := range res.Questions {
			if q.ID == "a" {
				return q.Versions
			}
		}
		t.Fatalf("no a in %s", path)
		return nil
	}
	pub := scaleA(webToken, "/api/v1/surveys/old/results")
	if len(pub) != 2 || pub[0].Hidden || *pub[0].N != 5 || !pub[1].Hidden || pub[1].N != nil || pub[1].Options != nil {
		t.Errorf("published: %+v", pub)
	}
	if adm := scaleA(adminToken, "/api/v1/admin/surveys/old/results"); len(adm) != 2 || adm[1].Hidden || *adm[1].N != 1 {
		t.Errorf("admin: %+v", adm)
	}
}

// TestPublishedCompareAfterRetention (COPILOT_SURVEY_TEST_DB_URL): the
// published series reads each wave as its /results does, so it is
// byte-identical, JSON and CSV, before and after each wave's rows are
// deleted, and every wave matches its published /results.
func TestPublishedCompareAfterRetention(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	ctx := t.Context()
	st, err := openSurveyStore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.pool.Close()
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	api := compareAPI(t, st.answers)
	api.now, api.final = func() time.Time { return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC) }, st.final
	for i := range api.surveys {
		api.surveys[i].Series, api.surveys[i].Published = "dev", true
	}
	st.defs = api.surveys
	for id, rows := range waveRows() {
		for _, r := range rows {
			aj, _ := json.Marshal(r.Answers)
			vj, _ := json.Marshal(r.QuestionVersions)
			cj, _ := json.Marshal(r.Context)
			if _, err = st.pool.Exec(ctx, `INSERT INTO survey_answers VALUES ($1, $2, $3, $4, '2027-01-01')`, id, aj, vj, cj); err != nil {
				t.Fatal(err)
			}
		}
	}
	st.closeOut(ctx, api.now())
	h := makeRouter(testAuthenticator(t), api)
	const path = "/api/v1/surveys/published/compare?series=dev"
	series := func() (string, string) {
		t.Helper()
		j, c := do(h, "GET", path, webToken, ""), do(h, "GET", path+"&format=csv", webToken, "")
		if j.Code != 200 || c.Code != 200 {
			t.Fatalf("series: %d %s / %d", j.Code, j.Body, c.Code)
		}
		return j.Body.String(), c.Body.String()
	}
	matchesResults := func(body string) {
		t.Helper()
		var cmp compareResult
		_ = json.Unmarshal([]byte(body), &cmp)
		for wi, m := range cmp.Surveys {
			var res surveyResults
			if rec := do(h, "GET", "/api/v1/surveys/"+m.ID+"/results", webToken, ""); rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil {
				t.Fatalf("%s results: %d %s", m.ID, rec.Code, rec.Body)
			}
			versions := map[string][]versionResult{}
			for _, q := range res.Questions {
				versions[q.ID] = q.Versions
				for _, it := range q.Items {
					versions[it.ID] = it.Versions
				}
			}
			for _, q := range cmp.Questions {
				if q.Items == nil {
					sameAsResults(t, q.Waves[wi], versions[q.ID])
				}
				for _, it := range q.Items {
					sameAsResults(t, it.Waves[wi], versions[it.ID])
				}
			}
		}
	}
	wantJSON, wantCSV := series()
	matchesResults(wantJSON)
	for _, id := range []string{"spring", "autumn"} {
		if _, err = st.pool.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if j, c := series(); j != wantJSON || c != wantCSV {
			t.Fatalf("after %s's rows are deleted:\n%s\nwant\n%s", id, j, wantJSON)
		}
		matchesResults(wantJSON)
	}
}

func TestShippedSeriesPublished(t *testing.T) {
	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		t.Fatal(err)
	}
	read := func(context.Context, string) ([]storedAnswer, error) {
		r := row(`{}`, `{}`)
		r.Client, r.Context = "web", map[string]any{"client": "web"}
		return []storedAnswer{r, r, r, r, r}, nil
	}
	api := &surveyAPI{surveys: defs, answers: read, now: func() time.Time { return time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC) }}
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/surveys/published/compare?series=utviklerundersokelsen", webToken, "")
	var got compareResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(got.Surveys) != 2 || got.Surveys[0].ID != "q2-2026" || got.Surveys[1].ID != "q4-2026" {
		t.Fatalf("waves %+v", got.Surveys)
	}
	for _, s := range got.Surveys {
		if s.Channels != nil {
			t.Errorf("channels published: %+v", s)
		}
	}
	if strings.Contains(rec.Body.String(), `"excluded"`) {
		t.Error("excluded published")
	}
}
