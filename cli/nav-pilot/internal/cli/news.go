package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// News from ki-utvikling in the CLI (#1024). The site publishes a static feed
// of its newest articles (/news.json, built with the site). After an
// interactive session that ended calmly, and when nothing else was brought
// up in that run (a survey, the opencode tip), nav-pilot shows one line
// about the newest article marked cli that this machine has not shown yet. nav-pilot news lists the feed.
//
// The line follows the survey rules: never without a terminal, in CI, on a
// launch with client args after --, after a Ctrl-C, with news = false, or with
// telemetry opted out (DO_NOT_TRACK, NAV_PILOT_TELEMETRY_ENABLED=false). The
// feed is fetched at most every newsFetchEvery with a newsLineTimeout
// timeout; out of reach, nav-pilot keeps what it fetched before. Nothing
// is sent back.

const (
	newsFeedDefault = "https://ki-utvikling.nav.no/news.json"
	newsFetchEvery  = 6 * time.Hour
	newsLineTimeout = 500 * time.Millisecond
	// newsPrepTimeout is the fetch's timeout while a session runs: nobody
	// waits on it there.
	newsPrepTimeout = 2 * time.Second
	// nudgePrepWait is how long the end of a session waits for the
	// background fetches before it goes without them.
	nudgePrepWait = 50 * time.Millisecond
	// newsMaxAge keeps a new install from bringing up an old article.
	newsMaxAge = 30 * 24 * time.Hour
	// newsSeenMax bounds the seen list; the feed holds far fewer.
	newsSeenMax = 100
)

type newsItem struct {
	Title   string `json:"title"`
	Date    string `json:"date"`
	URL     string `json:"url"`
	Summary string `json:"summary,omitempty"`
	CLI     bool   `json:"cli,omitempty"`
}

// newsState is ~/.nav-pilot/news.json: the cached feed and the URLs already
// shown.
type newsState struct {
	Fetched time.Time  `json:"fetched"`
	Items   []newsItem `json:"items"`
	Seen    []string   `json:"seen"`
}

func newsStatePath() (string, error) {
	dir, err := telemetrypkg.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "news.json"), nil
}

func readNewsState() newsState {
	var st newsState
	if path, err := newsStatePath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &st)
		}
	}
	return st
}

func writeNewsState(st newsState) {
	if len(st.Seen) > newsSeenMax {
		st.Seen = st.Seen[len(st.Seen)-newsSeenMax:]
	}
	if path, err := newsStatePath(); err == nil {
		writeStateFile(path, st)
	}
}

// newsFeedURL is the feed, or NAV_PILOT_NEWS_URL under the same rule as
// NAV_PILOT_COPILOT_CLI_URL: https on nav.no, or loopback.
func newsFeedURL() string {
	if v := os.Getenv("NAV_PILOT_NEWS_URL"); v != "" {
		if allowedCopilotCLIURL(v) {
			return v
		}
		warnIgnoredURL("NAV_PILOT_NEWS_URL")
	}
	return newsFeedDefault
}

