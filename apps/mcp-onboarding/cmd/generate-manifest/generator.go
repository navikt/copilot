// Package main provides generator functions for creating the manifest
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/mcp-onboarding/internal/discovery"
	"gopkg.in/yaml.v3"
)

// AgentFrontmatter represents the frontmatter in agent files
type AgentFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// InstructionFrontmatter represents the frontmatter in instruction files
type InstructionFrontmatter struct {
	ApplyTo string `yaml:"applyTo"`
}

// PromptFrontmatter represents the frontmatter in prompt files
type PromptFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// SkillMetadata represents the metadata.json for a skill
type SkillMetadata struct {
	Description string   `json:"description"`
	References  []string `json:"references"`
	Excluded    bool     `json:"excluded"`
}

// parseFrontmatter extracts YAML frontmatter from a markdown file
func parseFrontmatter(content string, v interface{}) error {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return fmt.Errorf("no frontmatter found")
	}

	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return fmt.Errorf("frontmatter not closed")
	}

	frontmatter := strings.Join(lines[1:endIdx], "\n")
	return yaml.Unmarshal([]byte(frontmatter), v)
}

// Generator generates manifest from .github files
type Generator struct {
	repoOwner string
	repoName  string
	branch    string
}

// NewGenerator creates a new generator
func NewGenerator(repoOwner, repoName, branch string) *Generator {
	return &Generator{
		repoOwner: repoOwner,
		repoName:  repoName,
		branch:    branch,
	}
}

// GenerateManifest scans .github directory and generates manifest
func (g *Generator) GenerateManifest(githubDir string) (*discovery.CustomizationsManifest, error) {
	manifest := &discovery.CustomizationsManifest{
		Agents:       []discovery.Customization{},
		Instructions: []discovery.Customization{},
		Prompts:      []discovery.Customization{},
		Skills:       []discovery.Customization{},
	}

	// Load agents
	agentsDir := filepath.Join(githubDir, "agents")
	agents, err := g.loadAgents(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}
	manifest.Agents = agents

	// Load instructions
	instructionsDir := filepath.Join(githubDir, "instructions")
	instructions, err := g.loadInstructions(instructionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load instructions: %w", err)
	}
	manifest.Instructions = instructions

	// Load prompts
	promptsDir := filepath.Join(githubDir, "prompts")
	prompts, err := g.loadPrompts(promptsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load prompts: %w", err)
	}
	manifest.Prompts = prompts

	// Load skills
	skillsDir := filepath.Join(githubDir, "skills")
	if _, err := os.Stat(skillsDir); err == nil {
		skills, err := g.loadSkills(skillsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load skills: %w", err)
		}
		manifest.Skills = skills
	}

	return manifest, nil
}

func (g *Generator) loadAgents(dir string) ([]discovery.Customization, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.agent.md"))
	if err != nil {
		return nil, err
	}

	var agents []discovery.Customization
	for _, file := range files {
		content, err := os.ReadFile(file) //nolint:gosec // Generator needs to read .github files
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}

		var fm AgentFrontmatter
		if err := parseFrontmatter(string(content), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter in %s: %w", file, err)
		}

		filename := filepath.Base(file)
		relPath := path.Join(".github/agents", filename)
		category := g.inferCategory(fm.Name, fm.Description)
		tags := g.extractTags(fm.Name, fm.Description)

		agents = append(agents, discovery.Customization{
			Type:        discovery.TypeAgent,
			Name:        fm.Name,
			DisplayName: "@" + fm.Name,
			Description: fm.Description,
			Category:    category,
			Tags:        tags,
			FilePath:    relPath,
			UseCases:    g.extractUseCases(string(content)),
			InstallURL:  g.generateInstallURL(discovery.TypeAgent, relPath),
			RawURL:      g.generateRawURL(relPath),
		})
	}

	return agents, nil
}

