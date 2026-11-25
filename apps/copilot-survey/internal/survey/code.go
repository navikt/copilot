package survey

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

// RunCode is `copilot-survey code --survey <id> --model <model> export.csv`:
// codes each distinct masked text of the export (the admin's export.csv,
// hidden texts already left out) with a model, one text per request, and
// writes the codes file POST .../freetext/coding/model takes. Texts are
// keyed by sha256 of the masked text, so the file holds no text.
//
// The model is reached through an OpenAI-compatible chat completions
// endpoint, by default GitHub Models with GITHUB_TOKEN (e.g. `gh auth
// token`). Nothing is printed about a text but its field and number, and no
// value the model or the endpoint chose.
func RunCode(args []string, out, errOut io.Writer, token string) error {
	fs := flag.NewFlagSet("code", flag.ContinueOnError)
	id := fs.String("survey", "", "survey id, e.g. q4-2026")
	model := fs.String("model", "", "model id at the endpoint, e.g. openai/gpt-4.1")
	endpoint := fs.String("endpoint", "https://models.github.ai/inference/chat/completions", "OpenAI-compatible chat completions URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || *model == "" || fs.NArg() != 1 {
		return errors.New("usage: copilot-survey code --survey <id> --model <model> [--endpoint <url>] <export.csv>")
	}
	if err := checkEndpoint(*endpoint); err != nil {
		return err
	}
	if token == "" {
		return errors.New("GITHUB_TOKEN is not set")
	}
	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(defs, func(s survey) bool { return s.ID == *id })
	if i < 0 {
		return fmt.Errorf("no survey %q", *id)
	}
	s := defs[i]
	cbs, err := loadCodebooks(surveyFiles)
	if err != nil {
		return err
	}
	cb, ok := cbs[s.Series]
	if !ok || s.Series == "" {
		return fmt.Errorf("%s has no codebook", s.ID)
	}
	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	rows, err := readExportCSV(s, f)
	_ = f.Close()
	if err != nil {
		return err
	}
	c := &modelCoder{client: &http.Client{Timeout: 2 * time.Minute}, endpoint: *endpoint, model: *model, token: token}
	file := modelCodes{Survey: s.ID, Model: *model, CodebookVersion: cb.Version, Codes: []maskedCode{}}
	for _, ff := range freetextFields(s) {
		field, ok := cb.field(ff.ID)
		if !ok {
			continue
		}
		system := codingPrompt(cb, field, ff)
		seen := map[string]bool{}
		failed, n := 0, 0
		for _, row := range rows {
			text, _ := row.Answers[field.ID].(string)
			// Masked as the export masks it, whatever the CSV came from;
			// redact leaves masked text as it is, so the hash still matches.
			text = redact(text)
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true
			n++
			codes, err := c.code(context.Background(), field, system, text)
			if err != nil {
				failed++
				fmt.Fprintf(errOut, "%s text %d: %v\n", field.ID, n, err)
				continue
			}
			file.Codes = append(file.Codes, maskedCode{Field: field.ID, Text: textHash(text), Codes: codes})
		}
		fmt.Fprintf(errOut, "%s: %d texts, %d coded\n", field.ID, n, n-failed)
	}
	return encodeJSON(out, file)
}

// checkEndpoint refuses to send the token anywhere but HTTPS, or HTTP on
// this machine.
func checkEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("--endpoint %q is not a URL", raw)
	}
	if u.Scheme != "https" && (u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1")) {
		return errors.New("--endpoint must be https, or http on localhost")
	}
	return nil
}

// codingPrompt is the instruction for one field: the rules and the field's
// categories, as the person sees them in admin.
func codingPrompt(cb codebook, f codebookField, ff freetextField) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Du koder fritekstsvar fra en spørreundersøkelse etter en kodebok (versjon %d). Hver melding fra brukeren er ett svar, maskert. Svar bare med JSON: {\"codes\":[{\"category\":\"<id>\",\"onske\":true|false}]}.\n\n", cb.Version)
	fmt.Fprintf(&b, "Spørsmål: %s\n", ff.Question)
	if ff.Other != "" {
		fmt.Fprintf(&b, "Svaret er teksten til alternativet «%s».\n", ff.Other)
	}
	fmt.Fprintf(&b, "Felt: %s. Høyst %d koder. onske: %s.\n\nRegler:\n", f.ID, f.MaxCodes, f.Onske)
	for _, r := range cb.Rules {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	b.WriteString("\nKategorier (bruk bare disse id-ene):\n")
	for _, id := range f.Categories {
		c := cb.category(id)
		fmt.Fprintf(&b, "\n## %s (%s)\n%s\n", c.ID, c.Name, c.Definition)
		for _, l := range []struct {
			head  string
			items []string
		}{{"Ta med", c.Include}, {"Hold utenfor", c.Exclude}, {"Eksempler", c.Examples}} {
			if len(l.items) > 0 {
				fmt.Fprintf(&b, "%s:\n- %s\n", l.head, strings.Join(l.items, "\n- "))
			}
		}
	}
	return b.String()
}

type modelCoder struct {
	client                 *http.Client
	endpoint, model, token string
}

// code asks the model for one text's codes, twice if the first answer does
// not pass checkCodes.
func (c *modelCoder) code(ctx context.Context, f codebookField, system, text string) ([]code, error) {
	var last error
	for range 2 {
		raw, err := c.complete(ctx, system, text)
		if err != nil {
			return nil, err
		}
		var ans struct {
			Codes []code `json:"codes"`
		}
		if err = json.Unmarshal([]byte(raw), &ans); err != nil {
			last = errors.New("answer is not the JSON asked for")
			continue
		}
		// Not checkCodes' error: it quotes the model's category, and the
		// model may echo the text there.
		if codes, err := checkCodes(f, ans.Codes); err == nil {
			return codes, nil
		}
		last = errors.New("answer breaks the codebook")
	}
	return nil, last
}

func (c *modelCoder) complete(ctx context.Context, system, text string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":           c.model,
		"temperature":     0,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": text},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", errors.New("model endpoint unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// The body may quote the text back: only the status.
		return "", fmt.Errorf("model endpoint: status %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil || len(out.Choices) == 0 {
		return "", errors.New("model endpoint: unreadable answer")
	}
	return out.Choices[0].Message.Content, nil
}
