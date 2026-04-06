package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type FileVersion struct {
	Hash string `json:"hash"`
	Type string `json:"type"`
}

type VersionManifest struct {
	GeneratedAt string                 `json:"generatedAt"`
	Files       map[string]FileVersion `json:"files"`
}

func main() {
	check := flag.Bool("check", false, "Check if customization-versions.json is up to date (exits 1 if stale)")
	flag.Parse()

	repoRoot := findRepoRoot()
	githubDir := filepath.Join(repoRoot, ".github")
	outputPath := filepath.Join(repoRoot, "customization-versions.json")

	manifest := VersionManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Files:       make(map[string]FileVersion),
	}

	scanFiles(githubDir, repoRoot, &manifest, "agents", "*.agent.md", "agent")
	scanFiles(githubDir, repoRoot, &manifest, "instructions", "*.instructions.md", "instruction")
	scanFiles(githubDir, repoRoot, &manifest, "prompts", "*.prompt.md", "prompt")
	scanSkills(githubDir, repoRoot, &manifest)

	copilotInstructions := filepath.Join(githubDir, "copilot-instructions.md")
	if content, err := os.ReadFile(copilotInstructions); err == nil {
		relPath := ".github/copilot-instructions.md"
		manifest.Files[relPath] = FileVersion{
			Hash: hashContent(content),
			Type: "copilot-instructions",
		}
	}

	agentsMD := filepath.Join(repoRoot, "AGENTS.md")
	if content, err := os.ReadFile(agentsMD); err == nil {
		manifest.Files["AGENTS.md"] = FileVersion{
			Hash: hashContent(content),
			Type: "agents-md",
		}
	}

	if *check {
		existing, err := os.ReadFile(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s not found — run 'go run scripts/generate-versions/main.go' to generate\n", outputPath)
			os.Exit(1)
		}

		var existingManifest VersionManifest
		if err := json.Unmarshal(existing, &existingManifest); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s is malformed: %v\n", outputPath, err)
			os.Exit(1)
		}

		if !filesMatch(existingManifest.Files, manifest.Files) {
			fmt.Fprintf(os.Stderr, "❌ %s is out of date — run 'go run scripts/generate-versions/main.go' to update\n", outputPath)
			os.Exit(1)
		}

		fmt.Println("✅ customization-versions.json is up to date")
		return
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal manifest: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, append(data, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("✅ Generated %s with %d files\n", outputPath, len(manifest.Files))
}

func scanFiles(githubDir, repoRoot string, manifest *VersionManifest, dir, pattern, fileType string) {
	dirPath := filepath.Join(githubDir, dir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match(pattern, entry.Name())
		if !matched {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		relPath := ".github/" + dir + "/" + entry.Name()
		manifest.Files[relPath] = FileVersion{
			Hash: hashContent(content),
			Type: fileType,
		}
	}
}

func scanSkills(githubDir, repoRoot string, manifest *VersionManifest) {
	skillsDir := filepath.Join(githubDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(skillsDir, entry.Name())
		skillFiles, err := os.ReadDir(skillDir)
		if err != nil {
			continue
		}

		for _, fileEntry := range skillFiles {
			if fileEntry.IsDir() {
				continue
			}

			fullPath := filepath.Join(skillDir, fileEntry.Name())
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			relPath := ".github/skills/" + entry.Name() + "/" + fileEntry.Name()
			manifest.Files[relPath] = FileVersion{
				Hash: hashContent(content),
				Type: "skill",
			}
		}
	}
}

func hashContent(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func filesMatch(a, b map[string]FileVersion) bool {
	if len(a) != len(b) {
		return false
	}
	for path, av := range a {
		bv, ok := b[path]
		if !ok || av.Hash != bv.Hash || av.Type != bv.Type {
			return false
		}
	}
	return true
}
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to get working directory")
		os.Exit(1)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".github")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback: check if .github/ exists relative to script location
	exe, err := os.Executable()
	if err == nil {
		dir = filepath.Dir(filepath.Dir(filepath.Dir(exe)))
		if _, err := os.Stat(filepath.Join(dir, ".github")); err == nil {
			return dir
		}
	}

	fmt.Fprintln(os.Stderr, "could not find repo root (looking for .github/ and AGENTS.md)")
	os.Exit(1)
	return ""
}


