// nav-pilot installs a nav-pilot collection into the current repository.
// It copies agents, skills, instructions, and prompts from navikt/copilot
// and tracks installed state for safe updates and uninstall.
//
// Usage:
//
//	nav-pilot install <collection>     # install a collection
//	nav-pilot install -n <collection>  # dry-run
//	nav-pilot list                     # list available collections
//	nav-pilot status                   # show installed state
//	nav-pilot uninstall                # remove installed files
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ─── Types ──────────────────────────────────────────────────────────────────

// Manifest represents a collection manifest.json.
type Manifest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Agents       []string `json:"agents"`
	Skills       []string `json:"skills"`
	Instructions []string `json:"instructions"`
	Prompts      []string `json:"prompts"`
}

// StateFile tracks what was installed, for safe updates and uninstall.
type StateFile struct {
	Collection  string         `json:"collection"`
	Version     string         `json:"version"`
	SourceSHA   string         `json:"source_sha"`
	InstalledAt string         `json:"installed_at"`
	Files       []InstalledFile `json:"files"`
}

// InstalledFile records a single installed file with its content hash.
type InstalledFile struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

const stateFilePath = ".github/.nav-pilot-state.json"

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

// ─── Version (injected at build time) ───────────────────────────────────────

var (
	version = "dev"
	commit  = "unknown"
)

// ─── Source resolution ──────────────────────────────────────────────────────

// Source holds a resolved source directory and optional temp dir to clean up.
type Source struct {
	Dir     string
	TempDir string
	SHA     string
}

func (s *Source) Cleanup() {
	if s.TempDir != "" {
		os.RemoveAll(s.TempDir)
	}
}

// resolveSource finds the navikt/copilot source. Priority:
//  1. Explicit --ref flag
//  2. Local repo (walk up from CWD — dev mode)
//  3. Clone from the release tag matching this binary's version
//  4. Clone from HEAD (only if version is "dev")
func resolveSource(ref string) (*Source, error) {
	// If explicit ref given, always clone that
	if ref != "" {
		return cloneRemote(ref)
	}

	// Try local: walk up from CWD to find the navikt/copilot repo
	if wd, err := os.Getwd(); err == nil {
		for d := wd; ; d = filepath.Dir(d) {
			candidate := filepath.Join(d, ".github", "collections")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				sha := getGitSHA(d)
				fmt.Printf("%s Using local source (%s)\n", dim("→"), dim(d))
				return &Source{Dir: d, SHA: sha}, nil
			}
			if d == filepath.Dir(d) {
				break
			}
		}
	}

	// For released binaries, clone from the matching release tag
	if version != "dev" {
		return cloneRemote(version)
	}

	// Dev builds: clone HEAD
	return cloneRemote("")
}

func cloneRemote(ref string) (*Source, error) {
	tmpDir, err := os.MkdirTemp("", "nav-pilot-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	args := []string{"clone", "--depth", "1", "--quiet"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, "https://github.com/navikt/copilot.git", tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		if ref != "" {
			return nil, fmt.Errorf("cloning navikt/copilot@%s: %w", ref, err)
		}
		return nil, fmt.Errorf("cloning navikt/copilot: %w", err)
	}

	sha := getGitSHA(tmpDir)
	return &Source{Dir: tmpDir, TempDir: tmpDir, SHA: sha}, nil
}

