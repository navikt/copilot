# Structural Anti-Patterns in READMEs

Checklist for reviewing README structure. These are **structural** issues — for language quality (AI markers, klarspråk, anglicisms), use `@forfatter`.

## Anti-Pattern Checklist

### 1. Template cargo-culting

**Signal:** Empty sections, placeholder text ("TODO", "Add description here"), or sections copied from a template that don't apply to this project.

**Fix:** Remove sections with no content. Only include sections you have content for today.

```markdown
# ❌ Empty section
## FAQ

## Roadmap

## Changelog

# ✅ Remove until you have content
(section deleted)
```

### 2. Zombie sections

**Signal:** Section heading exists but contains fewer than 2 substantive lines.

**Fix:** Either fill with real content or remove the heading entirely.

```markdown
# ❌ Zombie
## Contributing

See CONTRIBUTING.md.

# ✅ Substantive
## Contributing

1. Fork og klon repoet
2. Kjør `mise install`
3. Lag branch fra `main`, kjør `mise check` før PR
```

### 3. Over-structuring

**Signal:** More than 10 H2 headings for a project with fewer than 500 lines of code, or deeply nested headings (H4+) in a README.

**Fix:** Collapse related sections. Move deep content to `docs/`. Match README depth to project size.

| Project size | Reasonable H2 count |
|---|---|
| Small CLI tool (< 200 LOC) | 3–5 |
| Standard service | 6–10 |
| Monorepo root | 8–12 |
| Large platform | 10–15 (with ToC) |

### 4. Depth mismatch

**Signal:** Has detailed API documentation or architecture diagrams but no quick-start section. Reader cannot run the project without reading deep docs.

**Fix:** Add quick start. The first thing after the description should be "how do I run this?"

```markdown
# ❌ Depth mismatch
## Architecture
(500 words about hexagonal architecture)

## API Reference
(full endpoint documentation)

# But no "Quick start" section exists

# ✅ Fix: Add quick start before deep sections
## Quick start
mise dev    # starts on :8080
```

### 5. Context collapse

**Signal:** README tries to address developers, operators, end-users, and contributors all at once. Mixed "you" referring to different audiences.

**Fix:** Pick the primary audience (usually: developer who joins the team). Link to other docs for other audiences.

```markdown
# ❌ Context collapse
This service processes pension claims. Users can submit claims via
the web portal. Developers should run `./gradlew build`. Operators
can check the Grafana dashboard.

# ✅ Pick primary audience (developer)
Behandler pensjonssøknader. Se [brukerveiledning](docs/user-guide.md)
for innbygger-perspektivet.

## Kom i gang
./gradlew build
./gradlew run
```

### 6. Badge wall

**Signal:** More than 5 badges at the top of the README.

**Fix:** Keep 2–4 badges max. CI status and license are most useful. Remove decorative badges.

```markdown
# ❌ Badge wall (8 badges)
[![CI](...)][...] [![Coverage](...)][...] [![License](...)][...]
[![Downloads](...)][...] [![Contributors](...)][...] [![Stars](...)][...]
[![Last Commit](...)][...] [![Code Size](...)][...]

# ✅ Keep essential badges
[![CI](...)][...] [![License: MIT](...)][...]
```

### 7. README bloat

**Signal:** README exceeds 500 lines without linking to external documentation. High word count with low information density.

**Fix:** Move deep content to `docs/` and link from README. README is the front door, not the house.

| Content type | Where it belongs |
|---|---|
| Quick start, overview | README |
| Full API reference | `docs/api.md` or OpenAPI spec |
| Architecture decisions | `docs/adr/` |
| Runbooks, incident response | `docs/runbook.md` |
| Threat model | `docs/security/` |
| Detailed configuration guide | `docs/config.md` |

### 8. Stale examples

**Signal:** Code samples reference APIs, functions, or configuration that no longer exist in the codebase.

**Fix:** Verify all code examples compile and run against the current version. Consider extracting examples into runnable files (`examples/`).

### 9. Happy-path only

**Signal:** README shows how to set up and run the project but never mentions what can go wrong, prerequisites, or common errors.

**Fix:** Add a "Prerequisites" section listing required tools/versions, and consider a brief "Troubleshooting" section for common issues.

```markdown
# ❌ Happy-path only
## Quick start
mise dev

# ✅ Include prerequisites and gotchas
## Quick start

**Prerequisites:** mise ≥ 2024.1, Docker, access to navikt org

mise install
mise dev    # starts on :8080

> **Obs:** Krever VPN for tilgang til interne tjenester.
```

### 10. Aspirational docs

**Signal:** README documents features that are planned but not yet implemented, without clearly labeling them as future work.

**Fix:** Only document what exists today. If you include planned features, label them explicitly.

```markdown
# ❌ Aspirational (feature doesn't exist)
## GraphQL API
Query vedtak using our GraphQL endpoint at `/graphql`.

# ✅ If you must mention future plans
## Planned: GraphQL API
> 🚧 Under utvikling. Forventet Q3 2026.
```

### 11. AGENTS.md duplication

**Signal:** README repeats build commands, code standards, or boundaries that are already documented in AGENTS.md.

**Fix:** README is for humans. AGENTS.md is for AI agents. Don't duplicate:

- Build/test commands → keep in both (humans need them too)
- Code standards → AGENTS.md only (agents follow these, humans read PR reviews)
- Boundaries → AGENTS.md only

## Review Output Format

When reviewing, report findings as:

```markdown
### Anti-patterns found

- **Zombie sections**: "## FAQ" and "## Changelog" have no content — remove or fill
- **Depth mismatch**: Detailed API docs but no quick start — add "Kom i gang" section
- **Happy-path only**: No prerequisites listed — add mise version and Docker requirement
```

Only report anti-patterns that are **actually present**. Don't list the entire checklist.
