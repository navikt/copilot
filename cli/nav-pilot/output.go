package main

import (
	"fmt"
	"os"
)

// ─── Color helpers ──────────────────────────────────────────────────────────

var useColor = true

func init() {
	if os.Getenv("NO_COLOR") != "" {
		useColor = false
	}
}

func color(code, msg string) string {
	if !useColor {
		return msg
	}
	return fmt.Sprintf("\033[%sm%s\033[0m", code, msg)
}

func red(msg string) string    { return color("31", msg) }
func green(msg string) string  { return color("32", msg) }
func yellow(msg string) string { return color("33", msg) }
func dim(msg string) string    { return color("2", msg) }
func bold(msg string) string   { return color("1", msg) }
