package api

import (
	"context"
	"time"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// funcStore is a Store built from functions, so a test sets only the calls
// it needs; an unset call returns zero values.
type funcStore struct {
	submit       func(ctx context.Context, surveyID, hash string, closesOn time.Time, r survey.Response) (bool, error)
	answered     func(ctx context.Context, surveyID, hash string) (bool, error)
	answers      func(ctx context.Context, surveyID string) ([]survey.StoredAnswer, error)
	final        func(ctx context.Context, surveyID string) (*survey.Tally, error)
	finalCompare func(ctx context.Context, surveyID string) (*survey.WaveTally, error)
	hidden       func(ctx context.Context, surveyID string) (map[string]bool, error)
	hide         func(ctx context.Context, surveyID, field, hash string) error
	codes        func(ctx context.Context, surveyID string, version int) (map[string]*survey.TextCodes, error)
	saveCodes    func(ctx context.Context, surveyID, field, hash, coder string, version int, codes []survey.Code, by string) error
	finalCoding  func(ctx context.Context, surveyID string, version int) (map[string]survey.FieldTally, error)
}

// fake is a's funcStore, created when a has no store yet.
func fake(a *surveyAPI) *funcStore {
	if a.store == nil {
		a.store = &funcStore{}
	}
	return a.store.(*funcStore)
}

func (f *funcStore) Submit(ctx context.Context, surveyID, hash string, closesOn time.Time, r survey.Response) (bool, error) {
	if f.submit == nil {
		return false, nil
	}
	return f.submit(ctx, surveyID, hash, closesOn, r)
}

func (f *funcStore) Answered(ctx context.Context, surveyID, hash string) (bool, error) {
	if f.answered == nil {
		return false, nil
	}
	return f.answered(ctx, surveyID, hash)
}

func (f *funcStore) Answers(ctx context.Context, surveyID string) ([]survey.StoredAnswer, error) {
	if f.answers == nil {
		return nil, nil
	}
	return f.answers(ctx, surveyID)
}

func (f *funcStore) Final(ctx context.Context, surveyID string) (*survey.Tally, error) {
	if f.final == nil {
		return nil, nil
	}
	return f.final(ctx, surveyID)
}

func (f *funcStore) FinalCompare(ctx context.Context, surveyID string) (*survey.WaveTally, error) {
	if f.finalCompare == nil {
		return nil, nil
	}
	return f.finalCompare(ctx, surveyID)
}

func (f *funcStore) Hidden(ctx context.Context, surveyID string) (map[string]bool, error) {
	if f.hidden == nil {
		return nil, nil
	}
	return f.hidden(ctx, surveyID)
}

func (f *funcStore) Hide(ctx context.Context, surveyID, field, hash string) error {
	if f.hide == nil {
		return nil
	}
	return f.hide(ctx, surveyID, field, hash)
}

func (f *funcStore) Codes(ctx context.Context, surveyID string, version int) (map[string]*survey.TextCodes, error) {
	if f.codes == nil {
		return nil, nil
	}
	return f.codes(ctx, surveyID, version)
}

func (f *funcStore) SaveCodes(ctx context.Context, surveyID, field, hash, coder string, version int, codes []survey.Code, by string) error {
	if f.saveCodes == nil {
		return nil
	}
	return f.saveCodes(ctx, surveyID, field, hash, coder, version, codes, by)
}

func (f *funcStore) FinalCoding(ctx context.Context, surveyID string, version int) (map[string]survey.FieldTally, error) {
	if f.finalCoding == nil {
		return nil, nil
	}
	return f.finalCoding(ctx, surveyID, version)
}
