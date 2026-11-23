package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	runtimeID      = "55555555-5555-5555-5555-555555555555"
	otherRuntimeID = "66666666-6666-6666-6666-666666666666"
	// secondRuntimeID is allowlisted too, but holds no lease.
	secondRuntimeID = "77777777-7777-7777-7777-777777777777"
)

func testRuntimeAuth(t *testing.T) *runtimeAuth {
	t.Helper()
	ra, err := newRuntimeAuth(testAuthz(t), runtimeID+","+secondRuntimeID, "")
	if err != nil || ra == nil {
		t.Fatalf("runtime auth: %v", err)
	}
	ra.wait = 0 // claims answer at once in tests
	return ra
}

func TestRuntimeAuthConfig(t *testing.T) {
	a := testAuthz(t)
	if ra, err := newRuntimeAuth(a, " , ", ""); err != nil || ra != nil {
		t.Errorf("unset allowlist must leave the routes off: %v %v", ra, err)
	}
	if _, err := newRuntimeAuth(a, gatewayID, ""); err == nil {
		t.Error("a runtime id equal to another caller must be refused")
	}
	for _, ttl := range []string{"5s", "1h", "sixty"} {
		if _, err := newRuntimeAuth(a, runtimeID, ttl); err == nil {
			t.Errorf("ttl %q: want error", ttl)
		}
	}
	if ra, err := newRuntimeAuth(a, runtimeID+","+otherRuntimeID, "90s"); err != nil || ra.ttl != 90*time.Second || len(ra.allow) != 2 {
		t.Errorf("valid config: %+v %v", ra, err)
	}
	if ra, _ := newRuntimeAuth(a, runtimeID, ""); ra.ttl != defaultLeaseTTL {
		t.Errorf("default ttl: %v", ra.ttl)
	}
}

func TestRuntimeGuardsFailClosed(t *testing.T) {
	srv := newServer() // no database
	h := srv.routes(testAuthz(t), "", nil, testRuntimeAuth(t))
	body := `{"claim_token":"aaaaaaaaaaaaaaaa"}`
	cases := map[string]struct {
		h    http.Handler
		tok  string
		want int
	}{
		"routes off":        {srv.routes(testAuthz(t), "", nil, nil), runtimeID, 404},
		"no token":          {h, "", 401},
		"user token":        {h, "user:" + runtimeID + ":A111111", 401},
		"inactive":          {h, "inactive", 401},
		"texas fails":       {h, "texas-500", 503},
		"azp not allowed":   {h, otherRuntimeID, 403},
		"gateway not here":  {h, gatewayID, 403},
		"database down":     {h, runtimeID, 503},
		"allowlisted works": {h, runtimeID, 503},
	}
	for name, c := range cases {
		if rec := do(t, c.h, http.MethodPost, "/runtime/v1/claim", c.tok, body); rec.Code != c.want {
			t.Errorf("%s: got %d, want %d (%s)", name, rec.Code, c.want, rec.Body)
		}
	}
}

// The long-poll must outlive ReadTimeout: it bounds reading the request, not
// the response.
func TestLongPollOutlivesReadTimeout(t *testing.T) {
	ctxErr := make(chan error, 1)
	hs := newHTTPServer("0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(600 * time.Millisecond)
		ctxErr <- r.Context().Err()
		_, _ = io.WriteString(w, "job")
	}))
	if hs.WriteTimeout != 0 && hs.WriteTimeout <= claimWait+5*time.Second {
		t.Fatalf("WriteTimeout %v would cut the claim long-poll", hs.WriteTimeout)
	}
	hs.ReadTimeout = 200 * time.Millisecond // stands in for 30 s
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = hs.Serve(ln) }()
	t.Cleanup(func() { _ = hs.Close() })
	resp, err := http.Post("http://"+ln.Addr().String(), "application/json", nil) //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "job" || <-ctxErr != nil {
		t.Errorf("long-poll cut: %q", got)
	}
}

