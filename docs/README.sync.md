# 🔄 Keeping customizations up to date

Teams that have installed customization files run **nav-pilot sync** to check for updates, either locally or through a GitHub Actions workflow that opens the PRs for them.

📖 **Full documentation:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

## Quick reference

```bash
nav-pilot sync              # Sync all scopes (repo + user)
nav-pilot sync --apply      # Apply updates directly (all scopes)
nav-pilot sync --user       # Sync user-scope only (~/.copilot/)
nav-pilot sync --json       # Machine-readable output
nav-pilot sync --source navikt/my-team-copilot  # Sync from different source repo
nav-pilot --sync            # Sync all scopes and launch Copilot (non-interactive)
```

## Automated sync (GitHub Actions)

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

To sync from somewhere other than `navikt/copilot`, add a `source_repo` input to the job:

```yaml
    with:
      source_repo: navikt/my-team-copilot
```

The reusable workflow (`.github/workflows/copilot-customization-sync.yml`) uses `nav-pilot sync` internally:

1. Installs the `nav-pilot` CLI
2. Runs `nav-pilot sync --json` to detect updates
3. Applies them with `nav-pilot sync --apply` if it finds any
4. Creates or updates a PR on the `copilot-customization-sync` branch

It needs `contents: write` and `pull-requests: write` and nothing else, no tokens and no secrets, because it reads the public source files over `raw.githubusercontent.com`.

## How detection works

**State-based repos** (used `nav-pilot install`): the state file (`.github/.nav-pilot-state.json`) tracks exactly which files were installed.

**User-scope installs** (used `nav-pilot install --user`): the state file (`~/.copilot/.nav-pilot-state.json`) tracks installed agents, skills, and instructions. Paths are remapped during sync (`agents/x` ↔ `.github/agents/x` in source). Instructions use `.github/instructions/` in both local and source paths.

**Classic repos** (manually copied files): nav-pilot auto-detects files that also exist in the source repo:
- `.github/agents/*.agent.md`
- `.github/instructions/*.instructions.md`
- `.github/prompts/*.prompt.md`
- `.github/skills/*/` (entire directories)

> `AGENTS.md` and `.github/copilot-instructions.md` are never synced. They are always repo-specific.

## Overrides

A team that deliberately maintains its own version of a file can mark it as an override. Overridden files are skipped during sync, with no hash comparison and no PR diff, and you can safely delete them from your repo without them being re-added. This works for both state-based and auto-detected repos.

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

> **Important:** Sync only touches files whose names also exist in the source repo. If your team creates a file with the same name as a source file, say your own `kotlin-app-config` skill, sync sees a hash mismatch and proposes overwriting it. Add it to `overrides` to protect your version. Files with names that don't exist in the source are never affected by sync.

Overrides are also how you opt out of framework-specific files. Teams on Astro, Remix, or anything else that isn't Next.js can override the Next.js files that the `nextjs-frontend` and `fullstack` collections install, such as `.github/instructions/nextjs-aksel.instructions.md`, `.github/instructions/performance.instructions.md` and `.github/prompts/nextjs-api-route.prompt.md`.

> **Tip:** If you need no Next.js files at all, install the `frontend` collection instead. It only includes framework-agnostic tools (accessibility, testing, Aksel Design System, and so on).

## Suppressing new-item reminders (user scope)

With `nav-pilot install --user`, nav-pilot tracks every installed item and reminds you when new ones appear in the source. Use `nav-pilot ignore` to silence the reminder for an item you don't want, without installing it:

```bash
nav-pilot ignore instruction nextjs-aksel --user
nav-pilot ignore agent security-champion --user
nav-pilot ignore skill kotlin-app-config --user
```

The item is recorded in your state file with `status: "ignored"` and stops appearing in new-item reminders. `nav-pilot list --installed --user` prints a summary where excluded items are shown separately from auto-ignored (deleted) ones.

> **Note:** `nav-pilot ignore` only applies to user-scope `(all)` installs. For repo-scope installs, use `copilot-sync.json` overrides instead (see the section above).

## Formatting tolerance

Markdown files (`.md`) are compared with formatting tolerance, so these differences are ignored:
- Line endings: CRLF vs LF
- Trailing whitespace per line
- Consecutive blank lines (collapsed to single blank line)

Your team can therefore run its own formatters, Prettier with different settings for instance, without getting false-positive update PRs. JSON files (`.json`) are still compared byte-for-byte.

## Staleness tracking

The [copilot-adoption](../apps/copilot-adoption/) scanner tracks whether each customization file across all `navikt` repos is in sync with the source. It compares git blob OIDs and stores an `in_sync` boolean per file in BigQuery, which powers the staleness dashboard.