func (g *Generator) loadInstructions(dir string) ([]discovery.Customization, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.instructions.md"))
	if err != nil {
		return nil, err
	}

	var instructions []discovery.Customization
	for _, file := range files {
		content, err := os.ReadFile(file) //nolint:gosec // Generator needs to read .github files
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}

		var fm InstructionFrontmatter
		if err := parseFrontmatter(string(content), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter in %s: %w", file, err)
		}

		filename := filepath.Base(file)
		relPath := path.Join(".github/instructions", filename)
		name := strings.TrimSuffix(filename, ".instructions.md")
		displayName := g.humanizeName(name)
		description := g.extractDescription(string(content))
		tags := g.extractTags(name, description)

		instructions = append(instructions, discovery.Customization{
			Type:        discovery.TypeInstruction,
			Name:        name,
			DisplayName: displayName,
			Description: description,
			Tags:        tags,
			FilePath:    relPath,
			InstallURL:  g.generateInstallURL(discovery.TypeInstruction, relPath),
			RawURL:      g.generateRawURL(relPath),
		})
	}

	return instructions, nil
}

func (g *Generator) loadPrompts(dir string) ([]discovery.Customization, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.prompt.md"))
	if err != nil {
		return nil, err
	}

	var prompts []discovery.Customization
	for _, file := range files {
		content, err := os.ReadFile(file) //nolint:gosec // Generator needs to read .github files
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}

		var fm PromptFrontmatter
		if err := parseFrontmatter(string(content), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter in %s: %w", file, err)
		}

		filename := filepath.Base(file)
		relPath := path.Join(".github/prompts", filename)
		tags := g.extractTags(fm.Name, fm.Description)

		prompts = append(prompts, discovery.Customization{
			Type:        discovery.TypePrompt,
			Name:        fm.Name,
			DisplayName: "#" + fm.Name,
			Description: fm.Description,
			Tags:        tags,
			FilePath:    relPath,
			InstallURL:  g.generateInstallURL(discovery.TypePrompt, relPath),
			RawURL:      g.generateRawURL(relPath),
		})
	}

	return prompts, nil
}

func (g *Generator) loadSkills(dir string) ([]discovery.Customization, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []discovery.Customization
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue
		}

		content, err := os.ReadFile(skillFile) //nolint:gosec // Generator needs to read .github files
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", skillFile, err)
		}

		name := entry.Name()
		description := g.extractDescription(string(content))
		relPath := path.Join(".github/skills", name)

		// Read metadata.json (single source of truth for references and exclusion)
		var refs []discovery.SkillReference
		metaFile := filepath.Join(dir, name, "metadata.json")
		if metaData, err := os.ReadFile(metaFile); err == nil { //nolint:gosec // Generator needs to read .github files
			var meta SkillMetadata
			if err := json.Unmarshal(metaData, &meta); err == nil {
				if meta.Excluded {
					continue
				}
				for _, ref := range meta.References {
					refs = append(refs, discovery.SkillReference{
						Path:   ref,
						RawURL: g.generateRawURL(path.Join(".github/skills", name, ref)),
					})
				}
			}
		}

		skills = append(skills, discovery.Customization{
			Type:        discovery.TypeSkill,
			Name:        name,
			DisplayName: name,
			Description: description,
			FilePath:    relPath,
			InstallURL:  "",
			RawURL:      g.generateRawURL(path.Join(relPath, "SKILL.md")),
			References:  refs,
		})
	}

	return skills, nil
}

// Helper methods extracted from original discovery package
func (g *Generator) inferCategory(name, description string) string {
	lower := strings.ToLower(name + " " + description)
	if strings.Contains(lower, "nais") || strings.Contains(lower, "kubernetes") ||
		strings.Contains(lower, "deployment") || strings.Contains(lower, "observability") {
		return "platform"
	}
	if strings.Contains(lower, "auth") || strings.Contains(lower, "security") {
		return "security"
	}
	if strings.Contains(lower, "kafka") || strings.Contains(lower, "kotlin") || strings.Contains(lower, "ktor") {
		return "backend"
	}
	if strings.Contains(lower, "aksel") || strings.Contains(lower, "nextjs") || strings.Contains(lower, "react") {
		return "frontend"
	}
	return ""
}

