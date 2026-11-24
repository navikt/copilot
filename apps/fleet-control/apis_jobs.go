package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AgentJob on v1alpha1 (#2748 PR 3a). A job is ordered by a person in the
// team through my-copilot, or by a person through an app (delegated, OBO).
// The orderer is (orderer_client_id, orderer_subject): ('', '') for a person
// via my-copilot, (azp, oid) for a delegated job. Names are unique per
// orderer, and a caller only ever sees its own orderer's jobs.

// ProfileRef names an agent profile.
type ProfileRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type AgentJobSpec struct {
	Type         string          `json:"type"`
	Params       json.RawMessage `json:"params,omitempty"`
	OnExpiry     string          `json:"onExpiry,omitempty"`
	MaxAttempts  int             `json:"maxAttempts,omitempty"`
	AgentProfile *ProfileRef     `json:"agentProfile,omitempty"`
}

type BoundProfile struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Generation int    `json:"generation"`
}

type AgentJobStatus struct {
	Conditions     []Condition     `json:"conditions"`
	Attempts       int             `json:"attempts"`
	StartTime      *time.Time      `json:"startTime,omitempty"`
	CompletionTime *time.Time      `json:"completionTime,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
	CreatedBy      Actor           `json:"createdBy"`
	// Orderer is the app for a delegated job; absent for a person via my-copilot.
	Orderer      *Actor        `json:"orderer,omitempty"`
	AgentProfile *BoundProfile `json:"agentProfile,omitempty"`
}

type AgentJob struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   ObjectMeta     `json:"metadata"`
	Spec       AgentJobSpec   `json:"spec"`
	Status     AgentJobStatus `json:"status"`
}

// TerminationRequest is POST .../agentjobs/{name}/termination.
type TerminationRequest struct {
	Mode   string `json:"mode"`
	Reason string `json:"reason,omitempty"`
}

// Names of jobs created before v1alpha1; a client may not choose one.
var legacyJobName = regexp.MustCompile(`^job-[0-9]+$`)

// boundProfile is a published, live agent profile.
type boundProfile struct {
	BoundProfile
	write bool // some grant allows more than Read
}

const liveProfileSQL = `SELECT r.namespace, r.name, g.generation, g.spec::text
	FROM agent_profile_resources r
	JOIN agent_profile_generations g ON g.namespace = r.namespace AND g.name = r.name AND g.generation = r.published_generation
	WHERE NOT r.revoked AND g.revoked_at IS NULL AND NOT g.blocked AND g.approved_at IS NOT NULL
	  AND g.identity_verified_at IS NOT NULL AND g.expires_at > now() AND `

func scanLiveProfile(row pgx.Row) (*boundProfile, error) {
	var p boundProfile
	var raw string
	if err := row.Scan(&p.Namespace, &p.Name, &p.Generation, &raw); err != nil {
		return nil, err
	}
	var spec AgentProfileSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return nil, err
	}
	for _, g := range spec.Grants {
		p.write = p.write || g.MaxRisk != "Read"
	}
	return &p, nil
}

// jobScope checks that c may use agent jobs in ns. A person must be a
// member (404 otherwise). A delegated app must be the identity of a live
// published profile (401 otherwise), and that profile must be in ns (404).
// The returned profile is the app's, nil for a person.
func jobScope(ctx context.Context, db *DB, c caller, ns string) (*boundProfile, error) {
	if c.ClientID == "" {
		return nil, memberOf(c, ns)
	}
	p, err := scanLiveProfile(db.pool.QueryRow(ctx, liveProfileSQL+`r.published_client_id = $1::uuid AND g.entra_client_id = $1::uuid`, c.ClientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, stErr(http.StatusUnauthorized, "Unauthorized", "the token's client is not a published agent profile")
	}
	if err != nil {
		return nil, err
	}
	if p.Namespace != ns {
		return nil, errV1NotFound
	}
	return p, nil
}

// ordererKey is (orderer_client_id, orderer_subject) for c.
func ordererKey(c caller) (string, string) {
	if c.ClientID == "" {
		return "", ""
	}
	return c.ClientID, c.Oid
}

const jobCols = `j.id, j.name, j.kind, j.params::text, j.state, j.on_expiry, j.attempts, j.max_attempts, j.result::text,
	j.created_by, j.created_by_kind, j.orderer_client_id, j.submitted_spec::text, j.agent_profile_generation,
	j.resource_version, j.created_at, j.updated_at, (SELECT min(l.created_at) FROM runtime_leases l WHERE l.job_id = j.id)`

// jobWhere selects the caller's own jobs in a team: $1 team, $2 client, $3 subject.
const jobWhere = ` FROM runtime_jobs j WHERE j.team = $1 AND j.orderer_client_id = $2 AND j.orderer_subject = $3`

// byName matches a job by name, or an unnamed old job by job-<id>: $4.
const byName = ` AND (j.name = $4 OR (j.name IS NULL AND 'job-' || j.id = $4))`

func scanJob(row pgx.Row, ns string) (int64, AgentJob, error) {
	var (
		id                    int64
		name, result, spec    *string
		kind, params, state   string
		onExpiry, by          string
		byKind                *string
		attempts, maxAttempts int
		client                string
		profileGen            *int
		rv                    int64
		created, updated      time.Time
		started               *time.Time
	)
	err := row.Scan(&id, &name, &kind, &params, &state, &onExpiry, &attempts, &maxAttempts, &result, &by, &byKind, &client, &spec, &profileGen, &rv, &created, &updated, &started)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, AgentJob{}, errV1NotFound
	}
	if err != nil {
		return 0, AgentJob{}, err
	}
	j := AgentJob{APIVersion: apiGroupVersion, Kind: "AgentJob"}
	j.Metadata = ObjectMeta{Name: "job-" + strconv.FormatInt(id, 10), Namespace: ns, ResourceVersion: strconv.FormatInt(rv, 10), Generation: 1, CreationTimestamp: &created}
	if name != nil {
		j.Metadata.Name = *name
	}
	if spec != nil {
		if err := json.Unmarshal([]byte(*spec), &j.Spec); err != nil {
			return 0, j, err
		}
	} else {
		// Created through /admin/v1: the spec follows from the columns.
		j.Spec = AgentJobSpec{Type: kind, Params: json.RawMessage(params), OnExpiry: upperFirst(onExpiry), MaxAttempts: maxAttempts}
	}
	st := &j.Status
	st.Attempts, st.StartTime = attempts, started
	st.CreatedBy = Actor{Name: by}
	if byKind != nil {
		st.CreatedBy.Kind = *byKind
	}
	if client != "" {
		st.Orderer = &Actor{Kind: "Application", Name: client, ClientID: client}
	}
	if profileGen != nil && j.Spec.AgentProfile != nil {
		st.AgentProfile = &BoundProfile{Namespace: j.Spec.AgentProfile.Namespace, Name: j.Spec.AgentProfile.Name, Generation: *profileGen}
	}
	if result != nil {
		st.Result = json.RawMessage(*result)
	}
	reason := upperFirst(state)
	done := state == "succeeded" || state == "failed" || state == "cancelled" || state == "killed"
	if done {
		st.CompletionTime = &updated
	}
	st.Conditions = []Condition{
		cond("Running", state == "leased", reason, 1, updated),
		cond("Complete", state == "succeeded", reason, 1, updated),
		cond("Failed", done && state != "succeeded", reason, 1, updated),
	}
	return id, j, nil
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func listJobsV1(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if _, err := jobScope(r.Context(), db, c, ns); err != nil {
		return err
	}
	client, subject := ordererKey(c)
	rows, err := db.pool.Query(r.Context(), `SELECT `+jobCols+jobWhere+` ORDER BY j.id`, ns, client, subject)
	if err != nil {
		return err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentJob, error) {
		_, j, scanErr := scanJob(row, ns)
		return j, scanErr
	})
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "AgentJobList", "items": emptyIfNil(items)})
	return nil
}

func getJobV1(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if _, err := jobScope(r.Context(), db, c, ns); err != nil {
		return err
	}
	client, subject := ordererKey(c)
	_, j, err := scanJob(db.pool.QueryRow(r.Context(), `SELECT `+jobCols+jobWhere+byName, ns, client, subject, r.PathValue("name")), ns)
	if err == nil {
		writeObject(w, http.StatusOK, j)
	}
	return err
}

// canonJobSpec validates spec and fills in the defaults, so a retry that
// writes a default out gives the same job.
func canonJobSpec(spec AgentJobSpec, p *boundProfile) (AgentJobSpec, error) {
	if !dnsLabelRegex.MatchString(spec.Type) {
		return spec, invalid("spec.type", "must be a lowercase DNS label")
	}
	if len(spec.Params) == 0 || string(spec.Params) == "null" {
		spec.Params = json.RawMessage("{}")
	}
	var obj map[string]any
	if len(spec.Params) > maxParams || json.Unmarshal(spec.Params, &obj) != nil || obj == nil {
		return spec, invalid("spec.params", "must be a JSON object of at most %d bytes", maxParams)
	}
	switch spec.OnExpiry {
	case "":
		spec.OnExpiry = "Requeue"
	case "Requeue", "Fail":
	default:
		return spec, invalid("spec.onExpiry", "must be Requeue or Fail")
	}
	switch {
	case spec.MaxAttempts == 0 && p.write:
		spec.MaxAttempts = 1
	case spec.MaxAttempts == 0:
		spec.MaxAttempts = 3
	case spec.MaxAttempts < 1 || spec.MaxAttempts > 10:
		return spec, invalid("spec.maxAttempts", "must be 1 to 10")
	case spec.MaxAttempts > 1 && p.write:
		return spec, invalid("spec.maxAttempts", "must be 1 for a profile that may write")
	}
	spec.AgentProfile = &ProfileRef{Namespace: p.Namespace, Name: p.Name}
	return spec, nil
}

func createJobV1(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ctx, ns := r.Context(), r.PathValue("namespace")
	app, err := jobScope(ctx, db, c, ns)
	if err != nil {
		return err
	}
	in, err := decodeResource[AgentJobSpec](w, r, "AgentJob", ns, "")
	if err != nil {
		return err
	}
	name := in.Metadata.Name
	if legacyJobName.MatchString(name) {
		return invalid("metadata.name", "job-<number> is reserved for jobs created before v1alpha1")
	}
	if len(in.Metadata.Labels) > 0 || len(in.Metadata.Annotations) > 0 {
		return invalid("metadata", "labels and annotations are not supported on AgentJob")
	}
	p := app
	ref := in.Spec.AgentProfile
	switch {
	case app != nil && ref != nil && (ref.Namespace != app.Namespace || ref.Name != app.Name):
		return invalid("spec.agentProfile", "must be the profile of the ordering app, %s/%s", app.Namespace, app.Name)
	case app == nil && ref == nil:
		return invalid("spec.agentProfile", "is required")
	case app == nil:
		if ref.Namespace != ns {
			return invalid("spec.agentProfile", "must be a profile in %s", ns)
		}
		p, err = scanLiveProfile(db.pool.QueryRow(ctx, liveProfileSQL+`r.namespace = $1 AND r.name = $2`, ref.Namespace, ref.Name))
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid("spec.agentProfile", "%s/%s is not a published agent profile", ref.Namespace, ref.Name)
		}
		if err != nil {
			return err
		}
	}
	spec, err := canonJobSpec(in.Spec, p)
	if err != nil {
		return err
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	client, subject := ordererKey(c)
	// The name check comes before the kill switch: a retry of a job that
	// exists gets it back even while the switch is on.
	existing := func() error {
		var same bool
		id, j, lookupErr := scanJob(db.pool.QueryRow(ctx, `SELECT `+jobCols+jobWhere+byName, ns, client, subject, name), ns)
		if lookupErr != nil {
			return lookupErr
		}
		if cmpErr := db.pool.QueryRow(ctx, `SELECT COALESCE(submitted_spec = $2::jsonb, false) FROM runtime_jobs WHERE id = $1`, id, string(specJSON)).Scan(&same); cmpErr != nil {
			return cmpErr
		}
		if !same {
			return &statusError{code: http.StatusConflict, reason: "AlreadyExists", msg: "a job named " + name + " exists with another spec"}
		}
		writeObject(w, http.StatusOK, j)
		return nil
	}
	if err = existing(); !errors.Is(err, errV1NotFound) {
		return err
	}
	var switched bool
	if err = db.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM runtime_kill_switches WHERE scope IN ('global', 'team:' || $1))`, ns).Scan(&switched); err != nil {
		return err
	}
	if switched {
		return stErr(http.StatusServiceUnavailable, "ServiceUnavailable", "the kill switch is on for %s", ns)
	}
	actor := c.actor()
	var id int64
	err = pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		insErr := tx.QueryRow(ctx, `INSERT INTO runtime_jobs (team, kind, params, on_expiry, max_attempts, created_by, created_by_kind, created_by_client_id, created_by_oid,
				name, submitted_spec, orderer_client_id, orderer_subject, agent_profile_generation)
			VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7, NULLIF($8, ''), $9::uuid, $10, $11::jsonb, $8, $12, $13)
			ON CONFLICT (team, orderer_client_id, orderer_subject, name) WHERE name IS NOT NULL DO NOTHING RETURNING id`,
			ns, spec.Type, string(spec.Params), strings.ToLower(spec.OnExpiry), spec.MaxAttempts, actor.Name, actor.Kind, client, c.Oid,
			name, string(specJSON), subject, p.Generation).Scan(&id)
		if insErr != nil {
			return insErr
		}
		return jobAudit(ctx, tx, actor.Kind, actor.Name, "job_created", ns, id, map[string]any{"name": name, "type": spec.Type, "orderer": client, "agent_profile": p.Namespace + "/" + p.Name, "generation": p.Generation})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// A twin with the same name won the insert.
		return existing()
	}
	if err != nil {
		return err
	}
	_, j, err := scanJob(db.pool.QueryRow(ctx, `SELECT `+jobCols+` FROM runtime_jobs j WHERE j.id = $1`, id), ns)
	if err == nil {
		writeObject(w, http.StatusCreated, j)
	}
	return err
}