type jobEnv struct {
	t  *testing.T
	db *DB
	h  http.Handler
}

func newJobEnv(t *testing.T) *jobEnv {
	t.Helper()
	db := openTestDB(t)
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv := newServer()
	srv.cluster = "dev-gcp"
	srv.operatorTeam = "team-b"
	srv.store.Store(&storeBox{db})
	return &jobEnv{t, db, srv.routes(testAuthz(t), adminID, testTeams, testRuntimeAuth(t))}
}

func (e *jobEnv) createJob(body string) int64 {
	e.t.Helper()
	rec := do(e.t, e.h, http.MethodPost, "/admin/v1/jobs/team-a", userTok("A111111"), body)
	var out struct {
		JobID int64 `json:"job_id"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		e.t.Fatalf("create job: %d %s", rec.Code, rec.Body)
	}
	return out.JobID
}

func (e *jobEnv) claim(token string) (int, Lease) {
	e.t.Helper()
	rec := do(e.t, e.h, http.MethodPost, "/runtime/v1/claim", runtimeID, `{"claim_token":"`+token+`"}`)
	var l Lease
	if rec.Code == 200 {
		_ = json.Unmarshal(rec.Body.Bytes(), &l)
	}
	return rec.Code, l
}

func (e *jobEnv) heartbeat(lease string) (int, string) {
	e.t.Helper()
	rec := do(e.t, e.h, http.MethodPost, "/runtime/v1/heartbeat", runtimeID, `{"lease_id":"`+lease+`"}`)
	var out HeartbeatResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out.Command
}

func (e *jobEnv) state(id int64) (state string, attempts int) {
	e.t.Helper()
	if err := e.db.pool.QueryRow(context.Background(), `SELECT state, attempts FROM runtime_jobs WHERE id = $1`, id).Scan(&state, &attempts); err != nil {
		e.t.Fatal(err)
	}
	return state, attempts
}

func (e *jobEnv) audit(id int64) []string {
	e.t.Helper()
	rows, err := e.db.pool.Query(context.Background(), `SELECT action FROM audit_events WHERE name = $1 ORDER BY id`, fmt.Sprintf("job/%d", id))
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		out = append(out, a)
	}
	return out
}

func tok(i int) string { return fmt.Sprintf("claim-token-%04d", i) }

func TestRealPostgresJobCreateRefusals(t *testing.T) {
	e := newJobEnv(t)
	for name, c := range map[string]struct {
		path, tok, body string
		want            int
	}{
		"not a member": {"/admin/v1/jobs/team-a", userTok("B111111"), `{"kind":"k"}`, 403},
		"bad kind":     {"/admin/v1/jobs/team-a", userTok("A111111"), `{"kind":"K K"}`, 400},
		"params array": {"/admin/v1/jobs/team-a", userTok("A111111"), `{"kind":"k","params":[1]}`, 400},
		"bad expiry":   {"/admin/v1/jobs/team-a", userTok("A111111"), `{"kind":"k","on_expiry":"never"}`, 400},
		"app token":    {"/admin/v1/jobs/team-a", runtimeID, `{"kind":"k"}`, 401},
	} {
		if rec := do(t, e.h, http.MethodPost, c.path, c.tok, c.body); rec.Code != c.want {
			t.Errorf("%s: got %d, want %d (%s)", name, rec.Code, c.want, rec.Body)
		}
	}
}

func TestRealPostgresConcurrentClaimsOneWinner(t *testing.T) {
	e := newJobEnv(t)
	id := e.createJob(`{"kind":"review","params":{"repo":"navikt/x"}}`)
	var wg sync.WaitGroup
	codes := make(chan int, 20)
	for i := range 20 {
		wg.Go(func() {
			c, _ := e.claim(tok(i))
			codes <- c
		})
	}
	wg.Wait()
	close(codes)
	won := 0
	for c := range codes {
		switch c {
		case 200:
			won++
		case 204:
		default:
			t.Errorf("claim: %d", c)
		}
	}
	if won != 1 {
		t.Fatalf("want exactly one winner, got %d", won)
	}
	if st, n := e.state(id); st != "leased" || n != 1 {
		t.Errorf("job: %s attempts %d", st, n)
	}
}

func TestRealPostgresIdempotentClaim(t *testing.T) {
	e := newJobEnv(t)
	id := e.createJob(`{"kind":"review"}`)
	e.createJob(`{"kind":"review"}`)
	var wg sync.WaitGroup
	leases := make(chan Lease, 5)
	for range 5 {
		wg.Go(func() {
			if c, l := e.claim(tok(1)); c == 200 {
				leases <- l
			}
		})
	}
	wg.Wait()
	close(leases)
	first := <-leases
	for l := range leases {
		if l.LeaseID != first.LeaseID || l.JobID != first.JobID {
			t.Errorf("same token, different lease: %+v vs %+v", l, first)
		}
	}
	if first.JobID != id || first.LeaseID == "" || time.Until(first.LeaseExpiresAt) < 50*time.Second {
		t.Errorf("lease: %+v", first)
	}
	// The second job is still there for another token.
	if c, l := e.claim(tok(2)); c != 200 || l.JobID == id {
		t.Errorf("second claim: %d %+v", c, l)
	}
	// A finished lease is not handed out again under its token.
	if rec := do(t, e.h, http.MethodPost, "/runtime/v1/complete", runtimeID, `{"lease_id":"`+first.LeaseID+`","code":"ok"}`); rec.Code != 204 {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body)
	}
	if c, _ := e.claim(tok(1)); c != http.StatusGone {
		t.Errorf("claim on finished token: %d", c)
	}
	if st, _ := e.state(id); st != "succeeded" {
		t.Errorf("state %s", st)
	}
	if got := fmt.Sprint(e.audit(id)); got != "[job_created job_claimed job_succeeded]" {
		t.Errorf("audit: %s", got)
	}
}

func TestRealPostgresLeaseExpiry(t *testing.T) {
	e := newJobEnv(t)
	requeue := e.createJob(`{"kind":"review","max_attempts":2}`)
	fail := e.createJob(`{"kind":"review","on_expiry":"fail"}`)
	expire := func(job int64) {
		if _, err := e.db.pool.Exec(context.Background(), `UPDATE runtime_leases SET expires_at = now() - interval '1 second' WHERE job_id = $1 AND ended_at IS NULL`, job); err != nil {
			t.Fatal(err)
		}
	}
	_, l1 := e.claim(tok(1))
	_, l2 := e.claim(tok(2))
	if l1.JobID != requeue || l2.JobID != fail {
		t.Fatalf("claim order: %d %d", l1.JobID, l2.JobID)
	}
	expire(requeue)
	expire(fail)
	if c, _ := e.heartbeat(l1.LeaseID); c != http.StatusGone {
		t.Errorf("heartbeat on expired lease: %d", c)
	}
	// The next claim sweeps: requeue comes back as attempt 2, fail fails.
	c, l3 := e.claim(tok(3))
	if c != 200 || l3.JobID != requeue || l3.Attempt != 2 {
		t.Fatalf("requeued claim: %d %+v", c, l3)
	}
	if st, _ := e.state(fail); st != "failed" {
		t.Errorf("on_expiry fail: %s", st)
	}
	if rec := do(t, e.h, http.MethodPost, "/runtime/v1/complete", runtimeID, `{"lease_id":"`+l1.LeaseID+`","code":"ok"}`); rec.Code != http.StatusGone {
		t.Errorf("complete on expired lease: %d", rec.Code)
	}
	// Out of attempts: the job fails instead of returning.
	expire(requeue)
	if c, _ := e.claim(tok(4)); c != 204 {
		t.Errorf("claim after last attempt: %d", c)
	}
	if st, n := e.state(requeue); st != "failed" || n != 2 {
		t.Errorf("after max attempts: %s %d", st, n)
	}
	if got := fmt.Sprint(e.audit(requeue)); got != "[job_created job_claimed lease_expired job_claimed lease_expired]" {
		t.Errorf("audit: %s", got)
	}
}

func TestRealPostgresHeartbeatAndKill(t *testing.T) {
	e := newJobEnv(t)
	id := e.createJob(`{"kind":"review"}`)
	_, l := e.claim(tok(1))
	if c, cmd := e.heartbeat(l.LeaseID); c != 200 || cmd != cmdContinue {
		t.Fatalf("heartbeat: %d %s", c, cmd)
	}
	// Another allowlisted runtime cannot heartbeat or finish this lease.
	for path, body := range map[string]string{
		"/runtime/v1/heartbeat": `{"lease_id":"` + l.LeaseID + `"}`,
		"/runtime/v1/complete":  `{"lease_id":"` + l.LeaseID + `","code":"ok"}`,
	} {
		if rec := do(t, e.h, http.MethodPost, path, secondRuntimeID, body); rec.Code != 404 {
			t.Errorf("%s by another runtime: %d", path, rec.Code)
		}
	}
	if rec := do(t, e.h, http.MethodPost, fmt.Sprintf("/admin/v1/jobs/team-b/%d/kill", id), userTok("B111111"), ""); rec.Code != 404 {
		t.Errorf("kill from another team: %d", rec.Code)
	}
	if rec := do(t, e.h, http.MethodPost, fmt.Sprintf("/admin/v1/jobs/team-a/%d/kill", id), userTok("A222222"), ""); rec.Code != 204 {
		t.Fatalf("kill: %d %s", rec.Code, rec.Body)
	}
	if c, cmd := e.heartbeat(l.LeaseID); c != 200 || cmd != cmdKill {
		t.Fatalf("kill delivery: %d %s", c, cmd)
	}
	if c, _ := e.heartbeat(l.LeaseID); c != http.StatusGone {
		t.Errorf("heartbeat after kill: %d", c)
	}
	if st, _ := e.state(id); st != "killed" {
		t.Errorf("state %s", st)
	}
	if got := fmt.Sprint(e.audit(id)); got != "[job_created job_claimed job_kill kill_delivered]" {
		t.Errorf("audit: %s", got)
	}
}

func TestRealPostgresCancel(t *testing.T) {
	e := newJobEnv(t)
	id := e.createJob(`{"kind":"review"}`)
	queued := e.createJob(`{"kind":"review"}`)
	_, l := e.claim(tok(1))
	for _, job := range []int64{id, queued} {
		if rec := do(t, e.h, http.MethodPost, fmt.Sprintf("/admin/v1/jobs/team-a/%d/cancel", job), userTok("A111111"), ""); rec.Code != 204 {
			t.Fatalf("cancel: %d %s", rec.Code, rec.Body)
		}
	}
	if st, _ := e.state(queued); st != "cancelled" {
		t.Errorf("queued job cancelled at once: %s", st)
	}
	if c, cmd := e.heartbeat(l.LeaseID); c != 200 || cmd != cmdCancel {
		t.Fatalf("cancel delivery: %d %s", c, cmd)
	}
	if rec := do(t, e.h, http.MethodPost, "/runtime/v1/fail", runtimeID, `{"lease_id":"`+l.LeaseID+`","code":"cancelled"}`); rec.Code != 204 {
		t.Fatalf("fail: %d %s", rec.Code, rec.Body)
	}
	if st, _ := e.state(id); st != "cancelled" {
		t.Errorf("state %s", st)
	}
}

func TestRealPostgresKillSwitches(t *testing.T) {
	e := newJobEnv(t)
	id := e.createJob(`{"kind":"review"}`)
	e.createJob(`{"kind":"review"}`)
	_, l := e.claim(tok(1))
	sw := func(team, ident, body string) int {
		return do(t, e.h, http.MethodPut, "/admin/v1/jobs/"+team+"/kill-switch", userTok(ident), body).Code
	}
	if c := sw("team-a", "A111111", `{"scope":"global","on":true,"reason":"x"}`); c != 403 {
		t.Errorf("global by non-operator: %d", c)
	}
	if c := sw("team-a", "A111111", `{"scope":"team","on":true}`); c != 400 {
		t.Errorf("no reason: %d", c)
	}
	if c := sw("team-a", "A111111", `{"scope":"team","on":true,"reason":"incident"}`); c != 204 {
		t.Fatalf("team switch: %d", c)
	}
	if c, cmd := e.heartbeat(l.LeaseID); c != 200 || cmd != cmdKill {
		t.Errorf("team switch delivery: %d %s", c, cmd)
	}
	if c, _ := e.claim(tok(2)); c != 204 {
		t.Errorf("claim while team switch on: %d", c)
	}
	if c := sw("team-a", "A111111", `{"scope":"team","on":false}`); c != 204 {
		t.Fatalf("team switch off: %d", c)
	}
	c, l2 := e.claim(tok(3))
	if c != 200 {
		t.Fatalf("claim after switch off: %d", c)
	}
	if c := sw("team-b", "B111111", `{"scope":"global","on":true,"reason":"incident"}`); c != 204 {
		t.Fatalf("global switch: %d", c)
	}
	if c, cmd := e.heartbeat(l2.LeaseID); c != 200 || cmd != cmdKill {
		t.Errorf("global switch delivery: %d %s", c, cmd)
	}
	var n int
	_ = e.db.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'kill_switch'`).Scan(&n)
	if n != 3 {
		t.Errorf("kill switch audit rows: %d", n)
	}
	if st, _ := e.state(id); st != "killed" {
		t.Errorf("state %s", st)
	}
}

