package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Survey definitions are the files in surveys/, one survey per <id>.json, and
// ship with the service: changing a question is a pull request and a deploy.
// They are the single source of truth for every client (nav-pilot's TUI, the
// web), which render them from GET /api/v1/surveys/active. The format is
// checked by loadSurveyDir, at start and in CI (TestShippedSurveys); see
// surveys/README.md. The files are public and must never hold anything that
// is not.
//
//go:embed surveys
var surveyFiles embed.FS

// retention is how long answers are kept after a survey ends. The rows are
// then deleted, dedup hash and all.
const retention = 180 * 24 * time.Hour

type survey struct {
	ID string `json:"id"`
	// Series groups the waves of one survey over time (e.g.
	// utviklerundersokelsen); each wave is its own id.
	Series    string     `json:"series,omitempty"`
	Title     string     `json:"title"`
	Intro     string     `json:"intro,omitempty"`
	Starts    string     `json:"starts"` // YYYY-MM-DD, first day open (UTC)
	Ends      string     `json:"ends"`   // YYYY-MM-DD, last day open (UTC)
	Questions []question `json:"questions"`
}

// question is one question. ID and Version are what makes waves comparable:
// the same id and version in two waves is the same question, asked the same
// way. Any change to text, options or scale bumps Version, which marks it as
// not comparable with earlier waves. Construct and Reverse are for analysis
// only; clients ignore them.
type question struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Type      string `json:"type"` // scale, choice, multi or text
	Text      string `json:"text"`
	Required  bool   `json:"required,omitempty"`
	Construct string `json:"construct,omitempty"` // what it measures, e.g. space-satisfaction
	Reverse   bool   `json:"reverse,omitempty"`   // scale: agreeing is the negative end
	Min       int    `json:"min,omitempty"`       // scale
	Max       int    `json:"max,omitempty"`       // scale
	// Labels names every scale step from min to max (a labelled Likert).
	Labels     []string `json:"labels,omitempty"`
	Options    []string `json:"options,omitempty"`     // choice, multi
	MaxChoices int      `json:"max_choices,omitempty"` // multi: at most this many, 0 = any
	MaxLen     int      `json:"max_length,omitempty"`  // text, in characters
	// SkipIf skips this question when an earlier choice or multi question's
	// answer is, or includes, the given option.
	SkipIf *skipRule `json:"skip_if,omitempty"`
}

