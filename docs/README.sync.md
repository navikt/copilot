# 🔄 Keeping Customizations Up to Date

Copilot customizations in `navikt/copilot` evolve over time. Teams that have installed customization files into their repos can use **nav-pilot sync** to check for updates — either locally or via a GitHub Actions workflow that opens PRs automatically, similar to Dependabot.

## How It Works

```text
navikt/copilot (source)          your-repo
┌─────────────────────┐          ┌─────────────────────┐
│ .github/             │          │ .github/             │
│   agents/            │ nav-pilot│   agents/            │
│   instructions/      │  sync   │   instructions/      │
│   prompts/           │ ───────►│   prompts/           │
│   skills/            │ SHA-256 │   skills/            │
└─────────────────────┘          └─────────────────────┘
                                         │
                                         ▼
                                   Opens PR if
                                   files differ
```

**nav-pilot sync** works with both:
- **State-based repos** — repos that used `nav-pilot install` (tracks installed files via `.github/.nav-pilot-state.json`)
- **Classic repos** — repos that manually copied files (auto-detects customization files)

## Local Usage

```bash
# Check for updates (exit 1 if updates available)
nav-pilot sync

# Apply updates directly
nav-pilot sync --apply

# Machine-readable output for scripts
nav-pilot sync --json
```

## Automated Sync (GitHub Actions)

Create `.github/workflows/copilot-sync.yml` in your repo:

```yaml
name: Copilot Customization Sync
on:
  schedule:
    - cron: '0 7 * * 1'  # Weekly on Mondays at 07:00 UTC
  workflow_dispatch:       # Allow manual trigger
jobs:
  sync:
    uses: navikt/copilot/.github/workflows/copilot-customization-sync.yml@main
    permissions:
      contents: write
      pull-requests: write
```

That's it. The workflow will auto-detect all customization files in your repo and check them against the source.

## How nav-pilot sync Detects Files

**If you used `nav-pilot install`** — the state file (`.github/.nav-pilot-state.json`) tracks exactly which files were installed. Sync checks all of them.

**If you copied files manually** — nav-pilot auto-detects customization files that also exist in the source repo:

- `.github/agents/*.agent.md`
- `.github/agents/*.metadata.json`
- `.github/instructions/*.instructions.md`
- `.github/prompts/*.prompt.md`
- `.github/skills/*/` (entire skill directories)

> `AGENTS.md` and `.github/copilot-instructions.md` are never synced — they are always repo-specific.

Files that exist locally but not in the source are skipped (they're your custom additions).

## What the PR Looks Like

When updates are available, the workflow creates a PR on the `copilot-customization-sync` branch:

- **Title**: `chore: sync 2 Copilot customization(s)`
- **Label**: `dependencies`
- **Body**: Lists which files were updated with links to the source repo

The PR is automatically recreated if new updates appear before the previous one is merged.

## Staleness Tracking

The [copilot-adoption](../apps/copilot-adoption/) scanner independently tracks whether each customization file across all `navikt` repos is in sync with the source. It compares git blob OIDs (content hashes) and stores an `in_sync` boolean per file in BigQuery.

This data powers the staleness dashboard, giving org-wide visibility into which teams have outdated customizations — even if they haven't set up the sync workflow yet.

## FAQ

**Do I need a GitHub token or secret?**
No. The workflow uses the default `GITHUB_TOKEN` and reads public source files via `raw.githubusercontent.com`.

**What if I've customized a file locally?**
The PR will show the diff. You can review it, merge selectively, or close it. The workflow doesn't force-update anything — it only opens PRs.

**Can I check for updates locally without CI?**
Yes. Run `nav-pilot sync` to check, or `nav-pilot sync --apply` to update in place.

**What if a file exists in my repo but not in navikt/copilot?**
It's skipped. Only files that exist in both repos are compared.

**How is this different from Dependabot?**
Same concept — automated update PRs — but for Copilot customization files instead of package dependencies. The comparison uses SHA-256 hashes rather than semantic versioning.
