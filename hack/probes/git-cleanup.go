// git-cleanup looks for the TempDir cleanup failure "directory not empty":
// `git commit` starts `git maintenance run --auto --detach`, a process that
// can outlive the command. It makes 300 repositories at once, commits in
// each, deletes each right after the commit returns and counts deletions
// that fail. Run from the repo root:
//
//	go run hack/probes/git-cleanup.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
)

const n = 300

func git(dir string, args ...string) error {
	c := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "user.name=a", "-c", "user.email=a@example.test"}, args...)...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("git %v: %v: %s", args, err, out)
	}
	return nil
}

func main() {
	var fails atomic.Int32
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			root, err := os.MkdirTemp("", "git-cleanup")
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o644); err != nil {
				panic(err)
			}
			for _, args := range [][]string{{"init", "-q", "."}, {"add", "f"}, {"commit", "-qm", "x"}} {
				if err := git(root, args...); err != nil {
					panic(err)
				}
			}
			if err := os.RemoveAll(root); err != nil {
				fails.Add(1)
				fmt.Println(err)
				_ = os.RemoveAll(root)
			}
		}()
	}
	wg.Wait()
	fmt.Printf("failed deletions: %d of %d\n", fails.Load(), n)
}