func fetchNews(timeout time.Duration) ([]newsItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, newsFeedURL(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("news: %s", resp.Status)
	}
	var body struct {
		Items []newsItem `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&body); err != nil {
		return nil, err
	}
	return cleanNews(body.Items), nil
}

// cleanNews keeps what is safe to print: a feed is text from the network, and
// a control character in a title could drive the terminal. An item without a
// title or a link on nav.no is dropped: the site only ever links to itself.
func cleanNews(items []newsItem) []newsItem {
	strip := func(s string) string {
		return strings.TrimSpace(strings.Map(func(r rune) rune {
			// Not only control characters: format ones (U+202E and the
			// like) can reorder what the terminal shows.
			if !unicode.IsPrint(r) && r != ' ' {
				return -1
			}
			return r
		}, s))
	}
	var out []newsItem
	for _, it := range items {
		it.Title, it.Date, it.URL, it.Summary = strip(it.Title), strip(it.Date), strip(it.URL), strip(it.Summary)
		if it.Title == "" || !allowedCopilotCLIURL(it.URL) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// nextNews is the first cli item in the feed (newest first) that is recent,
// not dated in the future, and not shown before.
func nextNews(st newsState, now time.Time) *newsItem {
	for i, it := range st.Items {
		d, err := time.Parse(time.DateOnly, it.Date)
		if age := now.Sub(d); it.CLI && err == nil && age >= 0 && age <= newsMaxAge && !slices.Contains(st.Seen, it.URL) {
			return &st.Items[i]
		}
	}
	return nil
}

// maybeNews is the news line after a session. Like the survey prompt it never
// fails: news must not change how nav-pilot exits.
func maybeNews(client string) {
	prepareNews(client, newsLineTimeout)
	showNews(client)
}

// newsAllowed is whether a session may end with the news line.
func newsAllowed(client string) bool {
	cfg, _ := readConfig()
	return resolve(cfg, CLIOverrides{Client: client}).News && nudgesAllowed()
}

// prepareNews fetches the feed when a fetch is due. It prints nothing, so a
// launch runs it while the session has the terminal (startNudgePrep).
func prepareNews(client string, timeout time.Duration) {
	if !newsAllowed(client) {
		return
	}
	st := readNewsState()
	now := time.Now()
	if now.Sub(st.Fetched) < newsFetchEvery {
		return
	}
	// Out of reach (offline, slow): keep the items fetched before, and do
	// not try again before the next fetch is due.
	if items, err := fetchNews(timeout); err == nil {
		st.Items = items
	}
	st.Fetched = now
	writeNewsState(st)
}

// showNews prints the next unseen item, from what prepareNews fetched.
func showNews(client string) {
	if !newsAllowed(client) || sessionPrompted {
		return
	}
	st := readNewsState()
	if it := nextNews(st, time.Now()); it != nil && claimSessionPrompt() {
		st.Seen = append(st.Seen, it.URL)
		fmt.Fprintf(os.Stderr, "%s Nytt fra ki-utvikling: %s %s\n", dim("ℹ"), it.Title, it.URL)
		writeNewsState(st)
	}
}

// nudgeWait is nudgePrepWait, or in the e2e build NAV_PILOT_E2E_NUDGE_WAIT:
// the fake clients there exit before any fetch could finish.
func nudgeWait() time.Duration {
	if e2eSeams == "1" {
		if d, err := time.ParseDuration(os.Getenv("NAV_PILOT_E2E_NUDGE_WAIT")); err == nil {
			return d
		}
	}
	return nudgePrepWait
}

// startNudgePrep runs the network half of the survey prompt and the news
// line in the background, while the session runs, so neither holds up the
// return to the shell. ready reports whether that work is done: a session
// shorter than the fetch skips both, and the next session shows them.
func startNudgePrep(client string) (ready func() bool) {
	// Any warning about an ignored URL, now, before the client has the terminal.
	copilotCLIURL()
	newsFeedURL()
	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		wg.Go(func() { prepareSurvey(client) })
		wg.Go(func() { prepareNews(client, newsPrepTimeout) })
		wg.Wait()
	}()
	return func() bool {
		select {
		case <-done:
			return true
		case <-time.After(nudgeWait()):
			return false
		}
	}
}

// cmdNews is nav-pilot news: the newest items in the feed, whatever news is
// set to, since it was asked for. What it lists counts as shown.
func cmdNews(jsonOutput bool) error {
	st := readNewsState()
	items, err := fetchNews(5 * time.Second)
	switch {
	case err == nil:
		st.Items, st.Fetched = items, time.Now()
	case len(st.Items) == 0:
		return fmt.Errorf("fikk ikke hentet nyhetene fra ki-utvikling.nav.no: %w", err)
	default:
		fmt.Fprintf(os.Stderr, "%s Fikk ikke hentet nyhetene fra ki-utvikling.nav.no. Viser sakene nav-pilot hentet %s.\n", yellow("⚠"), st.Fetched.Local().Format("2.1.2006 15:04"))
	}
	list := slices.Clone(st.Items)
	if len(list) > 10 {
		list = list[:10]
	}
	if list == nil {
		list = []newsItem{}
	}
	for _, it := range list {
		if !slices.Contains(st.Seen, it.URL) {
			st.Seen = append(st.Seen, it.URL)
		}
	}
	writeNewsState(st)
	if jsonOutput {
		return outputJSON(list)
	}
	if len(list) == 0 {
		fmt.Println("Ingen nyheter akkurat nå.")
		return nil
	}
	for _, it := range list {
		fmt.Printf("%s  %s\n            %s\n", it.Date, bold(it.Title), dim(it.URL))
	}
	return nil
}
