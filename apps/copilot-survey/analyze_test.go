package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func toExportRows(rows []storedAnswer) []exportRow {
	out := make([]exportRow, len(rows))
	for i, r := range rows {
		ctx := map[string]any{"client": r.Client}
		out[i] = newExportRow(cloneAnswers(r.Answers), r.QuestionVersions, ctx, func(string, string) bool { return false })
	}
	return out
}

func cloneAnswers(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var c map[string]any
	_ = json.Unmarshal(b, &c)
	return c
}

func roundTrip(t *testing.T, s survey, rows []storedAnswer) []storedAnswer {
	t.Helper()
	var buf bytes.Buffer
	writeExportCSV(&buf, s, toExportRows(rows))
	got, err := readExportCSV(s, &buf)
	if err != nil {
		t.Fatalf("%v\n%s", err, buf.String())
	}
	return got
}

func sameJSON(t *testing.T, what string, want, got any) {
	t.Helper()
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if !bytes.Equal(a, b) {
		t.Errorf("%s differs\nonline %s\ncsv    %s", what, a, b)
	}
}

// The admin numbers from stored rows and from those rows exported to CSV and
// read back must be identical: «copilot-survey analyze» promises it.
func TestAnalyzeMatchesOnline(t *testing.T) {
	now := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	defs, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	s := defs[0]
	var rows []storedAnswer
	for i := range 6 {
		// fast answered as version 1 although the definition is at 2: the
		// export must write the version column for readExportCSV to see it.
		r := row(`{"overall":3,"client":"copilot","tools":["a","b"],"editor":["vim","Annet"],"editor.other":"=x","fast":4,"safe":2,"comment":"-hei"}`,
			`{"overall":1,"client":1,"tools":1,"editor":2,"fast":1,"safe":1,"comment":1}`)
		if i%2 == 0 {
			r = row(`{"overall":5,"client":"opencode","why":"other","tools":[],"comment":"x"}`, `{"overall":1,"client":1,"why":1,"tools":1,"comment":1}`)
		}
		r.Client = "web"
		rows = append(rows, r)
	}
	rows = append(rows, row(`{}`, `{}`))
	// Values the export must not lose or alter: a pick no longer an option,
	// and a choice that starts with an apostrophe.
	for range 2 {
		rows = append(rows, row(`{"tools":["old"],"client":"'=x"}`, `{"tools":1,"client":1}`))
	}
	got := roundTrip(t, s, rows)
	want, have := aggregate(s, rows, now), aggregate(s, got, now)
	sameJSON(t, "results", want.Questions, have.Questions)
	sameJSON(t, "responses", want.Responses, have.Responses)

	cd, _ := loadSurveys([]byte(compareSurveys))
	data := waveRows()
	online := compareWavesResult(cd, [][]storedAnswer{mergeClients(data["spring"]), mergeClients(data["autumn"])}, now)
	local := compareWavesResult(cd, [][]storedAnswer{roundTrip(t, cd[0], data["spring"]), roundTrip(t, cd[1], data["autumn"])}, now)
	sameJSON(t, "comparison", online.Questions, local.Questions)
	var a, b bytes.Buffer
	if writeCompareCSV(&a, online) != nil || writeCompareCSV(&b, local) != nil {
		t.Fatal("write")
	}
	// Byte for byte, channels too: autumn's web 3 and copilot 2 are «annet» in both.
	if a.String() != b.String() {
		t.Errorf("comparison CSV differs\n%s\n%s", a.String(), b.String())
	}
	if !strings.Contains(a.String(), "answer,spring,grid,fast,1,false,5,2,4,2,4,0.75") || !strings.Contains(a.String(), "channel,autumn,,,,,annet,5") {
		t.Errorf("comparison CSV:\n%s", a.String())
	}
}

func TestReadExportCSVRefusesOtherColumns(t *testing.T) {
	defs, _ := loadSurveys([]byte(compareSurveys))
	var buf bytes.Buffer
	writeExportCSV(&buf, defs[0], nil)
	if _, err := readExportCSV(defs[1], strings.NewReader(strings.Replace(buf.String(), "grid.fast", "grid.quick", 1))); err == nil {
		t.Error("renamed column accepted")
	}
	if _, err := readExportCSV(defs[0], strings.NewReader(buf.String()+strings.Repeat(",", strings.Count(strings.SplitN(buf.String(), "\n", 2)[0], ","))+"\n")); err != nil {
		t.Errorf("empty row: %v", err)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCompareCSVEdges(t *testing.T) {
	res := compareResult{
		Surveys:   []waveMeta{{surveySummary: surveySummary{ID: "spring"}}},
		Questions: []compareQuestion{{ID: "q", Waves: []waveCounts{{Excluded: 6, Versions: []waveStat{}}}}},
	}
	var buf bytes.Buffer
	if err := writeCompareCSV(&buf, res); err != nil || !strings.Contains(buf.String(), "answer,spring,q,,,false,,,,6,,") {
		t.Errorf("all excluded lost: %v\n%s", err, buf.String())
	}
	five := 5
	res.Questions[0].Waves[0] = waveCounts{Versions: []waveStat{{versionResult: versionResult{Version: 1, N: &five, Options: []optionCount{}}}}}
	buf.Reset()
	if _ = writeCompareCSV(&buf, res); !strings.Contains(buf.String(), "answer,spring,q,,1,false,,,5,0,,") {
		t.Errorf("version without options lost:\n%s", buf.String())
	}
	if writeCompareCSV(failWriter{}, res) == nil {
		t.Error("write error swallowed")
	}
}
