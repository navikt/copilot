// Package survey holds the survey definitions, validation, aggregation and
// the CLI commands. Storage is in internal/store, the HTTP API in internal/api.
package survey

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// SurveyFiles holds surveys/ and imports/ (SetFiles).
//
// Survey definitions are the files in surveys/, one survey per <id>.json, and
// ship with the service: changing a question is a pull request and a deploy.
// They are the single source of truth for every client (nav-pilot's TUI, the
// web), which render them from GET /api/v1/surveys/active (through copilot-cli
// or my-copilot). The format is checked by LoadSurveyDir, at start and in CI
// (TestShippedSurveys); see
// surveys/README.md. The files are public and must never hold anything that
// is not. main embeds them and passes them to SetFiles.
var SurveyFiles fs.ReadFileFS

// SetFiles hands over the embedded surveys/ and imports/ directories.
func SetFiles(files fs.ReadFileFS) {
	SurveyFiles, ImportFiles = files, files
}

// Retention is how long answers are kept after a survey ends, unless its
// definition sets retention_days. The rows are then deleted.
const Retention = 180 * 24 * time.Hour

// Survey is one survey definition.
type Survey struct {
	ID string `json:"id"`
	// Series groups the waves of one survey over time (e.g.
	// utviklerundersokelsen); each wave is its own id.
	Series string `json:"series,omitempty"`
	// Environments limits the survey to these NAIS clusters (e.g. dev-gcp
	// for a dummy survey); elsewhere it is not loaded at all. Left out means
	// every cluster. See ForCluster.
	Environments []string `json:"environments,omitempty"`
	Title        string   `json:"title"`
	Intro        string   `json:"intro,omitempty"`
	// Active must be set, in its own reviewed pull request, before a survey
	// is served or takes answers: a definition merges inactive.
	Active bool `json:"active,omitempty"`
	// Published shares a closed survey's aggregated results with signed-in
	// Nav users on /innsikt/undersokelser (#1775).
	Published bool `json:"published,omitempty"`
	// Nudge is where nav-pilot brings the survey up by itself: calm (after
	// a session ends; the default), start (a one-line hint as a session
	// starts) or off (only through nav-pilot survey).
	Nudge string `json:"nudge,omitempty"`
	// MinCLIVersion leaves the survey out of the list nav-pilot gets when
	// its version is older, or unknown (builds before this field send none).
	// Web clients always get it. YYYY.MM.DD or YYYY.MM.DD-HHMMSS, as in a
	// nav-pilot release tag.
	MinCLIVersion string `json:"min_cli_version,omitempty"`
	// RetentionDays overrides retention for this survey (e.g. an imported
	// earlier wave kept for comparison).
	RetentionDays int        `json:"retention_days,omitempty"`
	Starts        string     `json:"starts"` // YYYY-MM-DD, first day open (UTC)
	Ends          string     `json:"ends"`   // YYYY-MM-DD, last day open (UTC)
	Questions     []Question `json:"questions"`
}

