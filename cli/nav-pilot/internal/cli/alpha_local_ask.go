package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// cmdLocalAsk puts one question to the local model and prints the answer.
//
// Everything else in `alpha local` talks about the model at one remove: status
// says a server answered a completion, the benchmark numbers are medians of
// other people's tasks. None of it lets a developer form their own judgement of
// what this thing is like to work with, and that judgement is what the alpha is
// actually asking for.
//
// It also separates two failures that look identical from inside a session. A
// task that produced nothing can mean the model is broken, or that the
// orchestrator never dispatched it. One question with a visible answer settles
// which.
//
// Through the same machine-wide lock as the guard, because the reason that lock
// exists is that two concurrent requests wedge mlx-lm, and a developer poking
// the model while a session runs is exactly that case.
func cmdLocalAsk(args []string) error {
	prompt, err := askPrompt(args)
	if err != nil {
		return err
	}

	st, ok, err := local.LoadState()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no local server is running. Start it: nav-pilot alpha local start")
	}

	fmt.Printf("\n  %s %s\n\n", dim("asking"), bold(st.Model))
	started := time.Now()
	answer, in, out, err := local.Ask(context.Background(), prompt)
	if err != nil {
		return err
	}
	elapsed := time.Since(started)

	fmt.Println(answer)
	tokens := dim("no token count reported")
	if in > 0 || out > 0 {
		tokens = dim(fmt.Sprintf("%d tokens in, %d out", in, out))
	}
	fmt.Printf("\n  %s · %s · %s\n\n",
		dim(fmt.Sprintf("%.1fs", elapsed.Seconds())), tokens, dim("no AI credits"))
	return nil
}

// askPrompt takes the question from -p, from the arguments, or from stdin when
// it is a pipe. Stdin matters more than it looks: the useful question about a
// local model is usually about a file, and `cat x.kt | nav-pilot alpha local
// ask -p "what does this do"` is how a developer would ask it.
func askPrompt(args []string) (string, error) {
	var parts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p", "--prompt":
			if i+1 >= len(args) {
				return "", fmt.Errorf("-p needs a question after it")
			}
			parts = append(parts, args[i+1])
			i++
		default:
			parts = append(parts, args[i])
		}
	}
	prompt := strings.TrimSpace(strings.Join(parts, " "))

	// Stdin only when the arguments carried no question. Reading it whenever
	// stdin is not a terminal blocks forever on an open pipe that never sends
	// anything, which is every script, CI step and harness that launches this
	// without redirecting stdin. It hung exactly that way the first time it
	// ran. A question on the command line is unambiguous, so take it and do
	// not touch the pipe.
	// Stdin only when it is a pipe and the arguments carried no question.
	// Reading it whenever there is no prompt blocks an interactive terminal
	// waiting for EOF, which looks like a hang; reading it whenever stdin is
	// not a terminal blocks on any script's open pipe. Both were tried.
	if prompt == "" && !isCharDevice(os.Stdin) {
		piped, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		if err != nil {
			return "", fmt.Errorf("reading the question from stdin: %w", err)
		}
		prompt = strings.TrimSpace(string(piped))
	}
	if prompt == "" {
		return "", fmt.Errorf("nothing to ask. Try: nav-pilot alpha local ask -p \"hva gjør denne funksjonen?\"")
	}
	return prompt, nil
}

// isCharDevice reports whether the file is a terminal rather than a pipe or a
// redirect. os.ModeCharDevice is also set for /dev/null, which is why callers
// that must distinguish a real terminal use the kernel's answer instead.
func isCharDevice(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
