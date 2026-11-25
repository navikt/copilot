package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/navikt/copilot/fleet-control/internal/auth"
	"github.com/navikt/copilot/fleet-control/internal/store"
)

// Job API for fleet-runtime (#2689). The contract is the request and
// response types below; README.md describes the routes.
//
// fleet-runtime has no inbound surface. It claims a job (long-poll), holds
// the lease by heartbeat, and stops the job itself when the heartbeat
// response says cancel or kill, or when no heartbeat has been acknowledged
// before lease_expires_at. fleet-control expires the lease at the same time,
// so a lost connection stops the job on both sides.

const (
	// defaultLeaseTTL is FLEET_RUNTIME_LEASE_TTL when unset.
	defaultLeaseTTL = 60 * time.Second
	// claimWait is the longest a claim waits for a job. It stays below common
	// proxy idle timeouts; the server sets no WriteTimeout, so nothing cuts it.
	claimWait = 25 * time.Second
	// maxParams bounds a job's params: a job carries a reference to its work,
	// not the work itself.
	maxParams = 16 << 10
)

// Commands a heartbeat delivers.
const (
	cmdContinue = "continue"
	cmdCancel   = "cancel"
	cmdKill     = "kill"
)

// ClaimRequest is POST /runtime/v1/claim. ClaimToken is chosen by the
// runtime per attempt; a retry with the same token returns the same lease.
type ClaimRequest struct {
	ClaimToken string `json:"claim_token"`
}

// Clocks: the runtime must not compare lease_expires_at with its own clock.
// Its local deadline is the monotonic time it SENT the request plus
// lease_ttl_seconds, so time spent in transit only shortens the lease. Any
// heartbeat answer other than 200, a timeout, a network error, or an answer
// that arrives after the local deadline is not an ack: the deadline stays,
// and the job stops when it passes. Each heartbeat request is bounded by the
// time left.

// Lease is the claim response (200). No job: 204 after the long-poll.
type Lease struct {
	LeaseID string `json:"lease_id"`
	// LeaseTTLSeconds is what the runtime counts down (see above).
	LeaseTTLSeconds int `json:"lease_ttl_seconds"`
	// LeaseExpiresAt is fleet-control's clock, for information only.
	LeaseExpiresAt time.Time       `json:"lease_expires_at"`
	JobID          int64           `json:"job_id"`
	Team           string          `json:"team"`
	Kind           string          `json:"kind"`
	Params         json.RawMessage `json:"params"`
	Attempt        int             `json:"attempt"`
}

// HeartbeatRequest is POST /runtime/v1/heartbeat.
type HeartbeatRequest struct {
	LeaseID string `json:"lease_id"`
}

// HeartbeatResponse carries the command. continue extends the lease. cancel
// asks the runtime to stop and report; the lease then runs for at most one
// TTL from the first delivery and is never extended past it. kill means stop
// now: the lease has ended (TTL 0). 410 Gone means the lease is no longer
// held: stop at once.
type HeartbeatResponse struct {
	Command         string    `json:"command"`
	LeaseTTLSeconds int       `json:"lease_ttl_seconds"`
	LeaseExpiresAt  time.Time `json:"lease_expires_at"`
}

// ttlSQL is the lease's remaining whole seconds, rounded down.
const ttlSQL = `GREATEST(0, FLOOR(EXTRACT(EPOCH FROM expires_at - clock_timestamp())))::int`

// FinishRequest is POST /runtime/v1/complete and /runtime/v1/fail. The
// summary is a status, never the job's content.
type FinishRequest struct {
	LeaseID string `json:"lease_id"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// CreateJobRequest is POST /admin/v1/jobs/{namespace}, by a team member.
type CreateJobRequest struct {
	Kind        string          `json:"kind"`
	Params      json.RawMessage `json:"params,omitempty"`
	OnExpiry    string          `json:"on_expiry,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
}

// KillSwitchRequest is PUT /admin/v1/jobs/{namespace}/kill-switch. Scope
// global needs the operator team as namespace.
type KillSwitchRequest struct {
	Scope  string `json:"scope"`
	On     bool   `json:"on"`
	Reason string `json:"reason"`
}

