package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// At exit nav-pilot spools its last export and starts a detached
// `nav-pilot __telemetry-send`, which sends the spool and removes it; the
// command itself does not wait for that. Inside cplt's sandbox no child is
// started and the spool waits for the next run.
func TestTelemetryChildSendsSpool(t *testing.T) {
	for _, sandboxed := range []bool{false, true} {
		e := newEnv(t)
		var posts atomic.Int32
		collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			posts.Add(1)
		}))
		t.Cleanup(collector.Close)

		cmd := exec.Command(binary(t), "config", "get", "client")
		cmd.Dir = e.root
		// Any value marks the sandbox, an empty one too, so the variable is
		// left out rather than cleared.
		cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "__CPLT_WRAPPED=") })
		cmd.Env = append(cmd.Env, "NO_COLOR=1",
			"NAV_PILOT_TELEMETRY_ENABLED=true", "DO_NOT_TRACK=",
			"NAV_PILOT_TELEMETRY_ENDPOINT="+collector.URL+"/v1/metrics",
			"NAV_PILOT_E2E_TELEMETRY_CHILD=1",
			"HTTP_PROXY=", "HTTPS_PROXY=", "http_proxy=", "https_proxy=")
		if sandboxed {
			cmd.Env = append(cmd.Env, "__CPLT_WRAPPED=1")
		}
		for k, v := range e.commandEnv() {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("nav-pilot config get client: %v\n%s", err, out)
		}

		spooled := func() int {
			entries, _ := os.ReadDir(filepath.Join(e.home, "telemetry-spool"))
			n := 0
			for _, en := range entries {
				if strings.HasSuffix(en.Name(), ".pb") {
					n++
				}
			}
			return n
		}
		deadline := time.Now().Add(10 * time.Second)
		if sandboxed {
			deadline = time.Now().Add(time.Second)
		}
		for time.Now().Before(deadline) && (posts.Load() == 0 || spooled() != 0) {
			time.Sleep(20 * time.Millisecond)
		}
		if sandboxed {
			if posts.Load() != 0 || spooled() != 1 {
				t.Errorf("inside the sandbox: %d posts, %d spooled; want no child, the export left in the spool", posts.Load(), spooled())
			}
			continue
		}
		if posts.Load() != 1 || spooled() != 0 {
			t.Errorf("child: %d posts, %d spooled; want the export sent once and the spool emptied", posts.Load(), spooled())
		}
	}
}
