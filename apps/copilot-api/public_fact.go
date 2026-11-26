package main

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
	_ "time/tzdata"
)

// The front page's «Dagens innsikt»: one sentence a day, built from Nav-wide
// aggregates and shown to everyone, logged in or not. Rules from
// copilot-intern underlag/offentlige-tall.md:
//   - Nav-wide totals only: no team, editor, language or person split.
//   - A share or a named group needs at least minPublicGroup users on both
//     sides of the fraction, so groups under 20 are never shown.
//   - Percentages are whole numbers, people counts are rounded to the nearest 50.
//   - No credits, cost or code-suggestion numbers.
// The client gets only the finished sentence and a link.

const minPublicGroup = 20

// FactAggregates is one row of Nav-wide totals. The week is the seven days up
// to DataDay (the newest day in user_metrics), the month the 28 days up to it.
type FactAggregates struct {
	DataDay          string        `bigquery:"data_day"`
	WeekUsers        int64         `bigquery:"week_users"`
	MCPUsers         int64         `bigquery:"mcp_users"`
	SkillUsers       int64         `bigquery:"skill_users"`
	CustomAgentUsers int64         `bigquery:"custom_agent_users"`
	Users3PlusModels int64         `bigquery:"users_3plus_models"`
	MonthUsers       int64         `bigquery:"month_users"`
	MonthUserDays    int64         `bigquery:"month_user_days"`
	WeekendUserDays  int64         `bigquery:"weekend_user_days"`
	WeekendUsers     int64         `bigquery:"weekend_users"`
	WeekdayAvg       float64       `bigquery:"weekday_avg"`
	JuneUsers        int64         `bigquery:"june_users"`
	JuneWeekdayAvg   float64       `bigquery:"june_weekday_avg"`
	JulyWeekdayAvg   float64       `bigquery:"july_weekday_avg"`
	Families         []FamilyUsers `bigquery:"families"`
	// From the weekly repo scan, not user_metrics; filled by the handler.
	ActiveRepos            int64 `bigquery:"-"`
	ActiveReposWithCustoms int64 `bigquery:"-"`
}

// FamilyUsers counts distinct users per model family the past week, largest first.
type FamilyUsers struct {
	Family string `bigquery:"family"`
	Users  int64  `bigquery:"users"`
}

// Fact is all the public endpoint returns. WeekUsers is the rounded count of
// people the past week for the front page's insight strip, omitted below 50.
type Fact struct {
	Text      string `json:"text"`
	Href      string `json:"href"`
	WeekUsers int    `json:"weekUsers,omitempty"`
}

type factTemplate struct {
	id     string
	theme  string
	href   string
	render func(a *FactAggregates) (string, bool)
}

// publicPct is a whole-number share, ok only when both counts reach minPublicGroup.
func publicPct(num, den int64) (int, bool) {
	if num < minPublicGroup || den-num < minPublicGroup {
		return 0, false
	}
	return int(math.Round(float64(num) / float64(den) * 100)), true
}

func round50(n float64) int { return int(math.Round(n/50) * 50) }

// roundPeople rounds a people count to the nearest 50 and groups thousands
// Norwegian style (1 000). ok is false below 50, so «Rundt 0» never shows.
func roundPeople(n float64) (string, bool) {
	r := round50(n)
	if r < 1000 {
		return strconv.Itoa(r), r >= 50
	}
	return fmt.Sprintf("%d\u00a0%03d", r/1000, r%1000), true
}

