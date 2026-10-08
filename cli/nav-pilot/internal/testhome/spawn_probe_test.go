package testhome

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// TestSpawnProbe is a measurement, not a test: it times process starts from
// inside the test process while whatever else runs, for hack/probes/RESULTS.md
// (2026-10-08). Run it beside a full suite:
//
//	NAV_PILOT_SPAWN_PROBE=150s go test ./internal/testhome -run SpawnProbe -count=1 -v
func TestSpawnProbe(t *testing.T) {
	d, err := time.ParseDuration(os.Getenv("NAV_PILOT_SPAWN_PROBE"))
	if err != nil {
		t.Skip("set NAV_PILOT_SPAWN_PROBE to a duration to measure")
	}
	dir := t.TempDir()
	res := map[string][]time.Duration{}
	add := func(k string, since time.Time) { res[k] = append(res[k], time.Since(since)) }
	for i := 0; ; i++ {
		if d -= 200 * time.Millisecond; d < 0 {
			break
		}
		s := time.Now()
		c := exec.Command("/bin/sh", "-c", "exit 0")
		_ = c.Start()
		add("sh Start()", s)
		_ = c.Wait()
		add("sh -c exit", s)

		p := filepath.Join(dir, fmt.Sprintf("s%d", i))
		s = time.Now()
		if err := WriteExec(p, "#!/bin/sh\necho hi\n"); err != nil {
			t.Fatal(err)
		}
		add("WriteExec(write+warm)", s)
		s = time.Now()
		_ = exec.Command(p, "--version").Run()
		add("exec warmed script", s)

		f := filepath.Join(dir, fmt.Sprintf("p%d.py", i))
		_ = os.WriteFile(f, []byte("pass\n"), 0o644)
		s = time.Now()
		_ = exec.Command("/bin/cat", f).Run()
		add("cat fresh file", s)
		time.Sleep(200 * time.Millisecond)
	}
	for name, ds := range res {
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		n, over := len(ds), 0
		for _, x := range ds {
			if x > 2*time.Second {
				over++
			}
		}
		t.Logf("%-22s n=%d p50=%v p90=%v p99=%v max=%v >2s=%d", name, n, ds[n/2], ds[n*9/10], ds[n*99/100], ds[n-1], over)
	}
}