func (g *Generator) extractTags(name, description string) []string {
	tags := []string{}
	lower := strings.ToLower(name + " " + description)

	tagMap := map[string][]string{
		"nais":          {"nais", "kubernetes", "deployment", "infrastructure"},
		"auth":          {"auth", "azure-ad", "tokenx", "security"},
		"kafka":         {"kafka", "events", "rapids-rivers", "streaming"},
		"aksel":         {"aksel", "design-system", "nextjs", "react", "ui"},
		"observability": {"observability", "metrics", "logging", "tracing", "prometheus"},
		"security":      {"security", "compliance", "gdpr", "vulnerability"},
		"kotlin":        {"kotlin", "ktor", "backend"},
		"nextjs":        {"nextjs", "aksel", "frontend", "react"},
		"postgresql":    {"postgresql", "database", "migrations"},
		"testing":       {"testing", "jest", "junit", "quality"},
	}

	// Check keywords in a deterministic order
	keywords := []string{"aksel", "nextjs", "nais", "auth", "kafka", "observability", "security", "kotlin", "postgresql", "testing"}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			tags = append(tags, tagMap[keyword]...)
			break
		}
	}

	seen := make(map[string]bool)
	uniqueTags := []string{}
	for _, tag := range tags {
		if !seen[tag] {
			seen[tag] = true
			uniqueTags = append(uniqueTags, tag)
		}
	}
	return uniqueTags
}

func (g *Generator) extractUseCases(content string) []string {
	useCases := []string{}
	if strings.Contains(content, "deployment") || strings.Contains(content, ".nais/") {
		useCases = append(useCases, "Creating .nais/app.yaml manifests")
	}
	if strings.Contains(content, "PostgreSQL") || strings.Contains(content, "database") {
		useCases = append(useCases, "Adding PostgreSQL/Kafka")
	}
	if strings.Contains(content, "troubleshoot") {
		useCases = append(useCases, "Troubleshooting deployments")
	}
	if strings.Contains(content, "Azure AD") || strings.Contains(content, "TokenX") {
		useCases = append(useCases, "Azure AD integration", "TokenX token exchange")
	}
	if strings.Contains(content, "Kafka") || strings.Contains(content, "Rapids") {
		useCases = append(useCases, "Creating Kafka event consumers", "Designing event schemas")
	}
	if strings.Contains(content, "Aksel") || strings.Contains(content, "Design System") {
		useCases = append(useCases, "Converting Tailwind to Aksel tokens", "Responsive layouts")
	}
	if strings.Contains(content, "metrics") || strings.Contains(content, "Prometheus") {
		useCases = append(useCases, "Health endpoints", "Business metrics", "OpenTelemetry tracing")
	}
	if strings.Contains(content, "security") || strings.Contains(content, "GDPR") {
		useCases = append(useCases, "Network policies", "Secrets management", "GDPR compliance")
	}
	return useCases
}

func (g *Generator) extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	pastFrontmatter := false
	pastTitle := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			inFrontmatter = false
			pastFrontmatter = true
			continue
		}
		if inFrontmatter || !pastFrontmatter {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			pastTitle = true
			continue
		}
		if trimmed == "" {
			continue
		}
		if pastTitle && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return trimmed
		}
	}
	return "No description available"
}

func (g *Generator) humanizeName(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	result := strings.Join(parts, "/")
	if !strings.Contains(result, "Development") && !strings.Contains(result, "Standards") {
		result += " Development"
	}
	return result
}

func (g *Generator) generateInstallURL(customType discovery.CustomizationType, filename string) string {
	var protocol string
	switch customType {
	case discovery.TypeAgent:
		protocol = "vscode:chat-agent/install"
	case discovery.TypeInstruction:
		protocol = "vscode:chat-instructions/install"
	case discovery.TypePrompt:
		protocol = "vscode:chat-prompt/install"
	default:
		return ""
	}
	rawURL := g.generateRawURL(filename)
	return fmt.Sprintf("%s?url=%s", protocol, rawURL)
}

func (g *Generator) generateRawURL(filePath string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		g.repoOwner, g.repoName, g.branch, filePath)
}
