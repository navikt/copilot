# 🔄 Keeping customizations up to date

Teams that have installed customization files run **nav-pilot sync** to check for updates, either locally or through a GitHub Actions workflow that opens the PRs for them.

📖 **Full documentation:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

## Quick reference

```bash
nav-pilot sync              # Sync all scopes (repo + user)
nav-pilot sync --apply      # Apply updates directly (all scopes); in a terminal it asks before removing files
nav-pilot sync --apply --yes  # ...without asking
nav-pilot sync --user       # Sync user-scope only (~/.copilot/)
nav-pilot sync --json       # One JSON document: {"scopes": [{"scope": "repo", ...}, ...]}
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

Step 2 reads one JSON document on stdout, `{"scopes": [...]}`, with one entry per scope, each naming its `scope`. `sync --user`, `--repo` or `--target` prints that one scope's entry on its own. Exit codes are 0 for up to date, 1 for updates available and 2 for a sync that could not run. On 2, the scope that failed has `{"scope": ..., "error": ...}` in place of its lists, so the workflow can say what went wrong instead of finding stdout empty. `--apply --json` applies and reports what it did (`"applied": true`). The PR lists updated, added and deleted files, marks updates that replace a local edit, and lists the pin.

If the GitHub releases API cannot be reached, sync does not move a committed pin to the default branch on a guess. It keeps the pin, says why on stderr and exits 2. `--ref <branch|sha>` moves the pin deliberately.

Step 2 counts the pinned revision in `.nav-pilot/agentpakke.lock.json` as an update in its own right (`pin_bump` in the JSON). An agentpakke can move forward without any installed file changing — a change to a file this repo never installed, a docs change upstream — and the pin would otherwise stay behind for good, because step 3 only runs when step 2 found something. The PR lists it like any other change.

It needs `contents: write` and `pull-requests: write` and nothing else, no tokens and no secrets, because it reads the public source files over `raw.githubusercontent.com`.

## How detection works

**State-based repos** (used `nav-pilot install`): the state file (`.github/.nav-pilot-state.json`) tracks exactly which files were installed.

**User-scope installs** (used `nav-pilot install --user`): the state file (`~/.copilot/.nav-pilot-state.json`) tracks installed agents, skills, and instructions. Paths are remapped during sync (`agents/x` ↔ `.github/agents/x` in source). Instructions use `.github/instructions/` in both local and source paths.

An install that holds a whole agentpakke is also checked the other way round: an artifact the source ships that the install does not have is a pending change like any other, listed under `added` in `--json` and installed by `--apply`. Nothing is overwritten and nothing is deleted. Take one back out with `nav-pilot ignore <name>` in a user-scope install, or by deleting the file in a repo: the next sync records it as ignored and leaves it out from then on.

**Classic repos** (manually copied files): nav-pilot auto-detects files that also exist in the source repo:
- `.github/agents/*.agent.md`
- `.github/instructions/*.instructions.md`
- `.github/prompts/*.prompt.md`
- `.github/skills/*/` (entire directories)

> `AGENTS.md` and `.github/copilot-instructions.md` are never synced. They are always repo-specific.

## Files you have edited

`nav-pilot sync --apply` takes the source's version of a file even when the file changed here since nav-pilot installed it. Most people never edit a synced file, so an edit does not hold the update back. It is not lost either: sync saves your copy as `<file>.orig` beside the new one and says so.

```
⚠ agent nav-pilot: your local changes were replaced by the new version; your copy is saved as .github/agents/nav-pilot.agent.md.orig
```

In a skill directory only the files that differ are saved, each as `<file>.orig` inside the directory. There is one backup per file, and the next replacement overwrites it. `--json` lists these files under `updates` and again under `replaced_local_edits`, and the scheduled workflow marks them in the PR body. A plain `nav-pilot sync` marks them too, before anything is written.

To keep your own version for good, list the file under `overrides` (see below).

## Files that were there before install

A file that already exists when `nav-pilot install` runs, with a name the agentpakke also ships and different content, is your team's. Install leaves it alone and does not record it as nav-pilot's:

```
⚠ skipped agent nav-pilot: .github/agents/nav-pilot.agent.md already exists and isn't from nav-pilot. Keep it, or remove it and re-run to install nav-pilot's version.
```

No later sync touches it. Sync names it on every run, and `--json` lists it under `skipped_existing`. It is not an available update, so the scheduled workflow does not open a PR for it. `nav-pilot install --force` is the only way to replace it, and saves it as `<file>.orig` first.

Older nav-pilot versions did record such a file as nav-pilot's, marked as differing from what nav-pilot installed. Sync treats that record as a local edit: `--apply` takes the source's version and keeps yours as `<file>.orig`.

## What sync will not delete

When the source stops shipping a file, `nav-pilot sync --apply` removes your copy of it — but only when that copy is byte-for-byte what nav-pilot installed. A file whose content has changed since then is left on disk and named in the output:

```
⚠ 1 file(s) deleted in source differ from what nav-pilot installed and were kept (source: a1b2c3d)

  ⊘ .github/agents/nais.agent.md
Delete them yourself if you no longer want them, or list them under overrides in .github/copilot-sync.json to stop sync mentioning them.
```

There is no flag that makes sync delete it. A file that differs may be your team's own work, and a delete leaves no `.orig` behind. `--json` reports these under `kept`, separately from `deletions`, and a kept file is not counted as an available update, so the scheduled workflow does not open a PR for it.

A hook the source removes goes as a whole: the script, its entry in `.github/hooks/copilot-hooks.json` (repo) or its `~/.copilot/hooks/<name>.json` (user), and its record in the state file. If the script or its entry changed here, the hook stays, and sync warns on stderr that it is still active and names the file.

The same rule governs `nav-pilot uninstall`: it removes the files nav-pilot installed and still owns, leaves the ones that differ, and says how many. `nav-pilot uninstall --force` removes those too.

## Overrides

A team that deliberately maintains its own version of a file can mark it as an override. Overridden files are skipped during sync, with no hash comparison and no PR diff, and you can safely delete them from your repo without them being re-added. This works for both state-based and auto-detected repos.

Create `.github/copilot-sync.json` in your repo:

```json
{
  "overrides": [
    ".github/agents/nav-pilot.agent.md",
    ".github/instructions/security.instructions.md",
    ".github/skills/api-design/"
  ]
}
```

> **Important:** Sync only touches files whose names also exist in the source repo. In a repo that used `nav-pilot install`, a same-named file your team had before the install is never taken over (see above). In a classic repo with manually copied files, sync sees a hash mismatch and proposes overwriting it. Add it to `overrides` to protect your version. Files with names that don't exist in the source are never affected by sync.

Overrides are also how you opt out of framework-specific files. Teams on Astro, Remix, or anything else that isn't Next.js can override the Next.js files the agentpakke installs, such as `.github/instructions/nextjs-aksel.instructions.md`, `.github/instructions/performance.instructions.md` and `.github/prompts/nextjs-api-route.prompt.md`.

> **Tip:** If you need no Next.js files at all, deselect them in the interactive picker instead — see [installing less](README.nav-pilot.md#collections).

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