// Question is one Question. ID and Version are what makes waves comparable:
// the same id and version in two waves is the same Question, asked the same
// way. Any change to text, options or scale bumps Version, which marks it as
// not comparable with earlier waves. Construct and Reverse are for analysis
// only; clients ignore them.
type Question struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Type      string `json:"type"` // scale, choice, multi, text or matrix
	Text      string `json:"text"`
	Required  bool   `json:"required,omitempty"`
	Construct string `json:"construct,omitempty"` // what it measures, e.g. space-satisfaction
	Reverse   bool   `json:"reverse,omitempty"`   // scale: agreeing is the negative end
	Min       int    `json:"min,omitempty"`       // scale
	Max       int    `json:"max,omitempty"`       // scale
	// Unsure is one more answer after a scale's steps («Vet ikke»), stored as
	// this label and left out of mean and agree share.
	Unsure string `json:"unsure,omitempty"`
	// Labels names every scale step from min to max (a labelled Likert).
	Labels     []string `json:"labels,omitempty"`
	Options    []string `json:"options,omitempty"`     // choice, multi
	MaxChoices int      `json:"max_choices,omitempty"` // multi: at most this many, 0 = any
	Exclusive  string   `json:"exclusive,omitempty"`   // multi: an option that must be picked alone
	// Other adds one more option to a choice or multi, with this label,
	// that takes a short free text: its answer is stored under id + ".other".
	Other  string `json:"other,omitempty"`
	MaxLen int    `json:"max_length,omitempty"` // text, and other's free text, in characters
	// SkipIf skips this question when an earlier choice or multi question's
	// answer is, or includes, the given option. In JSON one rule or a list;
	// the question is skipped when any rule matches.
	SkipIf SkipRules `json:"skip_if,omitempty"`
	// Items are a matrix's statements, all on its scale (min, max, labels).
	// Each is answered, versioned and stored as if it were a scale question
	// of its own, so an item compares with a separate scale question of the
	// same id and version.
	Items []MatrixItem `json:"items,omitempty"`
}

// MatrixItem is one statement in a matrix question.
type MatrixItem struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Text      string `json:"text"`
	Construct string `json:"construct,omitempty"`
	Reverse   bool   `json:"reverse,omitempty"`
}

// SkipRule skips a question when another has a given answer.
type SkipRule struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// SkipRules is a question's skip_if: one rule or a list.
type SkipRules []SkipRule

// UnmarshalJSON takes one rule (every survey before #2230) or a list.
func (rs *SkipRules) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '{' {
		var r SkipRule
		err := DecodeStrict(b, &r) // a custom Unmarshaler loses the outer decoder's DisallowUnknownFields
		*rs = SkipRules{r}
		return err
	}
	return DecodeStrict(b, (*[]SkipRule)(rs))
}

// MarshalJSON keeps one rule an object, so older clients still read it.
func (rs SkipRules) MarshalJSON() ([]byte, error) {
	if len(rs) == 1 {
		return json.Marshal(rs[0])
	}
	return json.Marshal([]SkipRule(rs))
}

// skipped reports whether any rule applies to the canonical answers so far.
func (rs SkipRules) skipped(answers map[string]any) bool {
	return slices.ContainsFunc(rs, func(r SkipRule) bool { return r.skipped(answers) })
}

// matched returns the question of the first rule that applies, for errors.
func (rs SkipRules) matched(answers map[string]any) string {
	for _, r := range rs {
		if r.skipped(answers) {
			return r.Question
		}
	}
	return ""
}

func (r SkipRule) skipped(answers map[string]any) bool {
	switch v := answers[r.Question].(type) {
	case string:
		return v == r.Answer
	case []string:
		return slices.Contains(v, r.Answer)
	case []any: // stored answers, decoded from jsonb
		return slices.Contains(v, any(r.Answer))
	}
	return false
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// SchemaFile is the JSON Schema of the definition format, for other clients
// (GET /api/v1/surveys/schema). CI checks every definition against it
// (TestShippedSurveysMatchSchema); LoadSurveyDir checks more than it can say.
const SchemaFile = "surveys/schema.json"

// LoadSurveyDir reads and checks every surveys/*.json but the schema.
func LoadSurveyDir(fsys fs.FS) ([]Survey, error) {
	names, err := fs.Glob(fsys, "surveys/*.json")
	if err != nil {
		return nil, err
	}
	var surveys []Survey
	for _, name := range names {
		if name == SchemaFile {
			continue
		}
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var s Survey
		if err := DecodeStrict(raw, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if path.Base(name) != s.ID+".json" {
			return nil, fmt.Errorf("%s: the file must be named after the id, %s.json", name, s.ID)
		}
		surveys = append(surveys, s)
	}
	return surveys, ValidateSurveys(surveys)
}

// DecodeStrict decodes raw into v, refusing unknown and duplicate keys.
func DecodeStrict(raw []byte, v any) error {
	if err := rejectDuplicateKeys(raw); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if !AtEOF(dec) {
		return errors.New("trailing data")
	}
	return nil
}

// rejectDuplicateKeys refuses an object that names a key twice at any depth,
// ignoring case as encoding/json does. encoding/json merges the two values
// and jq keeps the last, so check-frozen-surveys.sh and the server would
// read different definitions.
func rejectDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if tok != json.Delim('{') && tok != json.Delim('[') {
			return nil
		}
		var seen []string
		for dec.More() {
			if tok == json.Delim('{') {
				k, kerr := dec.Token()
				if kerr != nil {
					return kerr
				}
				key, _ := k.(string)
				for _, s := range seen {
					if strings.EqualFold(s, key) {
						return fmt.Errorf("duplicate key %q", key)
					}
				}
				seen = append(seen, key)
			}
			if werr := walk(); werr != nil {
				return werr
			}
		}
		_, err = dec.Token()
		return err
	}
	return walk()
}

