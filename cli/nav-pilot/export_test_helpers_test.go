package main

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestSource creates a temporary source tree mimicking .github/ structure.
func setupTestSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	skillDir := filepath.Join(dir, ".github", "skills", "security-review")
	mustMkdir(t, skillDir)
	mustWrite(t, filepath.Join(skillDir, "SKILL.md"), `---
name: security-review
description: Security code review
license: MIT
metadata:
  domain: security
  tags:
    - security
    - review
---

# Security Review

Run security checks on your code.
`)
	mustWrite(t, filepath.Join(skillDir, "checklist.md"), "## Checklist\n\n- [ ] Check for secrets\n")

	agentDir := filepath.Join(dir, ".github", "agents")
	mustMkdir(t, agentDir)
	mustWrite(t, filepath.Join(agentDir, "nav-pilot.agent.md"), `---
name: nav-pilot
description: Plan and build Nav applications
tools:
  - execute
  - read
  - edit
  - search
---

You are nav-pilot, an expert on Nav's platform.
`)
	mustWrite(t, filepath.Join(agentDir, "auth.agent.md"), `---
name: auth
description: Authentication and authorization expert
tools:
  - read
  - search
---

You handle auth flows for Nav apps.
`)

	promptDir := filepath.Join(dir, ".github", "prompts")
	mustMkdir(t, promptDir)
	mustWrite(t, filepath.Join(promptDir, "aksel-component.prompt.md"), `---
name: aksel-component
description: Generate Aksel components
---

Create a responsive React component using Aksel Design System.
`)

	instrDir := filepath.Join(dir, ".github", "instructions")
	mustMkdir(t, instrDir)
	mustWrite(t, filepath.Join(instrDir, "accessibility.instructions.md"), `---
applyTo: "**/*.tsx"
---

# Accessibility Standards

Always use semantic HTML elements.
`)
	mustWrite(t, filepath.Join(instrDir, "database.instructions.md"), `---
applyTo: "**/db/migration/**/*.sql"
---

# Database Migration Standards

Follow Flyway naming convention.
`)

	mustWrite(t, filepath.Join(dir, ".github", "copilot-instructions.md"), `---
applyTo: "**"
---

# Global Nav Standards

These apply everywhere.
`)

	return dir
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
