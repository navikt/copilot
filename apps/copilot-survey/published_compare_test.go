package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	// Spring "fast": excluded 2, n 4. /results counts all 6, so the 2 would
	// show as the difference: the wave and the test are hidden. Admin shows
	// them (TestCompare).
	fast := got.Questions[1].Items[0]
	if s := fast.Waves[0]; !s.Hidden || s.Excluded != 0 || !s.Versions[0].Hidden || s.Versions[0].N != nil || fast.P != nil || fast.PHolm != nil {
		t.Errorf("small excluded group shown: %+v %+v", s, fast.significance)
	}
	if a := fast.Waves[1]; a.Hidden || *a.Versions[0].N != 5 {
		t.Errorf("autumn hidden: %+v", a)
	}
	if s := got.Questions[0].Waves[0]; s.Hidden || *s.Versions[0].N != 6 { // tools: nothing excluded
		t.Errorf("tools hidden: %+v", s)
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
		strings.Contains(rec.Body.String(), "\nchannel,") || !strings.Contains(rec.Body.String(), "\nanswer,spring,grid,fast,1,false,,,hidden,hidden,,,,\n") || !strings.Contains(rec.Body.String(), "\nanswer,autumn,") {
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

// publishedFast is the published comparison's "fast" item for spring rows
// built as groups of {count, tools answer, fast version}; autumn has 5 users.
func publishedFast(t *testing.T, groups ...[3]any) compareItem {
	t.Helper()
	data := waveRows()
	data["spring"] = nil
	for _, g := range groups {
		for range g[0].(int) {
			data["spring"] = append(data["spring"], row(`{"tools":["`+g[1].(string)+`"],"fast":5,"slow":3}`, fmt.Sprintf(`{"tools":1,"fast":%d,"slow":1}`, g[2].(int))))
		}
	}
	api := compareAPI(t, func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil })
	api.now = func() time.Time { return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC) }
	for i := range api.surveys {
		api.surveys[i].Series, api.surveys[i].Published = "dev", true
	}
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/surveys/published/compare?series=dev", webToken, "")
	var got compareResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return got.Questions[1].Items[0]
}

func TestPublishedCompareSmallGroupsPerVersion(t *testing.T) {
	hidden := func(w waveCounts) bool {
		for _, v := range w.Versions {
			if !v.Hidden || v.N != nil || v.Options != nil || v.Mean != nil || v.Agree != nil {
				return false
			}
		}
		return w.Hidden && w.Excluded == 0
	}
	// 20 answers, total excluded 5: 1 excluded in version 1 and 4 in version 2.
	fast := publishedFast(t, [3]any{7, "a", 1}, [3]any{1, "none", 1}, [3]any{8, "a", 2}, [3]any{4, "none", 2})
	if s := fast.Waves[0]; len(s.Versions) != 2 || !hidden(s) || fast.P != nil {
		t.Errorf("per-version excluded shown: %+v", s)
	}
	// 6 answers, 5 excluded: the one kept answer is a group of one.
	fast = publishedFast(t, [3]any{1, "a", 1}, [3]any{5, "none", 1})
	if s := fast.Waves[0]; len(s.Versions) != 1 || !hidden(s) || fast.P != nil {
		t.Errorf("kept n=1 shown: %+v", s)
	}
	// Control: 6 kept in version 1 and 5 excluded in version 2 hide nothing.
	fast = publishedFast(t, [3]any{6, "a", 1}, [3]any{5, "none", 2})
	if s := fast.Waves[0]; s.Hidden || s.Excluded != 5 || *s.Versions[0].N != 6 || s.Versions[0].Mean == nil {
		t.Errorf("control hidden: %+v", s)
	}
	if a := fast.Waves[1]; a.Hidden || *a.Versions[0].N != 5 {
		t.Errorf("autumn hidden: %+v", a)
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