// AtEOF reports whether nothing but whitespace follows the value dec read.
// dec.More() is no such check: it is false before a stray ] or }.
func AtEOF(dec *json.Decoder) bool {
	return errors.Is(dec.Decode(&struct{}{}), io.EOF)
}

// ValidateSurveys checks the definitions against each other and the rules.
func ValidateSurveys(surveys []Survey) error {
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
		if !slices.Contains([]string{"", "calm", "start", "off"}, s.Nudge) {
			return fmt.Errorf("survey %s: nudge is calm, start or off", s.ID)
		}
		if s.MinCLIVersion != "" && !validMinCLIVersion(s.MinCLIVersion) {
			return fmt.Errorf("survey %s: min_cli_version is YYYY.MM.DD or YYYY.MM.DD-HHMMSS", s.ID)
		}
		if s.Series != "" && !idPattern.MatchString(s.Series) {
			return fmt.Errorf("survey %s: bad series %q", s.ID, s.Series)
		}
		// Free text is what batching and segment suppression cannot keep from
		// naming its author: one text question at most, and at most
		// maxOthers short "other" texts.
		if len(slices.DeleteFunc(slices.Clone(s.Questions), func(q Question) bool { return q.Other == "" })) > maxOthers {
			return fmt.Errorf("survey %s: at most %d other options", s.ID, maxOthers)
		}
		if texts := slices.IndexFunc(s.Questions, func(q Question) bool { return q.Type == "text" }); texts >= 0 &&
			slices.ContainsFunc(s.Questions[texts+1:], func(q Question) bool { return q.Type == "text" }) {
			return fmt.Errorf("survey %s: at most one text question", s.ID)
		}
		qseen := map[string]bool{}
		for i, q := range s.Questions {
			ok := idPattern.MatchString(q.ID) && !qseen[q.ID] && q.Text != "" && q.Version >= 1 &&
				(q.Items == nil || q.Type == "matrix") &&
				(!q.Reverse || q.Type == "scale") && (q.MaxChoices == 0 || q.Type == "multi") &&
				(q.Other == "" || q.Type == "choice" || q.Type == "multi") &&
				(q.Unsure == "" || q.Type == "scale" || q.Type == "matrix") && !numeric(q.Unsure) &&
				(q.Exclusive == "" || q.Type == "multi" && slices.Contains(q.Options, q.Exclusive))
			ok = ok && (q.SkipIf == nil || len(q.SkipIf) > 0)
			for _, r := range q.SkipIf {
				j := slices.IndexFunc(s.Questions[:i], func(p Question) bool { return p.ID == r.Question })
				ok = ok && j >= 0 && (s.Questions[j].Type == "choice" || s.Questions[j].Type == "multi") &&
					slices.Contains(s.Questions[j].Options, r.Answer)
			}
			qseen[q.ID] = true
			switch q.Type {
			case "scale":
				ok = ok && q.Min < q.Max && q.Max-q.Min <= 10 && (q.Labels == nil || len(q.Labels) == q.Max-q.Min+1)
			case "matrix":
				ok = ok && q.Min < q.Max && q.Max-q.Min <= 10 && (q.Labels == nil || len(q.Labels) == q.Max-q.Min+1) &&
					!q.Reverse && q.Options == nil && q.Other == "" && q.MaxLen == 0 && len(q.Items) >= 2
				// Item ids share the question ids' namespace: that is where
				// their answers are stored.
				for _, it := range q.Items {
					ok = ok && idPattern.MatchString(it.ID) && !qseen[it.ID] && it.Text != "" && it.Version >= 1
					qseen[it.ID] = true
				}
			case "choice", "multi":
				ok = ok && len(q.Options) >= 2 && !hasDuplicates(q.Options) && q.MaxChoices >= 0 && q.MaxChoices <= len(q.options())
				// Other's text is kept short: it is free text like a text
				// question's, and can name its author the same way.
				if q.Other != "" {
					ok = ok && !slices.Contains(q.Options, q.Other) && q.MaxLen > 0 && q.MaxLen <= maxOtherLen
				} else {
					ok = ok && q.MaxLen == 0
				}
			case "text":
				ok = ok && q.MaxLen > 0 && q.MaxLen <= 2000
			default:
				ok = false
			}
			if !ok {
				return fmt.Errorf("survey %s: bad question %q", s.ID, q.ID)
			}
		}
	}
	return nil
}

