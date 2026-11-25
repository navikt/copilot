package store

import (
	"encoding/json"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

func row(answers, versions string) survey.StoredAnswer {
	var a survey.StoredAnswer
	if json.Unmarshal([]byte(`{"answers":`+answers+`,"question_versions":`+versions+`}`), &a) != nil {
		panic(answers)
	}
	return a
}
