package main

import (
	"context"
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

func (bq *BigQueryClient) GetFactAggregates(ctx context.Context) (*FactAggregates, error) {
	ref := bq.tableRef(bq.metricsDataset, "user_metrics")
	queryStr := fmt.Sprintf(`
WITH base AS (
  SELECT day, JSON_VALUE(raw_record, '$.user_login') AS u, raw_record AS r
  FROM %s
  WHERE scope = 'enterprise' AND day >= DATE '2026-06-01'
    AND (SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) > 0
      OR SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) > 0)
),
last AS (SELECT MAX(day) AS d FROM base),
w AS (SELECT base.* FROM base, last WHERE day > DATE_SUB(d, INTERVAL 7 DAY)),
m28 AS (SELECT base.* FROM base, last WHERE day > DATE_SUB(d, INTERVAL 28 DAY)),
models AS (
  SELECT DISTINCT u, JSON_VALUE(mf, '$.model') AS model
  FROM w, UNNEST(JSON_QUERY_ARRAY(r, '$.totals_by_model_feature')) AS mf
  WHERE JSON_VALUE(mf, '$.model') NOT IN ('auto', 'unknown', 'others')
),
fam AS (
  SELECT CASE
      WHEN REGEXP_CONTAINS(model, r'opus') THEN 'Claude Opus'
      WHEN REGEXP_CONTAINS(model, r'sonnet') THEN 'Claude Sonnet'
      WHEN REGEXP_CONTAINS(model, r'haiku') THEN 'Claude Haiku'
      WHEN REGEXP_CONTAINS(model, r'^gpt.*(mini|small|nano)') THEN 'GPT mini'
      WHEN STARTS_WITH(model, 'gpt') THEN 'GPT'
      WHEN STARTS_WITH(model, 'gemini') THEN 'Gemini'
      ELSE 'andre' END AS family,
    COUNT(DISTINCT u) AS users
  FROM models GROUP BY family
),
daily AS (SELECT day, COUNT(DISTINCT u) AS n FROM base WHERE EXTRACT(DAYOFWEEK FROM day) BETWEEN 2 AND 6 GROUP BY day)
SELECT
  CAST((SELECT d FROM last) AS STRING) AS data_day,
  (SELECT COUNT(DISTINCT u) FROM w) AS week_users,
  (SELECT COUNT(DISTINCT u) FROM w WHERE SAFE_CAST(JSON_VALUE(r, '$.distinct_mcp_use_count') AS INT64) > 0) AS mcp_users,
  (SELECT COUNT(DISTINCT u) FROM w WHERE SAFE_CAST(JSON_VALUE(r, '$.distinct_skill_use_count') AS INT64) > 0) AS skill_users,
  (SELECT COUNT(DISTINCT u) FROM w WHERE SAFE_CAST(JSON_VALUE(r, '$.distinct_custom_agent_use_count') AS INT64) > 0) AS custom_agent_users,
  (SELECT COUNT(*) FROM (SELECT u FROM models GROUP BY u HAVING COUNT(*) >= 3)) AS users_3plus_models,
  (SELECT COUNT(DISTINCT u) FROM m28) AS month_users,
  (SELECT COUNT(*) FROM m28) AS month_user_days,
  (SELECT COUNTIF(EXTRACT(DAYOFWEEK FROM day) IN (1, 7)) FROM m28) AS weekend_user_days,
  (SELECT COUNT(DISTINCT u) FROM m28 WHERE EXTRACT(DAYOFWEEK FROM day) IN (1, 7)) AS weekend_users,
  (SELECT IFNULL(AVG(n), 0) FROM daily, last WHERE day > DATE_SUB(d, INTERVAL 28 DAY)) AS weekday_avg,
  (SELECT COUNT(DISTINCT u) FROM base WHERE day BETWEEN DATE '2026-06-03' AND DATE '2026-06-30') AS june_users,
  (SELECT IFNULL(AVG(n), 0) FROM daily WHERE day BETWEEN DATE '2026-06-01' AND DATE '2026-06-30') AS june_weekday_avg,
  (SELECT IFNULL(AVG(n), 0) FROM daily WHERE day BETWEEN DATE '2026-07-01' AND DATE '2026-07-31') AS july_weekday_avg,
  ARRAY(SELECT AS STRUCT family, users FROM fam WHERE family != 'andre' ORDER BY users DESC) AS families
`, ref)
	// ponytail: June/July 2026 are fixed baselines for the growth and holiday
	// facts; move them forward when a newer year's data is the better story.
	it, err := bq.client.Query(queryStr).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readSingleRow[FactAggregates](it)
}

// GetFactAggregates is cached per Oslo date, so the day's fact is stable.
func (c *CachedBigQueryClient) GetFactAggregates(ctx context.Context) (*FactAggregates, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	date := time.Now().In(oslo).Format(time.DateOnly)
	key := "fact_aggregates_" + date
	agg, err := getCachedValue(c, key, func() (*FactAggregates, error) {
		agg, err := c.client.GetFactAggregates(ctx)
		if err != nil {
			// Cache the miss as nil (no fact) for the TTL, so a failing query
			// cannot be re-run by every anonymous front-page view.
			slog.Error("Failed to fetch fact aggregates", "error", err)
			return nil, nil
		}
		return agg, nil
	})
	if err != nil || agg == nil {
		return agg, err
	}
	// The repo count is optional: wait at most adoptionBudget for it, so a slow
	// scan query never costs the front page its fact. The load keeps running
	// and fills the cache for the next request.
	ch := make(chan *AdoptionSummary, 1)
	go func() {
		ch <- cachedFactAdoption(c.cache, "fact_adoption_"+date, func() (*AdoptionSummary, error) {
			v, err, _ := c.group.Do("fact_adoption_"+date, func() (any, error) {
				qctx, cancel := withQueryTimeout(ctx)
				defer cancel()
				return c.client.GetAdoptionSummary(qctx)
			})
			s, _ := v.(*AdoptionSummary)
			return s, err
		})
	}()
	select {
	case s := <-ch:
		return withRepoCounts(agg, s), nil
	case <-time.After(adoptionBudget):
		slog.Warn("Adoption summary too slow for fact; serving without repo count")
		return agg, nil
	}
}

// cachedFactAdoption loads the repo scan once per key. A failure or an empty
// scan is cached as nil for negativeCacheTTL, so anonymous front-page views
// cannot re-run a failing query (#1572).
func cachedFactAdoption(c *Cache, key string, load func() (*AdoptionSummary, error)) *AdoptionSummary {
	if v, ok := c.Get(key); ok {
		s, _ := v.(*AdoptionSummary)
		return s
	}
	s, err := load()
	if err != nil || s == nil {
		if err != nil {
			slog.Error("Failed to fetch adoption summary for fact", "error", err)
		}
		c.SetWithTTL(key, (*AdoptionSummary)(nil), negativeCacheTTL)
		return nil
	}
	c.SetWithTTL(key, s, 24*time.Hour)
	return s
}

const adoptionBudget = 2 * time.Second

// withRepoCounts adds the repo scan to a copy; the cached aggregates are shared.
func withRepoCounts(agg *FactAggregates, s *AdoptionSummary) *FactAggregates {
	if agg == nil || s == nil {
		return agg
	}
	a := *agg
	a.ActiveRepos = s.ActiveReposWithRecentCommits
	a.ActiveReposWithCustoms = int64(math.Round(s.AdoptionRateActiveOnly * float64(s.ActiveReposWithRecentCommits)))
	return &a
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