// maxOtherLen caps the free text of an "other" option, and maxOthers how many
// a survey may have.
const (
	maxOtherLen = 200
	maxOthers   = 3
)

// otherKey is where the free text of q's other option is sent and stored.
func otherKey(id string) string { return id + ".other" }

// options is what a choice or multi answer may be: the options, and other.
func (q Question) options() []string {
	if q.Other == "" {
		return q.Options
	}
	return append(slices.Clip(q.Options), q.Other)
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

// ForCluster drops the surveys that name environments not including cluster,
// so a dev-only test survey is absent from list, results, compare and
// submit in prod. An empty cluster (local runs) keeps everything.
func ForCluster(defs []Survey, cluster string) []Survey {
	if cluster == "" {
		return defs
	}
	return slices.DeleteFunc(defs, func(s Survey) bool {
		return len(s.Environments) > 0 && !slices.Contains(s.Environments, cluster)
	})
}

// ActivateInDev opens the surveys named in ids (SURVEY_ACTIVE_IDS,
// comma-separated) whatever their "active" says, for an end-to-end test of a
// dummy survey such as dev-e2e-test. Only in dev-gcp: anywhere else a survey
// opens only through its own reviewed "active": true.
func ActivateInDev(defs []Survey, cluster, ids string) {
	if ids == "" {
		return
	}
	if cluster != "dev-gcp" {
		slog.Warn("SURVEY_ACTIVE_IDS is ignored outside dev-gcp", "cluster", cluster)
		return
	}
	for _, id := range strings.Split(ids, ",") {
		id = strings.TrimSpace(id)
		i := slices.IndexFunc(defs, func(s Survey) bool { return s.ID == id })
		if i < 0 {
			slog.Warn("SURVEY_ACTIVE_IDS names no survey", "id", id)
			continue
		}
		defs[i].Active = true
		slog.Info("survey activated for dev", "survey", id)
	}
}

// ActiveOn tells whether s is active and inside its window on now.
func (s Survey) ActiveOn(now time.Time) bool {
	day := now.UTC().Format(time.DateOnly)
	return s.Active && s.Starts <= day && day <= s.Ends
}

// ClosesOn is the first day the survey no longer takes answers.
func (s Survey) ClosesOn() time.Time {
	end, _ := time.Parse(time.DateOnly, s.Ends)
	return end.AddDate(0, 0, 1)
}

// DeleteAfter is the last day the survey's answers are kept.
func (s Survey) DeleteAfter() time.Time {
	if s.RetentionDays > 0 {
		return s.ClosesOn().AddDate(0, 0, s.RetentionDays)
	}
	return s.ClosesOn().Add(Retention)
}

// Submission is the request body. Unknown fields are refused, so a client
// cannot slip anything past the data model below.
type Submission struct {
	Answers map[string]json.RawMessage `json:"answers"`
	Context TechContext                `json:"context"`
}

// TechContext is the only thing recorded about the respondent's setup:
// enums and a version string, no content, no identifiers.
type TechContext struct {
	Version     string `json:"version"`
	OS          string `json:"os"`
	Client      string `json:"client"`
	LocalModels bool   `json:"local_models"`
}

var versionPattern = regexp.MustCompile(`^(dev|web|v?(\d{1,4}\.\d{1,4})\.\d{1,4}(-[0-9A-Za-z.-]{1,40})?)$`)

// Validate checks the context and coarsens it: only the first two parts of
// the version are kept (2026.09 of 2026.09.24-120000-abc1234), so a row does
// not name the build a telemetry series ran on that day.
func (c *TechContext) Validate() error {
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
	case !slices.Contains([]string{"copilot", "opencode", "pi", "web", "none"}, c.Client):
		return errors.New("context.client is not one of copilot, opencode, pi, web, none")
	}
	return nil
}

