# 🔄 Keeping Customizations Up to Date

Copilot customizations in `navikt/copilot` evolve over time. Teams that have installed customization files into their repos can use the **Copilot Customization Sync** workflow to automatically detect updates and receive PRs — similar to Dependabot for dependencies.

## How It Works

```text
navikt/copilot (source)          your-repo
┌─────────────────────┐          ┌─────────────────────┐
│ .github/             │          │ .github/             │
│   agents/            │  compare │   agents/            │
│   instructions/      │ ───────► │   instructions/      │
│   prompts/           │  SHA-256 │   prompts/           │
│   skills/            │          │   skills/            │
└─────────────────────┘          └─────────────────────┘
                                         │
                                         ▼
                                   Opens PR if
                                   files differ
```

1. The workflow runs on a schedule (e.g., weekly)
2. For each customization file in your repo, it downloads the latest version from `navikt/copilot`
3. It compares SHA-256 hashes to detect changes
4. If any files are out of date, it creates a PR with the updates

## Quick Start

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

## Controlling Which Files to Sync

By default, the workflow checks **all customization files** that exist in your repo:

- `.github/agents/*.agent.md`
- `.github/instructions/*.instructions.md`
- `.github/prompts/*.prompt.md`
- `.github/skills/*/SKILL.md`
- `.github/skills/*/metadata.json`

> `AGENTS.md` and `.github/copilot-instructions.md` are not synced — they are always repo-specific.

To sync only specific files, create `.github/copilot-sync.json`:

```json
{
  "files": [
    ".github/agents/nais-platform.agent.md",
    ".github/instructions/kotlin-ktor.instructions.md",
    ".github/instructions/testing.instructions.md"
  ]
}
```

Only listed files will be checked and updated.

## Syncing from a Different Source Repo

By default, the workflow syncs from `navikt/copilot`. To sync from a team-specific repo instead (or in addition), pass the `source_repo` input:

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

This lets teams maintain their own set of customizations and distribute them to sub-repos using the same sync mechanism.

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

**Can I sync from a different repo?**
Yes. Pass `source_repo: owner/repo` as an input to the workflow. Default is `navikt/copilot`.

**Can I use a different source branch?**
The reusable workflow is pinned to `@main`. If you need a different branch, fork the workflow.

**What if a file exists in my repo but not in navikt/copilot?**
It's skipped. Only files that exist in both repos are compared.

**How is this different from Dependabot?**
Same concept — automated update PRs — but for Copilot customization files instead of package dependencies. The comparison uses SHA-256 hashes rather than semantic versioning.
