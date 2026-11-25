package api

import (
	"testing"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// The admin CSV keeps the model's codes back until the person has coded
// the text, as the coding page does.
func TestExportKeepsModelBlind(t *testing.T) {
	api := adminAPI(t, []survey.StoredAnswer{
		row(`{"comment":"rask"}`, `{"comment":1}`),
		row(`{"comment":"treg"}`, `{"comment":1}`),
	})
	f := withFakeCodes(api)
	f.codes[survey.HiddenKey("comment", survey.TextHash("rask"))] = &survey.TextCodes{Model: []survey.Code{{Category: "b", Onske: false}}}
	f.codes[survey.HiddenKey("comment", survey.TextHash("treg"))] = &survey.TextCodes{Model: []survey.Code{{Category: "c", Onske: false}}, Human: []survey.Code{{Category: "d", Onske: false}}}
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys/q4-2026/export.csv", adminToken, "")
	rows, err := survey.ReadExportCSV(api.surveys[0], rec.Body)
	if rec.Code != 200 || err != nil {
		t.Fatalf("%d %v", rec.Code, err)
	}
	for _, r := range rows {
		got, text := r.Codes["comment"], r.Answers["comment"]
		if text == "rask" && got.Model != nil || text == "treg" && got.Model == nil {
			t.Errorf("%s: model %v", text, got.Model)
		}
	}
}
