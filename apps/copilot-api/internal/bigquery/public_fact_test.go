package bigquery

import (
	"errors"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-api/internal/cache"
)

func TestWithRepoCounts(t *testing.T) {
	agg := fullAggregates()
	got := withRepoCounts(agg, &AdoptionSummary{ActiveReposWithRecentCommits: 1200, AdoptionRateActiveOnly: 0.0825})
	if got.ActiveRepos != 1200 || got.ActiveReposWithCustoms != 99 {
		t.Errorf("got %d of %d, want 99 of 1200", got.ActiveReposWithCustoms, got.ActiveRepos)
	}
	if agg.ActiveReposWithCustoms != 99 || got == agg {
		t.Error("withRepoCounts must return a copy")
	}
	if withRepoCounts(nil, &AdoptionSummary{}) != nil || withRepoCounts(agg, nil) != agg {
		t.Error("nil inputs must pass through")
	}
}

func TestCachedFactAdoptionCachesFailures(t *testing.T) {
	c := cache.New(time.Hour)
	calls := 0
	fail := func() (*AdoptionSummary, error) { calls++; return nil, errors.New("bq down") }
	for range 3 {
		if s := cachedFactAdoption(c, "k", fail); s != nil {
			t.Fatalf("got %+v, want nil", s)
		}
	}
	if calls != 1 {
		t.Fatalf("failing query ran %d times, want 1", calls)
	}
	ok := func() (*AdoptionSummary, error) {
		calls++
		return &AdoptionSummary{ActiveReposWithRecentCommits: 10}, nil
	}
	for range 2 {
		if s := cachedFactAdoption(c, "k2", ok); s == nil || s.ActiveReposWithRecentCommits != 10 {
			t.Fatalf("got %+v", s)
		}
	}
	if calls != 2 {
		t.Fatalf("query ran %d times, want 2", calls)
	}
}

func fullAggregates() *FactAggregates {
	return &FactAggregates{
		DataDay: "2026-10-09", WeekUsers: 532, MCPUsers: 302, SkillUsers: 258, CustomAgentUsers: 158,
		Users3PlusModels: 182, MonthUsers: 649, MonthUserDays: 7463, WeekendUserDays: 272, WeekendUsers: 120, WeekdayAvg: 359.55,
		JuneUsers: 605, JuneWeekdayAvg: 334.9, JulyWeekdayAvg: 159.1,
		Families:    []FamilyUsers{{"GPT", 385}, {"Claude Opus", 211}, {"Claude Sonnet", 121}, {"Claude Haiku", 11}, {"Gemini", 7}},
		ActiveRepos: 1200, ActiveReposWithCustoms: 99,
	}
}
