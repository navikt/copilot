package store

import (
	"context"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// Hidden returns a survey's hidden texts as HiddenKey(field, hash).
func (s *Store) Hidden(ctx context.Context, surveyID string) (map[string]bool, error) {
	rows, err := s.Pool.Query(ctx, `SELECT field, text_hash FROM survey_freetext_hidden WHERE survey_id = $1`, surveyID)
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
		out[survey.HiddenKey(f, h)] = true
	}
	return out, rows.Err()
}

// Hide hides a text in a survey field, by its hash.
func (s *Store) Hide(ctx context.Context, surveyID, field, hash string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO survey_freetext_hidden (survey_id, field, text_hash) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, surveyID, field, hash)
	return err
}
