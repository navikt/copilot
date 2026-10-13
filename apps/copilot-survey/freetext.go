package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
)

// Free text in admin (#1792). Each text stands alone: it comes without the
// other answers of the person who wrote it, without a row id and in random
// order, masked as in the export. Names are not masked.
//
// Hiding: each text carries a token, HMAC-SHA256 of survey, field and the
// raw text under textKey, a random key held in memory only. The token names
// a text, not a row: it cannot be computed without the key, gives no way to
// look up the row or the person's other answers, and dies with the process.
// POST .../freetext/hide recomputes the tokens of the current texts to find
// the one meant and stores sha256(raw text) for that survey and field in
// survey_freetext_hidden. Identical texts in the same field are hidden
// together. Hidden texts leave admin and the export; the answer itself stays
// in survey_answers until it is deleted.

type freetextField struct {
	ID       string         `json:"id"` // question id, or id + ".other"
	Question string         `json:"question"`
	Other    string         `json:"other,omitempty"` // the other option's label
	Texts    []freetextText `json:"texts"`
}

type freetextText struct {
	Token string `json:"token"`
	Text  string `json:"text"`
}

// freetextFields are a survey's free-text answer keys, in question order.
func freetextFields(s survey) []freetextField {
	var out []freetextField
	for _, q := range s.Questions {
		switch {
		case q.Type == "text":
			out = append(out, freetextField{ID: q.ID, Question: q.Text})
		case q.Other != "":
			out = append(out, freetextField{ID: otherKey(q.ID), Question: q.Text, Other: q.Other})
		}
	}
	return out
}

func textHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func hiddenKey(field, hash string) string { return field + "\x00" + hash }

func (a *surveyAPI) textToken(surveyID, field, raw string) string {
	m := hmac.New(sha256.New, a.textKey)
	m.Write([]byte(surveyID + "\x00" + field + "\x00" + raw))
	return hex.EncodeToString(m.Sum(nil))
}

// freetextRows reads a survey's rows and hidden texts, or writes the error.
func (a *surveyAPI) freetextRows(w http.ResponseWriter, r *http.Request) (survey, []storedAnswer, map[string]bool, bool) {
	i := slices.IndexFunc(a.surveys, func(s survey) bool { return s.ID == r.PathValue("id") })
	if i < 0 {
		writeError(w, http.StatusNotFound, "unknown survey")
		return survey{}, nil, nil, false
	}
	s := a.surveys[i]
	rows, err := a.read(r, s.ID)
	var hidden map[string]bool
	if err == nil && (a.hidden == nil || a.hide == nil || len(a.textKey) == 0) {
		err = errNoStorage
	}
	if err == nil {
		hidden, err = a.hidden(r.Context(), s.ID)
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return survey{}, nil, nil, false
	}
	return s, rows, hidden, true
}

func (a *surveyAPI) freetext(w http.ResponseWriter, r *http.Request) {
	s, rows, hidden, ok := a.freetextRows(w, r)
	if !ok {
		return
	}
	fields := freetextFields(s)
	n := 0
	for i := range fields {
		f := &fields[i]
		f.Texts = []freetextText{}
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			if raw == "" || hidden[hiddenKey(f.ID, textHash(raw))] {
				continue
			}
			f.Texts = append(f.Texts, freetextText{a.textToken(s.ID, f.ID, raw), redact(raw)})
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
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.Token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}
	s, rows, _, ok := a.freetextRows(w, r)
	if !ok {
		return
	}
	for _, f := range freetextFields(s) {
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			if raw == "" || !hmac.Equal([]byte(a.textToken(s.ID, f.ID, raw)), []byte(body.Token)) {
				continue
			}
			if err := a.hide(r.Context(), s.ID, f.ID, textHash(raw)); err != nil {
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

// hidden returns a survey's hidden texts as hiddenKey(field, hash).
func (s *surveyStore) hidden(ctx context.Context, surveyID string) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT field, text_hash FROM survey_freetext_hidden WHERE survey_id = $1`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var f, h string
		if err := rows.Scan(&f, &h); err != nil {
			return nil, err
		}
		out[hiddenKey(f, h)] = true
	}
	return out, rows.Err()
}

func (s *surveyStore) hide(ctx context.Context, surveyID, field, hash string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO survey_freetext_hidden (survey_id, field, text_hash) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, surveyID, field, hash)
	return err
}
