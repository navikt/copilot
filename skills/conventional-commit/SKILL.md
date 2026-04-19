---
name: conventional-commit
description: Generer conventional commit-meldinger med Nav-relevante scopes og breaking change-format
license: MIT
metadata:
  domain: general
  tags: git commit conventional-commits changelog
---

# Conventional Commit Skill

Generate commit messages following the Conventional Commits specification, adapted for Nav projects.

## Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

## Types

| Type | Usage |
|---|---|
| `feat` | New functionality |
| `fix` | Bug fix |
| `docs` | Documentation-only changes |
| `style` | Formatting, semicolons, etc. (no code change) |
| `refactor` | Code that neither fixes a bug nor adds a feature |
| `perf` | Performance changes |
| `test` | Adding or fixing tests |
| `build` | Build system or dependency changes |
| `ci` | CI configuration changes |
| `chore` | Other changes that don't affect code |

## Nav-relevante scopes

```
feat(vedtak): legg til støtte for klagevedtak
fix(auth): fiks token-validering for TokenX
docs(api): oppdater OpenAPI-spec for vedtak-endepunktet
refactor(repository): bruk CTE for bedre lesbarhet
test(controller): legg til integrasjonstest med MockOAuth2Server
build(deps): oppgrader Spring Boot til 3.4.1
ci(deploy): legg til prod-deploy steg
perf(db): legg til indeks på bruker_id
chore(nais): oppdater ressursgrenser
```

## Breaking Changes

```
feat(api)!: endre responsformat for vedtak-endepunktet

BREAKING CHANGE: Feltet `vedtakDato` er endret til `opprettetDato`.
Konsumenter må oppdatere sin parsing.
```

## Rules

- First line: max 72 characters
- Use imperative form: "add", not "added" or "adds"
- Don't end with a period
- Use Norwegian or English consistently within the project
- Reference Jira/GitHub issue in footer: `Closes #123` or `Refs NAV-1234`

## Examples

```bash
# Enkel feature
git commit -m "feat(søknad): legg til validering av fødselsnummer"

# Bugfix med referanse
git commit -m "fix(auth): håndter utløpt refresh-token

Refresh-tokenet ble ikke fornyet ved utløp, som førte til
at brukere ble logget ut uten varsel.

Fixes #456"

# Dependency-oppdatering
git commit -m "build(deps): oppgrader postgresql driver til 42.7.4"

# Breaking change
git commit -m "feat(api)!: fjern deprecated /api/v1/vedtak endepunkt

BREAKING CHANGE: /api/v1/vedtak er fjernet. Bruk /api/v2/vedtak."
```

## Analyzing Staged Changes

To generate a commit message, analyze staged changes:

```bash
git diff --cached --stat        # Overview of changed files
git diff --cached               # Detailed diff
```

Based on the diff:
1. Identify **type** (feat/fix/refactor/etc.)
2. Identify **scope** (which module/domain)
3. Write short, precise description
4. Add body if the change needs explanation
5. Add `BREAKING CHANGE` footer if the API changes
