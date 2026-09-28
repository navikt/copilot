package local

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// fakeMLXServer stands in for mlx_lm.server: a ThreadingHTTPServer whose POST
// sleeps for the milliseconds in its body and counts how many ran at once. GET
// /peak reports that count.
const fakeMLXServer = `import sys, threading, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
_mu = threading.Lock()
_now = [0]
_peak = [0]
class APIHandler(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass
    def _reply(self, body):
        body = body.encode()
        self.send_response(200)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def do_POST(self):
        ms = int(self.rfile.read(int(self.headers["Content-Length"])) or 0)
        with _mu:
            _now[0] += 1
            _peak[0] = max(_peak[0], _now[0])
        time.sleep(ms / 1000)
        with _mu:
            _now[0] -= 1
        self._reply("{}")
    def do_GET(self):
        self._reply(str(_peak[0]))
def main():
    port = int(sys.argv[sys.argv.index("--port") + 1])
    ThreadingHTTPServer(("127.0.0.1", port), APIHandler).serve_forever()
`

// startFakeMLX runs the fake under the system python, through the real
// bootstrap when queued is true and straight to its main otherwise, and returns
// its base URL once it answers.
func startFakeMLX(t testing.TB, queued bool) string {
	t.Helper()
	py := testhome.Python3(t)
	pkg := filepath.Join(t.TempDir(), "mlx_lm")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"__init__.py": "", "server.py": fakeMLXServer} {
		if err := os.WriteFile(filepath.Join(pkg, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	script := "from mlx_lm.server import main; main()"
	if queued {
		script = serverScript()
	}
	cmd := exec.Command(py, "-c", script, "--port", strconv.Itoa(port))
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Dir(pkg))
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if resp, err := http.Get(base + "/peak"); err == nil {
			resp.Body.Close()
			return base
		}
	}
	t.Fatal("the fake server never answered")
	return ""
}

// post sends a POST that holds the fake for ms and returns the status and body.
func post(t testing.TB, base string, ms int) (int, string, http.Header) {
	resp, err := http.Post(base+"/v1/chat/completions", "application/json", strings.NewReader(strconv.Itoa(ms)))
	if err != nil {
		t.Error(err)
		return 0, "", nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

func peak(t *testing.T, base string) string {
	resp, err := http.Get(base + "/peak")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func setQueue(t *testing.T, depth int, wait time.Duration) {
	oldDepth, oldWait := serverQueueDepth, serverQueueWait
	serverQueueDepth, serverQueueWait = depth, wait
	t.Cleanup(func() { serverQueueDepth, serverQueueWait = oldDepth, oldWait })
}

// TestServerQueueServesOneAtATime: ten clients at once, and the server sees
// one POST at a time, every one answered, while a GET is not held up.
func TestServerQueueServesOneAtATime(t *testing.T) {
	setQueue(t, 16, time.Minute)
	base := startFakeMLX(t, true)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, body, _ := post(t, base, 50); code != http.StatusOK {
				t.Errorf("POST = %d %s, want 200", code, body)
			}
		}()
	}
	time.Sleep(100 * time.Millisecond)
	started := time.Now()
	_ = peak(t, base)
	if d := time.Since(started); d > 200*time.Millisecond {
		t.Errorf("a GET took %v behind the queued POSTs; health checks must not queue", d)
	}
	wg.Wait()
	if got := peak(t, base); got != "1" {
		t.Errorf("peak concurrent POSTs = %s, want 1", got)
	}
}

// TestServerQueueWithoutTheBootstrapOverlaps proves the fake would see the
// overlap, so the test above is not passing on a server that serialises alone.
func TestServerQueueWithoutTheBootstrapOverlaps(t *testing.T) {
	base := startFakeMLX(t, false)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); post(t, base, 200) }()
	}
	wg.Wait()
	if got := peak(t, base); got == "1" {
		t.Error("the bare fake served four POSTs one at a time; it cannot show the queue working")
	}
}

// TestServerQueueFullAnswers503: with one running and the queue full, the next
// client is refused at once with a 503 that says why, not left hanging.
func TestServerQueueFullAnswers503(t *testing.T) {
	setQueue(t, 1, time.Minute)
	base := startFakeMLX(t, true)

	var wg sync.WaitGroup
	for range 2 { // one running, one waiting
		wg.Add(1)
		go func() { defer wg.Done(); post(t, base, 1500) }()
	}
	defer wg.Wait()
	time.Sleep(300 * time.Millisecond)

	started := time.Now()
	code, body, hdr := post(t, base, 0)
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "1 requests are already waiting") {
		t.Errorf("POST on a full queue = %d %s, want 503 naming the full queue", code, body)
	}
	if hdr.Get("Retry-After") == "" {
		t.Error("the 503 has no Retry-After")
	}
	if d := time.Since(started); d > 500*time.Millisecond {
		t.Errorf("the refusal took %v; a full queue must answer at once", d)
	}
}

// TestServerQueueWaitIsBounded: a client waiting longer than the bound gets a
// 503 instead of waiting for as long as the request ahead runs.
func TestServerQueueWaitIsBounded(t *testing.T) {
	setQueue(t, 4, 300*time.Millisecond)
	base := startFakeMLX(t, true)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); post(t, base, 2000) }()
	defer wg.Wait()
	time.Sleep(200 * time.Millisecond)

	started := time.Now()
	code, body, _ := post(t, base, 0)
	d := time.Since(started)
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "waited 0.3 s") {
		t.Errorf("POST behind a long one = %d %s, want 503 after the wait bound", code, body)
	}
	if d < 250*time.Millisecond || d > 1500*time.Millisecond {
		t.Errorf("gave up after %v, want about the 300ms bound", d)
	}
}

// BenchmarkServerQueueOverhead is the queue's cost on a decide-shaped request,
// one short POST at a time, against the fake with and without the bootstrap:
//
//	go test -run '^$' -bench ServerQueueOverhead ./internal/local/
func BenchmarkServerQueueOverhead(b *testing.B) {
	for _, queued := range []bool{false, true} {
		b.Run(map[bool]string{false: "direct", true: "queued"}[queued], func(b *testing.B) {
			base := startFakeMLX(b, queued)
			for b.Loop() {
				if code, _, _ := post(b, base, 0); code != http.StatusOK {
					b.Fatalf("POST = %d", code)
				}
			}
		})
	}
}