func getGitSHA(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// ─── Manifest loading ───────────────────────────────────────────────────────

func loadManifest(sourceDir, collection string) (*Manifest, error) {
	path := filepath.Join(sourceDir, ".github", "collections", collection, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("collection %q not found: %w", collection, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest for %q: %w", collection, err)
	}
	return &m, nil
}

func listCollectionDirs(sourceDir string) ([]string, error) {
	collectionsDir := filepath.Join(sourceDir, ".github", "collections")
	entries, err := os.ReadDir(collectionsDir)
	if err != nil {
		return nil, fmt.Errorf("reading collections dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			manifest := filepath.Join(collectionsDir, e.Name(), "manifest.json")
			if _, err := os.Stat(manifest); err == nil {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// ─── State file ─────────────────────────────────────────────────────────────

func readState(targetDir string) (*StateFile, error) {
	path := filepath.Join(targetDir, stateFilePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s StateFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &s, nil
}

func writeState(targetDir string, state *StateFile) error {
	path := filepath.Join(targetDir, stateFilePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// ─── File operations ────────────────────────────────────────────────────────

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func dirHash(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		h.Write([]byte(rel))
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// copyFile copies a single file, creating parent directories.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// copyDir copies a directory recursively, creating it fresh (removes stale files).
func copyDir(src, dst string) error {
	// Remove destination first to avoid stale files
	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

// ─── Conflict detection ─────────────────────────────────────────────────────

type conflict struct {
	Path    string
	Current string // hash of existing file
	New     string // hash of source file
}

func checkConflict(targetPath, sourcePath string, isDir bool) (*conflict, error) {
	if isDir {
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			return nil, nil // no conflict
		}
		currentHash, err := dirHash(targetPath)
		if err != nil {
			return nil, err
		}
		newHash, err := dirHash(sourcePath)
		if err != nil {
			return nil, err
		}
		if currentHash == newHash {
			return nil, nil // identical
		}
		return &conflict{Path: targetPath, Current: currentHash, New: newHash}, nil
	}

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil, nil
	}
	currentHash, err := fileHash(targetPath)
	if err != nil {
		return nil, err
	}
	newHash, err := fileHash(sourcePath)
	if err != nil {
		return nil, err
	}
	if currentHash == newHash {
		return nil, nil
	}
	return &conflict{Path: targetPath, Current: currentHash, New: newHash}, nil
}

// ─── Install logic ──────────────────────────────────────────────────────────

type installResult struct {
	Installed int
	Skipped   int
	Conflicts int
	Files     []InstalledFile
}

func installItems(sourceDir, targetDir string, manifest *Manifest, dryRun, force bool) (*installResult, error) {
	result := &installResult{}

	// Agents
	if len(manifest.Agents) > 0 {
		fmt.Println(bold(fmt.Sprintf("Agents (%d):", len(manifest.Agents))))
		for _, name := range manifest.Agents {
			if err := installAgent(sourceDir, targetDir, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	// Skills
	if len(manifest.Skills) > 0 {
		fmt.Println(bold(fmt.Sprintf("Skills (%d):", len(manifest.Skills))))
		for _, name := range manifest.Skills {
			if err := installSkill(sourceDir, targetDir, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	// Instructions
	if len(manifest.Instructions) > 0 {
		fmt.Println(bold(fmt.Sprintf("Instructions (%d):", len(manifest.Instructions))))
		for _, name := range manifest.Instructions {
			if err := installInstruction(sourceDir, targetDir, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	// Prompts
	if len(manifest.Prompts) > 0 {
		fmt.Println(bold(fmt.Sprintf("Prompts (%d):", len(manifest.Prompts))))
		for _, name := range manifest.Prompts {
			if err := installPrompt(sourceDir, targetDir, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	return result, nil
}

func installAgent(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	srcFile := filepath.Join(sourceDir, ".github", "agents", name+".agent.md")
	srcMeta := filepath.Join(sourceDir, ".github", "agents", name+".metadata.json")
	dstFile := filepath.Join(targetDir, ".github", "agents", name+".agent.md")
	dstMeta := filepath.Join(targetDir, ".github", "agents", name+".metadata.json")

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		fmt.Printf("  %s Agent not found: %s\n", yellow("⚠"), name)
		result.Skipped++
		return nil
	}

	if c, err := checkConflict(dstFile, srcFile, false); err != nil {
		return err
	} else if c != nil && !force {
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
		result.Conflicts++
		return nil
	}

	relPath := filepath.Join(".github", "agents", name+".agent.md")
	if dryRun {
		fmt.Printf("  %s %s\n", dim("→"), relPath)
		result.Installed++
		return nil
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		return fmt.Errorf("copying agent %s: %w", name, err)
	}
	hash, _ := fileHash(dstFile)
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	if _, err := os.Stat(srcMeta); err == nil {
		if err := copyFile(srcMeta, dstMeta); err != nil {
			return fmt.Errorf("copying agent metadata %s: %w", name, err)
		}
		metaRel := filepath.Join(".github", "agents", name+".metadata.json")
		metaHash, _ := fileHash(dstMeta)
		result.Files = append(result.Files, InstalledFile{Path: metaRel, Hash: metaHash})
	}

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

func installSkill(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	srcDir := filepath.Join(sourceDir, ".github", "skills", name)
	dstDir := filepath.Join(targetDir, ".github", "skills", name)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		fmt.Printf("  %s Skill not found: %s\n", yellow("⚠"), name)
		result.Skipped++
		return nil
	}

	if c, err := checkConflict(dstDir, srcDir, true); err != nil {
		return err
	} else if c != nil && !force {
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
		result.Conflicts++
		return nil
	}

	relPath := filepath.Join(".github", "skills", name) + "/"
	if dryRun {
		refCount := countDirFiles(filepath.Join(srcDir, "references"))
		extra := ""
		if refCount > 0 {
			extra = dim(fmt.Sprintf(" (%d reference file(s))", refCount))
		}
		fmt.Printf("  %s %s%s\n", dim("→"), relPath, extra)
		result.Installed++
		return nil
	}

	if err := copyDir(srcDir, dstDir); err != nil {
		return fmt.Errorf("copying skill %s: %w", name, err)
	}

	hash, _ := dirHash(dstDir)
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

func installInstruction(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	srcFile := filepath.Join(sourceDir, ".github", "instructions", name+".instructions.md")
	dstFile := filepath.Join(targetDir, ".github", "instructions", name+".instructions.md")

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		fmt.Printf("  %s Instruction not found: %s\n", yellow("⚠"), name)
		result.Skipped++
		return nil
	}

	if c, err := checkConflict(dstFile, srcFile, false); err != nil {
		return err
	} else if c != nil && !force {
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
		result.Conflicts++
		return nil
	}

	relPath := filepath.Join(".github", "instructions", name+".instructions.md")
	if dryRun {
		fmt.Printf("  %s %s\n", dim("→"), relPath)
		result.Installed++
		return nil
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		return fmt.Errorf("copying instruction %s: %w", name, err)
	}
	hash, _ := fileHash(dstFile)
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

func installPrompt(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	srcDir := filepath.Join(sourceDir, ".github", "prompts", name)
	srcFile := filepath.Join(sourceDir, ".github", "prompts", name+".prompt.md")

	// Try directory first, then flat file
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		dstDir := filepath.Join(targetDir, ".github", "prompts", name)

		if c, err := checkConflict(dstDir, srcDir, true); err != nil {
			return err
		} else if c != nil && !force {
			fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
			result.Conflicts++
			return nil
		}

		relPath := filepath.Join(".github", "prompts", name) + "/"
		if dryRun {
			fmt.Printf("  %s %s\n", dim("→"), relPath)
			result.Installed++
			return nil
		}

		if err := copyDir(srcDir, dstDir); err != nil {
			return fmt.Errorf("copying prompt dir %s: %w", name, err)
		}
		hash, _ := dirHash(dstDir)
		result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})
		fmt.Printf("  %s %s\n", green("✓"), name)
		result.Installed++
		return nil
	}

	// Flat file
	dstFile := filepath.Join(targetDir, ".github", "prompts", name+".prompt.md")

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		fmt.Printf("  %s Prompt not found: %s\n", yellow("⚠"), name)
		result.Skipped++
		return nil
	}

	if c, err := checkConflict(dstFile, srcFile, false); err != nil {
		return err
	} else if c != nil && !force {
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
		result.Conflicts++
		return nil
	}

	relPath := filepath.Join(".github", "prompts", name+".prompt.md")
	if dryRun {
		fmt.Printf("  %s %s\n", dim("→"), relPath)
		result.Installed++
		return nil
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		return fmt.Errorf("copying prompt %s: %w", name, err)
	}
	hash, _ := fileHash(dstFile)
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})
	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

func countDirFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

// ─── Commands ───────────────────────────────────────────────────────────────

func cmdList(ref string) error {
	fmt.Println(dim("Resolving source..."))
	src, err := resolveSource(ref)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	names, err := listCollectionDirs(src.Dir)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(bold("Available collections:"))
	fmt.Println()
	for _, name := range names {
		m, err := loadManifest(src.Dir, name)
		if err != nil {
			continue
		}
		total := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts)
		fmt.Printf("  %-20s %s %s\n", bold(name), m.Description, dim(fmt.Sprintf("(%d items)", total)))
	}
	fmt.Println()
	fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
	return nil
}

func cmdInstall(collection, targetDir, ref string, dryRun, force bool) error {
	// Preflight: verify target looks like a git repo
	if !dryRun {
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
			return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", targetDir)
		}
	}

	fmt.Println(dim("Resolving source..."))
	src, err := resolveSource(ref)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	manifest, err := loadManifest(src.Dir, collection)
	if err != nil {
		return err
	}

	fmt.Println()
	if dryRun {
		fmt.Println(bold(fmt.Sprintf("Dry run: %s", collection)))
	} else {
		fmt.Println(bold(fmt.Sprintf("Installing: %s", collection)))
	}
	fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("navikt/copilot@%s", src.SHA)))
	fmt.Printf("%s %s\n", dim("Target:"), dim(targetDir))
	fmt.Println()

	result, err := installItems(src.Dir, targetDir, manifest, dryRun, force)
	if err != nil {
		return err
	}

	// Copy global copilot-instructions.md if not present
	if !dryRun {
		globalSrc := filepath.Join(src.Dir, ".github", "copilot-instructions.md")
		globalDst := filepath.Join(targetDir, ".github", "copilot-instructions.md")
		if _, err := os.Stat(globalSrc); err == nil {
			if _, err := os.Stat(globalDst); os.IsNotExist(err) {
				if err := copyFile(globalSrc, globalDst); err != nil {
					fmt.Fprintf(os.Stderr, "%s Could not copy copilot-instructions.md: %v\n", yellow("⚠"), err)
				} else {
					hash, _ := fileHash(globalDst)
					result.Files = append(result.Files, InstalledFile{
						Path: ".github/copilot-instructions.md",
						Hash: hash,
					})
					fmt.Printf("%s Copied global copilot-instructions.md\n", green("✓"))
				}
			}
		}
	}

	// Summary
	if result.Conflicts > 0 {
		fmt.Printf("%s %d file(s) skipped due to conflicts. Use %s to overwrite.\n",
			yellow("⚠"), result.Conflicts, bold("--force"))
	}

	if dryRun {
		fmt.Printf("%s Would install %d items from %q.\n",
			dim("→"), result.Installed, collection)
		return nil
	}

	// Write state file
	state := &StateFile{
		Collection:  collection,
		Version:     manifest.Version,
		SourceSHA:   src.SHA,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		Files:       result.Files,
	}
	if err := writeState(targetDir, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("%s Installed %d items from %q (v%s, %s).\n",
		green("✓"), result.Installed, collection, manifest.Version, src.SHA)
	fmt.Println()
	fmt.Println(dim("Next steps:"))
	fmt.Println(dim("  1. Review the installed files in .github/"))
	fmt.Println(dim("  2. Commit and push to enable Copilot customization"))
	fmt.Println(dim("  3. Use @nav-pilot in Copilot to start planning"))

	return nil
}

func cmdStatus(targetDir string) error {
	state, err := readState(targetDir)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		fmt.Println("No nav-pilot collection installed.")
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
		return nil
	}

	fmt.Println(bold("nav-pilot install status"))
	fmt.Println()
	fmt.Printf("  Collection:  %s\n", bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Source:      %s\n", state.SourceSHA)
	fmt.Printf("  Installed:   %s\n", state.InstalledAt)
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	// Check file integrity
	missing := 0
	modified := 0
	ok := 0
	for _, f := range state.Files {
		path := filepath.Join(targetDir, f.Path)
		var currentHash string
		var err error
		if strings.HasSuffix(f.Path, "/") {
			currentHash, err = dirHash(path)
		} else {
			currentHash, err = fileHash(path)
		}
		if err != nil {
			missing++
			continue
		}
		if currentHash != f.Hash {
			modified++
			fmt.Printf("  %s %s (modified locally)\n", yellow("~"), f.Path)
		} else {
			ok++
		}
	}

	fmt.Printf("\n  %s %d ok, %s %d modified, %s %d missing\n",
		green("✓"), ok, yellow("~"), modified, red("✗"), missing)
	return nil
}

func cmdUninstall(targetDir string, dryRun bool) error {
	state, err := readState(targetDir)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		fmt.Println("No nav-pilot collection installed. Nothing to uninstall.")
		return nil
	}

	if dryRun {
		fmt.Println(bold("Dry run: would uninstall"))
	} else {
		fmt.Println(bold(fmt.Sprintf("Uninstalling: %s", state.Collection)))
	}
	fmt.Println()

	removed := 0
	for _, f := range state.Files {
		path := filepath.Join(targetDir, f.Path)

		if dryRun {
			fmt.Printf("  %s %s\n", dim("×"), f.Path)
			removed++
			continue
		}

		if strings.HasSuffix(f.Path, "/") {
			if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
				fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), f.Path, err)
				continue
			}
		} else {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), f.Path, err)
				continue
			}
		}
		fmt.Printf("  %s %s\n", red("×"), f.Path)
		removed++
	}

	if !dryRun {
		// Remove state file
		stPath := filepath.Join(targetDir, stateFilePath)
		os.Remove(stPath)

		// Clean up empty .github subdirectories
		for _, sub := range []string{"agents", "skills", "instructions", "prompts"} {
			dir := filepath.Join(targetDir, ".github", sub)
			entries, err := os.ReadDir(dir)
			if err == nil && len(entries) == 0 {
				os.Remove(dir)
			}
		}
	}

	fmt.Println()
	if dryRun {
		fmt.Printf("%s Would remove %d items.\n", dim("→"), removed)
	} else {
		fmt.Printf("%s Removed %d items.\n", green("✓"), removed)
	}
	return nil
}

// ─── Main ───────────────────────────────────────────────────────────────────

func usage() {
	fmt.Fprintf(os.Stderr, `nav-pilot — collection installer for Nav's Copilot toolkit

Usage:
  nav-pilot <command> [flags]

Commands:
  install <collection>    Install a collection into the current repo
  list                    List available collections
  status                  Show what's currently installed
  uninstall               Remove installed collection files
  version                 Show version information

Flags (install/uninstall):
  -n, --dry-run           Show what would happen without making changes
  -f, --force             Overwrite files that differ from source
  -t, --target <dir>      Target repository (default: current directory)
  -r, --ref <ref>         Git ref to install from (branch or tag)

Collections:
  kotlin-backend          Kotlin/Ktor and Spring Boot teams
  nextjs-frontend         Next.js with Aksel Design System
  fullstack               Full stack (backend + frontend)
  platform                Platform and DevOps teams

Examples:
  nav-pilot install kotlin-backend
  nav-pilot install --dry-run fullstack
  nav-pilot install --force fullstack
  nav-pilot list
  nav-pilot status
  nav-pilot uninstall
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	// Parse flags manually for simplicity (flag package doesn't handle subcommands well)
	var dryRun, force bool
	var targetDir, ref string
	var positional []string

	targetDir = "."

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--dry-run":
			dryRun = true
		case "-f", "--force":
			force = true
		case "-t", "--target":
			if i+1 >= len(args) {
				fatal("--target requires a value")
			}
			i++
			targetDir = args[i]
		case "-r", "--ref":
			if i+1 >= len(args) {
				fatal("--ref requires a value")
			}
			i++
			ref = args[i]
		case "-h", "--help":
			usage()
			os.Exit(0)
		default:
			if strings.HasPrefix(args[i], "-") {
				fatal("unknown flag: %s", args[i])
			}
			positional = append(positional, args[i])
		}
	}

	// Resolve target to absolute path
	if abs, err := filepath.Abs(targetDir); err == nil {
		targetDir = abs
	}

	var err error
	switch command {
	case "install":
		if len(positional) == 0 {
			fatal("install requires a collection name. Run 'list' to see available collections.")
		}
		err = cmdInstall(positional[0], targetDir, ref, dryRun, force)
	case "list":
		err = cmdList(ref)
	case "status":
		err = cmdStatus(targetDir)
	case "uninstall":
		err = cmdUninstall(targetDir, dryRun)
	case "version", "--version", "-v":
		fmt.Printf("nav-pilot %s (%s)\n", version, commit)
	case "-h", "--help", "help":
		usage()
	default:
		fatal("unknown command: %s. Run with --help for usage.", command)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %v\n", red("Error:"), err)
		os.Exit(1)
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, red("Error: ")+format+"\n", args...)
	os.Exit(1)
}
