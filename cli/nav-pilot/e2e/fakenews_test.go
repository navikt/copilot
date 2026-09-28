package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

// fake-news serves a news feed like ki-utvikling's /news.json on 127.0.0.1
// and points nav-pilot at it (NAV_PILOT_NEWS_URL): one item for the CLI,
// dated today, and one that is not. Each GET adds a line to
// $WORK/news-gets.log.
func cmdFakeNews(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: fake-news")
	}
	today := time.Now().Format(time.DateOnly)
	feed := fmt.Sprintf(`{"items":[
{"title":"E2E-nyhet for nav-pilot","date":%q,"url":"https://ki-utvikling.nav.no/nyheter/e2e-cli","summary":"Om nav-pilot.","cli":true},
{"title":"E2E-nyhet om noe annet","date":%q,"url":"https://ki-utvikling.nav.no/nyheter/e2e-annet","summary":"Ikke for CLI."}]}`, today, today)
	log := filepath.Join(ts.Getenv("WORK"), "news-gets.log")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/news.json" {
			http.NotFound(w, r)
			return
		}
		if f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintln(f, "GET")
			_ = f.Close()
		}
		fmt.Fprint(w, feed)
	}))
	ts.Defer(srv.Close)
	ts.Setenv("NAV_PILOT_NEWS_URL", srv.URL+"/news.json")
	// The fake clients exit at once; a real session outlasts the fetch.
	ts.Setenv("NAV_PILOT_E2E_NUDGE_WAIT", "10s")
}
