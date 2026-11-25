package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// Store is the storage the handlers use, implemented by internal/store.
// The handlers treat a nil Store as running without storage.
type Store interface {
	// Submit queues a submission for a batch write; false means hash has
	// already answered.
	Submit(ctx context.Context, surveyID, hash string, closesOn time.Time, r survey.Response) (bool, error)
	// Answered tells whether hash has answered, queued or written.
	Answered(ctx context.Context, surveyID, hash string) (bool, error)
	// Answers reads a survey's written answers for the admin aggregates.
	Answers(ctx context.Context, surveyID string) ([]survey.StoredAnswer, error)
	// Final reads a closed survey's aggregates; nil, nil when it has none.
	Final(ctx context.Context, surveyID string) (*survey.Tally, error)
	// FinalCompare reads what compare needs from a closed survey's
	// aggregates; nil, nil when it has none.
	FinalCompare(ctx context.Context, surveyID string) (*survey.WaveTally, error)
	// Hidden and Hide read and add free texts hidden by an admin.
	Hidden(ctx context.Context, surveyID string) (map[string]bool, error)
	Hide(ctx context.Context, surveyID, field, hash string) error
	// Codes, SaveCodes and FinalCoding read and write free-text codes and
	// their stored counts.
	Codes(ctx context.Context, surveyID string, version int) (map[string]*survey.TextCodes, error)
	SaveCodes(ctx context.Context, surveyID, field, hash, coder string, version int, codes []survey.Code, by string) error
	FinalCoding(ctx context.Context, surveyID string, version int) (map[string]survey.FieldTally, error)
}

// surveyAPI serves the definitions and takes submissions. Submissions to a
// survey get a 503 while storage or the e-mail lookup is missing, and a 404
// while its key is.
type surveyAPI struct {
	surveys []survey.Survey
	// keys holds each survey's secret key (SURVEY_KEY_<ID>). Deleting a
	// survey's key when it closes is what makes its participation hashes
	// unlinkable for good.
	keys map[string][]byte
	// emailFor finds the caller's Nav e-mail: from the Entra token, or for
	// a GitHub login through copilot-api's SAML lookup (nameIDClient). Used
	// only as the input to ParticipantHash, never stored or logged.
	emailFor func(context.Context, *caller) (string, error)
	store    Store
	now      func() time.Time
	// textKey keys the free-text tokens (freetext.go); random per process.
	textKey []byte
	// codebooks by series (codebook.go).
	codebooks map[string]survey.Codebook
}

// open: active, inside its window and with a key. A survey without a key
// is not listed, so nobody fills in a form that cannot be submitted.
func (a *surveyAPI) open(s survey.Survey) bool {
	return s.ActiveOn(a.now()) && len(a.keys[s.ID]) > 0
}

