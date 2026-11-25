// Package main implements copilot-survey, which owns Nav's user surveys: the
// definitions, the answers and the dedup that refuses a second answer. It
// has no ingress. copilot-cli calls it for nav-pilot, my-copilot for
// ki-utvikling.
package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/navikt/copilot/apps/copilot-survey/internal/config"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// files holds surveys/ and imports/, which stay at the app root because CI,
// nav-pilot and my-copilot read them by path.
//
//go:embed surveys imports
var files embed.FS

// No OpenTelemetry here, on purpose: a trace of a submission would time-stamp
// who answered (see internal/survey/store.go and the privacy notes in README.md).
func main() {
	survey.SetFiles(files)
	if len(os.Args) > 1 && os.Args[1] == "export" {
		if err := survey.RunExport(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "export:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "import" {
		if err := survey.RunImport(os.Args[2:], os.Stdin, os.Stdout, os.Getenv("DB_URL"), time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "import:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "aggregate" {
		if err := survey.RunAggregate(os.Args[2:], os.Stdout, os.Getenv("DB_URL"), time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "aggregate:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "code" {
		if err := survey.RunCode(os.Args[2:], os.Stdout, os.Stderr, os.Getenv("GITHUB_TOKEN")); err != nil {
			fmt.Fprintln(os.Stderr, "code:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "analyze" {
		if err := survey.RunAnalyze(os.Args[2:], os.Stdout, time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "analyze:", err)
			os.Exit(1)
		}
		return
	}
	config := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel})))

	survey.Serve(config)
}