type skipRule struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// skipped reports whether rule applies to the canonical answers so far.
func (r *skipRule) skipped(answers map[string]any) bool {
	if r == nil {
		return false
	}
	switch v := answers[r.Question].(type) {
	case string:
		return v == r.Answer
	case []string:
		return slices.Contains(v, r.Answer)
	}
	return false
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// loadSurveyDir reads and checks every surveys/*.json.
func loadSurveyDir(fsys fs.FS) ([]survey, error) {
	names, err := fs.Glob(fsys, "surveys/*.json")
	if err != nil {
		return nil, err
	}
	var surveys []survey
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var s survey
		if err := decodeStrict(raw, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if path.Base(name) != s.ID+".json" {
			return nil, fmt.Errorf("%s: the file must be named after the id, %s.json", name, s.ID)
		}
		surveys = append(surveys, s)
	}
	return surveys, validateSurveys(surveys)
}

// loadSurveys reads a JSON array of surveys (tests).
func loadSurveys(raw []byte) ([]survey, error) {
	var surveys []survey
	if err := decodeStrict(raw, &surveys); err != nil {
		return nil, err
	}
	return surveys, validateSurveys(surveys)
}

func decodeStrict(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

func validateSurveys(surveys []survey) error {
	seen := map[string]bool{}
	for _, s := range surveys {
		if !idPattern.MatchString(s.ID) || seen[s.ID] || s.Title == "" {
			return fmt.Errorf("survey %q: bad or duplicate id, or no title", s.ID)
		}
		seen[s.ID] = true
		start, err1 := time.Parse(time.DateOnly, s.Starts)
		end, err2 := time.Parse(time.DateOnly, s.Ends)
		if err1 != nil || err2 != nil || end.Before(start) || len(s.Questions) == 0 {
			return fmt.Errorf("survey %s: needs starts <= ends and questions", s.ID)
		}
		if s.Series != "" && !idPattern.MatchString(s.Series) {
			return fmt.Errorf("survey %s: bad series %q", s.ID, s.Series)
		}
		qseen := map[string]bool{}
		for i, q := range s.Questions {
			ok := idPattern.MatchString(q.ID) && !qseen[q.ID] && q.Text != "" && q.Version >= 1 &&
				(!q.Reverse || q.Type == "scale") && (q.MaxChoices == 0 || q.Type == "multi")
			if q.SkipIf != nil {
				j := slices.IndexFunc(s.Questions[:i], func(p question) bool { return p.ID == q.SkipIf.Question })
				ok = ok && j >= 0 && slices.Contains(s.Questions[j].Options, q.SkipIf.Answer) && !q.Required
			}
			switch q.Type {
			case "scale":
				ok = ok && q.Min < q.Max && q.Max-q.Min <= 10 && (q.Labels == nil || len(q.Labels) == q.Max-q.Min+1)
			case "choice", "multi":
				ok = ok && len(q.Options) >= 2 && !hasDuplicates(q.Options) && q.MaxChoices >= 0 && q.MaxChoices <= len(q.Options)
			case "text":
				ok = ok && q.MaxLen > 0 && q.MaxLen <= 2000
			default:
				ok = false
			}
			if !ok {
				return fmt.Errorf("survey %s: bad question %q", s.ID, q.ID)
			}
			qseen[q.ID] = true
		}
	}
	return nil
}

func hasDuplicates(xs []string) bool {
	seen := map[string]bool{}
	for _, x := range xs {
		if x == "" || seen[x] {
			return true
		}
		seen[x] = true
	}
	return false
}

func (s survey) activeOn(now time.Time) bool {
	day := now.UTC().Format(time.DateOnly)
	return s.Starts <= day && day <= s.Ends
}

// closesOn is the first day the survey no longer takes answers.
func (s survey) closesOn() time.Time {
	end, _ := time.Parse(time.DateOnly, s.Ends)
	return end.AddDate(0, 0, 1)
}

// submission is the request body. Unknown fields are refused, so a client
// cannot slip anything past the data model below.
type submission struct {
	Answers map[string]json.RawMessage `json:"answers"`
	Context techContext                `json:"context"`
}

// techContext is the only thing recorded about the respondent's setup:
// enums and a version string, no content, no identifiers.
type techContext struct {
	Version     string `json:"version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Client      string `json:"client"`
	LocalModels bool   `json:"local_models"`
}

var versionPattern = regexp.MustCompile(`^(dev|v?(\d{1,4}\.\d{1,4})\.\d{1,4}(-[0-9A-Za-z.-]{1,40})?)$`)

// validate checks the context and coarsens it: only the first two parts of
// the version are kept (2026.09 of 2026.09.24-120000-abc1234), so a row does
// not name the build a telemetry series ran on that day.
func (c *techContext) validate() error {
	m := versionPattern.FindStringSubmatch(c.Version)
	if m == nil {
		return errors.New("context.version is not a version")
	}
	if m[2] != "" {
		c.Version = m[2]
	}
	switch {
	case !slices.Contains([]string{"darwin", "linux", "windows", "other"}, c.OS):
		return errors.New("context.os is not one of darwin, linux, windows, other")
	case !slices.Contains([]string{"amd64", "arm64", "other"}, c.Arch):
		return errors.New("context.arch is not one of amd64, arm64, other")
	case !slices.Contains([]string{"copilot", "opencode", "pi", "web", "none"}, c.Client):
		return errors.New("context.client is not one of copilot, opencode, pi, web, none")
	}
	return nil
}

// validateAnswers checks every answer against its question and returns them
// in canonical form. Unknown question ids are refused.
func validateAnswers(s survey, raw map[string]json.RawMessage) (map[string]any, error) {
	out := map[string]any{}
	for id := range raw {
		if !slices.ContainsFunc(s.Questions, func(q question) bool { return q.ID == id }) {
			return nil, fmt.Errorf("unknown question %q", id)
		}
	}
	for _, q := range s.Questions {
		v, ok := raw[q.ID]
		if q.SkipIf.skipped(out) {
			if ok && string(v) != "null" {
				return nil, fmt.Errorf("question %q is skipped for this answer to %q", q.ID, q.SkipIf.Question)
			}
			continue
		}
		if !ok || string(v) == "null" {
			if q.Required {
				return nil, fmt.Errorf("question %q is required", q.ID)
			}
			continue
		}
		switch q.Type {
		case "scale":
			var n int
			if json.Unmarshal(v, &n) != nil || n < q.Min || n > q.Max {
				return nil, fmt.Errorf("question %q takes a whole number from %d to %d", q.ID, q.Min, q.Max)
			}
			out[q.ID] = n
		case "choice":
			var o string
			if json.Unmarshal(v, &o) != nil || !slices.Contains(q.Options, o) {
				return nil, fmt.Errorf("question %q takes one of its options", q.ID)
			}
			out[q.ID] = o
		case "multi":
			var picked []string
			if json.Unmarshal(v, &picked) != nil || len(picked) == 0 || hasDuplicates(picked) ||
				(q.MaxChoices > 0 && len(picked) > q.MaxChoices) ||
				slices.ContainsFunc(picked, func(p string) bool { return !slices.Contains(q.Options, p) }) {
				return nil, fmt.Errorf("question %q takes one or more of its options (at most max_choices)", q.ID)
			}
			out[q.ID] = picked
		case "text":
			var t string
			if json.Unmarshal(v, &t) != nil || !utf8.ValidString(t) {
				return nil, fmt.Errorf("question %q takes text", q.ID)
			}
			t = strings.TrimSpace(t)
			if utf8.RuneCountInString(t) > q.MaxLen {
				return nil, fmt.Errorf("question %q takes at most %d characters", q.ID, q.MaxLen)
			}
			if t == "" {
				if q.Required {
					return nil, fmt.Errorf("question %q is required", q.ID)
				}
				continue
			}
			out[q.ID] = t
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no answers")
	}
	return out, nil
}

// participantHash is the only thing that stops a second answer:
// HMAC-SHA256 under the survey's own secret key of the respondent's
// normalised Nav e-mail. It is stored in survey_participation, which shares
// no key, id or order with the answers (see store.go), and is deleted with
// the survey's key when the survey closes.
func participantHash(key []byte, email string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(normaliseEmail(email)))
	return hex.EncodeToString(mac.Sum(nil))
}

func normaliseEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// response is one set of answers as stored: nothing that names or points
// at a person, not even a hash.
type response struct {
	SurveyID         string
	Answers          map[string]any
	QuestionVersions map[string]int
	Context          techContext
	DeleteAfter      time.Time
}

// surveyAPI serves the definitions and takes submissions. Submissions to a
// survey get a 503 while storage, its key or the e-mail lookup is missing.
type surveyAPI struct {
	surveys []survey
	// keys holds each survey's secret key (SURVEY_KEY_<ID>). Deleting a
	// survey's key when it closes is what makes its participation hashes
	// unlinkable for good.
	keys map[string][]byte
	// emailFor finds the caller's Nav e-mail: from the Entra token, or for
	// a GitHub sign-in through navikt's SAML SSO identity. Used only as the
	// input to participantHash, never stored or logged.
	emailFor func(context.Context, *AuthenticatedUser) (string, error)
	// participate records that hash answered survey id; false means it
	// already had.
	participate func(ctx context.Context, surveyID, hash string, closesOn time.Time) (bool, error)
	// record queues the answers for a shuffled batch write.
	record func(context.Context, response)
	now    func() time.Time
}

func (a *surveyAPI) active(w http.ResponseWriter, _ *http.Request) {
	active := []survey{}
	for _, s := range a.surveys {
		if s.activeOn(a.now()) {
			active = append(active, s)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(map[string]any{"surveys": active})
}

var errNoNavIdentity = errors.New("no Nav identity is linked to this GitHub account")

// submit never logs who answered, what they answered or the hash.
func (a *surveyAPI) submit(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "missing authenticated user")
		return
	}
	id := r.PathValue("id")
	i := slices.IndexFunc(a.surveys, func(s survey) bool { return s.ID == id })
	if i < 0 || !a.surveys[i].activeOn(a.now()) {
		writeError(w, http.StatusNotFound, "no open survey with that id")
		return
	}
	s := a.surveys[i]
	key := a.keys[s.ID]
	if a.participate == nil || a.record == nil || a.emailFor == nil || len(key) == 0 {
		writeError(w, http.StatusServiceUnavailable, "this survey is not taking answers right now")
		return
	}

	var sub submission
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sub); err != nil || dec.More() {
		writeError(w, http.StatusBadRequest, "body is not a valid submission")
		return
	}
	if err := sub.Context.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	answers, err := validateAnswers(s, sub.Answers)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email, err := a.emailFor(r.Context(), user)
	switch {
	case errors.Is(err, errNoNavIdentity):
		writeError(w, http.StatusForbidden, err.Error()+"; answer on ki-utvikling.nav.no instead")
		return
	case err != nil || !strings.HasSuffix(normaliseEmail(email), "@nav.no"):
		slog.Warn("survey: e-mail lookup failed", "survey", s.ID, "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusServiceUnavailable, "could not confirm your Nav identity, try again later")
		return
	}

	fresh, err := a.participate(r.Context(), s.ID, participantHash(key, email), s.closesOn())
	if err != nil {
		slog.Error("survey: recording participation failed", "survey", s.ID, "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusServiceUnavailable, "could not store the answer, try again later")
		return
	}
	if !fresh {
		writeError(w, http.StatusConflict, "already answered")
		return
	}
	versions := map[string]int{}
	for _, q := range s.Questions {
		if _, ok := answers[q.ID]; ok {
			versions[q.ID] = q.Version
		}
	}
	// Detached from the request: a batch write it triggers must not be
	// cancelled because this caller hung up.
	a.record(context.WithoutCancel(r.Context()), response{
		SurveyID: s.ID, Answers: answers, QuestionVersions: versions,
		Context: sub.Context, DeleteAfter: s.closesOn().Add(retention),
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"status":"recorded","note":"answers cannot be changed or withdrawn: nothing links them to you"}`))
}