var factTemplates = []factTemplate{
	// Mandag: bruk og vaner
	{"helg", "bruk", "/innsikt/bruk", func(a *FactAggregates) (string, bool) {
		p, ok := publicPct(a.WeekendUserDays, a.MonthUserDays)
		ok = ok && a.WeekendUsers >= minPublicGroup && a.MonthUsers-a.WeekendUsers >= minPublicGroup
		return fmt.Sprintf("%d %% av bruken de siste fire ukene skjedde i helgene.", p), ok
	}},
	{"fellesferie", "bruk", "/innsikt/trender", func(a *FactAggregates) (string, bool) {
		if a.JuneWeekdayAvg < minPublicGroup || a.JulyWeekdayAvg < minPublicGroup || a.JulyWeekdayAvg >= a.JuneWeekdayAvg {
			return "", false
		}
		p := int(math.Round((1 - a.JulyWeekdayAvg/a.JuneWeekdayAvg) * 100))
		return fmt.Sprintf("I juli var det %d %% færre som brukte Copilot på en vanlig arbeidsdag enn i juni.", p), p >= 1
	}},
	{"arbeidsdag", "bruk", "/innsikt/bruk", func(a *FactAggregates) (string, bool) {
		n, ok := roundPeople(a.WeekdayAvg)
		return "Rundt " + n + " personer bruker Copilot på en vanlig arbeidsdag.", ok
	}},
	// Tirsdag: modeller
	{"modellfamilier", "modeller", "/innsikt/bruk", func(a *FactAggregates) (string, bool) {
		n := 0
		for _, f := range a.Families {
			if f.Users >= minPublicGroup {
				n++
			}
		}
		return fmt.Sprintf("%d modellfamilier hadde minst %d brukere den siste uka.", n, minPublicGroup), n >= 2
	}},
	{"ledende-familie", "modeller", "/innsikt/bruk", func(a *FactAggregates) (string, bool) {
		if len(a.Families) == 0 {
			return "", false
		}
		top := a.Families[0]
		p, ok := publicPct(top.Users, a.WeekUsers)
		return fmt.Sprintf("%s var den mest brukte modellfamilien den siste uka. %d %% av brukerne brukte en modell derfra.", top.Family, p), ok
	}},
	{"tre-modeller", "modeller", "/innsikt/bruk", func(a *FactAggregates) (string, bool) {
		p, ok := publicPct(a.Users3PlusModels, a.WeekUsers)
		return fmt.Sprintf("%d %% av brukerne brukte minst tre ulike modeller den siste uka.", p), ok
	}},
	// Onsdag: kode og verktøy
	{"mcp", "verktoy", "/innsikt/tilpasninger", func(a *FactAggregates) (string, bool) {
		p, ok := publicPct(a.MCPUsers, a.WeekUsers)
		return fmt.Sprintf("%d %% av brukerne brukte minst én MCP-server den siste uka.", p), ok
	}},
	{"skills", "verktoy", "/innsikt/tilpasninger", func(a *FactAggregates) (string, bool) {
		p, ok := publicPct(a.SkillUsers, a.WeekUsers)
		return fmt.Sprintf("%d %% av brukerne brukte minst én skill den siste uka.", p), ok
	}},
	{"egne-agenter", "verktoy", "/innsikt/tilpasninger", func(a *FactAggregates) (string, bool) {
		p, ok := publicPct(a.CustomAgentUsers, a.WeekUsers)
		return fmt.Sprintf("%d %% av brukerne brukte en egendefinert agent den siste uka.", p), ok
	}},
	// Torsdag: vekst og milepæler
	{"ukebrukere", "vekst", "/innsikt/trender", func(a *FactAggregates) (string, bool) {
		n, ok := roundPeople(float64(a.WeekUsers))
		return "Rundt " + n + " personer brukte Copilot den siste uka.", ok
	}},
	{"maanedsbrukere", "vekst", "/innsikt/bruk", func(a *FactAggregates) (string, bool) {
		n, ok := roundPeople(float64(a.MonthUsers))
		return "Rundt " + n + " personer brukte Copilot de siste 28 dagene.", ok
	}},
	{"repoer-tilpasninger", "verktoy", "/innsikt/tilpasninger", func(a *FactAggregates) (string, bool) {
		p, ok := publicPct(a.ActiveReposWithCustoms, a.ActiveRepos)
		return fmt.Sprintf("%d %% av de aktive navikt-repoene har tilpasninger for Copilot.", p), ok
	}},
	{"vekst-juni", "vekst", "/innsikt/trender", func(a *FactAggregates) (string, bool) {
		if a.JuneUsers < minPublicGroup || a.MonthUsers <= a.JuneUsers {
			return "", false
		}
		p := int(math.Round((float64(a.MonthUsers)/float64(a.JuneUsers) - 1) * 100))
		return fmt.Sprintf("Antallet som brukte Copilot de siste fire ukene er %d %% høyere enn i juni.", p), p >= 1
	}},
}

var oslo = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		panic(err) // tzdata is embedded below; a wrong day boundary is worse than no start
	}
	return loc
}()

var weekdayTheme = map[time.Weekday]string{
	time.Monday: "bruk", time.Tuesday: "modeller", time.Wednesday: "verktoy", time.Thursday: "vekst",
}

// factOrder lists the templates to try for a date, best first. Mon–Thu: the
// day's theme, starting at ISO week mod theme size, then everything else.
// Fri–Sun: every template, shuffled with the date as seed.
func factOrder(day time.Time) []factTemplate {
	theme, fixed := weekdayTheme[day.Weekday()]
	if !fixed {
		h := fnv.New64a()
		h.Write([]byte(day.Format(time.DateOnly)))
		order := append([]factTemplate(nil), factTemplates...)
		rand.New(rand.NewPCG(h.Sum64(), 0)).Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		return order
	}
	var inTheme, rest []factTemplate
	for _, t := range factTemplates {
		if t.theme == theme {
			inTheme = append(inTheme, t)
		} else {
			rest = append(rest, t)
		}
	}
	_, week := day.ISOWeek()
	k := week % len(inTheme)
	return append(append(inTheme[k:], inTheme[:k]...), rest...)
}

// pickFact renders the first template in the day's order that has enough data.
func pickFact(day time.Time, a *FactAggregates) (*Fact, string) {
	if a == nil {
		return nil, ""
	}
	for _, t := range factOrder(day) {
		if text, ok := t.render(a); ok {
			return &Fact{Text: text, Href: t.href}, t.id
		}
	}
	return nil, ""
}

func (h *BigQueryHandlers) handlePublicFact(w http.ResponseWriter, r *http.Request) {
	agg, err := h.bqClient.GetFactAggregates(r.Context())
	if err != nil {
		slog.Error("Failed to fetch fact aggregates", "error", err)
		respondError(w, "internal_error", "Failed to fetch fact", http.StatusInternalServerError)
		return
	}
	fact, id := pickFact(time.Now().In(oslo), agg)
	if fact != nil && id != "ukebrukere" { // the sentence already says it
		if n := round50(float64(agg.WeekUsers)); n >= 50 {
			fact.WeekUsers = n
		}
	}
	cacheControl(w, 3600, true)
	respondJSON(w, fact, http.StatusOK) // null when no template has enough data
}