// ValidateAnswers checks every answer against its question and returns them
// in canonical form. Unknown question ids are refused.
func ValidateAnswers(s Survey, raw map[string]json.RawMessage) (map[string]any, error) {
	out := map[string]any{}
	questions := s.Answerable()
	for id := range raw {
		if !slices.ContainsFunc(questions, func(q Question) bool { return q.ID == id || (q.Other != "" && otherKey(q.ID) == id) }) {
			return nil, fmt.Errorf("unknown question %q", id)
		}
	}
	for _, q := range questions {
		v, ok := raw[q.ID]
		other, hasOther := raw[otherKey(q.ID)]
		hasOther = hasOther && q.Other != "" && string(other) != "null"
		if q.SkipIf.skipped(out) {
			if (ok && string(v) != "null") || hasOther {
				return nil, fmt.Errorf("question %q is skipped for this answer to %q", q.ID, q.SkipIf.matched(out))
			}
			continue
		}
		if !ok || string(v) == "null" {
			if hasOther {
				return nil, fmt.Errorf("%q needs %q answered with %q", otherKey(q.ID), q.ID, q.Other)
			}
			if q.Required {
				return nil, fmt.Errorf("question %q is required", q.ID)
			}
			continue
		}
		switch q.Type {
		case "scale":
			var u string
			if q.Unsure != "" && json.Unmarshal(v, &u) == nil && u == q.Unsure {
				out[q.ID] = u
				break
			}
			var n int
			if json.Unmarshal(v, &n) != nil || n < q.Min || n > q.Max {
				return nil, fmt.Errorf("question %q takes a whole number from %d to %d", q.ID, q.Min, q.Max)
			}
			out[q.ID] = n
		case "choice":
			var o string
			if json.Unmarshal(v, &o) != nil || !slices.Contains(q.options(), o) {
				return nil, fmt.Errorf("question %q takes one of its options", q.ID)
			}
			out[q.ID] = o
		case "multi":
			var picked []string
			if json.Unmarshal(v, &picked) != nil || len(picked) == 0 || hasDuplicates(picked) ||
				(q.MaxChoices > 0 && len(picked) > q.MaxChoices) ||
				slices.ContainsFunc(picked, func(p string) bool { return !slices.Contains(q.options(), p) }) {
				return nil, fmt.Errorf("question %q takes one or more of its options (at most max_choices)", q.ID)
			}
			if len(picked) > 1 && slices.Contains(picked, q.Exclusive) {
				return nil, fmt.Errorf("question %q: %q must be picked alone", q.ID, q.Exclusive)
			}
			out[q.ID] = picked
		case "text":
			t, err := freeText(q.ID, v, q.MaxLen)
			if err != nil {
				return nil, err
			}
			if t == "" {
				if q.Required {
					return nil, fmt.Errorf("question %q is required", q.ID)
				}
				continue
			}
			out[q.ID] = t
		}
		if hasOther {
			chosen := out[q.ID] == q.Other
			if picked, isList := out[q.ID].([]string); isList {
				chosen = slices.Contains(picked, q.Other)
			}
			if !chosen {
				return nil, fmt.Errorf("%q needs %q answered with %q", otherKey(q.ID), q.ID, q.Other)
			}
			t, err := freeText(otherKey(q.ID), other, q.MaxLen)
			if err != nil {
				return nil, err
			}
			if t != "" {
				out[otherKey(q.ID)] = t
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no answers")
	}
	return out, nil
}

// Answerable is the questions as they are answered and stored: each matrix
// item becomes a scale question of its own, with the matrix's scale,
// required and skip_if.
func (s Survey) Answerable() []Question {
	var qs []Question
	for _, q := range s.Questions {
		if q.Type != "matrix" {
			qs = append(qs, q)
			continue
		}
		for _, it := range q.Items {
			item := q
			item.Type, item.Items = "scale", nil
			item.ID, item.Version, item.Text, item.Construct, item.Reverse = it.ID, it.Version, it.Text, it.Construct, it.Reverse
			qs = append(qs, item)
		}
	}
	return qs
}

// numeric reports whether s reads as a number, which a scale's unsure label
// must not: mean and the CSV tell the two apart that way.
func numeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// freeText checks a free-text answer and trims it; "" means not answered.
func freeText(id string, v json.RawMessage, maxLen int) (string, error) {
	// Checked on the raw bytes: Unmarshal would replace invalid UTF-8.
	var t string
	if !utf8.Valid(v) || json.Unmarshal(v, &t) != nil {
		return "", fmt.Errorf("%q takes text", id)
	}
	t = strings.TrimSpace(t)
	if utf8.RuneCountInString(t) > maxLen {
		return "", fmt.Errorf("%q takes at most %d characters", id, maxLen)
	}
	return t, nil
}

// ParticipantHash is the only thing that stops a second answer:
// HMAC-SHA256 under the survey's own secret key of the respondent's
// normalised Nav e-mail. It is stored in survey_participation, which shares
// no key, id or order with the answers (see store.go), and is deleted with
// the survey's key when the survey closes.
func ParticipantHash(key []byte, email string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(NormaliseEmail(email)))
	return hex.EncodeToString(mac.Sum(nil))
}

// NormaliseEmail lower-cases and trims an e-mail address.
func NormaliseEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// Response is one set of answers as stored: nothing that names or points
// at a person, not even a hash.
type Response struct {
	SurveyID         string
	Answers          map[string]any
	QuestionVersions map[string]int
	Context          TechContext
	DeleteAfter      time.Time
}

var (
	minCLIVersionPattern = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}(-\d{6})?$`)
	cliVersionPattern    = regexp.MustCompile(`^v?(\d{1,4})\.(\d{1,2})\.(\d{1,2})(?:-(\d{6}))?(?:[-+ ]|$)`)
)

// validMinCLIVersion: the shape, and a real date and time of day.
func validMinCLIVersion(v string) bool {
	if !minCLIVersionPattern.MatchString(v) {
		return false
	}
	date, clock, _ := strings.Cut(v, "-")
	_, err := time.Parse("2006.01.02", date)
	if clock != "" {
		_, err2 := time.Parse("150405", clock)
		err = errors.Join(err, err2)
	}
	return err == nil
}

// CliVersion reads a nav-pilot version (2026.09.29-073815-8801b8d) as
// year, month, day and build time, for comparison. false: not a version.
func CliVersion(v string) ([]int, bool) {
	m := cliVersionPattern.FindStringSubmatch(v)
	if m == nil {
		return nil, false
	}
	out := make([]int, 4)
	for i, p := range m[1:] {
		out[i], _ = strconv.Atoi(p) // "" for a missing build time reads as 0
	}
	return out, true
}
