// generate-docs reads .github/ customization files and regenerates
// the markdown tables in docs/README.{agents,instructions,prompts,skills}.md.
//
// Usage:
//
//	go run .              # regenerate docs
//	go run . --check      # exit 1 if docs are out of sync
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	beginMarker = "<!-- BEGIN GENERATED TABLE -->"
	endMarker   = "<!-- END GENERATED TABLE -->"

	beginCountsMarker = "<!-- BEGIN GENERATED COUNTS -->"
	endCountsMarker   = "<!-- END GENERATED COUNTS -->"
)

var checkMode bool

func main() {
	flag.BoolVar(&checkMode, "check", false, "Check if docs are up-to-date (exit 1 if not)")
	flag.Parse()

	root := findRepoRoot()

	errors := 0
	errors += processAgents(root)
	errors += processInstructions(root)
	errors += processPrompts(root)
	errors += processSkills(root)
	errors += processReadmeCounts(root)

	if errors > 0 {
		if checkMode {
			fmt.Fprintf(os.Stderr, "\n❌ %d doc(s) out of sync. Run 'mise run docs:generate' to fix.\n", errors)
		}
		os.Exit(1)
	}

	if checkMode {
		fmt.Println("✅ All docs are in sync with customizations")
	}
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		fatal("cannot get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".github")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fatal("cannot find repo root (no .github directory found)")
		}
		dir = parent
	}
}

// parseFrontmatter extracts key-value pairs from YAML frontmatter (--- delimited).
// Only handles flat key: value pairs; skips list items.
func parseFrontmatter(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		fatal("cannot open %s: %v", path, err)
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return result
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(line), "-") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			value = strings.Trim(value, "\"'")
			if key != "" && value != "" {
				result[key] = value
			}
		}
	}

	return result
}

type metadata struct {
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Domain      string   `json:"domain"`
	Tags        []string `json:"tags"`
	Excluded    bool     `json:"excluded"`
}

func readMetadata(path string) metadata {
	data, err := os.ReadFile(path)
	if err != nil {
		return metadata{}
	}
	var m metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return metadata{}
	}
	return m
}

