// spawn-under-cpu measures how long `sh -c 'exit 0'` takes, start to exit,
// in three conditions: the machine as it is, every core kept busy by
// processes that start nothing, and a stream of newly written scripts being
// started (what a parallel `go test` does with its test binaries and fake
// clients). Run from the repo root:
//
//	go run hack/probes/spawn-under-cpu.go
//
// /bin/sh is not a new file, so a slow start there is not its own first-exec
// check: it is waiting behind the checks of other processes' new files.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"time"
)

func measure(label string, d time.Duration) {
	var all []time.Duration
	for end := time.Now().Add(d); time.Now().Before(end); {
		s := time.Now()
		_ = exec.Command("/bin/sh", "-c", "exit 0").Run()
		all = append(all, time.Since(s))
		time.Sleep(100 * time.Millisecond)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	n, over := len(all), 0
	for _, x := range all {
		if x > 2*time.Second {
			over++
		}
	}
	fmt.Printf("%-12s n=%d p50=%v p90=%v max=%v over-2s=%d\n", label, n,
		all[n/2].Round(time.Millisecond), all[n*9/10].Round(time.Millisecond), all[n-1].Round(time.Millisecond), over)
}

func main() {
	measure("as-is", 30*time.Second)

	var spin []*exec.Cmd
	for range runtime.NumCPU() + 2 {
		c := exec.Command("/usr/bin/yes")
		c.Stdout, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if c.Start() == nil {
			spin = append(spin, c)
		}
	}
	time.Sleep(5 * time.Second)
	measure("cpu-busy", 30*time.Second)
	for _, c := range spin {
		_ = c.Process.Kill()
		_ = c.Wait()
	}

	dir, err := os.MkdirTemp("", "spawn-under-cpu")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	var stop atomic.Bool
	var n atomic.Int64
	for w := range 8 {
		go func() {
			for i := 0; !stop.Load(); i++ {
				p := filepath.Join(dir, fmt.Sprintf("w%d-%d", w, i))
				if os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755) == nil {
					_ = exec.Command(p).Run()
					n.Add(1)
				}
			}
		}()
	}
	time.Sleep(5 * time.Second)
	measure("new-scripts", 30*time.Second)
	stop.Store(true)
	fmt.Printf("(%d new scripts started meanwhile)\n", n.Load())
}
