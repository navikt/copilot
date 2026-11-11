---
name: repo-baseline-audit
description: Sjekk om et GitHub-repo i navikt følger grunnkravene og beste praksis fra navikt/offentlig, Golden Path og sikkerhetsplaybooken (sikkerhet.nav.no), og vanlig GitHub-hygiene som .gitignore og innsjekkede env-filer. Bruk når teamet vil vite hva som mangler i oppsettet av et repo, ikke for å lete etter sårbarheter i koden (bruk appsec-advisory) eller sjekke en endring før commit (bruk security-review)
license: MIT
metadata:
  domain: general
  tags: security github golden-path repo-hygiene compliance
---

# Repo baseline audit

Audit one repository against Nav's baseline and return a checklist report: what
is met, what is missing, what could not be verified, and the smallest fix for
each gap. This is a read-only configuration and practice audit, not a hunt for
vulnerabilities in application logic.

## Establish scope

1. Resolve the target: the current git checkout by default, plus its
   `owner/repo` slug for GitHub settings. Work from the repository root.
2. Detect what the repository contains — ecosystems and lockfiles,
   `Dockerfile`, `.nais/` or `nais.yaml`, workflows, frontend code. A rule
   applies only when its *Applies when* condition holds; otherwise it is `N/A`.
3. Note visibility. Public repositories have stricter README contact
   requirements; non-public repositories must justify being closed.

## Collect evidence

Load [the rule catalogue](references/checks.md) and
[the evidence commands](references/evidence.md). Each rule names the evidence
`[section]` it needs. Collect those sections from the repository root. The
commands never change the repository or its settings, run in bash, zsh and
PowerShell, and never print secret values. A 403 or 404 from GitHub, or a
missing tool, makes the rules that depend on it `Not verifiable`.

Then assess every applicable rule. For rules marked *judgment*, sample the
code directly and cite `file:line`.

## Judge the evidence

- **A scanner hit is a lead, not a verdict.** Apply the playbook's own
  exceptions before reporting. GitHub-owned actions (`actions/*`, `github/*`)
  may use tags, so zizmor's `unpinned-uses` on them is not a gap.
- **Missing access means `Not verifiable`, never `Gap`.** Branch protection,
  collaborators, deploy keys and alert lists often need admin rights.
- **Triage secret hits.** Public client keys, test fixtures and examples are
  common false positives. Report rule, file and short commit SHA only, never the
  value. Treat anything plausibly real as compromised: it is a `Must` gap whose
  fix is rotation, because history rewrites do not undo exposure.
- **Use the effective setting.** Org defaults, ecosystem defaults
  (`pnpm`, Nais) and settings inherited from rulesets count as met when the
  evidence shows they apply.
- **Leave out rules that do not exist.** A per-repo `SECURITY.md` is not
  required; the org-wide policy in `navikt/.github` covers every repository.

Developer-machine settings belong to `workstation-security`. Process
practices (threat modelling, Trygg nok, pentesting, incident readiness) cannot
be proven from a repository; list them once under *Ask the team*.

## Statuses

| Status | Meaning |
|---|---|
| `Met` | Evidence shows the rule is satisfied |
| `Partly` | Satisfied in some places or by an equivalent the source does not name |
| `Gap` | Evidence shows the rule is not satisfied |
| `Not verifiable` | The evidence needed was not accessible; say what would settle it |
| `N/A` | The rule's *Applies when* condition does not hold |

## Report

Write the report in the user's language. Order gaps by tier (`Must` before
`Should` before `Recommended`), then by how easy they are to fix.

```
Repository:  navikt/example-app @ 1a2b3c4 (public, default branch main)
Collected:   zizmor 1.31; gitleaks 8.30; gh as <user> (no admin)
Summary:     Must 1 gap · Should 3 gaps, 1 partly · Recommended 2 gaps · 3 not verifiable

Gaps
| Rule | Tier | Evidence | Fix |
|---|---|---|---|
| REQ-CODEOWNERS | Must | CODEOWNERS:1 `* @jdoe` | `* @navikt/<team>` |
| GP-GITHUB-PIN | Should | build.yml:24 `nais/docker-build-push@main` | Pin to SHA with `# vX` comment; `gh act pin` |
| HYG-GITIGNORE | Recommended | `.env` not ignored | Add `.env` and `.env.*`, keep `!.env.example` |

Partly:          GP-DEPENDABOT — npm has cooldown, github-actions does not
Not verifiable:  PB-BRANCH-PROTECTION — classic protection needs admin; no rulesets visible
Met:             REQ-LICENSE, REQ-README, PB-SECRET-SCANNING, …
N/A:             GP-DISTROLESS (no Dockerfile)
Ask the team:    threat model, Trygg nok, incident plan
```

Link each fix to the rule's source. Make changes only when the user asks for
fixes after the report; repository settings changes need someone with admin
rights.
