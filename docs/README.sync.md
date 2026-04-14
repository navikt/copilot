# 🔄 Keeping Customizations Up to Date

Teams that have installed customization files can use **nav-pilot sync** to check for updates — locally or via a GitHub Actions workflow that opens PRs automatically.

📖 **Full documentation:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

## Quick Reference

```bash
nav-pilot sync              # Check for updates (exit 1 if available)
nav-pilot sync --apply      # Apply updates directly
nav-pilot sync --json       # Machine-readable output
nav-pilot sync --user       # Sync user-scope install (~/.copilot/)
```

## Automated Sync (GitHub Actions)

Create `.github/workflows/copilot-sync.yml`:

```yaml
name: Copilot Customization Sync
on:
  schedule:
    - cron: '0 7 * * 1'  # Weekly on Mondays at 07:00 UTC
  workflow_dispatch:
jobs:
  sync:
    uses: navikt/copilot/.github/workflows/copilot-customization-sync.yml@main
    permissions:
      contents: write
      pull-requests: write
```

## How Detection Works

**State-based repos** (used `nav-pilot install`): The state file (`.github/.nav-pilot-state.json`) tracks exactly which files were installed.

**User-scope installs** (used `nav-pilot install --user`): The state file (`~/.copilot/.nav-pilot-state.json`) tracks installed agents and skills. Paths are remapped during sync (`agents/x` ↔ `.github/agents/x` in source).

**Classic repos** (manually copied files): nav-pilot auto-detects files that also exist in the source repo:
- `.github/agents/*.agent.md`
- `.github/instructions/*.instructions.md`
- `.github/prompts/*.prompt.md`
- `.github/skills/*/` (entire directories)

> `AGENTS.md` and `.github/copilot-instructions.md` are never synced — they are always repo-specific.

## Staleness Tracking

The [copilot-adoption](../apps/copilot-adoption/) scanner tracks whether each customization file across all `navikt` repos is in sync with the source. It compares git blob OIDs and stores an `in_sync` boolean per file in BigQuery, powering the staleness dashboard.

## Workflow Implementation Details

The reusable workflow (`.github/workflows/copilot-customization-sync.yml`) uses the `nav-pilot sync` command internally:

1. Installs `nav-pilot` CLI
2. Runs `nav-pilot sync --json` to detect updates
3. If updates found, applies them with `nav-pilot sync --apply`
4. Creates/updates a PR on the `copilot-customization-sync` branch

The workflow requires only `contents: write` and `pull-requests: write` permissions. No tokens or secrets needed — it reads public source files via `raw.githubusercontent.com`.
