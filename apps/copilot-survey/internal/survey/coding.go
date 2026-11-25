package survey

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// Coding in admin (#2239): GET .../freetext/coding gives the codebook, the
// progress, one masked text the person has not coded yet, the texts where
// the coders disagree, and the results. POST .../freetext/coding saves the
// person's codes (once: blind to the model until then) or a decision.
// POST .../freetext/coding/model imports the model's codes, keyed by
// sha256 of the masked text as `copilot-survey code` writes them. Logs carry
// survey, field and counts, never a text.

var (
	errCoded = errors.New("already coded")
	errGone  = errors.New("text deleted")
)

// surveyCodebook is s's codebook, or 404.
func (a *surveyAPI) surveyCodebook(w http.ResponseWriter, s survey) (codebook, bool) {
	cb, ok := a.codebooks[s.Series]
	if !ok || s.Series == "" {
		writeError(w, http.StatusNotFound, "no codebook for this survey")
	}
	return cb, ok && s.Series != ""
}

func (a *surveyAPI) codingTexts(w http.ResponseWriter, r *http.Request) (survey, codebook, []storedAnswer, map[string]bool, map[string]*textCodes, bool) {
	s, rows, hidden, ok := a.freetextRows(w, r)
	if !ok {
		return s, codebook{}, nil, nil, nil, false
	}
	cb, ok := a.surveyCodebook(w, s)
	if !ok {
		return s, cb, nil, nil, nil, false
	}
	if a.codes == nil || a.saveCodes == nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return s, cb, nil, nil, nil, false
	}
	codes, err := a.codes(r.Context(), s.ID, cb.Version)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return s, cb, nil, nil, nil, false
	}
	return s, cb, rows, hidden, codes, true
}

// respondentCodes is, per codebook field, the codes of every respondent's
// visible text in it, one entry per respondent.
func respondentCodes(cb codebook, rows []storedAnswer, hidden map[string]bool, codes map[string]*textCodes) map[string][]textCodes {
	return codesPerField(cb, rows, func(_ storedAnswer, field, raw string) (textCodes, bool) {
		key := hiddenKey(field, textHash(raw))
		var tc textCodes
		if c := codes[key]; c != nil {
			tc = *c
		}
		return tc, !hidden[key]
	})
}

// csvCodes is respondentCodes for rows read back from the admin CSV, which
// carry their codes and no hidden text: what `analyze` counts.
func csvCodes(cb codebook, rows []storedAnswer) map[string][]textCodes {
	return codesPerField(cb, rows, func(row storedAnswer, field, _ string) (textCodes, bool) { return row.Codes[field], true })
}

func codesPerField(cb codebook, rows []storedAnswer, codesOf func(row storedAnswer, field, raw string) (textCodes, bool)) map[string][]textCodes {
	out := map[string][]textCodes{}
	for _, f := range cb.Fields {
		out[f.ID] = []textCodes{}
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			if raw == "" {
				continue
			}
			if tc, ok := codesOf(row, f.ID, raw); ok {
				out[f.ID] = append(out[f.ID], tc)
			}
		}
	}
	return out
}

func tallyAll(cb codebook, texts map[string][]textCodes) map[string]fieldTally {
	out := map[string]fieldTally{}
	for _, f := range cb.Fields {
		out[f.ID] = tallyCodes(cb, f, texts[f.ID])
	}
	return out
}

type codingText struct {
	Field string `json:"field"`
	Token string `json:"token"`
	Text  string `json:"text"`
	Human []code `json:"human,omitempty"`
	Model []code `json:"model,omitempty"`
}

type codingProgress struct {
	Field     string `json:"field"`
	Texts     int    `json:"texts"` // distinct visible texts
	Human     int    `json:"human"`
	Model     int    `json:"model"`
	Final     int    `json:"final"`
	Disagreed int    `json:"disagreements"` // open
}

