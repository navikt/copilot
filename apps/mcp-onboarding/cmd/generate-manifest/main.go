// Package main provides a CLI tool to generate the NAV Copilot customizations manifest.
// It scans the .github directory and creates a JSON file with all customization metadata.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/navikt/copilot/mcp-onboarding/internal/discovery"
)

func main() {
	githubDir := flag.String("github-dir", "../..", "Path to source directory (formerly .github)")
	output := flag.String("output", "internal/discovery/copilot-manifest.json", "Output path for embedded manifest")
	repoOwner := flag.String("owner", "navikt", "Repository owner")
	repoName := flag.String("repo", "copilot", "Repository name")
	branch := flag.String("branch", "main", "Git branch for raw URLs")
	flag.Parse()

	// Create generator
	generator := NewGenerator(*repoOwner, *repoName, *branch)

	// Generate manifest from files
	manifest, err := generator.GenerateManifest(*githubDir)
	if err != nil {
		log.Fatalf("Failed to generate manifest: %v", err)
	}

	if manifest == nil {
		log.Fatal("Manifest is nil")
	}

	// Add metadata
	outputData := struct {
		Version      string                    `json:"version"`
		Repository   string                    `json:"repository"`
		Agents       []discovery.Customization `json:"agents"`
		Instructions []discovery.Customization `json:"instructions"`
		Prompts      []discovery.Customization `json:"prompts"`
		Skills       []discovery.Customization `json:"skills"`
	}{
		Version:      "1.0.0",
		Repository:   fmt.Sprintf("%s/%s", *repoOwner, *repoName),
		Agents:       manifest.Agents,
		Instructions: manifest.Instructions,
		Prompts:      manifest.Prompts,
		Skills:       manifest.Skills,
	}

	data, err := json.MarshalIndent(outputData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal manifest: %v", err)
	}

	// Write to internal/discovery (for embedding)
	outputDir := filepath.Dir(*output)
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.WriteFile(*output, data, 0600); err != nil {
		log.Fatalf("Failed to write manifest: %v", err)
	}

	fmt.Printf("✅ Generated embedded manifest: %s\n", *output)
	fmt.Printf("   Agents: %d\n", len(manifest.Agents))
	fmt.Printf("   Instructions: %d\n", len(manifest.Instructions))
	fmt.Printf("   Prompts: %d\n", len(manifest.Prompts))
	fmt.Printf("   Skills: %d\n", len(manifest.Skills))
}
