package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
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
	Title     string           `json:"title"`
	Intro     string           `json:"intro,omitempty"`
	Starts    string           `json:"starts"`
	Ends      string           `json:"ends"`
	Questions []surveyQuestion `json:"questions"`
}

type surveyQuestion struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"` // scale, choice or text
	Text     string   `json:"text"`
	Required bool     `json:"required,omitempty"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	Options  []string `json:"options,omitempty"`
	MaxLen   int      `json:"max_length,omitempty"`
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

// surveyPayload is everything that leaves the machine: the answers and five
// enums about the setup. No device id, no user name, no path, no content.
type surveyPayload struct {
	Answers map[string]any `json:"answers"`
	Context surveyContext  `json:"context"`
}

type surveyContext struct {
	Version     string `json:"version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Client      string `json:"client"`
	LocalModels bool   `json:"local_models"`
}

// surveyCollected is shown before any question, so nobody answers without
// knowing what is sent.
const surveyCollected = "Sent: your answers, nav-pilot version, OS and CPU type, which client you use, and whether local models are on.\n" +
	"Not sent: your name, GitHub user, device id, code or anything from your sessions.\n" +
	"You sign in with GitHub so each person answers once; copilot-cli stores only a per-survey hash of the account."

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
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, path)
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
	base := copilotCLIURL()
	now := time.Now()

	deliverPending(base, st)

	if now.Sub(st.Fetched) >= surveyFetchEvery {
		// Out of reach (no naisdevice, offline) is the same as no survey,
		// and is not retried until the next day.
		if active, err := fetchActiveSurveys(base); err == nil {
			st.Active = active
		}
		st.Fetched = now
		writeSurveyState(st)
	}

	var s *surveyDef
	for i := range st.Active {
		d := st.Active[i]
		rec := st.Surveys[d.ID]
		if d.openOn(now) && (rec == nil || (rec.Done == "" && rec.Asks < surveyMaxAsks && !now.Before(rec.NextAsk))) {
			s = &d
			break
		}
	}
	if s == nil || !canSignIn() {
		return
	}
	rec := st.Surveys[s.ID]
	if rec == nil {
		rec = &surveyRecord{}
		st.Surveys[s.ID] = rec
	}
	// Counted before it is shown: a prompt killed with the terminal still
	// used up one of the three.
	rec.Asks++
	rec.NextAsk = now.Add(surveyAskGap())
	if rec.Asks >= surveyMaxAsks {
		rec.Done = "exhausted"
	}
	writeSurveyState(st)

	choice := askSurvey(*s, rec.Asks)
	switch choice {
	case "never":
		rec.Done = "never"
		fmt.Println(dim("  Understood: nav-pilot won't ask about this survey again. Turn surveys off entirely: nav-pilot config set surveys false"))
	case "now":
		answers, ok := runSurveyForm(*s)
		if !ok {
			fmt.Println(dim("  Not sent. nav-pilot may ask again later."))
			break
		}
		rec.Done = "answered"
		payload, _ := json.Marshal(surveyPayload{Answers: answers, Context: surveyContextNow(r)})
		rec.Pending = payload
		writeSurveyState(st)
		st = sendAnswered(base, st, s.ID)
		return
	default:
		if rec.Done == "exhausted" {
			fmt.Println(dim("  That was the last time nav-pilot asks about this survey."))
		} else {
			fmt.Println(dim("  nav-pilot will ask again in a few days."))
		}
	}
	writeSurveyState(st)
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

// runSurveyForm asks the questions, one per screen. false means the form was
// left before the end.
func runSurveyForm(s surveyDef) (map[string]any, bool) {
	picks := make([]string, len(s.Questions))
	var groups []*huh.Group
	for i, q := range s.Questions {
		var field huh.Field
		switch q.Type {
		case "scale", "choice":
			var opts []huh.Option[string]
			if q.Type == "scale" {
				for n := q.Min; n <= q.Max; n++ {
					opts = append(opts, huh.NewOption(strconv.Itoa(n), strconv.Itoa(n)))
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
		case "text":
			in := huh.NewText().Title(q.Text).CharLimit(q.MaxLen).Value(&picks[i])
			if q.Required {
				in = in.Validate(func(v string) error {
					if len(bytes.TrimSpace([]byte(v))) == 0 {
						return errors.New("an answer is needed")
					}
					return nil
				})
			} else {
				in = in.Description("Optional. Leave empty to skip.")
			}
			field = in
		default:
			continue // a type this version doesn't know: leave it unanswered
		}
		groups = append(groups, huh.NewGroup(escHelpField{field}))
	}
	if huh.NewForm(groups...).WithShowHelp(true).WithTheme(navTheme()).Run() != nil {
		return nil, false
	}
	answers := map[string]any{}
	for i, q := range s.Questions {
		v := picks[i]
		switch {
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

func surveyContextNow(r ResolvedConfig) surveyContext {
	c := surveyContext{Version: Version, OS: "other", Arch: "other", Client: "none"}
	if !versionShape(Version) {
		c.Version = "dev"
	}
	if slices.Contains([]string{"darwin", "linux", "windows"}, runtime.GOOS) {
		c.OS = runtime.GOOS
	}
	if slices.Contains([]string{"amd64", "arm64"}, runtime.GOARCH) {
		c.Arch = runtime.GOARCH
	}
	if slices.Contains([]string{"copilot", "opencode", "pi"}, r.Client) {
		c.Client = r.Client
	}
	c.LocalModels = r.LocalEnabled
	return c
}

// versionShape is a release version (2026.09.24-120000-abc1234): anything
// else, a local build's "dev" or a hand-set string, is sent as dev.
func versionShape(v string) bool {
	if len(v) == 0 || len(v) > 40 || v[0] < '0' || v[0] > '9' {
		return false
	}
	for _, r := range v {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}

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
func sendAnswered(base string, st surveyState, id string) surveyState {
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
		fmt.Println(yellow("  ⚠ ") + "Answers saved. Log in with " + bold("nav-pilot auth login") + " and they are sent after your next session.")
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		fmt.Println(yellow("  ⚠ ") + "copilot-cli did not accept your GitHub sign-in. Answers saved; run " + bold("nav-pilot auth login") + " and they are sent after your next session.")
	case status >= 400 && status < 500:
		rec.Pending = nil
		fmt.Println(yellow("  ⚠ ") + "copilot-cli refused the answers" + surveyErrDetail(err) + ". Sorry, they were not saved.")
	default:
		fmt.Println(dim("  Could not reach copilot-cli (naisdevice on?). Answers saved; nav-pilot sends them after your next session."))
	}
	writeSurveyState(st)
	return st
}

// deliverPending quietly retries answers an earlier session could not send.
func deliverPending(base string, st surveyState) {
	changed := false
	for id, rec := range st.Surveys {
		if len(rec.Pending) == 0 {
			continue
		}
		status, _ := postSurvey(base, id, rec.Pending, false)
		if status == http.StatusCreated || status == http.StatusConflict || (status >= 400 && status < 500 && status != http.StatusUnauthorized && status != http.StatusForbidden && status != http.StatusTooManyRequests) {
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
		fmt.Println(dim("  Sending needs a GitHub sign-in (once)."))
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
	for i := range tries {
		if i > 0 {
			time.Sleep(time.Duration(i) * time.Second)
		}
		status, err := postSurveyOnce(base, id, payload, token)
		if status != 0 && status != http.StatusTooManyRequests && status < 500 {
			return status, err
		}
		lastErr = err
	}
	return 0, lastErr
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
