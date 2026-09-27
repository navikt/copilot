package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// In-CLI user surveys (#1023). After an interactive session ends, and only
// then, nav-pilot may ask whether you want to answer a short survey defined
// on copilot-cli (GET /api/v1/surveys/active). Answer now, later or never;
// "later" asks again after surveyAskGap(), at most surveyMaxAsks times per
// survey. Answers go to copilot-cli with the GitHub sign-in from
// `nav-pilot auth login`; the server keeps a per-survey hash of the account,
// never the account.
//
// Everything here is off without a terminal, in CI, on a launch with client
// args after --, with surveys = false, and with telemetry opted out
// (DO_NOT_TRACK, NAV_PILOT_TELEMETRY_ENABLED=false). With no open survey, or
// copilot-cli out of reach, it does nothing, so it can ship before any survey
// exists.

const (
	surveyMaxAsks    = 3
	surveyFetchEvery = 24 * time.Hour
)

// surveyAskGap is how long "later" waits. The e2e suite's build takes
// NAV_PILOT_E2E_SURVEY_GAP instead, so a journey can ask three times in a row.
func surveyAskGap() time.Duration {
	if e2eSeams == "1" {
		if d, err := time.ParseDuration(os.Getenv("NAV_PILOT_E2E_SURVEY_GAP")); err == nil {
			return d
		}
	}
	return 72 * time.Hour
}

type surveyDef struct {
	ID        string           `json:"id"`
	Nudge     string           `json:"nudge,omitempty"` // calm (default), start or off
	Title     string           `json:"title"`
	Intro     string           `json:"intro,omitempty"`
	Starts    string           `json:"starts"`
	Ends      string           `json:"ends"`
	Questions []surveyQuestion `json:"questions"`
}

// surveyQuestion is the part of copilot-cli's question format nav-pilot
// renders (apps/copilot-cli/surveys/README.md); the analysis fields are not
// needed here.
type surveyQuestion struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"` // scale, choice, multi or text
	Text       string   `json:"text"`
	Required   bool     `json:"required,omitempty"`
	Min        int      `json:"min,omitempty"`
	Max        int      `json:"max,omitempty"`
	Labels     []string `json:"labels,omitempty"`
	Options    []string `json:"options,omitempty"`
	MaxChoices int      `json:"max_choices,omitempty"`
	MaxLen     int      `json:"max_length,omitempty"`
	SkipIf     *struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	} `json:"skip_if,omitempty"`
}

func (s surveyDef) openOn(now time.Time) bool {
	day := now.UTC().Format(time.DateOnly)
	return s.Starts <= day && day <= s.Ends
}

// surveyState is ~/.nav-pilot/surveys.json: the cached definitions and, per
// survey id, how often it was asked and how it ended.
type surveyState struct {
	Fetched time.Time                `json:"fetched"`
	Active  []surveyDef              `json:"active"`
	Surveys map[string]*surveyRecord `json:"surveys"`
}

type surveyRecord struct {
	Asks    int       `json:"asks"`
	NextAsk time.Time `json:"next_ask,omitzero"`
	// Done is answered, never or exhausted (asked surveyMaxAsks times).
	Done string `json:"done,omitempty"`
	// Pending is an answered submission not delivered yet: sent again at the
	// next calm moment until copilot-cli takes it or refuses it for good.
	Pending json.RawMessage `json:"pending,omitempty"`
}

// surveyPayload is everything that leaves the machine: the answers and four
// enums about the setup. No device id, no user name, no path, no content.
type surveyPayload struct {
	Answers map[string]any `json:"answers"`
	Context surveyContext  `json:"context"`
}

type surveyContext struct {
	Version     string `json:"version"`
	OS          string `json:"os"`
	Client      string `json:"client"`
	LocalModels bool   `json:"local_models"`
}

