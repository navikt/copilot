package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestRunCode: one request per distinct masked text with the field's
// categories in the instruction, invalid answers retried, nothing of the
// text on stderr, and a file the import takes.
func TestRunCode(t *testing.T) {
	defs, _ := loadSurveyDir(surveyFiles)
	s := defs[slices.IndexFunc(defs, func(s survey) bool { return s.ID == "q4-2026" })]
	syn := "Syntetisk svar, skriv til ola@nav.no"
	rows := []storedAnswer{
		row(`{"opplevelse":"`+syn+`"}`, `{}`),
		row(`{"opplevelse":"`+syn+`"}`, `{}`),
		row(`{"opplevelse":"Annet syntetisk svar"}`, `{}`),
		row(`{"verktoy":["Annet"],"verktoy.other":"Syntetisk verktøy"}`, `{}`),
	}
	var csv bytes.Buffer
	writeExportCSV(&csv, s, toExportRows(rows))
	path := filepath.Join(t.TempDir(), "export.csv")
	if err := os.WriteFile(path, csv.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	var seen []string
	attempts := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string `json:"model"`
			Messages []struct{ Role, Content string }
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if r.Header.Get("Authorization") != "Bearer tok" || req.Model != "m" || len(req.Messages) != 2 {
			http.Error(w, "bad request", 400)
			return
		}
		sys, text := req.Messages[0].Content, req.Messages[1].Content
		seen = append(seen, text)
		attempts[text]++
		answer := `{"codes":[{"category":"raskere","onske":false}]}`
		switch {
		case strings.Contains(sys, "Felt: verktoy.other"):
			if !strings.Contains(sys, "## chat-assistent") || strings.Contains(sys, "## raskere") {
				t.Errorf("verktoy.other instruction lacks its own categories")
			}
			answer = `{"codes":[{"category":"chat-assistent","onske":true}]}`
		case strings.HasPrefix(text, "Annet") && attempts[text] == 1:
			answer = `{"codes":[{"category":"prototyping","onske":false}]}` // not in opplevelse
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	if err := runCode([]string{"--survey", "q4-2026", "--model", "m", "--endpoint", srv.URL, path}, &out, &errOut, "tok"); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	if len(seen) != 4 || slices.Contains(seen, syn) || !slices.Contains(seen, "Syntetisk svar, skriv til [e-post]") {
		t.Errorf("requests %q", seen)
	}
	if strings.Contains(errOut.String(), "Syntetisk") || !strings.Contains(errOut.String(), "opplevelse: 2 texts, 2 coded") {
		t.Errorf("stderr %s", errOut.String())
	}
	var file modelCodes
	if err := json.Unmarshal(out.Bytes(), &file); err != nil || file.CodebookVersion != 1 || len(file.Codes) != 3 || strings.Contains(out.String(), "Syntetisk") {
		t.Fatalf("%v %s", err, out.String())
	}
	for _, c := range file.Codes {
		if c.Field == "verktoy.other" && (c.Text != textHash("Syntetisk verktøy") || c.Codes[0].Onske) {
			t.Errorf("verktoy.other %+v", c)
		}
	}
}

func TestRunCodeRefuses(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		token string
		want  string
	}{
		{[]string{"--survey", "q4-2026", "--model", "m", "x.csv"}, "", "GITHUB_TOKEN"},
		{[]string{"--survey", "q4-2026", "--model", "m", "--endpoint", "http://example.com/v1", "x.csv"}, "tok", "https"},
		{[]string{"--survey", "dev-e2e-test", "--model", "m", "x.csv"}, "tok", "no codebook"},
		{[]string{"--survey", "q4-2026", "x.csv"}, "tok", "usage"},
	} {
		err := runCode(tc.args, &bytes.Buffer{}, &bytes.Buffer{}, tc.token)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: %v, want %q", tc.args, err, tc.want)
		}
	}
	if checkEndpoint("http://127.0.0.1:8080/v1/chat/completions") != nil {
		t.Error("local http refused")
	}
}

