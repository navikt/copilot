package store

import "github.com/navikt/copilot/apps/copilot-survey/internal/survey"

// testCodebook codes testSurveys' comment (onske per code) and editor.other
// (never a wish).
var testCodebook = survey.Codebook{
	Series: "dev", Version: 1,
	Fields: []survey.CodebookField{
		{ID: "editor.other", Onske: "aldri", MaxCodes: 2, Categories: []string{"a", "annet"}},
		{ID: "comment", Onske: "kodes", MaxCodes: 2, Categories: []string{"a", "b", "c", "d", "e", "f", "annet"}},
	},
	Categories: []survey.CodebookCategory{
		{ID: "a", Name: "A"}, {ID: "b", Name: "B", MergeWith: "c"}, {ID: "c", Name: "C", MergeWith: "b"},
		{ID: "d", Name: "D"}, {ID: "e", Name: "E", MergeWith: "f"}, {ID: "f", Name: "F", MergeWith: "e"}, {ID: "annet", Name: "Annet"},
	},
}
