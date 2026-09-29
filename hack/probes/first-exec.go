// first-exec measures how long a newly written executable script takes to
// start on this machine, 40 at a time, in four ways. Run from the repo root:
//
//	go run hack/probes/first-exec.go
//
// fresh-parallel: 40 new scripts started at once.
// fresh-serial:   40 new scripts started one after another.
// warm-parallel:  40 new scripts, each run once first, then started at once.
// writeexec:      40 scripts written the way testhome.WriteExec does (a
//
//	do-nothing script run once, then overwritten in place), started at once.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const n = 40

func main() {
	for _, mode := range []string{"fresh-parallel", "fresh-serial", "warm-parallel", "writeexec"} {
		dir, err := os.MkdirTemp("", "first-exec")
		if err != nil {
			panic(err)
		}
		paths := make([]string, n)
		for i := range paths {
			paths[i] = filepath.Join(dir, fmt.Sprintf("s%d", i))
			body := "#!/bin/sh\necho hi\n"
			if mode == "writeexec" {
				body = "#!/bin/sh\n"
			}
			if err := os.WriteFile(paths[i], []byte(body), 0o755); err != nil {
				panic(err)
			}
			if mode == "warm-parallel" || mode == "writeexec" {
				_ = exec.Command(paths[i]).Run()
			}
			if mode == "writeexec" {
				if err := os.WriteFile(paths[i], []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
					panic(err)
				}
			}
		}
		ds := make([]time.Duration, n)
		run := func(i int) {
			t := time.Now()
			if err := exec.Command(paths[i], "--version").Run(); err != nil {
				panic(err)
			}
			ds[i] = time.Since(t)
		}
		if mode == "fresh-serial" {
			for i := range paths {
				run(i)
			}
		} else {
			var wg sync.WaitGroup
			for i := range paths {
				wg.Add(1)
				go func() { defer wg.Done(); run(i) }()
			}
			wg.Wait()
		}
		sort.Slice(ds, func(a, b int) bool { return ds[a] < ds[b] })
		fmt.Printf("%-15s n=%d median=%v p90=%v max=%v\n", mode, n,
			ds[n/2].Round(time.Millisecond), ds[n*9/10].Round(time.Millisecond), ds[n-1].Round(time.Millisecond))
		_ = os.RemoveAll(dir)
	}
}
