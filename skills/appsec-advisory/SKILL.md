---
name: appsec-advisory
description: Rådgivende sikkerhetsgjennomgang av repoer og PR-er med etterprøvbare funn, vurdert eksponering og forslag til trygg testing. Bruk når teamet vil ha en dypere sikkerhetsvurdering eller prioritering av funn, ikke for sjekk før commit (bruk security-review) eller pentest
license: MIT
metadata:
  domain: auth
  tags: security appsec code-review pentest-planning
---

# AppSec advisory

Use this skill for a scoped security review of a repo or PR, with prioritized findings and safe verification advice. It is a **read-only source review**, not a penetration test or certification. Use `$security-review` for pre-commit scans, `$security-owasp` for vulnerability patterns, and `$threat-model` or `@security-champion-agent` for architecture and trust boundaries.

## Scope and evidence

1. Identify repo, revision, scope and exclusions; read repo instructions and resolve the diff base. For worktree reviews include committed, staged, unstaged and untracked changes, reading untracked files directly. Never work around denied access.
2. Map relevant entry points, data flows, assets and trust boundaries. Trace candidates through callers, auth, ownership, dependencies and deployment controls. Check defaults and effective policies, not only individual fragments. Verify third-party behavior against the versioned dependency or mark it unverified.
3. Run useful read-only checks and record commands, results and scope. Existing GitHub alerts (Dependabot, code scanning, secret scanning) read through `gh api` are cheap evidence. Do not install tools, start services, contact targets, read credentials, dump secrets or run active probes. Do not create files, even in the session workspace, unless requested. Record tools needing writes or unavailable tools as unperformed, not clean scans.
4. Try to disprove each candidate: attacker control, reachability, upstream auth, deny policies, compensating controls and plausible impact. Check benign paths. Drop unsupported claims; state unresolved ones as questions with a way to settle them.
5. For a PR, report only issues the change introduces or makes reachable; list existing issues separately as context. Check git history for reverted or previously fixed code, and treat changed security code without tests as higher risk.
6. Prioritize by plausible impact and exposure, not scanner labels. Separate **observed code defect**, **conditional risk** and **verified exploit**: source review cannot establish the last. Rate code evidence and deployment exposure separately; do not infer reachability from an egress-only policy or a single config fragment.

## Nais exposure

Assess deployment exposure from the app's `.nais/` manifests for each environment; use `$nais` and `$nav-auth` for details.

- **Ingress:** no ingress means cluster-internal only. `*.intern.nav.no` and `*.intern.dev.nav.no` are internal; `*.nav.no`, `*.dev.nav.no` and `*.ansatt.nav.no` are reachable from the internet, so authentication decides who gets in.
- **Inbound callers:** `accessPolicy.inbound.rules` lists which apps may call the service and, with Azure AD or TokenX, obtain tokens for it. Check that the code's token validation matches those rules.
- **Login:** with an ID-porten or Azure AD sidecar, `autoLogin` enforces login only outside `autoLoginIgnorePaths`; the application must validate tokens on every path it relies on. `azure.application.allowAllUsers: true` admits every Nav employee unless the code checks groups or roles.
- **Logs:** personal data belongs in team logs, not ordinary application logs.

## Severity and exclusions

Severity combines impact and exposure:

- **CRITICAL:** unauthenticated, internet-reachable path to personal data, special categories of personal data, credentials or code execution.
- **HIGH:** authentication or authorization bypass, or personal data (such as fødselsnummer) exposed in responses or ordinary logs.
- **MEDIUM:** impact that requires specific conditions, an authenticated or internal caller, or a second defect.
- **LOW:** defense in depth with no demonstrated path to impact.

Do not report a finding with code confidence below 7; state it as a question instead. Do not report as findings: denial of service, resource exhaustion or missing rate limiting; missing input validation without a demonstrated security consequence; attacks that need capabilities the attacker would not plausibly have, such as write access to the deployed filesystem or the repository; generic hardening already handled by the platform. Mention them under not inspected or as questions when they matter to the team.

