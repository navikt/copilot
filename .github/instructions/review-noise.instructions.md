---
applyTo: "apps/my-copilot/src/lib/copilot-manifest.json,docs/README.*.md,docs/nav-pilot-changelog.md,**/CHANGELOG*.md,**/*.png,**/*.svg,**/*.drawio"
excludeAgent: "cloud-agent"
---

# Review: skip these files

Review-only guidance.

- Generated files (`copilot-manifest.json`, `docs/README.*.md`) come from `mise run generate`. Do not comment on style or wording. Only flag that the file differs from what the source change would generate.
- Changelogs: Do not review wording or style. Flag only a missing entry for a user-facing change or a wrong date.
- Screenshots and diagrams: Do not comment on style or layout of images.
