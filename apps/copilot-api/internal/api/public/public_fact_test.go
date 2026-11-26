package public

import (
	"context"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fullAggregates() *bqstore.FactAggregates {
	return &bqstore.FactAggregates{
		DataDay: "2026-10-09", WeekUsers: 532, MCPUsers: 302, SkillUsers: 258, CustomAgentUsers: 158,
		Users3PlusModels: 182, MonthUsers: 649, MonthUserDays: 7463, WeekendUserDays: 272, WeekendUsers: 120, WeekdayAvg: 359.55,
		JuneUsers: 605, JuneWeekdayAvg: 334.9, JulyWeekdayAvg: 159.1,
		Families:    []bqstore.FamilyUsers{{Family: "GPT", Users: 385}, {Family: "Claude Opus", Users: 211}, {Family: "Claude Sonnet", Users: 121}, {Family: "Claude Haiku", Users: 11}, {Family: "Gemini", Users: 7}},
		ActiveRepos: 1200, ActiveReposWithCustoms: 99,
	}
}

// factQuerier answers GetFactAggregates; any other query panics on the
// nil embedded interface.
type factQuerier struct {
	bqstore.BigQueryQuerier
	aggregates *bqstore.FactAggregates
}

func (m *factQuerier) GetFactAggregates(context.Context) (*bqstore.FactAggregates, error) {
	return m.aggregates, nil
}

func day(s string) time.Time {
	d, _ := time.ParseInLocation(time.DateOnly, s, bqstore.Oslo)
	return d
}

func TestFactOrderWeekdayTheme(t *testing.T) {
	// 2026-10-05 is a Monday, ISO week 41.
	for date, theme := range map[string]string{"2026-10-05": "bruk", "2026-10-06": "modeller", "2026-10-07": "verktoy", "2026-10-08": "vekst"} {
		if got := factOrder(day(date))[0].theme; got != theme {
			t.Errorf("%s: first theme %q, want %q", date, got, theme)
		}
	}
	// Week 41 mod 3 templates = 2: the third template of the theme leads.
	if got := factOrder(day("2026-10-05"))[0].id; got != "arbeidsdag" {
		t.Errorf("week 41 Monday: %q, want arbeidsdag", got)
	}
	if got := factOrder(day("2026-10-12"))[0].id; got != "helg" { // week 42 mod 3 = 0
		t.Errorf("week 42 Monday: %q, want helg", got)
	}
}

func TestFactOrderWeekendSeededByDate(t *testing.T) {
	a, b := factOrder(day("2026-10-10")), factOrder(day("2026-10-10"))
	if len(a) != len(factTemplates) {
		t.Fatalf("weekend order has %d templates, want %d", len(a), len(factTemplates))
	}
	for i := range a {
		if a[i].id != b[i].id {
			t.Fatal("same date gave a different order")
		}
	}
	differs := false
	for _, d := range []string{"2026-10-09", "2026-10-11", "2026-10-16", "2026-10-17"} {
		differs = differs || factOrder(day(d))[0].id != a[0].id
	}
	if !differs {
		t.Error("Fri–Sun picks never change with the date")
	}
}

func TestPickFactFallsBack(t *testing.T) {
	a := fullAggregates()
	a.WeekdayAvg = 4 // arbeidsdag below the floor
	f, id := pickFact(day("2026-10-05"), a)
	if id != "helg" || f.Href != "/innsikt/bruk" {
		t.Fatalf("got %q, want fallback to helg", id)
	}
	// Every share under 20 users drops out; the count of people survives.
	a = &bqstore.FactAggregates{WeekUsers: 532, MCPUsers: 19}
	if _, id := pickFact(day("2026-10-07"), a); id != "ukebrukere" {
		t.Fatalf("got %q, want fallback to ukebrukere", id)
	}
	if f, _ := pickFact(day("2026-10-07"), &bqstore.FactAggregates{}); f != nil {
		t.Fatalf("empty aggregates gave %+v", f)
	}
}

func TestFactRounding(t *testing.T) {
	if p, ok := publicPct(272, 7463); !ok || p != 4 {
		t.Errorf("publicPct = %d, %v", p, ok)
	}
	if _, ok := publicPct(19, 500); ok {
		t.Error("19 users passed the floor of 20")
	}
	if _, ok := publicPct(520, 532); ok {
		t.Error("complement of 12 passed the floor of 20")
	}
	for in, want := range map[float64]string{532: "550", 524: "500", 371: "350", 375: "400", 1024: "1\u00a0000", 22: "0"} {
		got, ok := roundPeople(in)
		if got != want || ok != (in >= 25) {
			t.Errorf("roundPeople(%v) = %q, %v, want %q", in, got, ok, want)
		}
	}
}

func TestEveryTemplateRendersCleanly(t *testing.T) {
	a := fullAggregates()
	for _, tpl := range factTemplates {
		text, ok := tpl.render(a)
		if !ok || !strings.HasSuffix(text, ".") || strings.Contains(text, "AI") || !strings.HasPrefix(tpl.href, "/innsikt") {
			t.Errorf("%s: %q ok=%v href=%s", tpl.id, text, ok, tpl.href)
		}
	}
}

func TestPublicFactEndpoint(t *testing.T) {
	router := Router(&config.Config{}, nil, New(Deps{BQ: &factQuerier{aggregates: fullAggregates()}}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public/v1/fact", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// weekUsers is left out on days the sentence itself is the weekly count.
	weekly := strings.Contains(got["text"].(string), "den siste uka") && strings.HasPrefix(got["text"].(string), "Rundt")
	if got["text"] == "" || got["href"] == "" || (weekly && len(got) != 2) || (!weekly && (len(got) != 3 || got["weekUsers"] != float64(550))) {
		t.Fatalf("body %v, want text, href and (unless the weekly sentence) weekUsers 550", got)
	}
}

func TestNewTemplates(t *testing.T) {
	a := fullAggregates()
	render := func(id string) (string, bool) {
		for _, tpl := range factTemplates {
			if tpl.id == id {
				return tpl.render(a)
			}
		}
		t.Fatalf("no template %q", id)
		return "", false
	}
	if got, ok := render("maanedsbrukere"); !ok || got != "Rundt 650 personer brukte Copilot de siste 28 dagene." {
		t.Errorf("maanedsbrukere = %q, %v", got, ok)
	}
	if got, ok := render("repoer-tilpasninger"); !ok || got != "8 % av de aktive navikt-repoene har tilpasninger for Copilot." {
		t.Errorf("repoer-tilpasninger = %q, %v", got, ok)
	}
	a.ActiveReposWithCustoms = 19
	if _, ok := render("repoer-tilpasninger"); ok {
		t.Error("19 repos passed the floor of 20")
	}
}
