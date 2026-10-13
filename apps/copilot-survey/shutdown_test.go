package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestShutdownFlushesQueue runs the built binary against Postgres
// (COPILOT_SURVEY_TEST_DB_URL, as TestSurveyStore) and checks that SIGTERM
// keeps a written answer and exits in time, also while a slow request
// holds server.Shutdown to its deadline. Token introspection is a fake Texas: no seam in the binary.
func TestShutdownFlushesQueue(t *testing.T) {
	dbURL := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if dbURL == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	bin := filepath.Join(t.TempDir(), "copilot-survey")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil { //nolint:gosec // bin is a temp path we chose
		t.Fatalf("build: %v\n%s", err, out)
	}
	texas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"active":true,"azp":"web-id","NAVident":"Z1","preferred_username":"a@nav.no"}`)
	}))
	defer texas.Close()
	pool, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	count := func(table string) (n int) {
		if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	for _, slow := range []bool{false, true} {
		t.Run(fmt.Sprintf("slow=%v", slow), func(t *testing.T) {
			_, _ = pool.Exec(t.Context(), `DROP TABLE IF EXISTS survey_participation, survey_answers`)
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := l.Addr().String()
			_ = l.Close()
			var stdout bytes.Buffer
			cmd := exec.Command(bin)
			cmd.Stdout, cmd.Stderr = &stdout, &stdout
			cmd.Env = append(os.Environ(),
				"PORT="+addr[strings.LastIndex(addr, ":")+1:],
				"DB_URL="+dbURL,
				"NAIS_CLUSTER_NAME=dev-gcp",
				"SURVEY_ACTIVE_IDS=dev-e2e-test",
				"SURVEY_KEY_DEV_E2E_TEST="+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)),
				"NAIS_TOKEN_INTROSPECTION_ENDPOINT="+texas.URL,
				`AZURE_APP_PRE_AUTHORIZED_APPS=[{"name":"dev-gcp:copilot:my-copilot","clientId":"web-id"}]`,
			)
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill() }()
			base := "http://" + addr
			for i := 0; ; i++ {
				if resp, gerr := http.Get(base + "/ready"); gerr == nil {
					_ = resp.Body.Close()
					break
				}
				if i > 100 {
					t.Fatalf("not ready:\n%s", stdout.String())
				}
				time.Sleep(100 * time.Millisecond)
			}

			req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/surveys/dev-e2e-test/responses",
				strings.NewReader(`{"answers":{"fungerer":3},"context":{"version":"web","os":"linux","client":"web"}}`))
			req.Header.Set("Authorization", "Bearer x")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("submit = %d\n%s", resp.StatusCode, stdout.String())
			}
			if a, p := count("survey_answers"), count("survey_participation"); a != 1 || p != 1 {
				t.Fatalf("before SIGTERM: %d answers, %d participation; want 1, 1 (written at once)", a, p)
			}

			if slow {
				// Headers and half a body: the handler blocks in the body
				// read until ReadTimeout (10 s), as long as Shutdown's deadline.
				c, err := net.Dial("tcp", addr)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = c.Close() }()
				_, _ = fmt.Fprint(c, "POST /api/v1/surveys/dev-e2e-test/responses HTTP/1.1\r\nHost: x\r\nAuthorization: Bearer x\r\nContent-Length: 100\r\n\r\n{")
				time.Sleep(50 * time.Millisecond)
			}

			start := time.Now()
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("exit: %v\n%s", err, stdout.String())
				}
			case <-ctx.Done():
				t.Fatal("no exit within 30 s")
			}
			t.Logf("exit %v after SIGTERM\n%s", time.Since(start).Round(10*time.Millisecond), stdout.String())

			if a, p := count("survey_answers"), count("survey_participation"); a != 1 || p != 1 {
				t.Fatalf("after SIGTERM: %d answers, %d participation; want 1, 1\n%s", a, p, stdout.String())
			}
			if strings.Contains(stdout.String(), "lost at shutdown") {
				t.Fatalf("lost line logged:\n%s", stdout.String())
			}
		})
	}
}
