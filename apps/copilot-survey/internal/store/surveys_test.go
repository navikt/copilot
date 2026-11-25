package store

import "github.com/navikt/copilot/apps/copilot-survey/internal/survey"

const testSurveys = `[{"id":"q4-2026","title":"Q4","active":true,"starts":"2026-10-01","ends":"2026-11-30","questions":[
 {"id":"overall","version":1,"type":"scale","text":"How useful?","min":1,"max":5,"required":true},
 {"id":"client","version":1,"type":"choice","text":"Which client?","options":["copilot","opencode"]},
 {"id":"tools","version":1,"type":"multi","text":"Which tools?","options":["a","b","c"],"max_choices":2},
 {"id":"why","version":1,"type":"choice","text":"Why not copilot?","options":["habit","other"],"skip_if":{"question":"client","answer":"copilot"}},
 {"id":"editor","version":1,"type":"multi","text":"Which editors?","options":["vim","vscode"],"other":"Annet","max_length":10},
 {"id":"grid","version":1,"type":"matrix","text":"How much do you agree?","min":1,"max":5,"unsure":"Vet ikke","skip_if":{"question":"client","answer":"opencode"},
  "items":[{"id":"fast","version":2,"text":"It is fast."},{"id":"safe","version":1,"text":"It is safe.","reverse":true}]},
 {"id":"comment","version":1,"type":"text","text":"Anything else?","max_length":20}]},
 {"id":"old","title":"Old","active":true,"starts":"2025-01-01","ends":"2025-01-31","questions":[{"id":"a","version":1,"type":"scale","text":"?","min":1,"max":3}]}]`

func loadSurveys(raw []byte) ([]survey.Survey, error) {
	var surveys []survey.Survey
	if err := survey.DecodeStrict(raw, &surveys); err != nil {
		return nil, err
	}
	return surveys, survey.ValidateSurveys(surveys)
}
