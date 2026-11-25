package api

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// surveyCodebook is s's codebook, or 404.
func (a *surveyAPI) surveyCodebook(w http.ResponseWriter, s survey.Survey) (survey.Codebook, bool) {
	cb, ok := a.codebooks[s.Series]
	if !ok || s.Series == "" {
		writeError(w, http.StatusNotFound, "no codebook for this survey")
	}
	return cb, ok && s.Series != ""
}

func (a *surveyAPI) codingTexts(w http.ResponseWriter, r *http.Request) (survey.Survey, survey.Codebook, []survey.StoredAnswer, map[string]bool, map[string]*survey.TextCodes, bool) {
	s, rows, hidden, ok := a.freetextRows(w, r)
	if !ok {
		return s, survey.Codebook{}, nil, nil, nil, false
	}
	cb, ok := a.surveyCodebook(w, s)
	if !ok {
		return s, cb, nil, nil, nil, false
	}
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return s, cb, nil, nil, nil, false
	}
	codes, err := a.store.Codes(r.Context(), s.ID, cb.Version)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return s, cb, nil, nil, nil, false
	}
	return s, cb, rows, hidden, codes, true
}

func (a *surveyAPI) coding(w http.ResponseWriter, r *http.Request) {
	s, cb, rows, hidden, codes, ok := a.codingTexts(w, r)
	if !ok {
		return
	}
	type fieldInfo struct {
		survey.CodebookField
		Question   string                    `json:"question"`
		Other      string                    `json:"other,omitempty"`
		Categories []survey.CodebookCategory `json:"categories"`
	}
	var fields []fieldInfo
	var progress []codingProgress
	var todo []codingText
	disagreements := []codingText{}
	for _, ff := range survey.FreetextFields(s) {
		f, ok := cb.Field(ff.ID)
		if !ok {
			continue
		}
		info := fieldInfo{CodebookField: f, Question: ff.Question, Other: ff.Other}
		for _, c := range f.Categories {
			info.Categories = append(info.Categories, cb.Category(c))
		}
		fields = append(fields, info)
		p := codingProgress{Field: f.ID}
		seen := map[string]bool{}
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			key := survey.HiddenKey(f.ID, survey.TextHash(raw))
			if raw == "" || hidden[key] || seen[key] {
				continue
			}
			seen[key] = true
			p.Texts++
			tc := survey.TextCodes{}
			if c := codes[key]; c != nil {
				tc = *c
			}
			t := codingText{Field: f.ID, Token: a.textToken(s.ID, f.ID, raw), Text: survey.Redact(raw)}
			if tc.Human == nil {
				todo = append(todo, t)
			} else {
				p.Human++
			}
			if tc.Model != nil {
				p.Model++
			}
			if tc.FinalCodes() != nil {
				p.Final++
			}
			if tc.Disagree() {
				p.Disagreed++
				t.Human, t.Model = tc.Human, tc.Model
				disagreements = append(disagreements, t)
			}
		}
		progress = append(progress, p)
	}
	var next *codingText
	if len(todo) > 0 {
		next = &todo[rand.IntN(len(todo))] // #nosec G404 -- which text comes next, not a secret
	}
	tallies := survey.TallyAll(cb, survey.RespondentCodes(cb, rows, hidden, codes))
	if len(rows) == 0 && a.store != nil {
		stored, err := a.store.FinalCoding(r.Context(), s.ID, cb.Version)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		if stored != nil {
			tallies = stored
		}
	}
	rand.Shuffle(len(disagreements), func(i, j int) { disagreements[i], disagreements[j] = disagreements[j], disagreements[i] })
	slog.Info("admin freetext coding read", "survey", s.ID, "disagreements", len(disagreements))
	writeJSON(w, map[string]any{
		"id": s.ID, "title": s.Title,
		"codebook":      map[string]any{"version": cb.Version, "rules": cb.Rules, "fields": fields},
		"progress":      progress,
		"next":          next,
		"disagreements": disagreements,
		"results":       survey.CodingResults(s, cb, tallies),
	})
}