Reject these rationalizations:

| Thought | Required action |
|---|---|
| "This pattern looks dangerous" | Trace the data from source to sink before reporting |
| "This is clearly critical" | Models overrate severity; justify impact and exposure with evidence |
| "A race or edge case exists" | Show that the attacker capability it needs is realistic |
| "Similar code was vulnerable elsewhere" | Verify this instance and its callers independently |
| "The scanner flagged it" | Confirm reachability; a scanner label is not a finding |

## Team handoff

For each actionable item give:

- severity, status (observed defect or conditional risk), and separate confidence (1–10 or unknown) for code and deployment
- exact file:line, entry point and controls, distinguishing code, configuration, scanner results and inference
- attack preconditions and impact, counterevidence, unknowns, smallest mitigation and owner
- a non-destructive check with prerequisites (including accounts or roles), expected secure and risky results, and stop conditions, only in an environment the team owns or is authorized to assess; never execute it as part of this skill

Require human review before relying on security-critical severity or applying a fix; never include sensitive values in the report.

Example finding:

```
HIGH · observed defect · code 8/10 · deployment 7/10
src/main/kotlin/.../VedtakRoutes.kt:57 — GET /api/vedtak/{id} loads the vedtak by id
and returns it without checking that it belongs to the fødselsnummer in the token.
Entry point: ingress myapp.nav.no, ID-porten sidecar with autoLogin (Level4).
Preconditions: a logged-in citizen who knows or guesses another vedtak id.
Counterevidence: ids are sequential longs; no ownership filter in VedtakRepository.
Unknowns: whether an API gateway in front adds an ownership check (not in repo).
Mitigation: query by id and the token's fødselsnummer; return 404 otherwise. Owner: app team.
Check (dev only, two test users): user A requests user B's vedtak id; secure = 404,
risky = 200. Stop after the first response.
```

For an independent check, hand off only one testable claim and the primary evidence, not your conclusion or reasoning:

```
Revision:      <commit SHA and repository>
Claim:         <specific actor, entry point, missing control and possible impact>
Evidence:      <file:line references, including the relevant control or dependency>
Preconditions: <what must be true for the impact to follow>
Unknowns:      <evidence not available in this review>
```

The checker should stay read-only, seek counterevidence in the same revision and mark each part supported, contradicted or unknown, including controls that only narrow the claim. Rate code and deployment separately, using unknown where evidence is lacking. Show both assessments side by side; agreement without new primary evidence does not verify exploitation. If the checker narrows a claim, correct the original finding instead of repeating its broader wording. End with evidence still needed, not a default fix/commit menu.

## Review contract

For each relevant axis (entry points and trust boundaries, auth and ownership, injection and data, dependencies, deployment, sensitive output), report a finding or what was inspected and what it does not prove. Do not manufacture findings.

```
Inspected:     <specific revision, files, configurations, commands>
Not inspected: <in-scope evidence missing and reason>
Findings:      <confirmed code defects, conditional risks, unresolved questions>
Verdict:       BLOCK | CONCERNS | CLEAN | INCONCLUSIVE
```

`BLOCK`: supported risk of security or privacy breach if shipped. `CONCERNS`: verification or mitigation needed. `CLEAN`: no supported findings in the inspected scope, not a complete audit. `INCONCLUSIVE`: inaccessible or insufficient scope, or unresolved material disagreement; state what would settle it. List inspected and missing files, revisions, configs and tools with reasons; report failures, not passes. Keep unverified hypotheses outside the finding count.

Example of a clean result:

```
Inspected:     abc1234; src/main/kotlin/**/auth/*, .nais/prod.yaml, .nais/dev.yaml;
               gh api code-scanning and Dependabot alerts (0 open)
Not inspected: runtime config injected by the platform; reverse proxy rules (not in repo)
Findings:      none; question: is /internal/metrics reachable through the ingress?
Verdict:       CLEAN (inspected scope only)
```