func (a *surveyAPI) coding(w http.ResponseWriter, r *http.Request) {
	s, cb, rows, hidden, codes, ok := a.codingTexts(w, r)
	if !ok {
		return
	}
	type fieldInfo struct {
		codebookField
		Question   string             `json:"question"`
		Other      string             `json:"other,omitempty"`
		Categories []codebookCategory `json:"categories"`
	}
	var fields []fieldInfo
	var progress []codingProgress
	var todo []codingText
	disagreements := []codingText{}
	for _, ff := range freetextFields(s) {
		f, ok := cb.field(ff.ID)
		if !ok {
			continue
		}
		info := fieldInfo{codebookField: f, Question: ff.Question, Other: ff.Other}
		for _, c := range f.Categories {
			info.Categories = append(info.Categories, cb.category(c))
		}
		fields = append(fields, info)
		p := codingProgress{Field: f.ID}
		seen := map[string]bool{}
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			key := hiddenKey(f.ID, textHash(raw))
			if raw == "" || hidden[key] || seen[key] {
				continue
			}
			seen[key] = true
			p.Texts++
			tc := textCodes{}
			if c := codes[key]; c != nil {
				tc = *c
			}
			t := codingText{Field: f.ID, Token: a.textToken(s.ID, f.ID, raw), Text: redact(raw)}
			if tc.Human == nil {
				todo = append(todo, t)
			} else {
				p.Human++
			}
			if tc.Model != nil {
				p.Model++
			}
			if tc.final() != nil {
				p.Final++
			}
			if tc.disagree() {
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
	tallies := tallyAll(cb, respondentCodes(cb, rows, hidden, codes))
	if len(rows) == 0 && a.finalCoding != nil {
		stored, err := a.finalCoding(r.Context(), s.ID, cb.Version)
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
		"results":       codingResults(s, cb, tallies),
	})
}

// saveCoding is POST .../freetext/coding: {token, coder, codes}. coder is
// human (once per text, before the model's codes are shown) or final (a
// decision, once both have coded). The answer to human carries the model's
// codes, so the person sees them only after saving their own.
func (a *surveyAPI) saveCoding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
		Coder string `json:"coder"`
		Codes []code `json:"codes"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || !atEOF(dec) || body.Token == "" || (body.Coder != "human" && body.Coder != "final") {
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
			hash := textHash(raw)
			if raw == "" || hidden[hiddenKey(f.ID, hash)] || !hmac.Equal([]byte(a.textToken(s.ID, f.ID, raw)), []byte(body.Token)) {
				continue
			}
			checked, err := checkCodes(f, body.Codes)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			tc := textCodes{}
			if c := codes[hiddenKey(f.ID, hash)]; c != nil {
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
			err = a.saveCodes(r.Context(), s.ID, f.ID, hash, body.Coder, cb.Version, checked, by)
			if errors.Is(err, errCoded) {
				writeError(w, http.StatusConflict, "already coded")
				return
			}
			if errors.Is(err, errGone) {
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
	var body modelCodes
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || !atEOF(dec) {
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
	byMasked := map[string][]code{}
	for _, t := range body.Codes {
		f, ok := cb.field(t.Field)
		checked, err := checkCodes(f, t.Codes)
		if !ok || err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid codes in %s", t.Field))
			return
		}
		byMasked[hiddenKey(t.Field, t.Text)] = checked
	}
	saved, matched := 0, map[string]bool{}
	done := map[string]bool{}
	for _, f := range cb.Fields {
		for _, row := range rows {
			raw, _ := row.Answers[f.ID].(string)
			hash := textHash(raw)
			key, masked := hiddenKey(f.ID, hash), hiddenKey(f.ID, textHash(redact(raw)))
			cs, ok := byMasked[masked]
			if raw == "" || hidden[key] || done[key] || !ok {
				continue
			}
			done[key], matched[masked] = true, true
			err := a.saveCodes(r.Context(), s.ID, f.ID, hash, "model", cb.Version, cs, "")
			if errors.Is(err, errGone) {
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

// modelCodes is the file `copilot-survey code` writes.
type modelCodes struct {
	Survey          string       `json:"survey"`
	Model           string       `json:"model"`
	CodebookVersion int          `json:"codebook_version"`
	Codes           []maskedCode `json:"codes"`
}

type maskedCode struct {
	Field string `json:"field"`
	Text  string `json:"text"` // sha256 of the masked text, hex
	Codes []code `json:"codes"`
}

// codes reads a survey's codes for one codebook version, by hiddenKey.
func (s *surveyStore) codes(ctx context.Context, surveyID string, version int) (map[string]*textCodes, error) {
	return readCodes(ctx, s.pool, surveyID, version)
}

func readCodes(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, surveyID string, version int) (map[string]*textCodes, error) {
	rows, err := db.Query(ctx, `SELECT field, text_hash, coder, category, onske FROM survey_freetext_codes
		WHERE survey_id = $1 AND codebook_version = $2 ORDER BY category, onske`, surveyID, version)
	if err != nil {
		return nil, err
	}
	out := map[string]*textCodes{}
	var field, hash, coder, cat string
	var onske bool
	_, err = pgx.ForEachRow(rows, []any{&field, &hash, &coder, &cat, &onske}, func() error {
		k := hiddenKey(field, hash)
		if out[k] == nil {
			out[k] = &textCodes{}
		}
		c := code{cat, onske}
		switch coder {
		case "model":
			out[k].Model = append(out[k].Model, c)
		case "human":
			out[k].Human = append(out[k].Human, c)
		case "final":
			out[k].Final = append(out[k].Final, c)
		}
		return nil
	})
	return out, err
}

// saveCodes replaces one coder's codes for a text. human is written once
// (errCoded after). final also logs who decided and when, never the text.
// It takes the lock purge takes and refuses a text no row holds any more
// (errGone): a save can then not land between purge counting the codes and
// deleting the rows, where it would be lost.
func (s *surveyStore) saveCodes(ctx context.Context, surveyID, field, hash, coder string, version int, codes []code, by string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('survey_results:' || $1))`, surveyID); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM survey_answers WHERE survey_id = $1
			AND encode(sha256(convert_to(answers->>$2, 'UTF8')), 'hex') = $3)`, surveyID, field, hash).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errGone
		}
		if coder == "human" {
			var seen bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM survey_freetext_codes WHERE survey_id = $1 AND field = $2 AND text_hash = $3 AND coder = 'human' AND codebook_version = $4)`,
				surveyID, field, hash, version).Scan(&seen); err != nil {
				return err
			}
			if seen {
				return errCoded
			}
		}
		batch := &pgx.Batch{}
		batch.Queue(`DELETE FROM survey_freetext_codes WHERE survey_id = $1 AND field = $2 AND text_hash = $3 AND coder = $4 AND codebook_version = $5`, surveyID, field, hash, coder, version)
		for _, c := range codes {
			batch.Queue(`INSERT INTO survey_freetext_codes (survey_id, field, text_hash, coder, category, onske, codebook_version) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				surveyID, field, hash, coder, c.Category, c.Onske, version)
		}
		if coder == "final" {
			batch.Queue(`INSERT INTO survey_freetext_decisions (survey_id, field, text_hash, decided_by, decided_at) VALUES ($1, $2, $3, $4, $5)`,
				surveyID, field, hash, by, time.Now().UTC())
		}
		return tx.SendBatch(ctx, batch).Close()
	})
}

// writeCodingResults replaces s's coding counts in survey_results from its
// rows and codes, while every row is there, so the counts outlive the rows.
// Per field, kind freetext, version the codebook's: option "" has count
// coded and n written; option a category or merged pair has its count and
// kappa (mean). Kind freetext_onske has the wish counts.
func writeCodingResults(ctx context.Context, tx pgx.Tx, cb codebook, s survey, rows []storedAnswer) error {
	hidden := map[string]bool{}
	res, err := tx.Query(ctx, `SELECT field, text_hash FROM survey_freetext_hidden WHERE survey_id = $1`, s.ID)
	if err != nil {
		return err
	}
	var f, h string
	if _, err = pgx.ForEachRow(res, []any{&f, &h}, func() error { hidden[hiddenKey(f, h)] = true; return nil }); err != nil {
		return err
	}
	codes, err := readCodes(ctx, tx, s.ID, cb.Version)
	if err != nil {
		return err
	}
	batch := &pgx.Batch{}
	batch.Queue(`DELETE FROM survey_results WHERE survey_id = $1 AND kind IN ('freetext', 'freetext_onske')`, s.ID)
	insert := `INSERT INTO survey_results (survey_id, kind, key, version, option, count, n, mean) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	for field, t := range tallyAll(cb, respondentCodes(cb, rows, hidden, codes)) {
		batch.Queue(insert, s.ID, "freetext", field, cb.Version, "", t.Coded, t.Written, nil)
		for id, c := range t.Cats {
			batch.Queue(insert, s.ID, "freetext", field, cb.Version, id, c.Count, t.Written, c.Kappa)
			batch.Queue(insert, s.ID, "freetext_onske", field, cb.Version, id, c.Onske, t.Written, nil)
		}
	}
	return tx.SendBatch(ctx, batch).Close()
}

// finalCoding reads a survey's stored coding counts for one codebook
// version; nil when it has none.
func (s *surveyStore) finalCoding(ctx context.Context, surveyID string, version int) (map[string]fieldTally, error) {
	rows, err := s.pool.Query(ctx, `SELECT kind, key, option, count, n, mean FROM survey_results
		WHERE survey_id = $1 AND kind IN ('freetext', 'freetext_onske') AND version = $2`, surveyID, version)
	if err != nil {
		return nil, err
	}
	var out map[string]fieldTally
	var kind, field, opt string
	var count, n int
	var mean *float64
	_, err = pgx.ForEachRow(rows, []any{&kind, &field, &opt, &count, &n, &mean}, func() error {
		if out == nil {
			out = map[string]fieldTally{}
		}
		t := out[field]
		if t.Cats == nil {
			t.Cats = map[string]catTally{}
		}
		c := t.Cats[opt]
		switch {
		case kind == "freetext_onske":
			c.Onske = count
		case opt == "":
			t.Coded, t.Written = count, n
		default:
			c.Count, c.Kappa = count, mean
		}
		if opt != "" {
			t.Cats[opt] = c
		}
		out[field] = t
		return nil
	})
	return out, err
}

// coders names who coded, published with kappa: the human coder is not
// neutral (surveys/README.md#codebooks).
const coders = "modell og en fra verktøyteamet"

// waveFreetext is one wave's published coding.
type waveFreetext struct {
	Survey          string       `json:"survey_id"`
	CodebookVersion int          `json:"codebook_version"`
	Coders          string       `json:"coders"`
	Fields          []codedField `json:"fields"`
}

// publishedCoding is s's coding as the published series shows it: only
// fields where every text has final codes and at least minSegment wrote,
// only published categories with «<5» applied. nil when none.
func (a *surveyAPI) publishedCoding(ctx context.Context, s survey) (*waveFreetext, error) {
	cb, ok := a.codebooks[s.Series]
	if !ok || s.Series == "" || a.codes == nil {
		return nil, nil
	}
	t, err := a.codingTally(ctx, s, cb)
	if err != nil {
		return nil, err
	}
	return publishedFreetext(s, cb, t), nil
}

// publishedFreetext is publishedCoding from counts, for `analyze` too.
func publishedFreetext(s survey, cb codebook, t map[string]fieldTally) *waveFreetext {
	wf := waveFreetext{Survey: s.ID, CodebookVersion: cb.Version, Coders: coders, Fields: []codedField{}}
	for _, f := range codingResults(s, cb, t) {
		if f.Published == nil || f.Coded != f.Written {
			continue
		}
		f.Categories = nil
		wf.Fields = append(wf.Fields, f)
	}
	if len(wf.Fields) == 0 {
		return nil
	}
	return &wf
}

// codingTally is a survey's coding counts: live while it has rows, else
// as stored when they were deleted.
func (a *surveyAPI) codingTally(ctx context.Context, s survey, cb codebook) (map[string]fieldTally, error) {
	if a.answers == nil || a.hidden == nil {
		return nil, errNoStorage
	}
	rows, err := a.answers(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && a.finalCoding != nil {
		return a.finalCoding(ctx, s.ID, cb.Version)
	}
	hidden, err := a.hidden(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	codes, err := a.codes(ctx, s.ID, cb.Version)
	if err != nil {
		return nil, err
	}
	return tallyAll(cb, respondentCodes(cb, rows, hidden, codes)), nil
}
