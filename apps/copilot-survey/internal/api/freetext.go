package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

func (a *surveyAPI) textToken(surveyID, field, raw string) string {
	m := hmac.New(sha256.New, a.textKey)
	m.Write([]byte(surveyID + "\x00" + field + "\x00" + raw))
	return hex.EncodeToString(m.Sum(nil))
}

// freetextRows reads a survey's rows and hidden texts, or writes the error.
func (a *surveyAPI) freetextRows(w http.ResponseWriter, r *http.Request) (survey.Survey, []survey.StoredAnswer, map[string]bool, bool) {
	i := slices.IndexFunc(a.surveys, func(s survey.Survey) bool { return s.ID == r.PathValue("id") })
	if i < 0 {
		writeError(w, http.StatusNotFound, "unknown survey")
		return survey.Survey{}, nil, nil, false
	}
	s := a.surveys[i]
	rows, err := a.read(r, s.ID)
	var hidden map[string]bool
	if err == nil && (a.store == nil || len(a.textKey) == 0) {
		err = errNoStorage
	}
	if err == nil {
		hidden, err = a.store.Hidden(r.Context(), s.ID)
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return survey.Survey{}, nil, nil, false
	}
	return s, rows, hidden, true
}

func (a *surveyAPI) freetext(w http.ResponseWriter, r *http.Request) {
	s, rows, hidden, ok := a.freetextRows(w, r)
	if !ok {
		return
	}
	fields := survey.FreetextFields(s)
	n := 0
	for i := range fields {
		f := &fields[i]
		f.Texts = []survey.FreetextText{}
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			if raw == "" || hidden[survey.HiddenKey(f.ID, survey.TextHash(raw))] {
				continue
			}
			f.Texts = append(f.Texts, survey.FreetextText{Token: a.textToken(s.ID, f.ID, raw), Text: survey.Redact(raw)})
		}
		rand.Shuffle(len(f.Texts), func(i, j int) { f.Texts[i], f.Texts[j] = f.Texts[j], f.Texts[i] })
		n += len(f.Texts)
	}
	// Who read it is in my-copilot's access log; never the content here.
	slog.Info("admin freetext read", "survey", s.ID, "texts", n)
	writeJSON(w, map[string]any{"id": s.ID, "title": s.Title, "fields": fields})
}

func (a *surveyAPI) hideFreetext(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || !survey.AtEOF(dec) || body.Token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}
	s, rows, _, ok := a.freetextRows(w, r)
	if !ok {
		return
	}
	for _, f := range survey.FreetextFields(s) {
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			if raw == "" || !hmac.Equal([]byte(a.textToken(s.ID, f.ID, raw)), []byte(body.Token)) {
				continue
			}
			if err := a.store.Hide(r.Context(), s.ID, f.ID, survey.TextHash(raw)); err != nil {
				writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
				return
			}
			slog.Info("admin freetext hidden", "survey", s.ID, "field", f.ID)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeError(w, http.StatusNotFound, "unknown text")
}
