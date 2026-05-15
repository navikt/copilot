# 🔄 Keeping Customizations Up to Date

Teams that have installed customization files can use **nav-pilot sync** to check for updates — locally or via a GitHub Actions workflow that opens PRs automatically.

📖 **Full documentation:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

## Quick Reference

```bash
nav-pilot sync              # Sync all scopes (repo + user)
nav-pilot sync --apply      # Apply updates directly (all scopes)
nav-pilot sync --user       # Sync user-scope only (~/.copilot/)
nav-pilot sync --json       # Machine-readable output
nav-pilot sync --source navikt/my-team-copilot  # Sync from different source repo
nav-pilot --sync            # Sync all scopes and launch Copilot (non-interactive)
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

### Sync from a team repo

To sync from a different source repository instead of `navikt/copilot`:

```yaml
jobs:
  sync:
    uses: navikt/copilot/.github/workflows/copilot-customization-sync.yml@main
    with:
      source_repo: navikt/my-team-copilot
    permissions:
      contents: write
      pull-requests: write
```

## How Detection Works

**State-based repos** (used `nav-pilot install`): The state file (`.github/.nav-pilot-state.json`) tracks exactly which files were installed.

**User-scope installs** (used `nav-pilot install --user`): The state file (`~/.copilot/.nav-pilot-state.json`) tracks installed agents, skills, and instructions. Paths are remapped during sync (`agents/x` ↔ `.github/agents/x` in source). Instructions use `.github/instructions/` in both local and source paths.

**Classic repos** (manually copied files): nav-pilot auto-detects files that also exist in the source repo:
- `.github/agents/*.agent.md`
- `.github/instructions/*.instructions.md`
- `.github/prompts/*.prompt.md`
- `.github/skills/*/` (entire directories)

> `AGENTS.md` and `.github/copilot-instructions.md` are never synced — they are always repo-specific.

## Overrides

Teams that intentionally maintain their own versions of specific files can mark them as overrides. Overridden files are skipped during sync — no hash comparison, no PR diff.

Create `.github/copilot-sync.json` in your repo:

```json
{
  "overrides": [
    ".github/agents/nais.agent.md",
    ".github/instructions/security.instructions.md",
    ".github/skills/api-design/"
  ]
}
```

This works with both state-based and auto-detected repos.

> **Important:** Sync only touches files whose names also exist in the source repo. If your team creates a file with the same name as a source file (e.g., your own `kotlin-app-config` skill), sync will detect a hash mismatch and propose overwriting it. Add it to `overrides` to protect your version. Files with names that don't exist in the source are never affected by sync.

## Suppressing New-Item Reminders (User Scope)

When using `nav-pilot install --user`, nav-pilot tracks all installed items and reminds you when new items are added to the source. If you don't want a specific item, use `nav-pilot ignore` to suppress the reminder without installing it:

```bash
nav-pilot ignore instruction nextjs-aksel --user
nav-pilot ignore agent security-champion --user
nav-pilot ignore skill kotlin-app-config --user
```

The item is recorded in your state file with `status: "ignored"` and will no longer appear in new-item reminders. Run `nav-pilot list --installed --user` to see a summary — excluded items are shown separately from auto-ignored (deleted) ones.

> **Note:** `nav-pilot ignore` only applies to user-scope `(all)` installs. For repo-scope installs, use `copilot-sync.json` overrides instead (see section below).

### Opting out of framework-specific files

Teams using Astro, Remix, or other non-Next.js frameworks can use overrides to skip Next.js-specific files installed by the `nextjs-frontend` or `fullstack` collection:

```json
{
  "overrides": [
    ".github/instructions/nextjs-aksel.instructions.md",
    ".github/instructions/performance.instructions.md",
    ".github/prompts/nextjs-api-route.prompt.md"
  ]
}
```

These files will be completely ignored during sync. You can also safely delete them from your repo — they will not be re-added.

> **Tip:** If you don't need any Next.js files, consider installing the `frontend` collection instead, which only includes framework-agnostic tools (accessibility, testing, Aksel Design System, etc.).

## Formatting Tolerance

Markdown files (`.md`) are compared with formatting tolerance. The following differences are ignored:
- Line endings: CRLF vs LF
- Trailing whitespace per line
- Consecutive blank lines (collapsed to single blank line)

This means teams can run their own formatters (e.g. Prettier with different settings) without getting false-positive update PRs. JSON files (`.json`) are still compared byte-for-byte.

## Staleness Tracking

The [copilot-adoption](../apps/copilot-adoption/) scanner tracks whether each customization file across all `navikt` repos is in sync with the source. It compares git blob OIDs and stores an `in_sync` boolean per file in BigQuery, powering the staleness dashboard.

## Workflow Implementation Details

The reusable workflow (`.github/workflows/copilot-customization-sync.yml`) uses the `nav-pilot sync` command internally:

1. Installs `nav-pilot` CLI
2. Runs `nav-pilot sync --json` to detect updates
3. If updates found, applies them with `nav-pilot sync --apply`
4. Creates/updates a PR on the `copilot-customization-sync` branch

The workflow requires only `contents: write` and `pull-requests: write` permissions. No tokens or secrets needed — it reads public source files via `raw.githubusercontent.com`.