var (
	claimTokenRegex = regexp.MustCompile(`^[A-Za-z0-9._-]{16,128}$`)
	codeRegex       = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	uuidRegex       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// jobStore is implemented by *DB.
type jobStore interface {
	SetJobCommand(ctx context.Context, by caller, team string, id int64, cmd, reason string) error
	SetKillSwitch(ctx context.Context, actor, scope string, on bool, reason string, byOperator bool) error
	Claim(ctx context.Context, runtime string, teams []string, token string, ttl time.Duration) (*Lease, error)
	Heartbeat(ctx context.Context, runtime, leaseID string, ttl time.Duration) (HeartbeatResponse, error)
	Finish(ctx context.Context, runtime, leaseID string, ok bool, code, message string) error
}

// jobAudit keeps NAVident, kind, oid and client ID in their own columns
// (#2763); a caller without an oid (runtime, /admin/v1) leaves it empty.
func jobAudit(ctx context.Context, tx pgx.Tx, by caller, action, team string, id int64, detail map[string]any) error {
	return store.InsertAudit(ctx, tx, store.AuditRow{Actor: by.Ident, ActorKind: by.Kind, ActorOid: by.Oid, ActorClientID: by.ClientID, Action: action, Namespace: team, Name: "job/" + strconv.FormatInt(id, 10), Detail: detail})
}

// SetJobCommand cancels or kills one job. A queued job stops at once; a
// leased one gets the command on its next heartbeat.
func (d *DB) SetJobCommand(ctx context.Context, by caller, team string, id int64, cmd, reason string) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		var state, before string
		err := tx.QueryRow(ctx, `SELECT state, command FROM runtime_jobs WHERE id = $1 AND team = $2 FOR UPDATE`, id, team).Scan(&state, &before)
		if errors.Is(err, pgx.ErrNoRows) {
			return refuse(http.StatusNotFound, "job not found")
		}
		if err != nil {
			return err
		}
		next := state
		switch state {
		case "queued":
			next = map[string]string{cmdCancel: "cancelled", cmdKill: "killed"}[cmd]
		case "leased":
			if before == cmdKill {
				return refuse(http.StatusConflict, "job is already being killed")
			}
		default:
			return refuse(http.StatusConflict, "job is %s", state)
		}
		if _, err := tx.Exec(ctx, `UPDATE runtime_jobs SET command = $2, state = $3, updated_at = now() WHERE id = $1`, id, cmd, next); err != nil {
			return err
		}
		return jobAudit(ctx, tx, by, "job_"+cmd, team, id, map[string]any{"before": state, "after": next, "reason": reason})
	})
}

// killSwitchLock is the advisory lock key: SetKillSwitch holds it
// exclusively, a job insert shared.
const killSwitchLock = `hashtextextended('fleet-control/kill-switch', 0)`

