package api

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// TestAnalyzeCodingMatchesPublished: `analyze --csv` over the admin CSVs of
// two waves gives the same coded free-text rows as the published series.
func TestAnalyzeCodingMatchesPublished(t *testing.T) {
	defs, _ := survey.LoadSurveyDir(survey.SurveyFiles)
	var waves []survey.Survey
	for _, id := range []string{"q2-2026", "q4-2026"} {
		s := defs[slices.IndexFunc(defs, func(s survey.Survey) bool { return s.ID == id })]
		s.Published, s.Active = true, false
		waves = append(waves, s)
	}
	data, codes := map[string][]survey.StoredAnswer{}, map[string]map[string]*survey.TextCodes{}
	for wi, s := range waves {
		codes[s.ID] = map[string]*survey.TextCodes{}
		for i := range 12 {
			text := []string{"Syntetisk rask", "Syntetisk treg", "Syntetisk annet"}[(i+wi)%3]
			data[s.ID] = append(data[s.ID], row(`{"opplevelse":"`+text+`"}`, `{}`))
		}
		codes[s.ID][survey.HiddenKey("opplevelse", survey.TextHash("Syntetisk rask"))] = &survey.TextCodes{Model: []survey.Code{{Category: "raskere", Onske: false}}, Human: []survey.Code{{Category: "raskere", Onske: false}}}
		codes[s.ID][survey.HiddenKey("opplevelse", survey.TextHash("Syntetisk treg"))] = &survey.TextCodes{Model: []survey.Code{{Category: "friksjon", Onske: true}}, Human: []survey.Code{{Category: "friksjon", Onske: false}}, Final: []survey.Code{{Category: "friksjon", Onske: true}}}
		codes[s.ID][survey.HiddenKey("opplevelse", survey.TextHash("Syntetisk annet"))] = &survey.TextCodes{Model: []survey.Code{{Category: "annet", Onske: false}}, Human: []survey.Code{{Category: "annet", Onske: false}}}
	}
	cbs, _ := survey.LoadCodebooks(survey.SurveyFiles)
	api := &surveyAPI{
		surveys:   waves,
		now:       func() time.Time { return time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) },
		textKey:   []byte("k"),
		codebooks: cbs,
		store:     &funcStore{answers: func(_ context.Context, id string) ([]survey.StoredAnswer, error) { return data[id], nil }, hidden: func(context.Context, string) (map[string]bool, error) { return map[string]bool{}, nil }, hide: func(context.Context, string, string, string) error { return nil }, codes: func(_ context.Context, id string, _ int) (map[string]*survey.TextCodes, error) { return codes[id], nil }}}
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
	if err := survey.RunAnalyze(args, &out, api.now()); err != nil {
		t.Fatal(err)
	}
	if online == "" || !strings.Contains(online, "freetext,q4-2026,opplevelse,,1,false,raskere,") {
		t.Fatalf("control: no coded rows online:\n%s", online)
	}
	if local := freetextRows(out.String()); local != online {
		t.Errorf("analyze differs\nonline %s\nlocal  %s", online, local)
	}
}
