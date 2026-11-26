package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// fakeSlack records each posted text and answers with code.
type fakeSlack struct {
	mu    sync.Mutex
	texts []string
	code  int
}

func (f *fakeSlack) start(t *testing.T) (*slackNotifier, *sync.WaitGroup) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m struct{ Text string }
		_ = json.NewDecoder(r.Body).Decode(&m)
		f.mu.Lock()
		f.texts = append(f.texts, m.Text)
		f.mu.Unlock()
		w.WriteHeader(f.code)
	}))
	t.Cleanup(ts.Close)
	n := newSlackNotifier(ts.URL)
	var wg sync.WaitGroup
	n.done = wg.Done
	return n, &wg
}

func TestSlackNotifierUnsetIsNoop(t *testing.T) {
	if n := newSlackNotifier("  "); n != nil {
		t.Fatal("an unset URL must give a nil notifier")
	}
	var n *slackNotifier
	n.notify("x") // must not panic or send
	s := newServer()
	s.notifyServer(t.Context(), nil, "approve", "team-a", "svc", "owner") // nil db is never read
}

func TestServerNoticeEscapes(t *testing.T) {
	got := serverNotice("approve", "team-a", "<!channel>*a_b~`&", 3, "owner")
	for _, bad := range []string{"<", ">", "*", "_", "~", "`"} {
		if strings.Contains(got, bad) {
			t.Fatalf("notice keeps %q: %s", bad, got)
		}
	}
	if !strings.Contains(got, "&lt;!channel&gt;") || !strings.Contains(got, "&amp;") {
		t.Fatalf("notice not escaped: %s", got)
	}
	want := "MCPServer publisert: team-a/"
	if !strings.HasPrefix(got, want) || !strings.Contains(got, "generasjon 3, team team-a (utført av eierteamet)") {
		t.Fatalf("notice = %s", got)
	}
	if got := serverNotice("revoke", "a", "b", 1, "platform"); !strings.Contains(got, "trukket tilbake") || !strings.Contains(got, "plattformteamet") {
		t.Fatalf("revoke notice = %s", got)
	}
}

func TestSlackFailureIsCounted(t *testing.T) {
	f := &fakeSlack{code: 500}
	n, wg := f.start(t)
	before := testutil.ToFloat64(notifyFailures)
	wg.Add(1)
	n.notify("x")
	wg.Wait()
	if got := testutil.ToFloat64(notifyFailures) - before; got != 1 {
		t.Fatalf("failures counted %v, want 1", got)
	}
}

// TestApisNotifiesEachAction needs Postgres: one notice per publish, revoke and
// rollback, and a failing webhook leaves every answer unchanged.
func TestApisNotifiesEachAction(t *testing.T) {
	for _, code := range []int{200, 500} {
		e := newApisEnv(t)
		f := &fakeSlack{code: code}
		n, wg := f.start(t)
		e.srv.notifier = n
		base := nsBase + "team-a/mcpservers/svc"

		wg.Add(1)
		pub := e.publish("team-a", "svc", "A111111", "t1")
		wg.Wait()
		m := decodeServer(t, e.want(200, "PUT", base, person("A111111"), serverJSON("team-a", "svc", pub.Metadata.ResourceVersion, "t1", "t2")))
		wg.Add(1)
		e.want(200, "POST", base+"/approval", person("P999999"), approval(2, m.Status.Pending.ContentHash))
		wg.Wait()
		wg.Add(1)
		e.want(200, "POST", base+"/rollback", person("A111111"), `{"fromGeneration":2,"reason":"x"}`)
		wg.Wait()
		wg.Add(1)
		e.want(200, "POST", base+"/revocation", person("A111111"), `{"reason":"x"}`)
		wg.Wait()

		f.mu.Lock()
		texts := f.texts
		f.mu.Unlock()
		want := []string{"publisert: team-a/svc, generasjon 1", "publisert: team-a/svc, generasjon 2", "rullet tilbake: team-a/svc, generasjon 1", "trukket tilbake: team-a/svc, generasjon 1"}
		if len(texts) != len(want) {
			t.Fatalf("code %d: %d notices, want %d: %q", code, len(texts), len(want), texts)
		}
		for i, w := range want {
			if !strings.Contains(texts[i], w) || strings.Contains(texts[i], "A111111") {
				t.Fatalf("code %d: notice %d = %q, want %q", code, i, texts[i], w)
			}
		}
	}
}