func (a *surveyAPI) active(w http.ResponseWriter, r *http.Request) {
	// copilot-cli adds client=nav-pilot and passes on nav-pilot's
	// User-Agent; my-copilot adds nothing and gets every open survey.
	fromCLI := r.URL.Query().Get("client") == "nav-pilot"
	v, isCLI := strings.CutPrefix(r.UserAgent(), "nav-pilot/")
	have, known := survey.CliVersion(v)
	known = known && isCLI
	active := []survey.Survey{}
	for _, s := range a.surveys {
		if !a.open(s) {
			continue
		}
		if floor, _ := survey.CliVersion(s.MinCLIVersion); fromCLI && s.MinCLIVersion != "" && (!known || slices.Compare(have, floor) < 0) {
			continue
		}
		active = append(active, s)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Vary", "User-Agent")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(map[string]any{"surveys": active})
}

// schema serves surveys/schema.json: public, like the definitions.
func (a *surveyAPI) schema(w http.ResponseWriter, _ *http.Request) {
	raw, _ := survey.SurveyFiles.ReadFile(survey.SchemaFile) // embedded; TestShippedSurveysMatchSchema reads it too
	w.Header().Set("Content-Type", "application/schema+json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(raw)
}

// navEmail looks up the caller's Nav e-mail, writing the 403 or 503 itself
// when it cannot.
func (a *surveyAPI) navEmail(w http.ResponseWriter, r *http.Request, who *caller, surveyID string) (string, bool) {
	email, err := a.emailFor(r.Context(), who)
	switch {
	case errors.Is(err, errNoNavIdentity):
		writeError(w, http.StatusForbidden, err.Error()+"; answer on ki-utvikling.nav.no instead")
		return "", false
	case err != nil || survey.NormaliseEmail(email) == "":
		slog.Warn("survey: e-mail lookup failed", "survey", surveyID, "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusServiceUnavailable, "could not confirm your Nav identity, try again later")
		return "", false
	}
	return email, true
}

// status tells the caller whether they have answered an open survey. Like
// submit, it never logs who asked or the hash.
func (a *surveyAPI) status(w http.ResponseWriter, r *http.Request) {
	who, ok := callerFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "missing authenticated user")
		return
	}
	id := r.PathValue("id")
	i := slices.IndexFunc(a.surveys, func(s survey.Survey) bool { return s.ID == id })
	if i < 0 || !a.open(a.surveys[i]) {
		writeError(w, http.StatusNotFound, "no open survey with that id")
		return
	}
	s := a.surveys[i]
	if a.store == nil || a.emailFor == nil {
		writeError(w, http.StatusServiceUnavailable, "this survey is not taking answers right now")
		return
	}
	email, ok := a.navEmail(w, r, who, s.ID)
	if !ok {
		return
	}
	// Bounded like submit's store call so a stalled database does not hold
	// the request open for as long as the client waits.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	answered, err := a.store.Answered(ctx, s.ID, survey.ParticipantHash(a.keys[s.ID], email))
	if err != nil {
		slog.Error("survey: checking participation failed", "survey", s.ID, "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusServiceUnavailable, "could not check your answer, try again later")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]bool{"answered": answered})
}

var errNoNavIdentity = errors.New("no Nav identity is linked to this GitHub account")

// submit never logs who answered, what they answered or the hash.
func (a *surveyAPI) submit(w http.ResponseWriter, r *http.Request) {
	who, ok := callerFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "missing authenticated user")
		return
	}
	id := r.PathValue("id")
	i := slices.IndexFunc(a.surveys, func(s survey.Survey) bool { return s.ID == id })
	if i < 0 || !a.open(a.surveys[i]) {
		writeError(w, http.StatusNotFound, "no open survey with that id")
		return
	}
	s := a.surveys[i]
	key := a.keys[s.ID]
	if a.store == nil || a.emailFor == nil {
		writeError(w, http.StatusServiceUnavailable, "this survey is not taking answers right now")
		return
	}

	var sub survey.Submission
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sub); err != nil || !survey.AtEOF(dec) {
		writeError(w, http.StatusBadRequest, "body is not a valid submission")
		return
	}
	if err := sub.Context.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	answers, err := survey.ValidateAnswers(s, sub.Answers)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email, ok := a.navEmail(w, r, who, s.ID)
	if !ok {
		return
	}
	versions := map[string]int{}
	for _, q := range s.Answerable() {
		if _, ok := answers[q.ID]; ok {
			versions[q.ID] = q.Version
		}
	}
	// Not the request's context: the write it triggers must not be
	// cancelled because this caller hung up, and must carry nothing of this
	// request, since it may also write answers queued before it.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fresh, err := a.store.Submit(ctx, s.ID, survey.ParticipantHash(key, email), s.ClosesOn(), survey.Response{
		SurveyID: s.ID, Answers: answers, QuestionVersions: versions,
		Context: sub.Context, DeleteAfter: s.DeleteAfter(),
	})
	if err != nil {
		slog.Error("survey: storing the submission failed", "survey", s.ID, "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusServiceUnavailable, "could not store your answers, try again later")
		return
	}
	if !fresh {
		writeError(w, http.StatusConflict, "you have already answered this survey")
		return
	}
	// Counted once per person, on the answer that is kept.
	if who.email == "" && !strings.HasSuffix(survey.NormaliseEmail(email), "@nav.no") {
		unexpectedNameIDs.Inc()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"status":"recorded","note":"the answers have no id you can present, so they cannot be changed or withdrawn"}`))
}