// stubRuntime is the runtime side of the contract, as fleet-runtime must
// implement it. The job's context ends on cancel, kill or 410, and on a local
// deadline that a timer enforces even while a heartbeat hangs. The deadline is
// the monotonic send time of the acked request plus lease_ttl_seconds; each
// heartbeat is bounded by the time left, and a late ack is ignored. sent is
// when the claim request was sent.
func stubRuntime(parent context.Context, sent time.Time, ttl, every time.Duration, heartbeat func(context.Context) (HeartbeatResponse, error)) context.Context {
	job, stop := context.WithCancel(parent)
	deadline := sent.Add(ttl)
	timer := time.AfterFunc(time.Until(deadline), stop) // independent of the heartbeat loop
	go func() {
		defer stop()
		defer timer.Stop()
		for {
			select {
			case <-job.Done():
				return
			case <-time.After(every):
			}
			sentAt := time.Now()
			hctx, cancel := context.WithDeadline(job, deadline)
			hb, err := heartbeat(hctx)
			cancel()
			if err != nil || !time.Now().Before(deadline) {
				continue // no ack, or a late one: the deadline stays
			}
			if hb.Command != cmdContinue {
				return
			}
			deadline = sentAt.Add(time.Duration(hb.LeaseTTLSeconds) * time.Second)
			timer.Reset(time.Until(deadline))
		}
	}()
	return job
}

