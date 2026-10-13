---
name: appsec-advisory
description: Rådgivende sikkerhetsgjennomgang av repoer og PR-er med etterprøvbare funn og forslag til trygg testing
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
3. Run useful read-only checks; record commands, results and scope. Do not install tools, start services, contact targets, read credentials, dump secrets or run active probes. Do not create files, even in the session workspace, unless requested. Record tools needing writes or unavailable tools as unperformed, not clean scans.
4. Try to disprove each candidate: attacker control, reachability, upstream auth, deny policies, compensating controls and plausible impact. Check benign paths. Drop unsupported claims; state unresolved ones as questions with a way to settle them.
5. Prioritize by plausible impact and exposure, not scanner labels. Separate **observed code defect**, **conditional risk** and **verified exploit**: source review cannot establish the last. Rate code evidence and deployment exposure separately; do not infer reachability from an egress-only policy or a single config fragment.

## Team handoff

For each actionable item give severity (CRITICAL/HIGH/MEDIUM/LOW), status (observed defect or conditional risk), and separate confidence (1–10 or unknown) for code and deployment. Cite exact file:line, entry point and controls; distinguish code, configuration, scanner results and inference. State the attack preconditions and impact, counterevidence, unknowns, smallest mitigation and owner. Propose a non-destructive check with prerequisites (including accounts or roles), expected secure and risky results, and stop conditions, only in an environment the team owns or is authorized to assess. Never execute it as part of this skill. Require human review before relying on security-critical severity or applying a fix; never include sensitive values in the report.

For an independent check, hand off only one testable claim and the primary evidence, not your conclusion or reasoning:

```
Revision:      <commit SHA and repository>
Claim:         <specific actor, entry point, missing control and possible impact>
Evidence:      <file:line references, including the relevant control or dependency>
Preconditions: <what must be true for the impact to follow>
Unknowns:      <evidence not available in this review>
```

The checker should seek counterevidence in the same revision and mark each part supported, contradicted or unknown, including controls that only narrow the claim. Rate code and deployment separately, using unknown where evidence is lacking. Show both assessments side by side; agreement without new primary evidence does not verify exploitation. End with evidence still needed, not a default fix/commit menu.

## Review contract

For each relevant axis (entry points and trust boundaries, auth and ownership, injection and data, dependencies, deployment, sensitive output), report a finding or what was inspected and what it does not prove. Do not manufacture findings.

```
Inspected:     <specific revision, files, configurations, commands>
Not inspected: <in-scope evidence missing and reason>
Findings:      <confirmed code defects, conditional risks, unresolved questions>
Verdict:       BLOCK | CONCERNS | CLEAN | INCONCLUSIVE
```

`BLOCK`: supported risk of security or privacy breach if shipped. `CONCERNS`: verification or mitigation needed. `CLEAN`: no supported findings in the inspected scope, not a complete audit. `INCONCLUSIVE`: inaccessible or insufficient scope, or unresolved material disagreement; state what would settle it. List inspected and missing files, revisions, configs and tools with reasons; report failures, not passes. Keep unverified hypotheses outside the finding count.
