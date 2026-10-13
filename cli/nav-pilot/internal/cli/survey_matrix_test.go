package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

const noTools = "Jeg bruker ikke KI-kodeverktøy"

// q4-2026 as it is live: pastander (skipped for non-users) and pastander-alle
// share the text «Hvor enig er du i disse påstandene?».
func loadQ4(t *testing.T) surveyDef {
	t.Helper()
	b, err := os.ReadFile("../../../../apps/copilot-survey/surveys/q4-2026.json")
	if err != nil {
		t.Fatal(err)
	}
	var s surveyDef
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// q4Form fills the form state as a user or a non-user would, every matrix
// statement answered with 4, and returns the skip and answered closures the
// form builds.
func q4Form(s surveyDef, user bool) ([]string, [][]string, []string, [][]string, func(surveyQuestion) bool, func(string) []string) {
	n := len(s.Questions)
	picks, multis, others, grid := make([]string, n), make([][]string, n), make([]string, n), make([][]string, n)
	for i, q := range s.Questions {
		switch q.Type {
		case "choice":
			picks[i] = q.options()[0]
		case "multi":
			if q.ID == "verktoy" && !user {
				multis[i] = []string{noTools}
			} else {
				multis[i] = []string{q.options()[0]}
			}
		case "text":
			picks[i] = "fint"
		case "matrix":
			grid[i] = make([]string, len(q.Items))
			for j := range grid[i] {
				grid[i][j] = "4"
			}
		}
	}
	answered := func(id string) []string {
		i := slices.IndexFunc(s.Questions, func(q surveyQuestion) bool { return q.ID == id })
		if i < 0 {
			return nil
		}
		if s.Questions[i].Type == "multi" {
			return multis[i]
		}
		return []string{picks[i]}
	}
	skipped := func(q surveyQuestion) bool {
		return q.SkipIf != nil && slices.Contains(answered(q.SkipIf.Question), q.SkipIf.Answer)
	}
	return picks, multis, others, grid, skipped, answered
}

// The heading change must not touch the payload: these maps are what the
// form sent before it, for a user and a non-user.
func TestSurveyQ4AnswersUnchanged(t *testing.T) {
	s := loadQ4(t)
	opt := func(id string) string {
		i := slices.IndexFunc(s.Questions, func(q surveyQuestion) bool { return q.ID == id })
		return s.Questions[i].options()[0]
	}
	all := map[string]any{"eierskap": 4, "juss-og-sikkerhet": 4}
	base := map[string]any{
		"erfaring": opt("erfaring"), "en-endring": opt("en-endring"), "opplevelse": "fint",
	}
	for k, v := range all {
		base[k] = v
	}
	user := map[string]any{"verktoy": []string{opt("verktoy")}, "verdi": []string{opt("verdi")}}
	for _, id := range []string{"tilfredshet", "kognitiv-belastning", "oppgavegjennomforing", "code-review", "teknisk-kompetanse", "agent-tillit", "kostnad-tilbakeholdenhet"} {
		user[id] = 4
	}
	for k, v := range base {
		user[k] = v
	}
	nonUser := map[string]any{"verktoy": []string{noTools}}
	for k, v := range base {
		nonUser[k] = v
	}
	for _, tc := range []struct {
		name string
		user bool
		want map[string]any
	}{{"user", true, user}, {"non-user", false, nonUser}} {
		picks, multis, others, grid, skipped, answered := q4Form(s, tc.user)
		got := surveyAnswers(s, picks, multis, others, grid, skipped, answered)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: answers = %v, want %v", tc.name, got, tc.want)
		}
		gb, _ := json.Marshal(got)
		wb, _ := json.Marshal(tc.want)
		if string(gb) != string(wb) {
			t.Errorf("%s: payload = %s, want %s", tc.name, gb, wb)
		}
	}
}

// The second matrix continues the first one's count only when the first was
// shown; for a non-user it starts at 1 like any question.
func TestSurveyQ4MatrixHeadingRunsOn(t *testing.T) {
	s := loadQ4(t)
	p := slices.IndexFunc(s.Questions, func(q surveyQuestion) bool { return q.ID == "pastander" })
	a := p + 1
	const h = "Hvor enig er du i disse påstandene?"
	_, _, _, _, skipUser, _ := q4Form(s, true)
	_, _, _, _, skipNon, _ := q4Form(s, false)
	for _, tc := range []struct {
		skipped func(surveyQuestion) bool
		i, j    int
		want    string
	}{
		{skipUser, p, 0, h + " (1 av 9)"},
		{skipUser, p, 6, h + " (7 av 9)"},
		{skipUser, a, 0, h + " (8 av 9)"},
		{skipUser, a, 1, h + " (9 av 9)"},
		{skipNon, a, 0, h + " (1 av 2)"},
		{skipNon, a, 1, h + " (2 av 2)"},
	} {
		if got := matrixHeading(s, tc.i, tc.j, tc.skipped); got != tc.want {
			t.Errorf("matrixHeading(%d, %d) = %q, want %q", tc.i, tc.j, got, tc.want)
		}
	}
}
