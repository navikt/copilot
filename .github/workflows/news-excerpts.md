---
on:
  schedule: weekly
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

safe-outputs:
  create-pull-request:
    title-prefix: "[news] "
    labels: [news, automated]
    draft: true
    allowed-files:
      - docs/news/articles/**
---

# AI Coding News Excerpts

Scan the GitHub Blog changelog for new Copilot-related announcements, create excerpt files, and maintain a monthly draft summary in `docs/news/articles/`.

## What to do

1. Read the existing excerpt files in `docs/news/articles/` and find the most recent `date:` value in any frontmatter. This is the **cutoff date** — only create excerpts for announcements published AFTER this date.
2. Read `docs/news/articles/.newsignore` (if it exists). Each line is a URL to skip — never create excerpts for these.
3. Fetch `https://github.blog/changelog/` and find announcements tagged `copilot` that are newer than the cutoff date.
4. For each new announcement that is not in `.newsignore` and does NOT already have an excerpt file, create one.
5. After creating excerpt files, update or create the monthly draft summary (see below).

## Excerpt file format

Each excerpt is a markdown file with ONLY YAML frontmatter (no body content). Follow this exact format:

```markdown
---
title: "Title in Norwegian (bokmål)"
date: YYYY-MM-DD
category: copilot
excerpt: "One-sentence Norwegian summary of the announcement."
url: "https://github.blog/changelog/..."
tags:
  - relevant-tag
---
```

## Rules

- **Language**: Title and excerpt in Norwegian (bokmål). Use English tech terms where developers do (e.g. "MCP", "PR", "GA", "public preview").
- **Filename**: Derive from the URL slug, e.g. `figma-mcp-server.md`, `vscode-v1-110.md`. Keep it short and descriptive.
- **No duplicates**: If an excerpt file already exists for an announcement (check by URL in frontmatter), skip it.
- **Ignore list**: If the URL is listed in `docs/news/articles/.newsignore`, skip it.
- **Only Copilot-related**: Skip announcements not related to GitHub Copilot, AI coding, or developer tools.
- **Tags**: Use lowercase kebab-case. Reuse existing tags from other excerpt files when applicable.
- **Date**: Use the announcement date from the changelog, not today's date.
- **No changes if nothing new**: If there are no new announcements after the cutoff, do not create any files.

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

- Write in Norwegian (bokmål). Use English tech terms where developers do.
- Each section: 2-4 paragraphs with enough detail to be useful, not just a restatement of the excerpt.
- Fetch the full blog post URL to write an informed summary — do not just paraphrase the title.
- The "Relevans for Nav" table should consider: ~500 tech professionals, Nais platform (Kubernetes/GCP), Kotlin/Ktor, Next.js, strong security/privacy requirements, DORA/SPACE metrics.
- Keep `draft: true` in frontmatter — this will be reviewed and published manually.
