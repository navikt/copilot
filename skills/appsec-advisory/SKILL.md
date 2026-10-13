---
name: appsec-advisory
description: Rådgivende sikkerhetsgjennomgang av repoer og PR-er med etterprøvbare funn og forslag til trygg testing
license: MIT
metadata:
  domain: auth
  tags: security appsec code-review pentest-planning
---

# AppSec advisory

Use this skill when a team wants a scoped security assessment of its code or a PR, including priorities and a plan for checking suspected flaws. This is a **read-only source review**, not a penetration test or a security certification. For pre-commit scanner checks use `$security-review`; for code-level vulnerability patterns use `$security-owasp`; for architecture and trust boundaries use `$threat-model` or `@security-champion-agent`. Do not turn a request for advice into live testing.

## Scope and evidence

1. Identify the repository, revision, requested paths or diff, and any explicit exclusions. Read repository instructions and resolve the diff base before inspecting changes. Include committed, staged, unstaged and untracked changes when the request covers the worktree; read untracked files directly. If access is denied, stop for that path rather than finding another way in.
2. Map only the relevant entry points, data flows, assets and trust boundaries. Trace each candidate through callers, middleware, authentication, authorization, resource ownership, dependencies and deployment configuration. Inspect defaults and the effective policy where available, not just a configuration fragment. If a claim relies on a third-party package, verify its version and behavior against the checked-out dependency or authoritative versioned source; otherwise state the dependency as unverified.
3. Use repository-native read-only checks or installed scanners where useful, recording the command, result and scope. Do not silently install tools, start services, contact targets, read credential files, dump secrets or run active probes. Do not create plan, report, PoC or other files, including in the session workspace, unless the user explicitly requests an artifact. Skip tools that cannot run without writing files and record them as unperformed; a missing scanner is not a clean scan. Do not paste sensitive values into the report.
4. For every candidate, try to disprove it: Is the input attacker-controlled? Is the route reachable? Is there upstream auth, a deny policy, or a compensating control? Can the proposed impact actually follow from the observed code? Check benign code paths too. Drop unsupported candidates; keep unresolved ones as questions with a specific way to settle them.
5. Rank remaining work by plausible impact and exposure, not scanner labels alone. Separate **observed code defect**, **conditional risk** and **verified exploit**. A source review can establish the first two, but cannot claim the third. Give high confidence only to claims supported by the complete relevant path and effective controls. Separate confidence in the code observation from confidence in deployment exposure.

For example, an unauthenticated management handler plus a ClusterIP Service may establish an *internal unauthenticated route*. An egress-only NetworkPolicy in one chart does **not** establish that every workload can reach it: ingress policies, platform defaults and actual deployment may change that. Report the code observation, mark cluster reachability as unverified, and recommend checking effective policies before asserting cross-namespace impact.

## Team handoff

For each actionable item report:

| Field | Required content |
|---|---|
| Priority and status | Severity (CRITICAL/HIGH/MEDIUM/LOW), separate confidence (1–10) for code observation and deployment exposure, and observed defect or conditional risk |
| Evidence | Exact file:line, affected entry point and relevant control; distinguish code, configuration, scanner output and inference |
| Preconditions and impact | Who must control what, which boundary they cross, and the consequence if those conditions hold |
| Counterevidence and unknowns | Controls checked, credible alternative explanation and what could change the verdict |
| Action | Smallest mitigation and the team or owner that must verify the assumption |
| Safe verification | A non-destructive test or inspection with an expected secure result and a result that would confirm the risk |

For an independent check, hand off only one testable claim and the primary evidence, not your conclusion or reasoning:

```
Revision:      <commit SHA and repository>
Claim:         <specific actor, entry point, missing control and possible impact>
Evidence:      <file:line references, including the relevant control or dependency>
Preconditions: <what must be true for the impact to follow>
Unknowns:      <evidence not available in this review>
```

The checker should look for counterevidence in the same revision. Its answer must say which parts of the claim are supported, contradicted or still unknown, including controls that narrow rather than fully refute the claim. Report confidence in the code observation and in deployment exposure separately, or say "unknown" when the evidence cannot support a score; never give a single confidence score for both. A second opinion without new primary evidence does not turn a conditional risk into a verified exploit.

Recommend tests only in an environment the team owns or has explicit authorization to assess. Describe the test objective, prerequisites, accounts or roles needed, and stop conditions. Do not execute payloads or scan running services as part of this skill. Security-critical findings require a human security review before relying on the severity or applying a fix.

State which files, revisions, configurations and tools were inspected, which were not, and why. If a tool fails, record the failure rather than presenting the result as a pass. No findings means no supported findings **in the inspected scope**, not that the service is secure. Keep unverified hypotheses outside the vulnerability count.
For a review or independent check, end with the assessment and the evidence still needed. Do not append a default fix/commit menu or solicit remediation unless the user asks for it.

## Prior art

- [Codex-Sentinel](https://github.com/alicankiraz1/Codex-Sentinel): separate opt-in review from recommendation-only security test planning; show reviewed and unreviewed areas.
- [Trail of Bits fp-check](https://github.com/trailofbits/skills/tree/main/plugins/fp-check): actively challenge suspected findings before assigning a verdict.
- [Trail of Bits differential-review](https://github.com/trailofbits/skills/tree/main/plugins/differential-review): prioritize risky changes and follow their callers and blast radius.
- [ASVS Auditor](https://github.com/TarkinLarson/asvs-auditor): tie claims to precise source evidence without implying certification.

## Review contract

Each relevant axis (entry points and trust boundaries, auth and ownership, injection and data handling, dependencies, deployment exposure, and sensitive output) produces at least one finding **or** a line stating what was inspected and what it does not prove. Do not manufacture findings to fill a category.

```
Inspected:     <specific revision, files, configurations, commands>
Not inspected: <in-scope evidence missing and reason>
Findings:      <confirmed code defects, conditional risks, unresolved questions>
Verdict:       BLOCK | CONCERNS | CLEAN | INCONCLUSIVE
```

`BLOCK` means a supported finding risks a security or privacy breach if shipped as observed. `CONCERNS` means a risk needs verification or mitigation before acceptance. `CLEAN` means no supported finding within the inspected scope; never claim a complete audit. Use `INCONCLUSIVE` when the requested scope is inaccessible or too narrow to assess, or when material disagreement cannot be resolved from the available evidence. State what would settle it.
