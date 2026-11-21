---
applyTo: "apps/my-copilot/src/lib/copilot-manifest.json,docs/README.agents.md,docs/README.instructions.md,docs/README.prompts.md,docs/README.skills.md,docs/nav-pilot-changelog.md,**/*.png,**/*.svg"
excludeAgent: "cloud-agent"
---

# Review: skip these files

Review-only guidance.

- Generated files (`copilot-manifest.json`, and only the generated sections between the markers in `docs/README.{agents,instructions,prompts,skills}.md`) come from `mise run generate`. Do not comment on style or wording. Only flag that they differ from what the source change would generate. Hand-written prose outside the markers is reviewed as usual.
- Changelogs: Do not review wording or style. Flag only a missing entry for a user-facing change or a wrong date.
- Screenshots and diagrams: Do not comment on style or layout of images.