// titleCase converts "nais-agent" → "Nais Agent"
func titleCase(s string) string {
	words := strings.Split(s, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// escapeTableCell escapes pipe characters in markdown table cell content
func escapeTableCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func installBadge(installType, vscodeScheme, rawPath string) string {
	innerURL := fmt.Sprintf("vscode:%s/install?url=https://raw.githubusercontent.com/navikt/copilot/main/%s",
		vscodeScheme, rawPath)
	encodedInner := url.QueryEscape(innerURL)
	installURL := fmt.Sprintf("https://min-copilot.ansatt.nav.no/install/%s?url=%s",
		installType, encodedInner)
	return fmt.Sprintf("[![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](%s)", installURL)
}

func processAgents(root string) int {
	dir := filepath.Join(root, ".github", "agents")
	docPath := filepath.Join(root, "docs", "README.agents.md")

	entries, err := filepath.Glob(filepath.Join(dir, "*.agent.md"))
	if err != nil {
		fatal("cannot glob agents: %v", err)
	}
	sort.Strings(entries)

	var rows []string
	for _, entry := range entries {
		fname := filepath.Base(entry)
		stem := strings.TrimSuffix(fname, ".agent.md")

		metaPath := filepath.Join(dir, stem+".metadata.json")
		meta := readMetadata(metaPath)
		if meta.Excluded {
			continue
		}

		fm := parseFrontmatter(entry)
		name := fm["name"]
		desc := fm["description"]
		if name == "" {
			name = stem
		}

		displayName := titleCase(name)
		badge := installBadge("agent", "chat-agent", ".github/agents/"+fname)
		desc = escapeTableCell(desc)

		row := fmt.Sprintf("| **%s**<br/>[`@%s`](../.github/agents/%s) | %s | %s |",
			displayName, name, fname, desc, badge)
		rows = append(rows, row)
	}

	table := "| Agent | Description | VS Code |\n| ----- | ----------- | ------- |\n"
	table += strings.Join(rows, "\n")

	return updateDoc(docPath, table)
}

func processInstructions(root string) int {
	dir := filepath.Join(root, ".github", "instructions")
	docPath := filepath.Join(root, "docs", "README.instructions.md")

	entries, err := filepath.Glob(filepath.Join(dir, "*.instructions.md"))
	if err != nil {
		fatal("cannot glob instructions: %v", err)
	}
	sort.Strings(entries)

	var rows []string
	for _, entry := range entries {
		fname := filepath.Base(entry)
		stem := strings.TrimSuffix(fname, ".instructions.md")

		metaPath := filepath.Join(dir, stem+".metadata.json")
		meta := readMetadata(metaPath)
		if meta.Excluded {
			continue
		}

		displayName := meta.DisplayName
		desc := meta.Description
		if displayName == "" {
			displayName = titleCase(stem)
		}

		badge := installBadge("instructions", "chat-instructions", ".github/instructions/"+fname)
		desc = escapeTableCell(desc)

		row := fmt.Sprintf("| **%s**<br/>[View File](../.github/instructions/%s) | %s | %s |",
			displayName, fname, desc, badge)
		rows = append(rows, row)
	}

	table := "| Instruction | Description | VS Code |\n| ----------- | ----------- | ------- |\n"
	table += strings.Join(rows, "\n")

	return updateDoc(docPath, table)
}

func processPrompts(root string) int {
	dir := filepath.Join(root, ".github", "prompts")
	docPath := filepath.Join(root, "docs", "README.prompts.md")

	entries, err := filepath.Glob(filepath.Join(dir, "*.prompt.md"))
	if err != nil {
		fatal("cannot glob prompts: %v", err)
	}
	sort.Strings(entries)

	var rows []string
	for _, entry := range entries {
		fname := filepath.Base(entry)
		stem := strings.TrimSuffix(fname, ".prompt.md")

		metaPath := filepath.Join(dir, stem+".metadata.json")
		meta := readMetadata(metaPath)
		if meta.Excluded {
			continue
		}

		fm := parseFrontmatter(entry)
		name := fm["name"]
		desc := fm["description"]
		if name == "" {
			name = stem
		}

		badge := installBadge("prompt", "chat-prompt", ".github/prompts/"+fname)
		desc = escapeTableCell(desc)

		row := fmt.Sprintf("| **#%s**<br/>[View File](../.github/prompts/%s) | %s | %s |",
			name, fname, desc, badge)
		rows = append(rows, row)
	}

	table := "| Prompt | Description | VS Code |\n| ------ | ----------- | ------- |\n"
	table += strings.Join(rows, "\n")

	return updateDoc(docPath, table)
}

func processSkills(root string) int {
	dir := filepath.Join(root, ".github", "skills")
	docPath := filepath.Join(root, "docs", "README.skills.md")

	entries, err := os.ReadDir(dir)
	if err != nil {
		fatal("cannot read skills dir: %v", err)
	}

	var skillDirs []string
	for _, e := range entries {
		if e.IsDir() {
			skillDirs = append(skillDirs, e.Name())
		}
	}
	sort.Strings(skillDirs)

	var rows []string
	var excludedRows []string
	for _, name := range skillDirs {
		skillPath := filepath.Join(dir, name, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue
		}

		metaPath := filepath.Join(dir, name, "metadata.json")
		meta := readMetadata(metaPath)

		fm := parseFrontmatter(skillPath)
		desc := escapeTableCell(fm["description"])
		location := fmt.Sprintf("[`skills/%s/`](../skills/%s/SKILL.md)", name, name)

		row := fmt.Sprintf("| **%s** | %s | %s |", name, desc, location)

		if meta.Excluded {
			excludedRows = append(excludedRows, row)
		} else {
			rows = append(rows, row)
		}
	}

	table := "| Name | Description | Location |\n| ---- | ----------- | -------- |\n"
	for _, row := range excludedRows {
		table += fmt.Sprintf("<!-- %s -->\n", row)
	}
	table += strings.Join(rows, "\n")

	return updateDoc(docPath, table)
}

func updateDoc(path, newTable string) int {
	return updateDocWithMarkers(path, beginMarker, endMarker, newTable)
}

func updateDocWithMarkers(path, begin, end, newContent string) int {
	content, err := os.ReadFile(path)
	if err != nil {
		fatal("cannot read %s: %v", path, err)
	}

	text := string(content)
	beginIdx := strings.Index(text, begin)
	endIdx := strings.Index(text, end)

	if beginIdx == -1 || endIdx == -1 {
		fmt.Fprintf(os.Stderr, "⚠️  %s: missing markers (%s / %s)\n",
			filepath.Base(path), begin, end)
		return 1
	}

	before := text[:beginIdx+len(begin)]
	after := text[endIdx:]

	result := before + "\n" + newContent + "\n" + after

	if checkMode {
		if result != text {
			fmt.Fprintf(os.Stderr, "❌ %s is out of sync with customizations\n", filepath.Base(path))
			return 1
		}
		return 0
	}

	if result != text {
		if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
			fatal("cannot write %s: %v", path, err)
		}
		fmt.Printf("✅ Updated %s\n", filepath.Base(path))
	} else {
		fmt.Printf("   %s (no changes)\n", filepath.Base(path))
	}
	return 0
}

func processReadmeCounts(root string) int {
	readmePath := filepath.Join(root, "README.md")

	agentCount := countFiles(filepath.Join(root, ".github", "agents"), "*.agent.md")
	instructionCount := countFiles(filepath.Join(root, ".github", "instructions"), "*.instructions.md")
	promptCount := countFiles(filepath.Join(root, ".github", "prompts"), "*.prompt.md")
	skillCount := countPublicSkills(filepath.Join(root, ".github", "skills"))

	counts := fmt.Sprintf(`- **🤖 [%d Agenter](docs/README.agents.md)** — Spesialiserte AI-assistenter for Nav-domener
- **📋 [%d Instruksjoner](docs/README.instructions.md)** — Kodestandarder som aktiveres automatisk basert på filmønster
- **⚡ [%d Prompts](docs/README.prompts.md)** — Scaffolding-maler for vanlige Nav-mønstre
- **🎯 [%d Skills](docs/README.skills.md)** — Produksjonsmønstre fra ekte Nav-repoer
- **🔌 [MCP-servere](docs/README.mcp.md)** — Nav-godkjente MCP-servere fra registeret`,
		agentCount, instructionCount, promptCount, skillCount)

	return updateDocWithMarkers(readmePath, beginCountsMarker, endCountsMarker, counts)
}

func countFiles(dir, pattern string) int {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return 0
	}
	count := 0
	for _, m := range matches {
		stem := strings.TrimSuffix(filepath.Base(m), filepath.Ext(filepath.Base(m)))
		// Remove double extension (e.g. "foo.agent" from "foo.agent.md")
		stem = strings.TrimSuffix(filepath.Base(m), ".agent.md")
		if stem == filepath.Base(m) {
			stem = strings.TrimSuffix(filepath.Base(m), ".instructions.md")
		}
		if stem == filepath.Base(m) {
			stem = strings.TrimSuffix(filepath.Base(m), ".prompt.md")
		}
		metaPath := filepath.Join(dir, stem+".metadata.json")
		meta := readMetadata(metaPath)
		if !meta.Excluded {
			count++
		}
	}
	return count
}

func countPublicSkills(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err != nil {
			continue
		}
		meta := readMetadata(filepath.Join(dir, e.Name(), "metadata.json"))
		if !meta.Excluded {
			count++
		}
	}
	return count
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
