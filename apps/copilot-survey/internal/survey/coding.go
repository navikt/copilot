package survey

import (
	"errors"
)

// Coding in admin (#2239): GET .../freetext/coding gives the codebook, the
// progress, one masked text the person has not coded yet, the texts where
// the coders disagree, and the results. POST .../freetext/coding saves the
// person's codes (once: blind to the model until then) or a decision.
// POST .../freetext/coding/model imports the model's codes, keyed by
// sha256 of the masked text as `copilot-survey code` writes them. Logs carry
// survey, field and counts, never a text.

var (
	// ErrCoded is SaveCodes refusing a second human code for a text.
	ErrCoded = errors.New("already coded")
	// ErrGone is SaveCodes finding the text no longer among the answers.
	ErrGone = errors.New("text deleted")
)

// RespondentCodes is, per codebook field, the codes of every respondent's
// visible text in it, one entry per respondent.
func RespondentCodes(cb Codebook, rows []StoredAnswer, hidden map[string]bool, codes map[string]*TextCodes) map[string][]TextCodes {
	return codesPerField(cb, rows, func(_ StoredAnswer, field, raw string) (TextCodes, bool) {
		key := HiddenKey(field, TextHash(raw))
		var tc TextCodes
		if c := codes[key]; c != nil {
			tc = *c
		}
		return tc, !hidden[key]
	})
}

// CsvCodes is RespondentCodes for rows read back from the admin CSV, which
// carry their codes and no hidden text: what `analyze` counts.
func CsvCodes(cb Codebook, rows []StoredAnswer) map[string][]TextCodes {
	return codesPerField(cb, rows, func(row StoredAnswer, field, _ string) (TextCodes, bool) { return row.Codes[field], true })
}

func codesPerField(cb Codebook, rows []StoredAnswer, codesOf func(row StoredAnswer, field, raw string) (TextCodes, bool)) map[string][]TextCodes {
	out := map[string][]TextCodes{}
	for _, f := range cb.Fields {
		out[f.ID] = []TextCodes{}
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

// TallyAll tallies the codes of every codebook field.
func TallyAll(cb Codebook, texts map[string][]TextCodes) map[string]FieldTally {
	out := map[string]FieldTally{}
	for _, f := range cb.Fields {
		out[f.ID] = tallyCodes(cb, f, texts[f.ID])
	}
	return out
}

// ModelCodes is the file `copilot-survey code` writes.
type ModelCodes struct {
	Survey          string       `json:"survey"`
	Model           string       `json:"model"`
	CodebookVersion int          `json:"codebook_version"`
	Codes           []MaskedCode `json:"codes"`
}

// MaskedCode is a model code for a text, keyed by the masked text's hash.
type MaskedCode struct {
	Field string `json:"field"`
	Text  string `json:"text"` // sha256 of the masked text, hex
	Codes []Code `json:"codes"`
}

// coders names who coded, published with kappa: the human coder is not
// neutral (surveys/README.md#codebooks).
const coders = "modell og en fra verktøyteamet"

// WaveFreetext is one wave's published coding.
type WaveFreetext struct {
	Survey          string       `json:"survey_id"`
	CodebookVersion int          `json:"codebook_version"`
	Coders          string       `json:"coders"`
	Fields          []CodedField `json:"fields"`
}

// PublishedFreetext is publishedCoding from counts, for `analyze` too.
func PublishedFreetext(s Survey, cb Codebook, t map[string]FieldTally) *WaveFreetext {
	wf := WaveFreetext{Survey: s.ID, CodebookVersion: cb.Version, Coders: coders, Fields: []CodedField{}}
	for _, f := range CodingResults(s, cb, t) {
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
