# copilot-adoption

Naisjob that scans all repositories in the `navikt` GitHub organization for AI coding tool customization files and writes daily snapshots to BigQuery.

## What It Scans

The scanner checks every non-archived repository for **16 customization categories** across 4 AI coding tools:

### GitHub Copilot (8 categories)

| Category               | Path                                     | Type      | Detail Level            |
| ---------------------- | ---------------------------------------- | --------- | ----------------------- |
| `copilot_instructions` | `.github/copilot-instructions.md`        | file      | exists only             |
| `agents_md`            | `AGENTS.md`                              | file      | exists only             |
| `agents`               | `.github/agents/*.agent.md`              | directory | individual filenames    |
| `instructions`         | `.github/instructions/*.instructions.md` | directory | individual filenames    |
| `prompts`              | `.github/prompts/*.prompt.md`            | directory | individual filenames    |
| `skills`               | `.github/skills/*`                       | directory | individual folder names |
| `mcp_config`           | `.vscode/mcp.json`                       | file      | exists only             |
| `copilot_dir`          | `.github/copilot/*`                      | directory | individual filenames    |

### Cursor (3 categories)

| Category           | Path                  | Type      | Detail Level         |
| ------------------ | --------------------- | --------- | -------------------- |
| `cursorrules`      | `.cursorrules`        | file      | exists only          |
| `cursor_rules_dir` | `.cursor/rules/*.mdc` | directory | individual filenames |
| `cursorignore`     | `.cursorignore`       | file      | exists only          |

### Claude Code (2 categories)

| Category          | Path                    | Type | Detail Level |
| ----------------- | ----------------------- | ---- | ------------ |
| `claude_md`       | `CLAUDE.md`             | file | exists only  |
| `claude_settings` | `.claude/settings.json` | file | exists only  |

### Windsurf (1 category)

| Category        | Path             | Type | Detail Level |
| --------------- | ---------------- | ---- | ------------ |
| `windsurfrules` | `.windsurfrules` | file | exists only  |

> To add a new category, append an entry to `DefaultCriteria()` in [criteria.go](criteria.go) — no other code changes needed.

## Data Model

### BigQuery Table: `repo_scan`

Partitioned by `scan_date`, clustered on `(org, has_any_customization, primary_language)`.

| Column                  | Type              | Description                                |
| ----------------------- | ----------------- | ------------------------------------------ |
| `scan_date`             | DATE              | Date of the scan (partition key)           |
| `org`                   | STRING            | GitHub organization (`navikt`)             |
| `repo`                  | STRING            | Repository name                            |
| `default_branch`        | STRING            | Default branch name                        |
| `primary_language`      | STRING            | Primary programming language               |
| `is_archived`           | BOOLEAN           | Whether the repo is archived               |
| `is_fork`               | BOOLEAN           | Whether the repo is a fork                 |
| `visibility`            | STRING            | `public`, `private`, or `internal`         |
| `created_at`            | TIMESTAMP         | Repo creation time                         |
| `pushed_at`             | TIMESTAMP         | Last push time                             |
| `topics`                | STRING (repeated) | Repository topics                          |
| `teams`                 | JSON              | Teams with access (see below)              |
| `customizations`        | JSON              | Search results per category (see below)    |
| `has_any_customization` | BOOLEAN           | Quick filter: any customization found      |
| `customization_count`   | INTEGER           | Number of distinct categories found (0–16) |
| `loaded_at`             | TIMESTAMP         | When the row was inserted                  |

### JSON Column: `customizations`

```json
{
  "copilot_instructions": { "exists": true },
  "agents": {
    "exists": true,
    "files": ["nais-platform.agent.md", "auth.agent.md", "observability.agent.md"]
  },
  "instructions": {
    "exists": true,
    "files": ["kotlin-ktor.instructions.md", "nextjs-aksel.instructions.md"]
  },
  "skills": {
    "exists": true,
    "files": ["aksel-spacing", "flyway-migration", "kotlin-app-config"]
  },
  "prompts": { "exists": false },
  "cursorrules": { "exists": false },
  "claude_md": { "exists": true }
}
```

- **File checks** (`CheckFile`): `{ "exists": true/false }` — no filename (it's implicit from the path)
- **Directory checks** (`CheckDirectory`): `{ "exists": true/false, "files": ["name1", "name2"] }` — individual filenames/folder names matching the glob pattern

### JSON Column: `teams`

```json
[
  { "slug": "team-platform", "name": "Team Platform", "permission": "admin" },
  { "slug": "security", "name": "Security", "permission": "push" }
]
```

## BigQuery Views

| View                      | Purpose                                                                                      |
| ------------------------- | -------------------------------------------------------------------------------------------- |
| `v_adoption_summary`      | Daily aggregate: total repos, adoption rate, per-category counts, non-Copilot AI tool counts |
| `v_team_adoption`         | Per-team adoption rates and category breakdown                                               |
| `v_language_adoption`     | Per-language adoption rates (languages with ≥5 repos)                                        |
| `v_customization_details` | One row per file per category — for "top 10 most used agents/skills" queries                 |

### Example Queries

**Top 10 most used agents across all repos:**

```sql
SELECT file_name, COUNT(DISTINCT repo) AS repo_count
FROM `copilot_adoption.v_customization_details`
WHERE category = 'agents' AND scan_date = CURRENT_DATE()
GROUP BY file_name
ORDER BY repo_count DESC
LIMIT 10
```

**Top used skills:**

```sql
SELECT file_name, COUNT(DISTINCT repo) AS repo_count
FROM `copilot_adoption.v_customization_details`
WHERE category = 'skills' AND scan_date = CURRENT_DATE()
GROUP BY file_name
ORDER BY repo_count DESC
LIMIT 10
```

**Adoption rate over time:**

```sql
SELECT scan_date, adoption_rate, repos_with_any_customization, active_repos
FROM `copilot_adoption.v_adoption_summary`
ORDER BY scan_date
```

## How It Works

1. **List repos** — REST API fetches all repos in the org with metadata
2. **Build team map** — REST API maps repos to teams with access levels
3. **Split archived/active** — Archived repos get metadata only (no file scan)
4. **Batch GraphQL scan** — Active repos scanned in batches of 3 (configurable, max 10) using `repository.object` queries against the Git tree
5. **Assemble results** — Combine repo metadata, team data, and scan results
6. **Load to BigQuery** — JSONL load job with `WriteTruncate` on partition decorator (`table$YYYYMMDD`) for atomic daily snapshots

## Commands

```bash
mise check    # Run all checks (fmt, vet, staticcheck, lint, test)
mise test     # Run tests
mise dev      # Run locally with .env.local
```

## Configuration

| Variable              | Description                     | Default            |
| --------------------- | ------------------------------- | ------------------ |
| `GCP_TEAM_PROJECT_ID` | GCP project for BigQuery        | (required)         |
| `BIGQUERY_DATASET`    | BigQuery dataset name           | `copilot_adoption` |
| `BIGQUERY_TABLE`      | BigQuery table name             | `repo_scan`        |
| `GITHUB_ORG`          | GitHub organization to scan     | `navikt`           |
| `SLACK_WEBHOOK_URL`   | Slack webhook for notifications | (optional)         |

### CLI Flags

| Flag         | Description              | Default |
| ------------ | ------------------------ | ------- |
| `--run-once` | Run single scan and exit | `false` |
| `--dry-run`  | Skip BigQuery writes     | `false` |
