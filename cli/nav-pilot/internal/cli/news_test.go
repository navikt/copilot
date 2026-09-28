package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewsCleanDropsWhatIsUnsafeToPrint(t *testing.T) {
	got := cleanNews([]newsItem{
		{Title: "Ny\x1b]0;pwned\x07 sak", URL: "https://ki-utvikling.nav.no/nyheter/a"},
		{Title: "Ingen lenke", URL: ""},
		{Title: "Feil skjema", URL: "javascript:alert(1)"},
		{Title: "Annet sted", URL: "https://example.com/nyheter/a"},
		{Title: "Snudd\u202e tekst", URL: "https://ki-utvikling.nav.no/nyheter/c"},
		{Title: "", URL: "https://ki-utvikling.nav.no/nyheter/b"},
	})
	if len(got) != 2 || got[0].Title != "Ny]0;pwned sak" || got[1].Title != "Snudd tekst" {
		t.Fatalf("cleanNews = %+v, want the two nav.no items with the control and format characters gone", got)
	}
}

func TestNewsNextIsNewestUnseenRecentCLIItem(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	st := newsState{Items: []newsItem{
		{Title: "scheduled", Date: "2026-10-01", URL: "u9", CLI: true},
		{Title: "not cli", Date: "2026-09-27", URL: "u0"},
		{Title: "seen", Date: "2026-09-26", URL: "u1", CLI: true},
		{Title: "this", Date: "2026-09-25", URL: "u2", CLI: true},
		{Title: "old", Date: "2026-07-01", URL: "u3", CLI: true},
	}, Seen: []string{"u1"}}
	if it := nextNews(st, now); it == nil || it.URL != "u2" {
		t.Fatalf("nextNews = %+v, want u2", it)
	}
	st.Seen = append(st.Seen, "u2")
	if it := nextNews(st, now); it != nil {
		t.Fatalf("nextNews = %+v, want nothing: the rest are not cli or too old", it)
	}
}

// The line shows each item once, and a slow feed costs at most the short
// timeout at the end of a session.
func TestNewsLineOnceAndNeverSlow(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "true")
	t.Setenv("DO_NOT_TRACK", "")
	prev := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = prev; sessionPrompted = false })
	// One run per call: each call is a session of its own.
	session := func() {
		sessionPrompted = false
		maybeNews("copilot")
	}

	var slow atomic.Bool
	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		if slow.Load() {
			time.Sleep(1500 * time.Millisecond)
		}
		fmt.Fprintf(w, `{"items":[{"title":"Ny sak","date":%q,"url":"https://ki-utvikling.nav.no/nyheter/ny","cli":true}]}`, time.Now().Format(time.DateOnly))
	}))
	defer srv.Close()
	t.Setenv("NAV_PILOT_NEWS_URL", srv.URL)

	session()
	if st := readNewsState(); len(st.Seen) != 1 {
		t.Fatalf("seen = %v after the first session, want the item", st.Seen)
	}
	session()
	if gets.Load() != 1 {
		t.Fatalf("gets = %d, want 1: the feed is cached", gets.Load())
	}

	st := readNewsState()
	st.Fetched, st.Seen = time.Time{}, nil
	writeNewsState(st)
	slow.Store(true)
	start := time.Now()
	session()
	if d := time.Since(start); d > time.Second {
		t.Fatalf("maybeNews took %s with a slow feed, want about %s", d, newsLineTimeout)
	}
}

// A backlog of articles is one news line a day, not one after every session.
func TestNewsLineAtMostOnceADay(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "true")
	t.Setenv("DO_NOT_TRACK", "")
	prev := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = prev; sessionPrompted = false })
	today := time.Now().Format(time.DateOnly)
	writeNewsState(newsState{Fetched: time.Now(), Items: []newsItem{
		{Title: "a", Date: today, URL: "https://ki-utvikling.nav.no/nyheter/a", CLI: true},
		{Title: "b", Date: today, URL: "https://ki-utvikling.nav.no/nyheter/b", CLI: true},
	}})
	session := func() int {
		sessionPrompted = false
		showNews("copilot")
		return len(readNewsState().Seen)
	}
	if n := session(); n != 1 {
		t.Fatalf("first session showed %d items, want 1", n)
	}
	if n := session(); n != 1 {
		t.Fatalf("second session the same day showed another item (%d seen)", n)
	}
	st := readNewsState()
	st.Shown = time.Now().Add(-newsShowEvery)
	writeNewsState(st)
	if n := session(); n != 2 {
		t.Fatalf("a day later: %d seen, want the second item", n)
	}
}