// saveCoding is POST .../freetext/coding: {token, coder, codes}. coder is
// human (once per text, before the model's codes are shown) or final (a
// decision, once both have coded). The answer to human carries the model's
// codes, so the person sees them only after saving their own.
func (a *surveyAPI) saveCoding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string        `json:"token"`
		Coder string        `json:"coder"`
		Codes []survey.Code `json:"codes"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || !survey.AtEOF(dec) || body.Token == "" || (body.Coder != "human" && body.Coder != "final") {
		writeError(w, http.StatusBadRequest, "token, coder (human or final) and codes required")
		return
	}
	s, cb, rows, hidden, codes, ok := a.codingTexts(w, r)
	if !ok {
		return
	}
	for _, f := range cb.Fields {
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			hash := survey.TextHash(raw)
			if raw == "" || hidden[survey.HiddenKey(f.ID, hash)] || !hmac.Equal([]byte(a.textToken(s.ID, f.ID, raw)), []byte(body.Token)) {
				continue
			}
			checked, err := survey.CheckCodes(f, body.Codes)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			tc := survey.TextCodes{}
			if c := codes[survey.HiddenKey(f.ID, hash)]; c != nil {
				tc = *c
			}
			if body.Coder == "final" && (tc.Human == nil || tc.Model == nil) {
				writeError(w, http.StatusConflict, "both coders must code the text first")
				return
			}
			by := ""
			if c, ok := callerFromContext(r.Context()); ok && body.Coder == "final" {
				by = c.email
			}
			err = a.store.SaveCodes(r.Context(), s.ID, f.ID, hash, body.Coder, cb.Version, checked, by)
			if errors.Is(err, survey.ErrCoded) {
				writeError(w, http.StatusConflict, "already coded")
				return
			}
			if errors.Is(err, survey.ErrGone) {
				writeError(w, http.StatusNotFound, "unknown text")
				return
			}
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
				return
			}
			slog.Info("admin freetext coded", "survey", s.ID, "field", f.ID, "coder", body.Coder)
			writeJSON(w, map[string]any{"model": tc.Model})
			return
		}
	}
	writeError(w, http.StatusNotFound, "unknown text")
}

// importModelCodes is POST .../freetext/coding/model: the file `copilot-survey
// code` writes, {codebook_version, codes: [{field, text, codes}]} where text
// is sha256 of the masked text. Each visible text whose masked form matches
// gets the codes as the model's, replacing earlier ones.
func (a *surveyAPI) importModelCodes(w http.ResponseWriter, r *http.Request) {
	var body survey.ModelCodes
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || !survey.AtEOF(dec) {
		writeError(w, http.StatusBadRequest, "invalid codes file")
		return
	}
	s, cb, rows, hidden, _, ok := a.codingTexts(w, r)
	if !ok {
		return
	}
	if body.CodebookVersion != cb.Version {
		writeError(w, http.StatusConflict, fmt.Sprintf("coded with codebook version %d, the codebook is at %d", body.CodebookVersion, cb.Version))
		return
	}
	byMasked := map[string][]survey.Code{}
	for _, t := range body.Codes {
		f, ok := cb.Field(t.Field)
		checked, err := survey.CheckCodes(f, t.Codes)
		if !ok || err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid codes in %s", t.Field))
			return
		}
		byMasked[survey.HiddenKey(t.Field, t.Text)] = checked
	}
	saved, matched := 0, map[string]bool{}
	done := map[string]bool{}
	for _, f := range cb.Fields {
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			hash := survey.TextHash(raw)
			key, masked := survey.HiddenKey(f.ID, hash), survey.HiddenKey(f.ID, survey.TextHash(survey.Redact(raw)))
			cs, ok := byMasked[masked]
			if raw == "" || hidden[key] || done[key] || !ok {
				continue
			}
			done[key], matched[masked] = true, true
			err := a.store.SaveCodes(r.Context(), s.ID, f.ID, hash, "model", cb.Version, cs, "")
			if errors.Is(err, survey.ErrGone) {
				continue
			}
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
				return
			}
			saved++
		}
	}
	slog.Info("admin freetext model codes imported", "survey", s.ID, "texts", saved, "unmatched", len(byMasked)-len(matched))
	writeJSON(w, map[string]int{"saved": saved, "unmatched": len(byMasked) - len(matched)})
}

// publishedCoding is s's coding as the published series shows it: only
// fields where every text has final codes and at least MinSegment wrote,
// only published categories with «<5» applied. nil when none.
func (a *surveyAPI) publishedCoding(ctx context.Context, s survey.Survey) (*survey.WaveFreetext, error) {
	cb, ok := a.codebooks[s.Series]
	if !ok || s.Series == "" || a.store == nil {
		return nil, nil
	}
	t, err := a.codingTally(ctx, s, cb)
	if err != nil {
		return nil, err
	}
	return survey.PublishedFreetext(s, cb, t), nil
}

// codingTally is a survey's coding counts: live while it has rows, else
// as stored when they were deleted.
func (a *surveyAPI) codingTally(ctx context.Context, s survey.Survey, cb survey.Codebook) (map[string]survey.FieldTally, error) {
	if a.store == nil {
		return nil, errNoStorage
	}
	rows, err := a.store.Answers(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return a.store.FinalCoding(ctx, s.ID, cb.Version)
	}
	hidden, err := a.store.Hidden(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	codes, err := a.store.Codes(ctx, s.ID, cb.Version)
	if err != nil {
		return nil, err
	}
	return survey.TallyAll(cb, survey.RespondentCodes(cb, rows, hidden, codes)), nil
}

type codingText struct {
	Field string        `json:"field"`
	Token string        `json:"token"`
	Text  string        `json:"text"`
	Human []survey.Code `json:"human,omitempty"`
	Model []survey.Code `json:"model,omitempty"`
}

type codingProgress struct {
	Field     string `json:"field"`
	Texts     int    `json:"texts"` // distinct visible texts
	Human     int    `json:"human"`
	Model     int    `json:"model"`
	Final     int    `json:"final"`
	Disagreed int    `json:"disagreements"` // open
}
