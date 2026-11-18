package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestAnalyzeCodingMatchesPublished: `analyze --csv` over the admin CSVs of
// two waves gives the same coded free-text rows as the published series.
func TestAnalyzeCodingMatchesPublished(t *testing.T) {
	defs, _ := loadSurveyDir(surveyFiles)
	var waves []survey
	for _, id := range []string{"q2-2026", "q4-2026"} {
		s := defs[slices.IndexFunc(defs, func(s survey) bool { return s.ID == id })]
		s.Published, s.Active = true, false
		waves = append(waves, s)
	}
	data, codes := map[string][]storedAnswer{}, map[string]map[string]*textCodes{}
	for wi, s := range waves {
		codes[s.ID] = map[string]*textCodes{}
		for i := range 12 {
			text := []string{"Syntetisk rask", "Syntetisk treg", "Syntetisk annet"}[(i+wi)%3]
			data[s.ID] = append(data[s.ID], row(`{"opplevelse":"`+text+`"}`, `{}`))
		}
		codes[s.ID][hiddenKey("opplevelse", textHash("Syntetisk rask"))] = &textCodes{Model: []code{{"raskere", false}}, Human: []code{{"raskere", false}}}
		codes[s.ID][hiddenKey("opplevelse", textHash("Syntetisk treg"))] = &textCodes{Model: []code{{"friksjon", true}}, Human: []code{{"friksjon", false}}, Final: []code{{"friksjon", true}}}
		codes[s.ID][hiddenKey("opplevelse", textHash("Syntetisk annet"))] = &textCodes{Model: []code{{"annet", false}}, Human: []code{{"annet", false}}}
	}
	cbs, _ := loadCodebooks(surveyFiles)
	api := &surveyAPI{
		surveys:   waves,
		now:       func() time.Time { return time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) },
		answers:   func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil },
		hidden:    func(context.Context, string) (map[string]bool, error) { return map[string]bool{}, nil },
		hide:      func(context.Context, string, string, string) error { return nil },
		textKey:   []byte("k"),
		codes:     func(_ context.Context, id string, _ int) (map[string]*textCodes, error) { return codes[id], nil },
		codebooks: cbs,
	}
	h := makeRouter(testAuthenticator(t), api)
	freetextRows := func(csv string) string {
		var out []string
		for _, l := range strings.Split(csv, "\n") {
			if strings.HasPrefix(l, "freetext") {
				out = append(out, strings.TrimSpace(l))
			}
		}
		return strings.Join(out, "\n")
	}
	online := freetextRows(do(h, "GET", "/api/v1/surveys/published/compare?series=utviklerundersokelsen&format=csv", webToken, "").Body.String())
	args := []string{"--csv"}
	for _, s := range waves {
		rec := do(h, "GET", "/api/v1/admin/surveys/"+s.ID+"/export.csv", adminToken, "")
		path := filepath.Join(t.TempDir(), s.ID+".csv")
		if err := os.WriteFile(path, rec.Body.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, s.ID+"="+path)
	}
	var out bytes.Buffer
	if err := runAnalyze(args, &out, api.now()); err != nil {
		t.Fatal(err)
	}
	if online == "" || !strings.Contains(online, "freetext,q4-2026,opplevelse,,1,false,raskere,") {
		t.Fatalf("control: no coded rows online:\n%s", online)
	}
	if local := freetextRows(out.String()); local != online {
		t.Errorf("analyze differs\nonline %s\nlocal  %s", online, local)
	}
}
