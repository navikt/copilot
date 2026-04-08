// Package discovery provides NAV Copilot customization discovery functionality.
// It loads a pre-compiled manifest of agents, instructions, prompts, and skills.
package discovery

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

//go:embed copilot-manifest.json
var embeddedManifest []byte

// CustomizationType represents the type of a NAV Copilot customization
type CustomizationType string

const (
	// TypeAgent represents a Copilot agent customization
	TypeAgent CustomizationType = "agent"
	// TypeInstruction represents a Copilot instruction customization
	TypeInstruction CustomizationType = "instruction"
	// TypePrompt represents a Copilot prompt customization
	TypePrompt CustomizationType = "prompt"
	// TypeSkill represents a Copilot skill customization
	TypeSkill CustomizationType = "skill"
)

// SkillReference represents a reference file bundled with a skill
type SkillReference struct {
	Path   string `json:"path"`
	RawURL string `json:"rawUrl"`
}

// Customization represents a NAV Copilot customization
type Customization struct {
	Type        CustomizationType `json:"type"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	FilePath    string            `json:"filePath"`
	UseCases    []string          `json:"useCases,omitempty"`
	InstallURL  string            `json:"installUrl"`
	RawURL      string            `json:"rawUrl"`
	References  []SkillReference  `json:"references,omitempty"`
}

// CustomizationsManifest represents the complete customizations catalog
type CustomizationsManifest struct {
	Agents       []Customization `json:"agents"`
	Instructions []Customization `json:"instructions"`
	Prompts      []Customization `json:"prompts"`
	Skills       []Customization `json:"skills"`
}

// Service handles customization discovery
type Service struct {
	repoOwner string
	repoName  string
	branch    string
	baseURL   string
	manifest  *CustomizationsManifest
}

// NewService creates a new discovery service
func NewService(repoOwner, repoName, branch, baseURL string) *Service {
	return &Service{
		repoOwner: repoOwner,
		repoName:  repoName,
		branch:    branch,
		baseURL:   baseURL,
	}
}

// GetManifest returns the loaded manifest
func (d *Service) GetManifest() *CustomizationsManifest {
	return d.manifest
}

// LoadManifest loads the embedded customizations manifest
func (d *Service) LoadManifest() error {
	var manifest CustomizationsManifest
	if err := json.Unmarshal(embeddedManifest, &manifest); err != nil {
		return fmt.Errorf("failed to unmarshal embedded manifest: %w", err)
	}
	d.manifest = &manifest
	return nil
}

// Search searches customizations by query, type, and tags
func (d *Service) Search(query string, customType string, tags []string) []Customization {
	if d.manifest == nil {
		return nil
	}

	var results []Customization
	queryLower := strings.ToLower(query)

	searchIn := func(items []Customization) {
		for _, item := range items {
			// Filter by type if specified
			if customType != "" && string(item.Type) != customType {
				continue
			}

			// Filter by tags if specified
			if len(tags) > 0 {
				hasTag := false
				for _, filterTag := range tags {
					for _, itemTag := range item.Tags {
						if strings.EqualFold(itemTag, filterTag) {
							hasTag = true
							break
						}
					}
					if hasTag {
						break
					}
				}
				if !hasTag {
					continue
				}
			}

			// Search in name, description, tags
			if query == "" ||
				strings.Contains(strings.ToLower(item.Name), queryLower) ||
				strings.Contains(strings.ToLower(item.DisplayName), queryLower) ||
				strings.Contains(strings.ToLower(item.Description), queryLower) ||
				containsAny(item.Tags, queryLower) {
				results = append(results, item)
			}
		}
	}

	searchIn(d.manifest.Agents)
	searchIn(d.manifest.Instructions)
	searchIn(d.manifest.Prompts)
	searchIn(d.manifest.Skills)

	return results
}

// ListByType returns all customizations of a specific type
func (d *Service) ListByType(customType CustomizationType, category string) []Customization {
	if d.manifest == nil {
		return nil
	}

	var items []Customization
	switch customType {
	case TypeAgent:
		items = d.manifest.Agents
	case TypeInstruction:
		items = d.manifest.Instructions
	case TypePrompt:
		items = d.manifest.Prompts
	case TypeSkill:
		items = d.manifest.Skills
	default:
		return nil
	}

	if category == "" {
		return items
	}

	// Filter by category
	var filtered []Customization
	for _, item := range items {
		if strings.EqualFold(item.Category, category) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// GenerateInstallationGuide generates installation instructions for a customization
func (d *Service) GenerateInstallationGuide(customType CustomizationType, name string) (string, error) {
	if d.manifest == nil {
		return "", fmt.Errorf("manifest not loaded")
	}

	var item *Customization
	var items []Customization

	switch customType {
	case TypeAgent:
		items = d.manifest.Agents
	case TypeInstruction:
		items = d.manifest.Instructions
	case TypePrompt:
		items = d.manifest.Prompts
	case TypeSkill:
		items = d.manifest.Skills
	}

	for i := range items {
		if items[i].Name == name {
			item = &items[i]
			break
		}
	}

	if item == nil {
		return "", fmt.Errorf("customization not found: %s", name)
	}

	// Skills with references get a multi-file install guide
	if customType == TypeSkill {
		guide := fmt.Sprintf("# Installing %s\n\n## Manual Install\n\n```bash\n", item.DisplayName)
		skillDir := ".github/skills/" + item.Name
		guide += fmt.Sprintf("mkdir -p \"%s\"\n", skillDir)
		guide += fmt.Sprintf("curl -fsSL -o \"%s/SKILL.md\" \"%s\"\n", skillDir, item.RawURL)
		if len(item.References) > 0 {
			refDirs := map[string]bool{}
			for _, ref := range item.References {
				if dir := path.Dir(ref.Path); dir != "." {
					refDirs[dir] = true
				}
			}
			for dir := range refDirs {
				guide += fmt.Sprintf("mkdir -p \"%s/%s\"\n", skillDir, dir)
			}
			for _, ref := range item.References {
				guide += fmt.Sprintf("curl -fsSL -o \"%s/%s\" \"%s\"\n", skillDir, ref.Path, ref.RawURL)
			}
		}
		guide += "```\n"
		guide += fmt.Sprintf("\n## Description\n\n%s\n", item.Description)
		return guide, nil
	}

	guide := fmt.Sprintf(`# Installing %s

## One-Click Install (Recommended)

Click to install in VS Code:
[Install %s](%s)

Or for VS Code Insiders:
[Install in VS Code Insiders](%s)

## Manual Install

1. Download the file from: %s
2. Copy to your repository's `+"`%s`"+` directory
3. The customization will automatically apply

## Description

%s
`,
		item.DisplayName,
		item.DisplayName,
		item.InstallURL,
		strings.Replace(item.InstallURL, "vscode:", "vscode-insiders:", 1),
		item.RawURL,
		strings.Replace(item.FilePath, "/"+strings.Split(item.FilePath, "/")[len(strings.Split(item.FilePath, "/"))-1], "", 1),
		item.Description,
	)

	// Add usage examples for specific types
	if customType == TypeAgent && len(item.UseCases) > 0 {
		guide += "\n## Usage Examples\n\n"
		for _, useCase := range item.UseCases {
			guide += fmt.Sprintf("- %s: `%s %s`\n", useCase, item.DisplayName, strings.ToLower(useCase))
		}
	}

	if customType == TypePrompt {
		guide += fmt.Sprintf("\n## Usage\n\nAfter installation, use in VS Code Chat:\n```\n%s [your request]\n```\n", item.DisplayName)
	}

	return guide, nil
}

func containsAny(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(strings.ToLower(s), substr) {
			return true
		}
	}
	return false
}