// SetKillSwitch turns a team or the global kill switch on or off. While on,
// no job in scope is claimed and every leased one is killed on its next
// heartbeat. A switch the operator team set only the operator team clears.
func (d *DB) SetKillSwitch(ctx context.Context, actor, scope string, on bool, reason string, byOperator bool) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(`+killSwitchLock+`)`); err != nil {
			return err
		}
		var tag pgconn.CommandTag
		var err error
		if on {
			// The operator's mark sticks once set, so a team cannot downgrade it by setting the switch again.
			tag, err = tx.Exec(ctx, `INSERT INTO runtime_kill_switches (scope, reason, set_by, set_by_operator) VALUES ($1, $2, $3, $4)
				ON CONFLICT (scope) DO UPDATE SET reason = $2, set_by = $3, set_at = now(), set_by_operator = runtime_kill_switches.set_by_operator OR $4`, scope, reason, actor, byOperator)
		} else {
			var operatorSet bool
			err = tx.QueryRow(ctx, `SELECT set_by_operator FROM runtime_kill_switches WHERE scope = $1 FOR UPDATE`, scope).Scan(&operatorSet)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			if operatorSet && !byOperator {
				return refuse(http.StatusForbidden, "the operator team set this kill switch; only the operator team can turn it off")
			}
			tag, err = tx.Exec(ctx, `DELETE FROM runtime_kill_switches WHERE scope = $1`, scope)
		}
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		team, _ := strings.CutPrefix(scope, "team:")
		return store.InsertAudit(ctx, tx, store.AuditRow{Actor: actor, Action: "kill_switch", Namespace: team, Name: scope, Detail: map[string]any{"scope": scope, "on": on, "reason": reason}})
	})
}

// expireLeases ends leases past expiry. Their jobs go back to the queue or
// fail per on_expiry; a job with a pending cancel or kill does not return.
func expireLeases(ctx context.Context, tx pgx.Tx) (int, error) {
	rows, err := tx.Query(ctx, `UPDATE runtime_leases SET ended_at = now()
		WHERE id IN (SELECT id FROM runtime_leases WHERE ended_at IS NULL AND expires_at <= now() FOR UPDATE SKIP LOCKED)
		RETURNING id::text, job_id, runtime`)
	if err != nil {
		return 0, err
	}
	type expired struct {
		lease, runtime string
		job            int64
	}
	var all []expired
	for rows.Next() {
		var e expired
		if err := rows.Scan(&e.lease, &e.job, &e.runtime); err != nil {
			rows.Close()
			return 0, err
		}
		all = append(all, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, e := range all {
		var team, state string
		if err := tx.QueryRow(ctx, `UPDATE runtime_jobs SET updated_at = now(), state = CASE
				WHEN command = 'kill' THEN 'killed'
				WHEN command = 'cancel' THEN 'cancelled'
				WHEN on_expiry = 'requeue' AND attempts < max_attempts THEN 'queued'
				ELSE 'failed' END
			WHERE id = $1 RETURNING team, state`, e.job).Scan(&team, &state); err != nil {
			return 0, err
		}
		if err := jobAudit(ctx, tx, caller{Kind: "Application", Ident: e.runtime}, "lease_expired", team, e.job, map[string]any{"lease_id": e.lease, "after": state}); err != nil {
			return 0, err
		}
	}
	return len(all), nil
}

// SweepLeases expires leases in one transaction. SKIP LOCKED lets several
// replicas sweep at once without handling a lease twice.
func (d *DB) SweepLeases(ctx context.Context) (int, error) {
	var n int
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		var err error
		n, err = expireLeases(ctx, tx)
		return err
	})
	return n, err
}

// sweepLoop runs SweepLeases every interval until ctx ends, so leases expire
// even when no runtime claims.
func sweepLoop(ctx context.Context, d *DB, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if n, err := d.SweepLeases(sctx); err != nil {
			slog.Error("lease sweep failed", "error", err)
		} else if n > 0 {
			slog.Info("lease sweep", "expired", n)
		}
		cancel()
	}
}

// sweepIntervalFromEnv reads FLEET_RUNTIME_SWEEP_INTERVAL (1s to 5m, default 15s).
func sweepIntervalFromEnv() (time.Duration, error) {
	raw := os.Getenv("FLEET_RUNTIME_SWEEP_INTERVAL")
	if raw == "" {
		return 15 * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < time.Second || d > 5*time.Minute {
		return 0, errors.New("FLEET_RUNTIME_SWEEP_INTERVAL must be a duration from 1s to 5m")
	}
	return d, nil
}

// errNoJob means nothing was claimable.
var errNoJob = errors.New("no job")

// Claim leases the oldest claimable job of teams to runtime. A repeated token returns
// the lease it got first; concurrent claims never share a job (SKIP LOCKED).
func (d *DB) Claim(ctx context.Context, runtime string, teams []string, token string, ttl time.Duration) (*Lease, error) {
	if l, err := d.leaseByToken(ctx, runtime, token); err == nil || !errors.Is(err, pgx.ErrNoRows) {
		return l, err
	}
	// Also swept here, so a claim sees freed jobs before the next tick. It
	// commits on its own: a claim that finds no job rolls back.
	if _, err := d.SweepLeases(ctx); err != nil {
		return nil, err
	}
	var leaseID string
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		var id int64
		var team string
		var attempt int
		// Serialise claims per token: otherwise a racing twin can skip-lock past
		// the oldest job and win with a younger one. The unique index stays the backstop.
		// Separate statements: READ COMMITTED snapshots per statement, so the
		// check must start after the lock is held to see the twin's lease.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || '/' || $2, 0))`, runtime, token); err != nil {
			return err
		}
		var leased bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM runtime_leases WHERE runtime = $1 AND claim_token = $2)`, runtime, token).Scan(&leased); err != nil {
			return err
		}
		if leased {
			return nil
		}
		err := tx.QueryRow(ctx, `SELECT id, team, attempts + 1 FROM runtime_jobs j
			WHERE state = 'queued' AND team = ANY($1)
			  AND NOT EXISTS (SELECT 1 FROM runtime_kill_switches k WHERE k.scope IN ('global', 'team:' || j.team))
			ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`, teams).Scan(&id, &team, &attempt)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoJob
		}
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO runtime_leases (job_id, runtime, claim_token, expires_at) VALUES ($1, $2, $3, now() + $4 * interval '1 millisecond') RETURNING id::text`,
			id, runtime, token, ttl.Milliseconds()).Scan(&leaseID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE runtime_jobs SET state = 'leased', attempts = $2, updated_at = now() WHERE id = $1`, id, attempt); err != nil {
			return err
		}
		return jobAudit(ctx, tx, caller{Kind: "Application", Ident: runtime}, "job_claimed", team, id, map[string]any{"lease_id": leaseID, "attempt": attempt})
	})
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		// The same token raced itself; the winner's lease is the answer.
		return d.leaseByToken(ctx, runtime, token)
	}
	if err != nil {
		return nil, err
	}
	return d.leaseByToken(ctx, runtime, token)
}

func (d *DB) leaseByToken(ctx context.Context, runtime, token string) (*Lease, error) {
	var l Lease
	var params string
	var ended *time.Time
	var expired bool
	err := d.pool.QueryRow(ctx, `SELECT l.id::text, `+strings.ReplaceAll(ttlSQL, "expires_at", "l.expires_at")+`, l.expires_at, l.ended_at, l.expires_at <= clock_timestamp(), j.id, j.team, j.kind, j.params::text, j.attempts
		FROM runtime_leases l JOIN runtime_jobs j ON j.id = l.job_id WHERE l.runtime = $1 AND l.claim_token = $2`, runtime, token).
		Scan(&l.LeaseID, &l.LeaseTTLSeconds, &l.LeaseExpiresAt, &ended, &expired, &l.JobID, &l.Team, &l.Kind, &params, &l.Attempt)
	if err != nil {
		return nil, err
	}
	// Expired but not yet swept counts as ended, as in heartbeat and finish.
	if ended != nil || expired {
		return nil, refuse(http.StatusGone, "lease has ended; claim with a new token")
	}
	l.Params = json.RawMessage(params)
	return &l, nil
}

// lockLease locks runtime's active lease and its job for the rest of tx.
func lockLease(ctx context.Context, tx pgx.Tx, runtime, leaseID string) (job int64, team, cmd string, err error) {
	var ended *time.Time
	var expired bool
	err = tx.QueryRow(ctx, `SELECT l.job_id, l.ended_at, j.team, j.command
		FROM runtime_leases l JOIN runtime_jobs j ON j.id = l.job_id
		WHERE l.id = $1::uuid AND l.runtime = $2 FOR UPDATE OF l, j`, leaseID, runtime).Scan(&job, &ended, &team, &cmd)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", "", refuse(http.StatusNotFound, "lease not found")
	}
	// Check expiry after the lock, on the wall clock: now() is fixed at
	// transaction start and would pass a lease that expired while waiting.
	if err == nil {
		err = tx.QueryRow(ctx, `SELECT expires_at <= clock_timestamp() FROM runtime_leases WHERE id = $1::uuid`, leaseID).Scan(&expired)
	}
	if err == nil && (ended != nil || expired) {
		err = refuse(http.StatusGone, "lease is no longer held; stop the job")
	}
	return job, team, cmd, err
}

// Heartbeat extends a held lease and returns the job's command. A kill
// switch in scope turns it into kill. Kill ends the lease at once.
func (d *DB) Heartbeat(ctx context.Context, runtime, leaseID string, ttl time.Duration) (HeartbeatResponse, error) {
	var out HeartbeatResponse
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		job, team, cmd, err := lockLease(ctx, tx, runtime, leaseID)
		if err != nil {
			return err
		}
		var switched bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM runtime_kill_switches WHERE scope IN ('global', 'team:' || $1))`, team).Scan(&switched); err != nil {
			return err
		}
		if switched {
			cmd = cmdKill
		}
		out.Command = cmd
		if cmd == cmdKill {
			if err := tx.QueryRow(ctx, `UPDATE runtime_leases SET ended_at = now() WHERE id = $1::uuid RETURNING ended_at`, leaseID).Scan(&out.LeaseExpiresAt); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE runtime_jobs SET state = 'killed', command = 'kill', updated_at = now() WHERE id = $1`, job); err != nil {
				return err
			}
			return jobAudit(ctx, tx, caller{Kind: "Application", Ident: runtime}, "kill_delivered", team, job, map[string]any{"lease_id": leaseID, "kill_switch": switched})
		}
		// shortcut: a plain extension is not audited (one row per heartbeat
		// per job); claim, commands and lease end are.
		// After cancel is delivered the lease is capped at one TTL from the
		// first delivery; the sweeper then marks the job cancelled.
		var first bool
		if err := tx.QueryRow(ctx, `UPDATE runtime_leases SET
				cancel_delivered_at = CASE WHEN $3 THEN COALESCE(cancel_delivered_at, now()) END,
				expires_at = CASE WHEN $3 THEN LEAST(clock_timestamp(), COALESCE(cancel_delivered_at, now())) ELSE clock_timestamp() END + $2 * interval '1 millisecond'
			WHERE id = $1::uuid
			RETURNING expires_at, `+ttlSQL+`, $3 AND cancel_delivered_at = now()`, leaseID, ttl.Milliseconds(), cmd == cmdCancel).
			Scan(&out.LeaseExpiresAt, &out.LeaseTTLSeconds, &first); err != nil {
			return err
		}
		if first {
			return jobAudit(ctx, tx, caller{Kind: "Application", Ident: runtime}, "cancel_delivered", team, job, map[string]any{"lease_id": leaseID})
		}
		return nil
	})
	return out, err
}

// Finish ends a held lease with a result. A pending kill (job command or
// kill switch) or cancel wins over the reported result; the result is kept
// in result and the audit row.
func (d *DB) Finish(ctx context.Context, runtime, leaseID string, ok bool, code, message string) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		job, team, cmd, err := lockLease(ctx, tx, runtime, leaseID)
		if err != nil {
			return err
		}
		var switched bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM runtime_kill_switches WHERE scope IN ('global', 'team:' || $1))`, team).Scan(&switched); err != nil {
			return err
		}
		state := "failed"
		switch {
		case cmd == cmdKill || switched:
			state = "killed"
		case cmd == cmdCancel:
			state = "cancelled"
		case ok:
			state = "succeeded"
		}
		result, _ := json.Marshal(map[string]string{"code": code, "message": message})
		if _, err := tx.Exec(ctx, `UPDATE runtime_leases SET ended_at = now() WHERE id = $1::uuid`, leaseID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE runtime_jobs SET state = $2, result = $3::jsonb, updated_at = now() WHERE id = $1`, job, state, string(result)); err != nil {
			return err
		}
		return jobAudit(ctx, tx, caller{Kind: "Application", Ident: runtime}, "job_"+state, team, job, map[string]any{"lease_id": leaseID, "code": code, "reported_ok": ok})
	})
}

// runtimeAuth lets only fleet-runtime's Entra app tokens through: idtyp=app
// and an azp on the exact allowlist.
type runtimeAuth struct {
	a     *auth.Authz
	allow map[string][]string // client id to the teams whose jobs it may claim
	ttl   time.Duration
	wait  time.Duration
}

// runtimeAuthFromEnv reads FLEET_RUNTIME_CLIENT_IDS (comma-separated
// <Entra client id>=<team>[|<team>...]) and FLEET_RUNTIME_LEASE_TTL. An unset allowlist returns nil:
// the /runtime routes are not mounted.
func runtimeAuthFromEnv(a *auth.Authz) (*runtimeAuth, error) {
	return newRuntimeAuth(a, os.Getenv("FLEET_RUNTIME_CLIENT_IDS"), os.Getenv("FLEET_RUNTIME_LEASE_TTL"))
}

func newRuntimeAuth(a *auth.Authz, ids, ttlRaw string) (*runtimeAuth, error) {
	allow := map[string][]string{}
	for entry := range strings.SplitSeq(ids, ",") {
		if entry = strings.TrimSpace(entry); entry == "" {
			continue
		}
		// A runtime claims only the jobs of the teams it serves (#2854).
		id, teamList, _ := strings.Cut(entry, "=")
		var teams []string
		for team := range strings.SplitSeq(teamList, "|") {
			if !dnsLabelRegex.MatchString(team) {
				return nil, fmt.Errorf("FLEET_RUNTIME_CLIENT_IDS: %s needs =<team>[|<team>...] with valid team names", id)
			}
			teams = append(teams, team)
		}
		if a.KnownClient(id) {
			return nil, fmt.Errorf("FLEET_RUNTIME_CLIENT_IDS: %s is already another caller", id)
		}
		allow[id] = teams
	}
	if len(allow) == 0 {
		return nil, nil
	}
	ttl := defaultLeaseTTL
	if ttlRaw != "" {
		var err error
		if ttl, err = time.ParseDuration(ttlRaw); err != nil || ttl < 10*time.Second || ttl > 10*time.Minute {
			return nil, errors.New("FLEET_RUNTIME_LEASE_TTL must be a duration from 10s to 10m")
		}
	}
	return &runtimeAuth{a: a, allow: allow, ttl: ttl, wait: claimWait}, nil
}

type runtimeHandler func(w http.ResponseWriter, r *http.Request, js jobStore, runtime string) error

// guard checks the token (401, 503 when Texas fails), the allowlist (403)
// and the database (503).
func (s *server) runtimeRoute(ra *runtimeAuth, h runtimeHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := ra.a.Introspect(r.Context(), r.Header.Get("Authorization"))
		if err == nil && claims.Idtyp != "app" {
			err = auth.ErrUnauthorized
		}
		if !writeAuthError(w, err) {
			return
		}
		if _, ok := ra.allow[claims.Azp]; !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		js := s.jobs()
		if js == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		writeAPIError(w, "runtime", h(w, r, js, claims.Azp))
	}
}

func (s *server) jobs() jobStore {
	if box := s.store.Load(); box != nil {
		js, _ := box.snapshotStore.(jobStore)
		return js
	}
	return nil
}

func dbCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

func (ra *runtimeAuth) claim(w http.ResponseWriter, r *http.Request, js jobStore, runtime string) error {
	var req ClaimRequest
	if err := decodeBody(w, r, &req); err != nil {
		return err
	}
	if !claimTokenRegex.MatchString(req.ClaimToken) {
		return refuse(http.StatusBadRequest, "claim_token must be 16 to 128 characters of [A-Za-z0-9._-]")
	}
	deadline := time.Now().Add(ra.wait)
	for {
		ctx, cancel := dbCtx(r)
		l, err := js.Claim(ctx, runtime, ra.allow[runtime], req.ClaimToken, ra.ttl)
		cancel()
		if err == nil {
			writeJSON(w, l)
			return nil
		}
		if !errors.Is(err, errNoJob) {
			return err
		}
		// shortcut: polls every second; LISTEN/NOTIFY if claim load matters.
		if time.Until(deadline) < time.Second {
			w.WriteHeader(http.StatusNoContent)
			return nil
		}
		select {
		case <-r.Context().Done():
			return nil
		case <-time.After(time.Second):
		}
	}
}

func validLease(id string) error {
	if !uuidRegex.MatchString(id) {
		return refuse(http.StatusBadRequest, "lease_id must be a lowercase UUID")
	}
	return nil
}

func (ra *runtimeAuth) heartbeat(w http.ResponseWriter, r *http.Request, js jobStore, runtime string) error {
	var req HeartbeatRequest
	if err := decodeBody(w, r, &req); err != nil {
		return err
	}
	if err := validLease(req.LeaseID); err != nil {
		return err
	}
	ctx, cancel := dbCtx(r)
	defer cancel()
	out, err := js.Heartbeat(ctx, runtime, req.LeaseID, ra.ttl)
	if err == nil {
		writeJSON(w, out)
	}
	return err
}

func finish(ok bool) runtimeHandler {
	return func(w http.ResponseWriter, r *http.Request, js jobStore, runtime string) error {
		var req FinishRequest
		if err := decodeBody(w, r, &req); err != nil {
			return err
		}
		if err := validLease(req.LeaseID); err != nil {
			return err
		}
		if !codeRegex.MatchString(req.Code) || len(req.Message) > 500 {
			return refuse(http.StatusBadRequest, "code must match [a-z0-9_]{1,64} and message be at most 500 characters")
		}
		ctx, cancel := dbCtx(r)
		defer cancel()
		if err := js.Finish(ctx, runtime, req.LeaseID, ok, req.Code, req.Message); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// Admin job handlers run behind s.admin, which has checked team membership
// of {namespace}.

// putKillSwitch sets the team switch, or the global one when the caller's
// namespace is operatorTeam. An unset operatorTeam refuses global. The
// operator team may also name another team ("team:<name>").
func putKillSwitch(operatorTeam string) adminHandler {
	return func(w http.ResponseWriter, r *http.Request, js jobStore, actor, team string) error {
		var req KillSwitchRequest
		if err := decodeBody(w, r, &req); err != nil {
			return err
		}
		if req.Reason = strings.TrimSpace(req.Reason); req.On && (req.Reason == "" || len(req.Reason) > 500) {
			return refuse(http.StatusBadRequest, "reason must be 1 to 500 characters")
		}
		byOperator := operatorTeam != "" && team == operatorTeam
		scope := "team:" + team
		other, named := strings.CutPrefix(req.Scope, "team:")
		switch {
		case req.Scope == "team":
		case req.Scope == "global" || named:
			if !byOperator {
				return refuse(http.StatusForbidden, "the global kill switch and other teams' switches are for the operator team")
			}
			if scope = "global"; named {
				if !dnsLabelRegex.MatchString(other) {
					return refuse(http.StatusBadRequest, "team:<name> needs a valid team name")
				}
				scope = req.Scope
			}
		default:
			return refuse(http.StatusBadRequest, "scope must be team, team:<name> or global")
		}
		if err := js.SetKillSwitch(r.Context(), actor, scope, req.On, req.Reason, byOperator); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// writeAuthError answers a failed token check and reports whether err was nil.
func writeAuthError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, auth.ErrUnauthorized):
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	default:
		slog.Error("token introspection failed", "error", err)
		http.Error(w, "token validation unavailable", http.StatusServiceUnavailable)
	}
	return false
}

// writeAPIError answers an apiError with its code and anything else with 503.
func writeAPIError(w http.ResponseWriter, what string, err error) {
	if err == nil {
		return
	}
	if apiErr, ok := errors.AsType[*apiError](err); ok {
		http.Error(w, apiErr.msg, apiErr.code)
		return
	}
	slog.Error(what+" failed", "error", err)
	http.Error(w, "database unavailable", http.StatusServiceUnavailable)
}