// surveyCollected is shown before any question, so nobody answers without
// knowing what is sent.
const surveyCollected = "Sent: your answers, nav-pilot version, OS, which client you use, and whether local models are on.\n" +
	"Not sent: your name, GitHub user, device id, code or anything from your sessions.\n" +
	"Your GitHub sign-in only stops a second answer. Nothing stored links the answers to you,\n" +
	"so they cannot be changed or withdrawn afterwards."

func surveyStatePath() (string, error) {
	dir, err := telemetrypkg.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "surveys.json"), nil
}

func readSurveyState() surveyState {
	st := surveyState{Surveys: map[string]*surveyRecord{}}
	path, err := surveyStatePath()
	if err != nil {
		return st
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	if st.Surveys == nil {
		st.Surveys = map[string]*surveyRecord{}
	}
	return st
}

func writeSurveyState(st surveyState) {
	path, err := surveyStatePath()
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	// A temp file of its own: two nav-pilots ending at once must not write
	// into one temp file and rename a half-written state into place.
	f, err := os.CreateTemp(filepath.Dir(path), "surveys-*.json")
	if err != nil {
		return
	}
	_, werr := f.Write(data)
	if cerr := f.Close(); werr != nil || cerr != nil || os.Rename(f.Name(), path) != nil {
		_ = os.Remove(f.Name())
	}
}

// surveysAllowed is every gate that does not need the network.
func surveysAllowed(r ResolvedConfig) bool {
	return r.Surveys && isInteractive() && telemetrypkg.TelemetryEnabled()
}

// surveyToken is the GitHub token from `nav-pilot auth login`, or "" when
// there is none or it is known to have expired. The e2e suite's build reads
// NAV_PILOT_E2E_GITHUB_TOKEN instead, so a journey never touches a keychain.
func surveyToken() string {
	if e2eSeams == "1" {
		return os.Getenv("NAV_PILOT_E2E_GITHUB_TOKEN")
	}
	t, err := loadToken()
	if err != nil || t.expired() {
		return ""
	}
	return t.AccessToken
}

// canSignIn reports whether answers could be sent: a stored token, or a
// GitHub App to log in with.
func canSignIn() bool {
	return surveyToken() != "" || navPilotGitHubClientID() != navPilotGitHubClientIDDefault
}

// maybeSurvey runs at the calm moment after an interactive session. It never
// returns an error: a survey must not change how nav-pilot exits.
func maybeSurvey(client string) {
	cfg, _ := readConfig()
	r := resolve(cfg, CLIOverrides{Client: client})
	if !surveysAllowed(r) {
		return
	}
	st := readSurveyState()
	now := time.Now()
	hasPending := slices.ContainsFunc(slices.Collect(maps.Values(st.Surveys)), func(r *surveyRecord) bool { return len(r.Pending) > 0 })
	fetchDue := now.Sub(st.Fetched) >= surveyFetchEvery
	// Nothing to do: no keychain read. Nothing can be sent without a
	// sign-in, so no network without one either.
	if (!hasPending && !fetchDue && nextSurvey(st, now, "calm") == nil) || !canSignIn() {
		return
	}
	base := copilotCLIURL()

	dropStalePending(st, now)
	deliverPending(base, st)

	if fetchDue {
		// Out of reach (no naisdevice, offline) is the same as no survey,
		// and is not retried until the next day.
		if active, err := fetchActiveSurveys(base); err == nil {
			st.Active = active
		}
		st.Fetched = now
		writeSurveyState(st)
	}

	s := nextSurvey(st, now, "calm")
	if s == nil {
		return
	}
	rec := countAsk(st, s.ID, now)

	switch askSurvey(*s, rec.Asks) {
	case "never":
		rec.Done = "never"
		fmt.Println(dim("  nav-pilot won't ask about this survey again. To turn off all surveys: nav-pilot config set surveys false"))
	case "now":
		answerSurvey(r, st, *s, base)
		return
	default:
		if rec.Done == "exhausted" {
			fmt.Println(dim("  That was the last time nav-pilot asks about this survey. You can still answer it with nav-pilot survey."))
		} else {
			fmt.Println(dim("  nav-pilot will ask again in a few days."))
		}
	}
	writeSurveyState(st)
}

// nudgeOf is where a survey is brought up by itself: calm unless set.
func (s surveyDef) nudgeOf() string {
	if s.Nudge == "" {
		return "calm"
	}
	return s.Nudge
}

// nextSurvey is the first open survey with this nudge that may be brought up
// now: not answered or declined, asked fewer than surveyMaxAsks times, and not
// asked again before its wait is over.
func nextSurvey(st surveyState, now time.Time, nudge string) *surveyDef {
	for i := range st.Active {
		d := st.Active[i]
		rec := st.Surveys[d.ID]
		if d.nudgeOf() == nudge && d.openOn(now) && d.renderable() &&
			(rec == nil || (rec.Done == "" && rec.Asks < surveyMaxAsks && !now.Before(rec.NextAsk))) {
			return &d
		}
	}
	return nil
}

// countAsk records that the survey is being brought up, before it is shown:
// a prompt killed with the terminal still used up one of the three.
func countAsk(st surveyState, id string, now time.Time) *surveyRecord {
	rec := st.Surveys[id]
	if rec == nil {
		rec = &surveyRecord{}
		st.Surveys[id] = rec
	}
	rec.Asks++
	rec.NextAsk = now.Add(surveyAskGap())
	if rec.Asks >= surveyMaxAsks {
		rec.Done = "exhausted"
	}
	writeSurveyState(st)
	return rec
}

// answerSurvey runs the questions and sends the answers. false means the
// form was left before the end, and nothing was sent.
func answerSurvey(r ResolvedConfig, st surveyState, s surveyDef, base string) bool {
	answers, ok := runSurveyForm(s)
	if !ok {
		fmt.Println(dim("  Not sent."))
		writeSurveyState(st)
		return false
	}
	rec := st.Surveys[s.ID]
	if rec == nil {
		rec = &surveyRecord{}
		st.Surveys[s.ID] = rec
	}
	rec.Done = "answered"
	payload, _ := json.Marshal(surveyPayload{Answers: answers, Context: surveyContextNow(r)})
	rec.Pending = payload
	writeSurveyState(st)
	sendAnswered(base, st, s.ID)
	return true
}

// maybeSurveyHint is the start nudge: one line on stderr as a session starts,
// for a survey whose definition asks for it. It reads only the cached
// definitions, so it never touches the network or delays the launch, and it
// counts as one of the three asks.
func maybeSurveyHint(client string) {
	cfg, _ := readConfig()
	if !surveysAllowed(resolve(cfg, CLIOverrides{Client: client})) {
		return
	}
	st := readSurveyState()
	now := time.Now()
	// The keychain only when there is something to show: reading it can
	// start a subprocess or an unlock dialog.
	s := nextSurvey(st, now, "start")
	if s == nil || !canSignIn() {
		return
	}
	countAsk(st, s.ID, now)
	fmt.Fprintf(os.Stderr, "%s User survey: %s (%d questions). Answer it any time with %s\n\n", dim("ℹ"), s.Title, len(s.Questions), bold("nav-pilot survey"))
}

// cmdSurvey is nav-pilot survey: lists the open surveys, and in a terminal
// opens the chosen one (or the only one). It is asked for, so it runs even
// with automatic prompts turned off and after the three asks, but the server
// still refuses a second answer.
func cmdSurvey(jsonOutput bool) error {
	st := readSurveyState()
	base := copilotCLIURL()
	now := time.Now()
	if active, err := fetchActiveSurveys(base); err == nil {
		st.Active, st.Fetched = active, now
		writeSurveyState(st)
	} else if len(st.Active) == 0 {
		return fmt.Errorf("could not reach copilot-cli to find open surveys (naisdevice on?): %w", err)
	}
	var open []surveyDef
	for _, d := range st.Active {
		switch {
		case !d.openOn(now):
		case !d.renderable():
			fmt.Fprintf(os.Stderr, "%s %s needs a newer nav-pilot: %s\n", yellow("⚠"), d.Title, bold("nav-pilot upgrade"))
		default:
			open = append(open, d)
		}
	}
	if jsonOutput || !isInteractive() {
		if jsonOutput {
			list := []map[string]any{}
			for _, d := range open {
				list = append(list, map[string]any{"id": d.ID, "title": d.Title, "ends": d.Ends, "questions": len(d.Questions)})
			}
			return outputJSON(list)
		}
		if len(open) == 0 {
			fmt.Println("No open surveys right now.")
			return nil
		}
		for _, d := range open {
			fmt.Printf("%s  %s (%d questions, open until %s)\n", d.ID, d.Title, len(d.Questions), d.Ends)
		}
		fmt.Println(dim("Run nav-pilot survey in a terminal to answer."))
		return nil
	}
	if len(open) == 0 {
		fmt.Println("No open surveys right now.")
		return nil
	}
	if !canSignIn() {
		return fmt.Errorf("answering needs a GitHub sign-in: run %s", bold("nav-pilot auth login"))
	}
	deliverPending(base, st)
	s := open[0]
	if len(open) > 1 {
		var opts []huh.Option[string]
		for _, d := range open {
			opts = append(opts, huh.NewOption(d.Title, d.ID))
		}
		id := open[0].ID
		if err := runField(huh.NewSelect[string]().Title("Which survey?").Options(opts...).Value(&id)); err != nil {
			return cancelledError{}
		}
		s = open[slices.IndexFunc(open, func(d surveyDef) bool { return d.ID == id })]
	}
	fmt.Printf("\n%s (%d questions)\n", bold(s.Title), len(s.Questions))
	if s.Intro != "" {
		fmt.Println(s.Intro)
	}
	fmt.Println(dim(surveyCollected))
	fmt.Println()
	cfg, _ := readConfig()
	if !answerSurvey(resolve(cfg, CLIOverrides{}), st, s, base) {
		return cancelledError{}
	}
	return nil
}

// askSurvey is the answer now / later / never question. Esc and Ctrl-C mean
// later.
func askSurvey(s surveyDef, ask int) string {
	desc := surveyCollected
	if s.Intro != "" {
		desc = s.Intro + "\n\n" + desc
	}
	choice := "later"
	err := runField(huh.NewSelect[string]().
		Title(fmt.Sprintf("User survey: %s (%d questions, asked %d of %d)", s.Title, len(s.Questions), ask, surveyMaxAsks)).
		Description(desc).
		Options(
			huh.NewOption("Answer now", "now"),
			huh.NewOption("Later", "later"),
			huh.NewOption("Never for this survey", "never"),
		).
		Value(&choice))
	if err != nil {
		return "later"
	}
	return choice
}

// runSurveyForm asks the questions, one per screen, skipping those whose
// skip_if matches an earlier answer. false means the form was left before the
// end.
func runSurveyForm(s surveyDef) (map[string]any, bool) {
	picks := make([]string, len(s.Questions))
	multis := make([][]string, len(s.Questions))
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
	var groups []*huh.Group
	for i, q := range s.Questions {
		var field huh.Field
		switch q.Type {
		case "scale", "choice":
			var opts []huh.Option[string]
			if q.Type == "scale" {
				for n := q.Min; n <= q.Max; n++ {
					label := strconv.Itoa(n)
					if len(q.Labels) == q.Max-q.Min+1 {
						label += "  " + q.Labels[n-q.Min]
					}
					opts = append(opts, huh.NewOption(label, strconv.Itoa(n)))
				}
			} else {
				for _, o := range q.Options {
					opts = append(opts, huh.NewOption(o, o))
				}
			}
			if !q.Required {
				opts = append(opts, huh.NewOption("Skip", ""))
			}
			field = huh.NewSelect[string]().Title(q.Text).Options(opts...).Value(&picks[i])
		case "multi":
			ms := huh.NewMultiSelect[string]().Title(q.Text).Options(huh.NewOptions(q.Options...)...).Value(&multis[i])
			desc := "Space to pick, enter when done."
			if q.MaxChoices > 0 {
				ms = ms.Limit(q.MaxChoices)
				desc = fmt.Sprintf("Pick up to %d. %s", q.MaxChoices, desc)
			}
			if q.Required {
				ms = ms.Validate(func(v []string) error {
					if len(v) == 0 {
						return errors.New("pick at least one")
					}
					return nil
				})
			}
			field = ms.Description(desc)
		case "text":
			in := huh.NewText().Title(q.Text).CharLimit(q.MaxLen).Value(&picks[i])
			if q.Required {
				in = in.Validate(func(v string) error {
					if strings.TrimSpace(v) == "" {
						return errors.New("write an answer")
					}
					return nil
				})
			}
			desc := "Write nothing that identifies you or anyone else."
			if !q.Required {
				desc += " Optional: leave empty to skip."
			}
			field = in.Description(desc)
		}
		g := huh.NewGroup(escHelpField{field})
		if q.SkipIf != nil {
			g = g.WithHideFunc(func() bool { return skipped(q) })
		}
		groups = append(groups, g)
	}
	if huh.NewForm(groups...).WithShowHelp(true).WithTheme(navTheme()).Run() != nil {
		return nil, false
	}
	answers := map[string]any{}
	for i, q := range s.Questions {
		v := strings.TrimSpace(picks[i])
		switch {
		case skipped(q):
		case q.Type == "multi":
			if len(multis[i]) > 0 {
				answers[q.ID] = multis[i]
			}
		case v == "":
		case q.Type == "scale":
			n, _ := strconv.Atoi(v)
			answers[q.ID] = n
		default:
			answers[q.ID] = v
		}
	}
	return answers, len(answers) > 0
}

// renderable reports whether this version can show every question.
func (s surveyDef) renderable() bool {
	return !slices.ContainsFunc(s.Questions, func(q surveyQuestion) bool {
		return !slices.Contains([]string{"scale", "choice", "multi", "text"}, q.Type)
	})
}

func surveyContextNow(r ResolvedConfig) surveyContext {
	c := surveyContext{Version: Version, OS: "other", Client: "none"}
	if !versionShape(Version) {
		c.Version = "dev"
	}
	if slices.Contains([]string{"darwin", "linux", "windows"}, runtime.GOOS) {
		c.OS = runtime.GOOS
	}
	if slices.Contains([]string{"copilot", "opencode", "pi"}, r.Client) {
		c.Client = r.Client
	}
	c.LocalModels = r.LocalEnabled
	return c
}

// versionShape is a version copilot-cli accepts (its versionPattern), such
// as a release's 2026.09.24-120000-abc1234. Anything else, a local build's
// "dev" or a hand-set string, is sent as dev rather than refused.
var versionShapePattern = regexp.MustCompile(`^v?\d{1,4}\.\d{1,4}\.\d{1,4}(-[0-9A-Za-z.-]{1,40})?$`)

func versionShape(v string) bool { return versionShapePattern.MatchString(v) }

func fetchActiveSurveys(base string) ([]surveyDef, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/surveys/active", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("surveys: %s", resp.Status)
	}
	var body struct {
		Surveys []surveyDef `json:"surveys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&body); err != nil {
		return nil, err
	}
	return body.Surveys, nil
}

// sendAnswered delivers the survey's pending answers and says how it went.
func sendAnswered(base string, st surveyState, id string) {
	rec := st.Surveys[id]
	status, err := postSurvey(base, id, rec.Pending, true)
	switch {
	case status == http.StatusCreated:
		rec.Pending = nil
		fmt.Println(green("  ✓ ") + "Thank you! Your answers were sent.")
	case status == http.StatusConflict:
		rec.Pending = nil
		fmt.Println(dim("  You have already answered this survey, so these answers were not sent. Thank you!"))
	case status == http.StatusUnauthorized && surveyToken() == "":
		fmt.Println(yellow("  ⚠ ") + "Answers saved. Log in with " + bold("nav-pilot auth login") + " and nav-pilot sends them after your next session.")
	case status == http.StatusUnauthorized:
		fmt.Println(yellow("  ⚠ ") + "copilot-cli did not accept your GitHub sign-in. Answers saved. Run " + bold("nav-pilot auth login") + " and nav-pilot sends them after your next session.")
	case status >= 400 && status < 500:
		rec.Pending = nil
		fmt.Println(yellow("  ⚠ ") + "copilot-cli refused the answers" + surveyErrDetail(err) + ". Sorry, they were not saved.")
	case status >= 500 || status == http.StatusTooManyRequests:
		fmt.Println(dim("  copilot-cli could not take the answers now" + surveyErrDetail(err) + ". Answers saved. nav-pilot sends them after your next session."))
	default:
		fmt.Println(dim("  Could not reach copilot-cli (naisdevice on?). Answers saved. nav-pilot sends them after your next session."))
	}
	writeSurveyState(st)
}

// dropStalePending forgets answers to a survey that has closed: they can no
// longer be sent, and the free text should not sit on disk.
func dropStalePending(st surveyState, now time.Time) {
	changed := false
	for id, rec := range st.Surveys {
		if len(rec.Pending) == 0 {
			continue
		}
		open := slices.ContainsFunc(st.Active, func(d surveyDef) bool { return d.ID == id && d.openOn(now) })
		if !open {
			rec.Pending = nil
			changed = true
		}
	}
	if changed {
		writeSurveyState(st)
	}
}

// deliverPending quietly retries answers an earlier session could not send.
func deliverPending(base string, st surveyState) {
	changed := false
	for id, rec := range st.Surveys {
		if len(rec.Pending) == 0 {
			continue
		}
		status, _ := postSurvey(base, id, rec.Pending, false)
		if status == http.StatusCreated || status == http.StatusConflict || (status >= 400 && status < 500 && status != http.StatusUnauthorized && status != http.StatusTooManyRequests) {
			rec.Pending = nil
			changed = true
		}
	}
	if changed {
		writeSurveyState(st)
	}
}

// postSurvey sends a submission and returns the status, 0 when copilot-cli
// was not reached. It retries a network failure, a 429 and a 5xx: three
// tries when the user is waiting for the answer, one when not. A missing
// sign-in runs the login when interactive is true.
func postSurvey(base, id string, payload []byte, interactive bool) (int, error) {
	token := surveyToken()
	if token == "" {
		if !interactive || navPilotGitHubClientID() == navPilotGitHubClientIDDefault {
			return http.StatusUnauthorized, nil
		}
		fmt.Println(dim("  Sign in with GitHub once to send your answers."))
		if err := cmdAuthLogin(); err != nil {
			return http.StatusUnauthorized, err
		}
		if token = surveyToken(); token == "" {
			return http.StatusUnauthorized, nil
		}
	}
	tries := 1
	if interactive {
		tries = 3
	}
	var lastErr error
	var lastStatus int
	for i := range tries {
		if i > 0 {
			time.Sleep(time.Duration(i) * time.Second)
		}
		status, err := postSurveyOnce(base, id, payload, token)
		if status != 0 && status != http.StatusTooManyRequests && status < 500 {
			return status, err
		}
		lastStatus, lastErr = status, err
	}
	return lastStatus, lastErr
}

func postSurveyOnce(base, id string, payload []byte, token string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/surveys/"+url.PathEscape(id)+"/responses", bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var body cliErrorBody
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&body)
		return resp.StatusCode, errors.New(body.detail())
	}
	return resp.StatusCode, nil
}

func surveyErrDetail(err error) string {
	if err == nil || err.Error() == "" {
		return ""
	}
	return " (" + err.Error() + ")"
}
