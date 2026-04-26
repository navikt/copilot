---
name: readme-review
description: "Strukturell gjennomgang og generering av README-er tilpasset prosjekttype — tjeneste, bibliotek, monorepo eller naisjob"
license: MIT
metadata:
  domain: general
  tags: readme documentation review scaffold
---

# README Review & Scaffold

Structural review and generation of READMEs adapted to project type. Complements `@forfatter` (language quality) with structural guidance (what sections, what order, what depth).

## Workflow

```
Step 0: Detect README scope
Step 1: Route → review or generate
Step 2a: Review existing README
Step 2b: Generate new README
Step 3: Hand off language issues to @forfatter
```

## Step 0: Detect README scope

Before reviewing or generating, determine **what kind of README** this is. Use file location and nearby manifests:

| Scope | Signals | Example |
|-------|---------|---------|
| **Monorepo root** | Root `README.md`, multiple `apps/` or `packages/`, workspace config | `navikt/fp-sak/README.md` with `apps/` dir |
| **Service / API** | `.nais/` dir, `main.go` / `Application.kt`, Dockerfile | `apps/my-service/README.md` |
| **Library / package** | Published to npm/Maven, no `.nais/`, exports public API | `libs/shared-utils/README.md` |
| **Naisjob** | `.nais/naisjob.yaml`, scheduled execution, no HTTP endpoints | `apps/batch-processor/README.md` |
| **Docs-only** | No source code, only markdown files | `docs/README.md` |

If scope is unclear, **ask the user**.

## Step 1: Route

- **Existing README** + user says "review", "check", "improve" → **Step 2a**
- **No README** or user says "create", "scaffold", "generate" → **Step 2b**
- **Ambiguous** → check if `README.md` exists in the target directory

## Step 2a: Review existing README

### 2a.1 Check sections against spec

Read the existing README. For each section in the spec for this project type (see [section-spec.md](./references/section-spec.md)):

| Status | Meaning |
|--------|---------|
| ✅ OK | Section exists with substantive content |
| ⚠️ Weak | Section exists but is thin, outdated, or misplaced |
| ❌ Missing | Required section is absent |
| — | Section not applicable for this project type |

### 2a.2 Check for anti-patterns

Scan for structural anti-patterns (see [anti-patterns.md](./references/anti-patterns.md)). Flag only patterns that are actually present.

### 2a.3 Output

```markdown
## README review — {project name}

**Scope:** {service / library / monorepo / naisjob}

### Section check

| Section | Status | Notes |
|---------|--------|-------|
| Title + one-liner | ✅ | — |
| Quick start | ❌ | No runnable commands found |
| ... | ... | ... |

### Anti-patterns found

- **{Pattern name}**: {One-sentence description of what's wrong and how to fix it}

### Top 3 fixes

1. {Most impactful fix}
2. {Second fix}
3. {Third fix}
```

Surface **top 3 fixes** ordered by impact. Don't overwhelm with minor issues.

## Step 2b: Generate new README

1. Pick template from `references/` based on detected scope
2. Fill in what you can detect from the codebase:
   - Project name (from directory or manifest)
   - Tech stack (from `go.mod`, `package.json`, `build.gradle.kts`)
   - Build/test commands (from `Makefile`, `.mise.toml`, `package.json` scripts)
   - Endpoints (from route definitions)
   - Config (from environment variable usage)
3. Mark remaining placeholders with `{TODO: description}`
4. Output the draft README

Templates: [service](./references/template-service.md) · [library](./references/template-library.md) · [monorepo](./references/template-monorepo.md) · [naisjob](./references/template-naisjob.md)

## Step 3: Language handoff

After structural review or generation, if the text has language issues (AI markers, passive voice, anglicisms), suggest:

> For language polish, use `@forfatter` or the `norwegian-text` instruction (auto-applies to `*.md` files).

Do not duplicate `@forfatter`'s work. This skill handles **structure**; `@forfatter` handles **language**.

## Principles

### Cognitive funneling

Structure README from broad to specific. Readers scan top-down and bail when they have enough information:

1. **Title + one-liner** — what is this? (< 120 characters)
2. **Quick start** — how do I run it? (copy-paste commands)
3. **Details** — API, config, architecture
4. **Meta** — contributing, license, team

> "Your documentation is complete when someone can use your module without ever having to look at its code." — Ken Williams

### README is the front door

README should orient and get people started. Deep content belongs elsewhere:

| In README | In external docs |
|-----------|-----------------|
| One-liner description | Full architecture docs |
| Quick start commands | Detailed runbooks |
| Config table (env vars) | ADRs, threat models |
| API overview or link | Full OpenAPI spec |
| Team + Slack channel | Incident response procedures |

### Code over prose

```markdown
# ❌ Prose
You can start the development server by running the development command
using the mise task runner.

# ✅ Code
mise dev
```

### Fewer sections = less drift

Only include sections you will maintain. An empty "## Roadmap" is worse than no roadmap section. README rots faster than code.

## Boundaries

### ✅ Always

- Detect project scope before suggesting sections
- Check for missing quick start (most common gap)
- Use Missing/Weak/OK status, not numeric scores
- Surface top 3 fixes, not an exhaustive list

### ⚠️ Ask first

- Rewriting large sections of an existing README
- Removing sections that may have historical context
- Changing README language (Norwegian ↔ English)

### 🚫 Never

- Duplicate content from AGENTS.md into README
- Dump full runbooks, security policies, or ADRs into README
- Add numeric scores or gamified ratings
- Rewrite prose style (that's `@forfatter`'s job)

## Related

| Resource | Use for |
|----------|---------|
| `@forfatter` | Language quality — klarspråk, AI markers, anglicisms |
| `norwegian-text.instructions.md` | Auto-applied Norwegian text rules for `*.md` |
| `nav-architecture-review` skill | ADR generation (link from README, don't inline) |
| `mcp-onboarding` | Agent readiness assessment and AGENTS.md generation |

## Sources

- [Art of README](https://github.com/noffle/art-of-readme) — cognitive funneling
- [Standard Readme](https://github.com/RichardLitt/standard-readme) — section ordering spec
- [Make a README](https://www.makeareadme.com) — practical guide
- [Diátaxis](https://diataxis.fr) — documentation framework (tutorials/how-to/reference/explanation)
- [Readme Driven Development](https://tom.preston-werner.com/2010/08/23/readme-driven-development.html) — write README first
