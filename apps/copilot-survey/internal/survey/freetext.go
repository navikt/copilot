package survey

import (
	"crypto/sha256"
	"encoding/hex"
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

// FreetextField is a free-text answer key.
type FreetextField struct {
	ID       string         `json:"id"` // question id, or id + ".other"
	Question string         `json:"question"`
	Other    string         `json:"other,omitempty"` // the other option's label
	Texts    []FreetextText `json:"texts"`
}

// FreetextText is one free text and its token.
type FreetextText struct {
	Token string `json:"token"`
	Text  string `json:"text"`
}

// FreetextFields are a survey's free-text answer keys, in question order.
func FreetextFields(s Survey) []FreetextField {
	var out []FreetextField
	for _, q := range s.Questions {
		switch {
		case q.Type == "text":
			out = append(out, FreetextField{ID: q.ID, Question: q.Text})
		case q.Other != "":
			out = append(out, FreetextField{ID: otherKey(q.ID), Question: q.Text, Other: q.Other})
		}
	}
	return out
}

// TextHash is the sha256 of a raw text, hex.
func TextHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// HiddenKey keys a hidden text by field and hash.
func HiddenKey(field, hash string) string { return field + "\x00" + hash }
