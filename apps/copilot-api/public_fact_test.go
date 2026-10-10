package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fullAggregates() *FactAggregates {
	return &FactAggregates{
		DataDay: "2026-10-09", WeekUsers: 532, MCPUsers: 302, SkillUsers: 258, CustomAgentUsers: 158,
		Users3PlusModels: 182, MonthUsers: 649, MonthUserDays: 7463, WeekendUserDays: 272, WeekdayAvg: 359.55,
		JuneUsers: 605, JuneWeekdayAvg: 334.9, JulyWeekdayAvg: 159.1,
		Families: []FamilyUsers{{"GPT", 385}, {"Claude Opus", 211}, {"Claude Sonnet", 121}, {"Claude Haiku", 11}, {"Gemini", 7}},
	}
}

func day(s string) time.Time {
	d, _ := time.ParseInLocation(time.DateOnly, s, oslo)
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
	a = &FactAggregates{WeekUsers: 532, MCPUsers: 19}
	if _, id := pickFact(day("2026-10-07"), a); id != "ukebrukere" {
		t.Fatalf("got %q, want fallback to ukebrukere", id)
	}
	if f, _ := pickFact(day("2026-10-07"), &FactAggregates{}); f != nil {
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
	for in, want := range map[float64]int{532: 550, 524: 500, 371: 350, 375: 400} {
		if got := roundPeople(in); got != want {
			t.Errorf("roundPeople(%v) = %d, want %d", in, got, want)
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
	router := makePublicRouter(&Config{}, nil, newBigQueryHandlers(&mockBigQueryClient{factAggregates: fullAggregates()}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public/v1/fact", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["text"] == "" || got["href"] == "" {
		t.Fatalf("body %v, want only text and href", got)
	}
}
