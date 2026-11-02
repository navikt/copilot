package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestDB starts a throwaway Postgres cluster if TEST_DATABASE_URL is unset,
// or connects to TEST_DATABASE_URL. Skips the test if unreachable or initdb is missing/fails.
func setupTestDB(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()

	if testURL := os.Getenv("TEST_DATABASE_URL"); testURL != "" {
		s, connErr := NewStore(ctx, testURL)
		if connErr != nil {
			t.Skipf("skipping test: TEST_DATABASE_URL unreachable: %v", connErr)
			return nil
		}
		t.Cleanup(func() { s.Close() })

		if migErr := s.Migrate(ctx); migErr != nil {
			t.Skipf("skipping test: running migrations on TEST_DATABASE_URL failed: %v", migErr)
			return nil
		}

		// Truncate tables for clean test state
		_, truncErr := s.pool.Exec(ctx, "TRUNCATE TABLE registry_entries, registry_entry_versions, registry_audit, schema_migrations CASCADE")
		if truncErr != nil {
			t.Skipf("skipping test: truncating test tables: %v", truncErr)
			return nil
		}

		if reMigErr := s.Migrate(ctx); reMigErr != nil {
			t.Skipf("skipping test: re-running migrations after truncate: %v", reMigErr)
			return nil
		}

		return s
	}

	initdbPath := "/opt/homebrew/bin/initdb"
	if _, err := os.Stat(initdbPath); err != nil {
		if path, lookErr := exec.LookPath("initdb"); lookErr == nil {
			initdbPath = path
		} else {
			t.Skip("skipping test: initdb not found in /opt/homebrew/bin or PATH")
			return nil
		}
	}

	pgCtlPath := "/opt/homebrew/bin/pg_ctl"
	if _, err := os.Stat(pgCtlPath); err != nil {
		if path, lookErr := exec.LookPath("pg_ctl"); lookErr == nil {
			pgCtlPath = path
		} else {
			t.Skip("skipping test: pg_ctl not found in /opt/homebrew/bin or PATH")
			return nil
		}
	}

	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	// Initialize throwaway cluster
	cmd := exec.Command(initdbPath, "-D", dataDir, "-U", "postgres", "-A", "trust", "--no-sync") //nolint:gosec // test helper path
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping test: initdb failed to initialize: %v (output: %s)", err, string(out))
		return nil
	}

	// Find random open port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	logFile := filepath.Join(tempDir, "postgres.log")
	startCmd := exec.Command(pgCtlPath, "-D", dataDir, "-o", fmt.Sprintf("-k %s -p %d -h 127.0.0.1", tempDir, port), "-l", logFile, "start", "-w") //nolint:gosec // test helper path
	if out, startErr := startCmd.CombinedOutput(); startErr != nil {
		t.Skipf("skipping test: pg_ctl failed to start: %v (output: %s)", startErr, string(out))
		return nil
	}

	t.Cleanup(func() {
		_ = exec.Command(pgCtlPath, "-D", dataDir, "-m", "immediate", "stop").Run() //nolint:gosec // test cleanup path
	})

	dbURL := fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable", port)
	s, err := NewStore(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to throwaway postgres: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	return s
}

func testMux(cfg *Config, store *Store, introspect IntrospectFunc) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler(store))
	mux.Handle("/metrics", metricsHandler())
	mux.HandleFunc("POST /api/v1/entries", makeSubmitHandler(cfg, store, introspect))
	mux.HandleFunc("POST /api/v1/entries/{name...}", makeEntryActionHandler(cfg, store, introspect))
	return mux
}

func testFakeIntrospect() IntrospectFunc {
	return func(_ context.Context, token string) (*IntrospectionResult, error) {
		if strings.HasPrefix(token, "token-") {
			user := strings.TrimPrefix(token, "token-")
			return &IntrospectionResult{
				Active:   true,
				Username: user,
				NAVident: "A123456",
			}, nil
		}
		return &IntrospectionResult{Active: false}, nil
	}
}