func TestStubRuntimeStopsWhileHeartbeatHangs(t *testing.T) {
	start := time.Now()
	var bounded atomic.Bool
	job := stubRuntime(context.Background(), start, 300*time.Millisecond, 20*time.Millisecond, func(ctx context.Context) (HeartbeatResponse, error) {
		_, ok := ctx.Deadline()
		bounded.Store(ok)
		time.Sleep(2 * time.Second) // hangs, ignoring ctx
		return HeartbeatResponse{Command: cmdContinue, LeaseTTLSeconds: 60}, nil
	})
	select {
	case <-job.Done():
		if d := time.Since(start); d > time.Second {
			t.Errorf("stopped late: %v", d)
		}
		if !bounded.Load() {
			t.Error("heartbeat request not bounded by the lease")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("job kept running while the heartbeat hung")
	}
}

func TestStubRuntimeAnchorsToSendTime(t *testing.T) {
	// The claim was sent 900 ms before the answer arrived: 1 s TTL leaves 100 ms.
	start := time.Now()
	job := stubRuntime(context.Background(), start.Add(-900*time.Millisecond), time.Second, time.Hour, nil)
	select {
	case <-job.Done():
		if d := time.Since(start); d > 500*time.Millisecond {
			t.Errorf("deadline counted from receipt: %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job outlived its lease")
	}
}

func TestStubRuntimeIgnoresLateAck(t *testing.T) {
	start := time.Now()
	job := stubRuntime(context.Background(), start, 200*time.Millisecond, 10*time.Millisecond, func(context.Context) (HeartbeatResponse, error) {
		time.Sleep(300 * time.Millisecond) // the ack lands after the deadline
		return HeartbeatResponse{Command: cmdContinue, LeaseTTLSeconds: 60}, nil
	})
	time.Sleep(600 * time.Millisecond)
	if job.Err() == nil {
		t.Error("a late ack revived the job")
	}
}

func TestStubRuntimeStopsWithoutHeartbeatAck(t *testing.T) {
	start := time.Now()
	job := stubRuntime(context.Background(), start, 300*time.Millisecond, 50*time.Millisecond, func(context.Context) (HeartbeatResponse, error) {
		return HeartbeatResponse{}, io.ErrUnexpectedEOF // fleet-control unreachable
	})
	select {
	case <-job.Done():
		if d := time.Since(start); d > time.Second {
			t.Errorf("stopped late: %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job kept running without a heartbeat ack")
	}
}

func TestStubRuntimeStopsOnKill(t *testing.T) {
	job := stubRuntime(context.Background(), time.Now(), time.Minute, 10*time.Millisecond, func(context.Context) (HeartbeatResponse, error) {
		return HeartbeatResponse{Command: cmdKill}, nil
	})
	select {
	case <-job.Done():
	case <-time.After(time.Second):
		t.Fatal("job kept running after kill")
	}
}

func TestRealPostgresSweepWorker(t *testing.T) {
	e := newJobEnv(t)
	var ids []int64
	for i := range 10 {
		ids = append(ids, e.createJob(`{"kind":"review"}`))
		e.claim(tok(i))
	}
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE runtime_leases SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	// Two replicas sweep at once: every lease expires exactly once.
	var wg sync.WaitGroup
	counts := make(chan int, 2)
	for range 2 {
		wg.Go(func() {
			n, err := e.db.SweepLeases(context.Background())
			if err != nil {
				t.Error(err)
			}
			counts <- n
		})
	}
	wg.Wait()
	close(counts)
	total := 0
	for n := range counts {
		total += n
	}
	if total != 10 {
		t.Errorf("expired %d leases, want 10", total)
	}
	for _, id := range ids {
		if st, _ := e.state(id); st != "queued" {
			t.Errorf("job %d: %s", id, st)
		}
		if got := fmt.Sprint(e.audit(id)); got != "[job_created job_claimed lease_expired]" {
			t.Errorf("job %d audit: %s", id, got)
		}
	}
	// The loop sweeps without any claim.
	e.claim(tok(100))
	_, _ = e.db.pool.Exec(context.Background(), `UPDATE runtime_leases SET expires_at = now() - interval '1 second' WHERE ended_at IS NULL`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sweepLoop(ctx, e.db, 20*time.Millisecond)
	for deadline := time.Now().Add(2 * time.Second); ; {
		var active int
		_ = e.db.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime_leases WHERE ended_at IS NULL`).Scan(&active)
		if active == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sweep loop did not expire the lease")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSweepIntervalFromEnv(t *testing.T) {
	if d, err := sweepIntervalFromEnv(); err != nil || d != 15*time.Second {
		t.Errorf("default: %v %v", d, err)
	}
	for _, v := range []string{"500ms", "1h", "x"} {
		t.Setenv("FLEET_RUNTIME_SWEEP_INTERVAL", v)
		if _, err := sweepIntervalFromEnv(); err == nil {
			t.Errorf("%q: want error", v)
		}
	}
}

func (e *jobEnv) heartbeatResp(lease string) (int, HeartbeatResponse) {
	e.t.Helper()
	rec := do(e.t, e.h, http.MethodPost, "/runtime/v1/heartbeat", runtimeID, `{"lease_id":"`+lease+`"}`)
	var out HeartbeatResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// The runtime counts lease_ttl_seconds from receipt, so every lease answer
// carries it, and a repeated claim gets what is left, not a fresh TTL.
func TestRealPostgresLeaseTTLSeconds(t *testing.T) {
	e := newJobEnv(t)
	e.createJob(`{"kind":"review"}`)
	_, l := e.claim(tok(1))
	if l.LeaseTTLSeconds < 59 || l.LeaseTTLSeconds > 60 {
		t.Errorf("claim ttl: %d", l.LeaseTTLSeconds)
	}
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE runtime_leases SET expires_at = now() + interval '10.5 seconds'`); err != nil {
		t.Fatal(err)
	}
	if _, again := e.claim(tok(1)); again.LeaseTTLSeconds != 10 {
		t.Errorf("repeated claim ttl: %d, want 10", again.LeaseTTLSeconds)
	}
	if c, hb := e.heartbeatResp(l.LeaseID); c != 200 || hb.LeaseTTLSeconds < 59 || hb.LeaseTTLSeconds > 60 {
		t.Errorf("heartbeat ttl: %d %d", c, hb.LeaseTTLSeconds)
	}
}

// After cancel is delivered, heartbeats repeat it but never extend the lease
// past one TTL from the first delivery; then the sweeper cancels the job.
func TestRealPostgresCancelDoesNotExtendLease(t *testing.T) {
	e := newJobEnv(t)
	id := e.createJob(`{"kind":"review"}`)
	_, l := e.claim(tok(1))
	if rec := do(t, e.h, http.MethodPost, fmt.Sprintf("/admin/v1/jobs/team-a/%d/cancel", id), userTok("A111111"), ""); rec.Code != 204 {
		t.Fatalf("cancel: %d", rec.Code)
	}
	if c, hb := e.heartbeatResp(l.LeaseID); c != 200 || hb.Command != cmdCancel || hb.LeaseTTLSeconds < 59 {
		t.Fatalf("first delivery: %d %+v", c, hb)
	}
	// 55 s later the runtime still heartbeats: it gets cancel and what is left.
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE runtime_leases SET cancel_delivered_at = cancel_delivered_at - interval '55 seconds', expires_at = expires_at - interval '55 seconds'`); err != nil {
		t.Fatal(err)
	}
	c, hb := e.heartbeatResp(l.LeaseID)
	if c != 200 || hb.Command != cmdCancel || hb.LeaseTTLSeconds > 5 {
		t.Fatalf("repeat delivery extended the lease: %d %+v", c, hb)
	}
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE runtime_leases SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if c, _ := e.heartbeatResp(l.LeaseID); c != http.StatusGone {
		t.Errorf("after grace: %d", c)
	}
	if _, err := e.db.SweepLeases(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.state(id); st != "cancelled" {
		t.Errorf("state %s", st)
	}
	if got := fmt.Sprint(e.audit(id)); got != "[job_created job_claimed job_cancel cancel_delivered lease_expired]" {
		t.Errorf("audit: %s", got)
	}
}

func TestRealPostgresRepeatedClaimOnExpiredLease(t *testing.T) {
	e := newJobEnv(t)
	e.createJob(`{"kind":"review"}`)
	e.claim(tok(1))
	// Expired by DB time but not yet swept. Claim sweeps first, so this
	// checks the lookup it falls back to on its own.
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE runtime_leases SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	_, err := e.db.leaseByToken(context.Background(), runtimeID, tok(1))
	if apiErr, ok := errors.AsType[*apiError](err); !ok || apiErr.code != http.StatusGone {
		t.Errorf("repeated claim on an expired, unswept lease: %v, want 410", err)
	}
}

func TestRealPostgresKillWinsOverFinish(t *testing.T) {
	for name, path := range map[string]string{"complete": "/runtime/v1/complete", "fail": "/runtime/v1/fail"} {
		t.Run(name, func(t *testing.T) {
			e := newJobEnv(t)
			id := e.createJob(`{"kind":"review"}`)
			_, l := e.claim(tok(1))
			if rec := do(t, e.h, http.MethodPost, fmt.Sprintf("/admin/v1/jobs/team-a/%d/kill", id), userTok("A111111"), ""); rec.Code != 204 {
				t.Fatalf("kill: %d", rec.Code)
			}
			if rec := do(t, e.h, http.MethodPost, path, runtimeID, `{"lease_id":"`+l.LeaseID+`","code":"ok"}`); rec.Code != 204 {
				t.Fatalf("finish: %d %s", rec.Code, rec.Body)
			}
			if st, _ := e.state(id); st != "killed" {
				t.Errorf("state %s, want killed", st)
			}
		})
	}
	e := newJobEnv(t)
	id := e.createJob(`{"kind":"review"}`)
	_, l := e.claim(tok(1))
	if c := do(t, e.h, http.MethodPut, "/admin/v1/jobs/team-a/kill-switch", userTok("A111111"), `{"scope":"team","on":true,"reason":"x"}`).Code; c != 204 {
		t.Fatalf("switch: %d", c)
	}
	_ = do(t, e.h, http.MethodPost, "/runtime/v1/complete", runtimeID, `{"lease_id":"`+l.LeaseID+`","code":"ok"}`)
	if st, _ := e.state(id); st != "killed" {
		t.Errorf("kill switch: state %s, want killed", st)
	}
}

// A lease that expires while a call waits for its row lock is gone: expiry is
// checked after the lock, not at transaction start.
func TestRealPostgresLeaseExpiresWhileWaitingForLock(t *testing.T) {
	for name, path := range map[string]string{"heartbeat": "/runtime/v1/heartbeat", "complete": "/runtime/v1/complete"} {
		t.Run(name, func(t *testing.T) {
			extra := map[string]string{"heartbeat": "", "complete": `,"code":"ok"`}[name]
			e := newJobEnv(t)
			e.createJob(`{"kind":"review"}`)
			_, l := e.claim(tok(1))
			ctx := context.Background()
			if _, err := e.db.pool.Exec(ctx, `UPDATE runtime_leases SET expires_at = clock_timestamp() + interval '300 milliseconds' WHERE id = $1::uuid`, l.LeaseID); err != nil {
				t.Fatal(err)
			}
			hold, err := e.db.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := hold.Exec(ctx, `SELECT 1 FROM runtime_leases WHERE id = $1::uuid FOR UPDATE`, l.LeaseID); err != nil {
				t.Fatal(err)
			}
			code := make(chan int, 1)
			go func() {
				code <- do(t, e.h, http.MethodPost, path, runtimeID, `{"lease_id":"`+l.LeaseID+`"`+extra+`}`).Code
			}()
			time.Sleep(600 * time.Millisecond) // past expiry, the call waits on the lock
			if err := hold.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if c := <-code; c != http.StatusGone {
				t.Errorf("got %d, want 410", c)
			}
		})
	}
}