// analyze adds the coding results when the CSV carries codes.
func TestAnalyzeReadsCodes(t *testing.T) {
	defs, _ := loadSurveyDir(surveyFiles)
	s := defs[slices.IndexFunc(defs, func(s survey) bool { return s.ID == "q4-2026" })]
	var rows []exportRow
	for range 5 {
		r := toExportRows([]storedAnswer{row(`{"opplevelse":"Syntetisk"}`, `{}`)})[0]
		r.Codes = map[string]textCodes{"opplevelse": {Model: []code{{"raskere", false}}, Human: []code{{"raskere", false}}}}
		rows = append(rows, r)
	}
	var csv bytes.Buffer
	writeExportCSV(&csv, s, rows)
	path := filepath.Join(t.TempDir(), "export.csv")
	if err := os.WriteFile(path, csv.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runAnalyze([]string{"q4-2026=" + path}, &out, s.closesOn()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"freetext"`) || !strings.Contains(out.String(), `"id": "raskere"`) {
		t.Errorf("no coding results:\n%.400s", out.String())
	}
}

// The model chooses the category, and may echo the text in it: stderr
// carries fixed messages only. A CSV that was never masked is masked
// before anything leaves, and hashed as the import hashes.
func TestRunCodeLeaksNothing(t *testing.T) {
	defs, _ := loadSurveyDir(surveyFiles)
	s := defs[slices.IndexFunc(defs, func(s survey) bool { return s.ID == "q4-2026" })]
	raw := "Syntetisk ekko, ring 22225555 eller ola@nav.no"
	var csv bytes.Buffer
	writeExportCSV(&csv, s, []exportRow{{Answers: map[string]any{"opplevelse": raw}, Context: map[string]string{}}}) // unmasked
	if !strings.Contains(csv.String(), "ola@nav.no") {
		t.Fatal("control: the CSV should hold the raw text")
	}
	path := filepath.Join(t.TempDir(), "export.csv")
	if err := os.WriteFile(path, csv.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Messages []struct{ Content string } }
		_ = json.NewDecoder(r.Body).Decode(&req)
		text := req.Messages[1].Content
		sent = append(sent, text)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{
			"content": `{"codes":[{"category":` + strconvQuote(text) + `,"onske":false}]}`}}}})
	}))
	defer srv.Close()
	var out, errOut bytes.Buffer
	if err := runCode([]string{"--survey", "q4-2026", "--model", "m", "--endpoint", srv.URL, path}, &out, &errOut, "tok"); err != nil {
		t.Fatal(err)
	}
	masked := redact(raw)
	if len(sent) != 2 || sent[0] != masked || strings.Contains(sent[0], "ola@nav.no") {
		t.Errorf("sent %q", sent)
	}
	if strings.Contains(errOut.String(), "Syntetisk") || !strings.Contains(errOut.String(), "opplevelse text 1: answer breaks the codebook") {
		t.Errorf("stderr %s", errOut.String())
	}
	if redact(masked) != masked {
		t.Error("redact is not idempotent: hashes of masked text would not match the import")
	}
}

// An error status shows its code only, never a reason the endpoint chose.
func TestModelEndpointErrorIsFixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(422)
		_, _ = w.Write([]byte("Syntetisk ekko"))
	}))
	defer srv.Close()
	c := &modelCoder{client: srv.Client(), endpoint: srv.URL, model: "m", token: "tok"}
	_, err := c.code(t.Context(), codebookField{}, "", "Syntetisk ekko")
	if err == nil || err.Error() != "model endpoint: status 422" {
		t.Errorf("%v", err)
	}
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

// The admin CSV keeps the model's codes back until the person has coded
// the text, as the coding page does.
func TestExportKeepsModelBlind(t *testing.T) {
	api := adminAPI(t, []storedAnswer{
		row(`{"comment":"rask"}`, `{"comment":1}`),
		row(`{"comment":"treg"}`, `{"comment":1}`),
	})
	f := withFakeCodes(api)
	f.codes[hiddenKey("comment", textHash("rask"))] = &textCodes{Model: []code{{"b", false}}}
	f.codes[hiddenKey("comment", textHash("treg"))] = &textCodes{Model: []code{{"c", false}}, Human: []code{{"d", false}}}
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys/q4-2026/export.csv", adminToken, "")
	rows, err := readExportCSV(api.surveys[0], rec.Body)
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
