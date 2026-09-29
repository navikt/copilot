package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
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

// surveyQuestion is the part of copilot-survey's question format nav-pilot
// renders (apps/copilot-survey/surveys/README.md); the analysis fields are not
// needed here.
type surveyQuestion struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"` // scale, choice, multi, text or matrix
	Text       string   `json:"text"`
	Required   bool     `json:"required,omitempty"`
	Min        int      `json:"min,omitempty"`
	Max        int      `json:"max,omitempty"`
	Labels     []string `json:"labels,omitempty"`
	Options    []string `json:"options,omitempty"`
	MaxChoices int      `json:"max_choices,omitempty"`
	Other      string   `json:"other,omitempty"` // one more option, with a short free text sent as <id>.other
	MaxLen     int      `json:"max_length,omitempty"`
	SkipIf     *struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	} `json:"skip_if,omitempty"`
	// Items are a matrix's statements on its scale, asked one per screen and
	// sent as scale answers under their own ids.
	Items []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"items,omitempty"`
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
const surveyCollected = "Dette sendes: svarene dine, nav-pilot-versjon, operativsystem, hvilken klient du\n" +
	"bruker og om lokale modeller er på.\n" +
	"Dette sendes ikke: navn, GitHub-bruker, enhets-ID, kode eller noe fra øktene dine.\n" +
	"GitHub-innloggingen brukes bare til å hindre at noen svarer to ganger. Svarene\n" +
	"lagres uten noe som knytter dem til deg, så de kan ikke endres eller trekkes\n" +
	"tilbake etterpå."

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
	writeStateFile(path, st)
}

// writeStateFile writes v as JSON to path, best effort: a state file that
// could not be written only means nav-pilot may say something again.
func writeStateFile(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	// A temp file of its own: two nav-pilots ending at once must not write
	// into one temp file and rename a half-written state into place.
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
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
	return r.Surveys && nudgesAllowed()
}

// nudgesAllowed is what the survey prompt and the news line both need:
// someone at a terminal, not CI, and telemetry not opted out.
func nudgesAllowed() bool {
	return isInteractive() && telemetrypkg.TelemetryEnabled()
}

// surveyToken is the GitHub token from `nav-pilot auth login`, or "" when
// there is none or it is known to have expired. The e2e suite's build reads
// NAV_PILOT_E2E_GITHUB_TOKEN instead, so a journey never touches a keychain.
func surveyToken() string {
	if e2eSeams == "1" {
		return os.Getenv("NAV_PILOT_E2E_GITHUB_TOKEN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, err := currentToken(ctx)
	if err != nil || t.expired() {
		return ""
	}
	return t.AccessToken
}

// canSignIn reports whether answers could be sent: a stored token, or a
// GitHub App to log in with. The App first: it runs at launch, and
// surveyToken may renew the token over the network.
func canSignIn() bool {
	return hasGitHubApp() || surveyToken() != ""
}

// prepareSurvey is the network half of the survey prompt after a session: it sends answers an
// earlier session could not, and fetches the open surveys once a day. It
// prints nothing, so a launch runs it while the session has the terminal
// (startNudgePrep) and the prompt afterwards does not wait on the network.
func prepareSurvey(client string) {
	cfg, _ := readConfig()
	r := resolve(cfg, CLIOverrides{Client: client})
	if !surveysAllowed(r) {
		return
	}
	st := readSurveyState()
	now := time.Now()
	hasPending := slices.ContainsFunc(slices.Collect(maps.Values(st.Surveys)), func(r *surveyRecord) bool { return len(r.Pending) > 0 })
	fetchDue := now.Sub(st.Fetched) >= surveyFetchEvery
	// Nothing to do: no keychain read.
	if !hasPending && !fetchDue && nextSurvey(st, now, "calm") == nil {
		return
	}
	// Nothing can be sent without a sign-in, so no network without one.
	// Checked every session, so a new sign-in is noticed at once. Here and
	// not in promptSurvey: renewing the token can take the network.
	if !canSignIn() {
		return
	}
	surveySignedIn.Store(true)
	if !hasPending && !fetchDue {
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
}

// surveySignedIn is prepareSurvey's answer to canSignIn, for promptSurvey.
var surveySignedIn atomic.Bool

// promptSurvey asks about the next open survey at the calm moment after an
// interactive session, from what prepareSurvey fetched. It reads only the
// state file, and never fails: a survey must not change how nav-pilot exits.
func promptSurvey(client string) {
	cfg, _ := readConfig()
	r := resolve(cfg, CLIOverrides{Client: client})
	if !surveysAllowed(r) {
		return
	}
	st := readSurveyState()
	now := time.Now()
	s := nextSurvey(st, now, "calm")
	if s == nil || !surveySignedIn.Load() || !claimSessionPrompt() {
		return
	}
	base := copilotCLIURL()
	rec := countAsk(st, s.ID, now)

	switch askSurvey(*s, rec.Asks) {
	case "never":
		rec.Done = "never"
		fmt.Println(dim("  nav-pilot spør ikke om denne undersøkelsen igjen. Slå av alle undersøkelser: nav-pilot config set surveys false"))
	case "now":
		answerSurvey(r, st, *s, base)
		return
	default:
		if rec.Done == "exhausted" {
			fmt.Println(dim("  Dette var siste gang nav-pilot spurte om denne undersøkelsen. Du kan fortsatt svare med nav-pilot survey."))
		} else {
			fmt.Println(dim("  nav-pilot spør igjen om noen dager."))
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
		fmt.Println(dim("  Ikke sendt."))
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
	if s == nil || sessionPrompted || !canSignIn() || !claimSessionPrompt() {
		return
	}
	countAsk(st, s.ID, now)
	fmt.Fprintf(os.Stderr, "%s Brukerundersøkelse: %s (%d spørsmål). Svar når det passer deg: %s\n\n", dim("ℹ"), s.Title, s.count(), bold("nav-pilot survey"))
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
	} else if len(st.Active) == 0 && !noSurveyService(err) {
		// Any HTTP status is caught by noSurveyService above; what is left is
		// a transport failure (no route, DNS, connection refused).
		return fmt.Errorf("fikk ikke kontakt med copilot-cli for å finne åpne undersøkelser (er naisdevice på?): %w", err)
	}
	var open []surveyDef
	for _, d := range st.Active {
		switch {
		case !d.openOn(now):
		case !d.renderable():
			fmt.Fprintf(os.Stderr, "%s %s krever en nyere nav-pilot: %s\n", yellow("⚠"), d.Title, bold("nav-pilot upgrade"))
		default:
			open = append(open, d)
		}
	}
	if jsonOutput || !isInteractive() {
		if jsonOutput {
			list := []map[string]any{}
			for _, d := range open {
				list = append(list, map[string]any{"id": d.ID, "title": d.Title, "ends": d.Ends, "questions": d.count()})
			}
			return outputJSON(list)
		}
		if len(open) == 0 {
			fmt.Println("Ingen åpne undersøkelser akkurat nå.")
			return nil
		}
		for _, d := range open {
			fmt.Printf("%s  %s (%d spørsmål, åpen til %s)\n", d.ID, d.Title, d.count(), d.Ends)
		}
		fmt.Println(dim("Kjør nav-pilot survey i en terminal for å svare."))
		return nil
	}
	if len(open) == 0 {
		fmt.Println("Ingen åpne undersøkelser akkurat nå.")
		return nil
	}
	if !canSignIn() {
		return fmt.Errorf("du må logge inn med GitHub for å svare: kjør %s", bold("nav-pilot auth login"))
	}
	deliverPending(base, st)
	s := open[0]
	if len(open) > 1 {
		var opts []huh.Option[string]
		for _, d := range open {
			opts = append(opts, huh.NewOption(d.Title, d.ID))
		}
		id := open[0].ID
		if err := runField(huh.NewSelect[string]().Title("Hvilken undersøkelse?").Options(opts...).Value(&id)); err != nil {
			return cancelledError{}
		}
		s = open[slices.IndexFunc(open, func(d surveyDef) bool { return d.ID == id })]
	}
	fmt.Printf("\n%s (%d spørsmål)\n", bold(s.Title), s.count())
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
		Title(fmt.Sprintf("Brukerundersøkelse: %s (%d spørsmål, spurt %d av %d ganger)", s.Title, s.count(), ask, surveyMaxAsks)).
		Description(desc).
		Options(
			huh.NewOption("Svar nå", "now"),
			huh.NewOption("Senere", "later"),
			huh.NewOption("Aldri for denne undersøkelsen", "never"),
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
	others := make([]string, len(s.Questions))
	grid := make([][]string, len(s.Questions))
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
		case "matrix":
			grid[i] = make([]string, len(q.Items))
			for j, it := range q.Items {
				g := huh.NewGroup(escHelpField{surveySelect(q, fmt.Sprintf("%s (%d av %d)", q.Text, j+1, len(q.Items)), &grid[i][j]).Title(it.Text)})
				if q.SkipIf != nil {
					g = g.WithHideFunc(func() bool { return skipped(q) })
				}
				groups = append(groups, g)
			}
			continue
		case "scale", "choice":
			field = surveySelect(q, "", &picks[i]).Title(q.Text)
		case "multi":
			ms := huh.NewMultiSelect[string]().Title(q.Text).Options(huh.NewOptions(q.options()...)...).Value(&multis[i])
			desc := "Mellomrom for å velge, Enter når du er ferdig."
			if q.MaxChoices > 0 {
				ms = ms.Limit(q.MaxChoices)
				desc = fmt.Sprintf("Velg opptil %d. %s", q.MaxChoices, desc)
			}
			if q.Required {
				ms = ms.Validate(func(v []string) error {
					if len(v) == 0 {
						return errors.New("velg minst ett svar")
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
						return errors.New("skriv et svar")
					}
					return nil
				})
			}
			desc := surveyNoIdentify
			if !q.Required {
				desc += " Valgfritt: la feltet stå tomt for å hoppe over."
			}
			field = in.Description(desc)
		}
		g := huh.NewGroup(escHelpField{field})
		if q.SkipIf != nil {
			g = g.WithHideFunc(func() bool { return skipped(q) })
		}
		groups = append(groups, g)
		if q.Other != "" {
			in := huh.NewInput().Title(q.Other + ": skriv gjerne hva du tenker på").CharLimit(q.MaxLen).Value(&others[i]).
				Description(surveyNoIdentify + " Valgfritt: la feltet stå tomt for å hoppe over.")
			groups = append(groups, huh.NewGroup(escHelpField{in}).WithHideFunc(func() bool {
				return skipped(q) || !slices.Contains(answered(q.ID), q.Other)
			}))
		}
	}
	if huh.NewForm(groups...).WithShowHelp(true).WithTheme(navTheme()).Run() != nil {
		return nil, false
	}
	answers := map[string]any{}
	for i, q := range s.Questions {
		v := strings.TrimSpace(picks[i])
		for j, it := range q.Items {
			if n, err := strconv.Atoi(grid[i][j]); err == nil && !skipped(q) {
				answers[it.ID] = n
			}
		}
		if t := strings.TrimSpace(others[i]); t != "" && !skipped(q) && slices.Contains(answered(q.ID), q.Other) {
			answers[q.ID+".other"] = t
		}
		switch {
		case skipped(q):
		case q.Type == "multi":
			if len(multis[i]) > 0 {
				answers[q.ID] = multis[i]
			}
		case v == "" || q.Type == "matrix":
		case q.Type == "scale":
			n, _ := strconv.Atoi(v)
			answers[q.ID] = n
		default:
			answers[q.ID] = v
		}
	}
	return answers, len(answers) > 0
}

// surveyNoIdentify goes with every free-text field.
const surveyNoIdentify = "Skriv ikke noe som kan identifisere deg eller andre."

// surveySelect is a scale, matrix item or choice as a list to pick one from.
// No answer is preselected (#1251): the cursor starts on the entry whose
// value is "", as *value is. For a required question that is a placeholder
// Enter cannot pick, for an optional one "Hopp over".
func surveySelect(q surveyQuestion, desc string, value *string) *huh.Select[string] {
	var opts []huh.Option[string]
	if q.Required {
		opts = append(opts, huh.NewOption(surveyUnpicked, ""))
	}
	if q.Type == "choice" {
		for _, o := range q.options() {
			opts = append(opts, huh.NewOption(o, o))
		}
	} else {
		for n := q.Min; n <= q.Max; n++ {
			label := strconv.Itoa(n)
			if len(q.Labels) == q.Max-q.Min+1 {
				label += "  " + q.Labels[n-q.Min]
			}
			opts = append(opts, huh.NewOption(label, strconv.Itoa(n)))
		}
	}
	if !q.Required {
		opts = append(opts, huh.NewOption("Hopp over", ""))
	}
	sel := huh.NewSelect[string]().Description(desc).Options(opts...).Value(value)
	if q.Required {
		sel = sel.Validate(requirePick)
	}
	return sel
}

// count is how many answers the survey asks for: a matrix counts its items.
func (s surveyDef) count() int {
	n := 0
	for _, q := range s.Questions {
		n += max(1, len(q.Items))
	}
	return n
}

// surveyUnpicked is the entry a required scale or choice question starts on,
// so Enter alone records nothing.
const surveyUnpicked = "(ikke valgt)"

func requirePick(v string) error {
	if v == "" {
		return errors.New("velg et svar med piltastene")
	}
	return nil
}

// options is what a choice or multi question offers: its options, then other.
func (q surveyQuestion) options() []string {
	if q.Other == "" {
		return q.Options
	}
	return append(slices.Clip(q.Options), q.Other)
}

// renderable reports whether this version can show every question.
func (s surveyDef) renderable() bool {
	return !slices.ContainsFunc(s.Questions, func(q surveyQuestion) bool {
		return !slices.Contains([]string{"scale", "choice", "multi", "text", "matrix"}, q.Type)
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

// surveyHTTPError is copilot-cli answering with something other than 200.
// That is a different fault from not reaching it at all, so it must not be
// reported as "naisdevice is off".
type surveyHTTPError struct {
	base   string
	status string
	code   int
}

// noSurveyService is a fetch that means "no survey to answer" rather than a
// fault to report: any non-2xx from copilot-cli (a 404, what Nav's ingress
// answers where the gateway is not deployed; a 5xx, copilot-cli up but
// copilot-survey not; or anything else it might answer), or no answer within
// the timeout. nav-pilot survey then says there is nothing open, as for an
// empty list.
func noSurveyService(err error) bool {
	var httpErr surveyHTTPError
	var netErr net.Error
	return errors.As(err, &httpErr) ||
		errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout())
}

func (e surveyHTTPError) Error() string {
	return fmt.Sprintf("%s svarte %s på /api/v1/surveys/active", e.base, e.status)
}

func fetchActiveSurveys(base string) ([]surveyDef, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/surveys/active", nil)
	if err != nil {
		return nil, err
	}
	// copilot-survey leaves out surveys that need a newer nav-pilot
	// (min_cli_version) by this.
	req.Header.Set("User-Agent", "nav-pilot/"+Version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, surveyHTTPError{base: base, status: resp.Status, code: resp.StatusCode}
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
		fmt.Println(green("  ✓ ") + "Takk! Svaret ditt er sendt.")
	case status == http.StatusConflict:
		rec.Pending = nil
		fmt.Println(dim("  Du har allerede svart på denne undersøkelsen, så dette svaret ble ikke sendt. Takk!"))
	case status == http.StatusUnauthorized && surveyToken() == "":
		fmt.Println(yellow("  ⚠ ") + "Svaret er lagret. Logg inn med " + bold("nav-pilot auth login") + ", så sender nav-pilot det etter neste økt.")
	case status == http.StatusUnauthorized:
		fmt.Println(yellow("  ⚠ ") + "copilot-cli godtok ikke GitHub-innloggingen din. Svaret er lagret. Kjør " + bold("nav-pilot auth login") + ", så sender nav-pilot det etter neste økt.")
	case status >= 400 && status < 500:
		rec.Pending = nil
		fmt.Println(yellow("  ⚠ ") + "copilot-cli avviste svaret" + surveyErrDetail(err) + ". Det ble dessverre ikke lagret.")
	case status >= 500 || status == http.StatusTooManyRequests:
		fmt.Println(dim("  copilot-cli kunne ikke ta imot svaret nå" + surveyErrDetail(err) + ". Svaret er lagret, og nav-pilot sender det etter neste økt."))
	default:
		fmt.Println(dim("  Fikk ikke kontakt med copilot-cli (er naisdevice på?). Svaret er lagret, og nav-pilot sender det etter neste økt."))
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
		if !interactive || !hasGitHubApp() {
			return http.StatusUnauthorized, nil
		}
		fmt.Println(dim("  Logg inn med GitHub én gang for å sende svaret."))
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