func terminateJobV1(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ctx, ns := r.Context(), r.PathValue("namespace")
	if _, err := jobScope(ctx, db, c, ns); err != nil {
		return err
	}
	var req TerminationRequest
	if err := decodeBody(w, r, &req); err != nil {
		return stErr(http.StatusBadRequest, "BadRequest", "%v", err)
	}
	cmd := map[string]string{"Cancel": cmdCancel, "Kill": cmdKill}[req.Mode]
	if cmd == "" || len(req.Reason) > 500 {
		return stErr(http.StatusBadRequest, "BadRequest", "mode must be Cancel or Kill, and reason at most 500 characters")
	}
	client, subject := ordererKey(c)
	var id int64
	err := db.pool.QueryRow(ctx, `SELECT j.id`+jobWhere+byName, ns, client, subject, r.PathValue("name")).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errV1NotFound
	}
	if err != nil {
		return err
	}
	if err = db.SetJobCommand(ctx, c.actor().Name, ns, id, cmd, req.Reason); err != nil {
		return err
	}
	_, j, err := scanJob(db.pool.QueryRow(ctx, `SELECT `+jobCols+` FROM runtime_jobs j WHERE j.id = $1`, id), ns)
	if err == nil {
		writeObject(w, http.StatusOK, j)
	}
	return err
}
