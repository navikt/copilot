---
on:
  schedule: "0 9 * * 1,3,5"
  workflow_dispatch:

engine:
  id: copilot
  model: claude-opus-4.6

permissions:
  contents: read

network:
  allowed:
    - defaults
    - github
    - "github.blog"
    - "openai.com"
    - "anthropic.com"
    - "blog.google"
    - "ai.google.dev"
    - "code.visualstudio.com"
    - "devblogs.microsoft.com"

safe-outputs:
  create-pull-request:
    title-prefix: "[news] "
    labels: [news, automated]
    draft: true
    allowed-files:
      - docs/news/articles/**
---

# AI Coding News Excerpts

Scan multiple sources for AI coding news relevant to Nav developers, create excerpt files for noteworthy announcements, and maintain a monthly draft summary in `docs/news/articles/`.

## What to do

1. Read the existing excerpt files in `docs/news/articles/` and find the most recent `date:` value in any frontmatter. This is the **cutoff date** — only create excerpts for announcements published AFTER this date.
2. Read `docs/news/articles/.newsignore` (if it exists). Each line is a URL to skip — never create excerpts for these.
3. Scan sources (see "Sources to scan" below) for announcements newer than the cutoff date.
4. Apply the newsworthiness criteria (see "Newsworthiness" below) to decide which items deserve an excerpt.
5. For each newsworthy announcement that is not in `.newsignore` and does NOT already have an excerpt file, create one.
6. After creating excerpt files, update or create the monthly draft summary (see below).

## Sources to scan

Scan these sources in order of priority:

| Priority | Source | URL | What to look for |
| --- | --- | --- | --- |
| 1 | GitHub Changelog | `https://github.blog/changelog/label/copilot/` | Official Copilot feature launches, deprecations, policy changes |
| 2 | GitHub Blog | `https://github.blog/` | Deep-dives, architecture posts, usage data, strategy changes |
| 3 | VS Code release notes | `https://code.visualstudio.com/updates/` | Copilot features in new VS Code releases |
| 4 | OpenAI announcements | `https://openai.com/index/` | New models available in Copilot, API changes, Codex updates |
| 5 | Anthropic announcements | `https://anthropic.com/news` | Claude model updates, agentic coding research, trend reports |
| 6 | Google AI announcements | `https://blog.google/technology/developers/` | Gemini/Gemma models in Copilot, open-source model releases |

Only create excerpts for items that directly impact developers using GitHub Copilot or AI coding tools. Skip pure marketing, pricing-only announcements, and items unrelated to coding workflows.

## Newsworthiness

Not every changelog entry deserves an excerpt. Apply these criteria:

### ✅ Create an excerpt when

- **New capability**: a feature that changes how developers work (e.g. agent mode, code review, new tool)
- **Breaking change**: deprecation, API sunset, policy change that requires action
- **New model**: a model becomes available (or is removed) in Copilot
- **Enterprise governance**: changes to admin controls, security, compliance, or audit features
- **Platform milestone**: significant adoption numbers, architectural shifts, or ecosystem changes (e.g. MCP standard adoption)
- **Agentic workflow shift**: changes to coding agent, cloud agent, agent hooks, agent skills, or AGENTS.md support

### ⚠️ Consider carefully (only if substantial)

- IDE-specific updates — only if they introduce a fundamentally new capability, not incremental polish
- Performance improvements — only if quantified and significant (e.g. "50% faster", not "improved performance")
- Competing platform news — only if it directly impacts the Copilot ecosystem or represents an industry shift

### 🚫 Skip

- Minor UI tweaks, icon changes, tooltip updates
- Bug fixes without broader implications
- Announcements that repeat information from a previous excerpt
- Features limited to platforms Nav does not use (e.g. Xcode-only, Eclipse-only)
- Pure pricing or plan changes with no feature implications
- Marketing blog posts without concrete technical content

## Excerpt file format

Each excerpt is a markdown file with ONLY YAML frontmatter (no body content). Follow this exact format:

```markdown
---
title: "Title in Norwegian (bokmål)"
date: YYYY-MM-DD
category: copilot
excerpt: "One-sentence Norwegian summary of the announcement."
url: "https://..."
tags:
  - relevant-tag
---
```

### Category guidelines

- `copilot` — GitHub Copilot features, models, and platform changes
- `praksis` — Broader AI coding trends, competing tools, industry reports, open-source models

## Rules

- **Language**: Title and excerpt in Norwegian (bokmål). Use English tech terms where developers do (e.g. "MCP", "PR", "GA", "public preview").
- **Filename**: Derive from the URL slug, e.g. `figma-mcp-server.md`, `vscode-v1-110.md`. Keep it short and descriptive.
- **No duplicates**: If an excerpt file already exists for an announcement (check by URL in frontmatter), skip it.
- **Ignore list**: If the URL is listed in `docs/news/articles/.newsignore`, skip it.
- **Tags**: Use lowercase kebab-case. Reuse existing tags from other excerpt files when applicable.
- **Date**: Use the announcement date from the source, not today's date.
- **No changes if nothing new**: If there are no newsworthy announcements after the cutoff, do not create any files. Call `noop` with a message like "No new newsworthy announcements found since [cutoff date]. Scanned all sources." to confirm the scan completed successfully.

## Scope

Only create or modify files under `docs/news/articles/`. Do not modify any other files.

## Monthly draft summary

After creating new excerpt files, update (or create) a monthly draft summary for the current month. The filename pattern is the Norwegian month name + year, e.g. `mars-2026.md`, `april-2026.md`.

### If the monthly file already exists

- Read the existing file to understand the current numbered sections and structure.
- Add new numbered sections for each new excerpt, continuing from the last section number.
- Update the frontmatter `excerpt:` string to include the new topics.
- Add rows to the "Relevans for Nav" table at the end for each new section.
- Do NOT rewrite or modify existing sections — only append new ones.

### If the monthly file does not exist

Create it with this structure:

```markdown
---
title: "Nyheter og trender — [Month in Norwegian] [Year]"
date: YYYY-MM-DD
draft: true
category: copilot
excerpt: "Comma-separated summary of all topics."
tags:
  - relevant-tags
---

[One-paragraph Norwegian introduction summarizing the month's themes.]

---

## 1. [Topic title]

[2-4 paragraph Norwegian summary of the announcement. Fetch the full blog post URL to get enough detail.]

**Kilde:** [English title](url) (GitHub Changelog, date)

---

[...more numbered sections...]

---

## Relevans for Nav

| Trend | Hva det betyr for Nav |
| ----- | --------------------- |
| ...   | ...                   |
```

### Monthly summary rules

- Write in Norwegian (bokmål), direct tone, short sentences. Use English tech terms where developers do (e.g. "public preview", "PR", "repo").
- Each section: 2-4 paragraphs with enough detail to be useful, not just a restatement of the excerpt.
- Fetch the full blog post URL to write an informed summary — do not just paraphrase the title.
- The "Relevans for Nav" table should consider: ~500 tech professionals, Nais platform (Kubernetes/GCP), Kotlin/Ktor, Next.js, strong security/privacy requirements, DORA/SPACE metrics.
- Keep `draft: true` in frontmatter — this will be reviewed and published manually.