func sampleServerEntryJSON(name, desc string) string {
	return fmt.Sprintf(`{
		"name": %q,
		"description": %q,
		"version": "1.0.0",
		"owner": "team-registry",
		"remotes": [
			{"type": "streamable-http", "url": "https://mcp.example.com/api"}
		]
	}`, name, desc)
}

func TestStore_RealPostgres_LifecycleAndRequirements(t *testing.T) {
	store := setupTestDB(t)
	if store == nil {
		return
	}

	cfg := &Config{
		Port:       "8080",
		AdminUsers: map[string]bool{"admin@nav.no": true},
	}
	introspect := testFakeIntrospect()
	mux := testMux(cfg, store, introspect)

	const serverName = "io.github.navikt/test-service"
	actor := "developer@nav.no"
	adminActor := "admin@nav.no"

	// 1. Submit creates pending + audit row with actor and time
	t.Run("submit creates pending + audit row with actor and time", func(t *testing.T) {
		body := fmt.Sprintf(`{"entry": %s}`, sampleServerEntryJSON(serverName, "Initial version"))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer token-"+actor)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal submit response: %v", err)
		}
		if resp["status"] != "pending" || resp["version"] != float64(1) {
			t.Fatalf("unexpected submit response: %+v", resp)
		}

		// Verify row in registry_entries
		var status string
		var version int
		var pendingVersion *int
		err := store.pool.QueryRow(t.Context(), `
			SELECT status, version, pending_version
			FROM registry_entries
			WHERE name = $1
		`, serverName).Scan(&status, &version, &pendingVersion)
		if err != nil {
			t.Fatalf("querying registry_entries: %v", err)
		}
		if status != "pending" || version != 1 || pendingVersion != nil {
			t.Fatalf("registry_entries = (status:%s, ver:%d, pending:%v), want (pending, 1, nil)", status, version, pendingVersion)
		}

		// Verify audit row
		var auditActor, auditAction, auditName string
		var auditVer int
		var auditAt time.Time
		err = store.pool.QueryRow(t.Context(), `
			SELECT actor, action, name, version, at
			FROM registry_audit
			WHERE name = $1 AND action = 'submit'
			ORDER BY id DESC LIMIT 1
		`, serverName).Scan(&auditActor, &auditAction, &auditName, &auditVer, &auditAt)
		if err != nil {
			t.Fatalf("querying registry_audit: %v", err)
		}
		if auditActor != actor || auditAction != "submit" || auditName != serverName || auditVer != 1 {
			t.Fatalf("audit row mismatch: actor=%s action=%s name=%s ver=%d", auditActor, auditAction, auditName, auditVer)
		}
		if time.Since(auditAt) > 1*time.Minute {
			t.Fatalf("audit timestamp %v is too old", auditAt)
		}
	})

	// 2. Approve makes it visible via the reader query
	t.Run("approve makes it visible via the reader query", func(t *testing.T) {
		// Before approval: reader query returns nothing for this entry
		entriesBefore, err := store.GetApprovedEntries(t.Context())
		if err != nil {
			t.Fatalf("GetApprovedEntries before approve: %v", err)
		}
		for _, e := range entriesBefore {
			if e.Name == serverName {
				t.Fatalf("entry %s should not be visible before approval", serverName)
			}
		}

		// Approve non-pending version (e.g. version 2) returns 409
		reqBadVer := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/approve", strings.NewReader(`{"version": 2}`))
		reqBadVer.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqBadVer.Header.Set("Content-Type", "application/json")
		recBadVer := httptest.NewRecorder()
		mux.ServeHTTP(recBadVer, reqBadVer)
		if recBadVer.Code != http.StatusConflict {
			t.Fatalf("expected status 409 when approving non-pending version 2, got %d", recBadVer.Code)
		}

		// Approve version 1 as admin
		approveBody := `{"version": 1}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/approve", strings.NewReader(approveBody))
		req.Header.Set("Authorization", "Bearer token-"+adminActor)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 on approve, got %d: %s", rec.Code, rec.Body.String())
		}

		// Re-approving version 1 (which is now approved, not pending) returns 409
		recReApprove := httptest.NewRecorder()
		reqReApprove := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/approve", strings.NewReader(approveBody))
		reqReApprove.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqReApprove.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(recReApprove, reqReApprove)
		if recReApprove.Code != http.StatusConflict {
			t.Fatalf("expected status 409 on re-approving already approved version, got %d", recReApprove.Code)
		}

		// Reader query now sees the entry
		entriesAfter, err := store.GetApprovedEntries(t.Context())
		if err != nil {
			t.Fatalf("GetApprovedEntries after approve: %v", err)
		}
		var found bool
		for _, e := range entriesAfter {
			if e.Name == serverName {
				found = true
				if !strings.Contains(string(e.Entry), "Initial version") {
					t.Fatalf("reader saw unexpected entry content: %s", string(e.Entry))
				}
			}
		}
		if !found {
			t.Fatalf("entry %s not found in reader query after approval", serverName)
		}
	})

	// 3. Resubmit of an approved name keeps the approved entry served
	t.Run("resubmit of an approved name keeps the approved entry served", func(t *testing.T) {
		v2Body := fmt.Sprintf(`{"entry": %s}`, sampleServerEntryJSON(serverName, "Second version with changes"))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(v2Body))
		req.Header.Set("Authorization", "Bearer token-"+actor)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201 on resubmit, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["version"] != float64(2) {
			t.Fatalf("expected version 2 on resubmit, got %v", resp["version"])
		}

		// Verify registry_entries state: status is approved, version is still 1, pending_version is 2
		var status string
		var version int
		var pendingVer *int
		err := store.pool.QueryRow(t.Context(), `
			SELECT status, version, pending_version
			FROM registry_entries
			WHERE name = $1
		`, serverName).Scan(&status, &version, &pendingVer)
		if err != nil {
			t.Fatalf("querying registry_entries: %v", err)
		}
		if status != "approved" || version != 1 || pendingVer == nil || *pendingVer != 2 {
			t.Fatalf("registry_entries = (%s, ver %d, pending %v), want (approved, 1, 2)", status, version, pendingVer)
		}

		// Reader query MUST still return version 1
		entries, err := store.GetApprovedEntries(t.Context())
		if err != nil {
			t.Fatalf("GetApprovedEntries during pending resubmit: %v", err)
		}
		var foundApprovedV1 bool
		for _, e := range entries {
			if e.Name == serverName {
				foundApprovedV1 = true
				if !strings.Contains(string(e.Entry), "Initial version") {
					t.Fatalf("reader served pending version instead of approved version: %s", string(e.Entry))
				}
			}
		}
		if !foundApprovedV1 {
			t.Fatalf("approved entry was unexpectedly removed from reader query")
		}
	})

	// 4. Reject needs reason and only pending version can be rejected
	t.Run("reject needs reason and only pending version can be rejected", func(t *testing.T) {
		// Reject without reason
		reqNoReason := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/reject", strings.NewReader(`{"version": 2}`))
		reqNoReason.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqNoReason.Header.Set("Content-Type", "application/json")
		recNoReason := httptest.NewRecorder()
		mux.ServeHTTP(recNoReason, reqNoReason)

		if recNoReason.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when reason is missing, got %d: %s", recNoReason.Code, recNoReason.Body.String())
		}

		// Reject with empty whitespace reason
		reqEmptyReason := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/reject", strings.NewReader(`{"version": 2, "reason": "   "}`))
		reqEmptyReason.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqEmptyReason.Header.Set("Content-Type", "application/json")
		recEmptyReason := httptest.NewRecorder()
		mux.ServeHTTP(recEmptyReason, reqEmptyReason)

		if recEmptyReason.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when reason is whitespace, got %d: %s", recEmptyReason.Code, recEmptyReason.Body.String())
		}

		// Reject already approved version 1: must return 409 with message that revocation comes later
		reqRejectApproved := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/reject", strings.NewReader(`{"version": 1, "reason": "revocation"}`))
		reqRejectApproved.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqRejectApproved.Header.Set("Content-Type", "application/json")
		recRejectApproved := httptest.NewRecorder()
		mux.ServeHTTP(recRejectApproved, reqRejectApproved)

		if recRejectApproved.Code != http.StatusConflict {
			t.Fatalf("expected 409 when rejecting already approved version, got %d: %s", recRejectApproved.Code, recRejectApproved.Body.String())
		}
		if !strings.Contains(recRejectApproved.Body.String(), "revocation comes later") {
			t.Fatalf("expected 409 body to mention revocation comes later, got: %s", recRejectApproved.Body.String())
		}

		// Reject non-pending version (e.g. version 99)
		reqRejectNonPending := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/reject", strings.NewReader(`{"version": 99, "reason": "invalid"}`))
		reqRejectNonPending.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqRejectNonPending.Header.Set("Content-Type", "application/json")
		recRejectNonPending := httptest.NewRecorder()
		mux.ServeHTTP(recRejectNonPending, reqRejectNonPending)

		if recRejectNonPending.Code != http.StatusConflict {
			t.Fatalf("expected 409 when rejecting non-pending version, got %d", recRejectNonPending.Code)
		}

		// Reject with valid reason for pending version 2
		validReject := `{"version": 2, "reason": "Requires documentation update before approval"}`
		reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/reject", strings.NewReader(validReject))
		reqValid.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqValid.Header.Set("Content-Type", "application/json")
		recValid := httptest.NewRecorder()
		mux.ServeHTTP(recValid, reqValid)

		if recValid.Code != http.StatusOK {
			t.Fatalf("expected 200 on valid reject, got %d: %s", recValid.Code, recValid.Body.String())
		}

		// Verify audit row for reject contains reason
		var auditReason string
		err := store.pool.QueryRow(t.Context(), `
			SELECT reason
			FROM registry_audit
			WHERE name = $1 AND action = 'reject' AND version = 2
			ORDER BY id DESC LIMIT 1
		`, serverName).Scan(&auditReason)
		if err != nil {
			t.Fatalf("querying reject audit row: %v", err)
		}
		if auditReason != "Requires documentation update before approval" {
			t.Fatalf("audit reason = %q, want 'Requires documentation update before approval'", auditReason)
		}

		// Verify pending_version was cleared in registry_entries
		var pendingVer *int
		err = store.pool.QueryRow(t.Context(), `
			SELECT pending_version FROM registry_entries WHERE name = $1
		`, serverName).Scan(&pendingVer)
		if err != nil {
			t.Fatalf("querying pending_version: %v", err)
		}
		if pendingVer != nil {
			t.Fatalf("expected pending_version to be nil after rejection, got %v", *pendingVer)
		}
	})

	// 5. Ownership: submit for existing name allowed only from original submitter or admin
	t.Run("ownership: submit for existing name allowed only from original submitter or admin", func(t *testing.T) {
		const ownedName = "io.github.navikt/ownership-service"
		originalOwner := "owner-user@nav.no"
		otherUser := "other-user@nav.no"

		// Original user submits version 1 -> 201
		bodyV1 := fmt.Sprintf(`{"entry": %s}`, sampleServerEntryJSON(ownedName, "v1 by owner"))
		reqV1 := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(bodyV1))
		reqV1.Header.Set("Authorization", "Bearer token-"+originalOwner)
		reqV1.Header.Set("Content-Type", "application/json")
		recV1 := httptest.NewRecorder()
		mux.ServeHTTP(recV1, reqV1)
		if recV1.Code != http.StatusCreated {
			t.Fatalf("expected 201 for original submitter v1, got %d: %s", recV1.Code, recV1.Body.String())
		}

		// Another user submits v2 -> 403 Forbidden
		bodyV2 := fmt.Sprintf(`{"entry": %s}`, sampleServerEntryJSON(ownedName, "v2 by other user"))
		reqOther := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(bodyV2))
		reqOther.Header.Set("Authorization", "Bearer token-"+otherUser)
		reqOther.Header.Set("Content-Type", "application/json")
		recOther := httptest.NewRecorder()
		mux.ServeHTTP(recOther, reqOther)
		if recOther.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for unauthorized resubmit by other user, got %d: %s", recOther.Code, recOther.Body.String())
		}

		// Original user submits v2 -> 201 Created
		reqOwnerV2 := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(bodyV2))
		reqOwnerV2.Header.Set("Authorization", "Bearer token-"+originalOwner)
		reqOwnerV2.Header.Set("Content-Type", "application/json")
		recOwnerV2 := httptest.NewRecorder()
		mux.ServeHTTP(recOwnerV2, reqOwnerV2)
		if recOwnerV2.Code != http.StatusCreated {
			t.Fatalf("expected 201 for original submitter v2, got %d: %s", recOwnerV2.Code, recOwnerV2.Body.String())
		}

		// Admin submits v3 -> 201 Created
		bodyV3 := fmt.Sprintf(`{"entry": %s}`, sampleServerEntryJSON(ownedName, "v3 by admin"))
		reqAdmin := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(bodyV3))
		reqAdmin.Header.Set("Authorization", "Bearer token-"+adminActor)
		reqAdmin.Header.Set("Content-Type", "application/json")
		recAdmin := httptest.NewRecorder()
		mux.ServeHTTP(recAdmin, reqAdmin)
		if recAdmin.Code != http.StatusCreated {
			t.Fatalf("expected 201 for admin resubmit, got %d: %s", recAdmin.Code, recAdmin.Body.String())
		}
	})

	// 6. Non-admin approve is 403
	t.Run("non-admin approve is 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/approve", strings.NewReader(`{"version": 1}`))
		req.Header.Set("Authorization", "Bearer token-"+actor) // developer@nav.no is not in ADMIN_USERS
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for non-admin approve, got %d: %s", rec.Code, rec.Body.String())
		}

		// Also check reject by non-admin is 403
		rejReq := httptest.NewRequest(http.MethodPost, "/api/v1/entries/"+serverName+"/reject", strings.NewReader(`{"version": 1, "reason": "fail"}`))
		rejReq.Header.Set("Authorization", "Bearer token-"+actor)
		rejReq.Header.Set("Content-Type", "application/json")
		rejRec := httptest.NewRecorder()

		mux.ServeHTTP(rejRec, rejReq)

		if rejRec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for non-admin reject, got %d: %s", rejRec.Code, rejRec.Body.String())
		}
	})

	// 7. 11th submit within an hour is 429
	t.Run("11th submit within an hour is 429", func(t *testing.T) {
		rateLimitActor := "spammer@nav.no"

		for i := 1; i <= 10; i++ {
			entryName := fmt.Sprintf("io.github.navikt/rate-limit-test-%d", i)
			body := fmt.Sprintf(`{"entry": %s}`, sampleServerEntryJSON(entryName, "Burst test"))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer token-"+rateLimitActor)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("submit #%d expected 201, got %d: %s", i, rec.Code, rec.Body.String())
			}
		}

		// 11th submit must be 429
		entryName11 := "io.github.navikt/rate-limit-test-11"
		body11 := fmt.Sprintf(`{"entry": %s}`, sampleServerEntryJSON(entryName11, "11th submit"))
		req11 := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(body11))
		req11.Header.Set("Authorization", "Bearer token-"+rateLimitActor)
		req11.Header.Set("Content-Type", "application/json")
		rec11 := httptest.NewRecorder()

		mux.ServeHTTP(rec11, req11)

		if rec11.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 11th submit to return 429, got %d: %s", rec11.Code, rec11.Body.String())
		}
	})

	// 8. Internal host rejected with 400
	t.Run("internal host rejected with 400", func(t *testing.T) {
		badHosts := []string{
			"https://my-service.intern.nav.no/api",
			"https://my-service.intern.dev.nav.no/api",
			"https://my-service.svc.cluster.local/api",
			"https://my-service.adeo.no/api",
			"https://localhost:8080/api",
			"https://127.0.0.1:8080/api",
			"https://[::1]:8080/api",
			"http://external.example.com/api", // non-https
		}

		for _, badURL := range badHosts {
			entryJSON := fmt.Sprintf(`{
				"name": "io.github.navikt/bad-host-test",
				"description": "Test entry",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": %q}]
			}`, badURL)
			body := fmt.Sprintf(`{"entry": %s}`, entryJSON)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer token-"+actor)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for bad URL %q, got %d: %s", badURL, rec.Code, rec.Body.String())
			}
		}
	})
}
