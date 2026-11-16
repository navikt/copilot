---
name: repo-pii-audit
description: Kartlegg personopplysninger i repo-filer og Git-historikk og planlegg trygg opprydding. Bruk ved PII-audit, fødselsnummer i testdata, sensitive logger, BFG Repo-Cleaner og git-filter-repo.
license: MIT
metadata:
  domain: auth
  tags: privacy pii git history bfg git-filter-repo
---

# Repository PII audit and cleanup

Audit first. Do not rewrite history, delete evidence, force-push, change protections, or contact third parties without explicit approval for that action. A request to audit is not permission to clean up.

History rewriting is **rød sone — forstå dette grundig**: explain the affected refs, lost signatures, changed SHAs, dependent systems, and recovery limits before asking for approval. Follow repository rules even if a generic cleanup guide suggests bypassing protections.

## 1. Establish scope and contain exposure

- Read `AGENTS.md`, security policy, and incident procedures. Confirm repository, owner, visibility, approved local workspace, audit time, and whether the clone is shallow or partial. Never execute repository code or hooks to inspect it.
- Identify current tracked, staged, unstaged, untracked and ignored files; all available branches and tags; commit/tag messages and author/committer identities; filenames; submodules and Git LFS. A mirror is not a copy of every server-side object.
- Include forks, other clones, PR refs/diffs/comments, issues, wikis, releases, Actions logs/artifacts/caches, packages, deployed images, backups and external indexes as separate surfaces. Establish which are accessible and who owns them.
- Treat real personal data as a potential incident. Notify the repository owner and use the approved security/privacy channel; involve the privacy officer for notification and retention decisions. Do not make legal breach-reporting decisions yourself. Rotate/revoke exposed credentials immediately through the owner; PII cannot be rotated.
- Keep raw data out of prompts, tool output, terminal scrollback, CI logs, tickets and PRs. Use an approved local-only scanner with network/telemetry disabled or verified absent. Do not send suspected data to an external model or online validator.
- Store reports, replacement maps and any necessary original copy outside the repo in approved restricted storage. Use restrictive permissions (`umask 077`), minimal access, an owner and a deletion deadline. Backups preserve the exposure: make them only if incident policy permits. Do not assume deleting a local file securely erases SSDs or backups.

If the tool cannot suppress matched text, redirect output to restricted storage before running it and inspect locally, not through the agent. Paths, commit identities, surrounding lines and scanner errors may themselves disclose PII. Even a redacted report needs review before sharing.

## 2. Choose detectors, not a cleanup tool

| Tool | Useful for | Not proof of |
|---|---|---|
| Local rules and structured-data inspection | FNR/D-number candidates, emails, phones, addresses, identifiers in fixtures/dumps/logs | Identity, lawfulness, or complete coverage |
| Presidio, locally configured | Contextual PII candidates in supported languages; custom recognizers | Reliable Norwegian coverage out of the box; zero false negatives |
| Gitleaks | Credential patterns in files and Git changes; custom rules | General PII detection or complete historical blob coverage |
| `git-filter-repo` | Removing known paths, replacing known text, rewriting metadata | Discovering PII or cleaning GitHub caches/forks |
| BFG Repo-Cleaner | Bulk removal of known blobs/files or text | PII discovery or complete removal by default |

Prefer `git-filter-repo` for new cleanup plans: GitHub documents its sensitive-data-removal workflow. BFG is a valid narrower alternative, not a PII scanner. BFG protects the latest commit by default: clean that tree first and verify protected content. Do not casually disable protection. Its filename/blob-oriented removal is not a substitute for precise path and metadata handling.

If the owner chooses BFG, follow its documented fresh mirror-clone workflow. Filename patterns can remove same-named files in unrelated directories: review the entire target set first. Review its protected-blob report and rescan rewritten refs. Old objects can remain locally in reflogs and unreachable storage; reflog expiry and garbage collection are separate destructive steps requiring retention approval and no concurrent writers. They do not erase server copies. Do not copy BFG's generic mirror-push example into an automated runbook.

Use the team's approved versions and verify their `--help` and upstream documentation before execution. Do not install tools or download language models without approval. Secret-scanning defaults are not a PII policy.

## 3. Audit without printing matches

1. Inventory files and refs with output routed locally if names may be sensitive. Record shallow/partial status and missing permissions. Obtain an approved fresh full clone for history work; retain the developer's existing working tree unchanged.
2. Scan current contents separately, including staged versions and approved untracked/ignored files. Prioritize test fixtures, SQL/CSV/JSON exports, notebooks and outputs, HAR files, screenshots, PDFs, logs, attachments and config. Do not follow symlinks outside the approved scope.
3. Inspect **every unique blob reachable from the audited refs**, with a local scanner that reports object ID/category/count, not matched text. A `git log -p` search alone misses content outside the selected diffs. Scan ref names (including branch and lightweight tag names), paths and commit/tag messages and identities separately; do not dump `git show`, `git log -p` or `git grep` matches into chat. Local reflogs/unreachable objects are a separate approved scope, not covered by `--all`.
4. Fetching more refs changes the local copy and may access more sensitive data: agree the scope first. Fetch and inspect accessible PR refs where authorized. Audit submodules as separate repos; obtain actual LFS objects, not just pointer files. Handle archives, encodings, binaries and OCR with approved tools and resource limits. Report unsupported formats and absent objects as gaps, never as clean.
5. Use multiple signals. An 11-digit candidate plus appropriate checksum/date validation is useful for Norwegian FNR/D-numbers, but validation does not prove a real identity. Invalid checksums, synthetic-number ranges, whitespace and split fields must not become blanket exclusions. Names and addresses need context; regex alone is insufficient. Combined fields may identify someone without a direct identifier.
6. Human-triage locally: confirmed personal data, approved synthetic/public data, false positive, or unresolved. Legitimate contributor names/emails are not automatic deletion targets. Minimize any exception by rule/path and owner; never allowlist a broad directory to hide findings.

Before trusting a detector, test it on an isolated synthetic fixture: current-only content, an old deleted/renamed file, a non-default branch/tag, metadata, and the supported archive/binary cases. Include negative controls. Confirm it finds historical content after deletion and emits no matching values. Never use real personal data as a test fixture. If writing a helper, add this regression test with it.

Report: repository and audited ref IDs/time, tools/versions/config, surfaces covered, counts by category/confidence, restricted finding IDs, gaps, incident owner and proposed next actions. Keep sensitive paths and object IDs in the restricted record if they help locate exposed data. Say **“no findings within this scope”**, not “PII-free”.

## 4. Propose the smallest cleanup

- Replace current fixtures with deliberately synthetic data; remove production dumps and fix the export/logging source. Deleting a file in a normal commit does not erase history.
- Decide with the owner whether rewriting is warranted. For confirmed PII, secret rotation alone does not remove the privacy exposure. A normal PR can fix current files; it cannot purge earlier commits.
- For a rewrite, agree a write freeze, exact repository/ref/path scope, all historical names of moved files, replacement rules, affected open PRs, forks, releases and SHA-pinned consumers. Include tags, signatures, provenance and deployment automation. A shared module's changed SHAs can affect every downstream team.
- Require explicit owner approval for the local rewrite, retention/recovery plan, and a separate publication decision. Coordinate publication and protection changes through authorized maintainers. Do not bypass hooks, fresh-clone safeguards or branch rules; never automate a mirror force-push.

### Local rewrite templates — approval required

Run only in an approved fresh disposable clone after testing the plan on synthetic data. `--sensitive-data-removal` requires `git-filter-repo` 2.47 or newer. These templates are not commands to run during an audit:

`--sensitive-data-removal` fetches additional refs from the origin by default. Approval must cover that network access and the expanded ref set, which must also be audited. If the agreed scope excludes the fetch, stop and revise the plan; do not run these templates or silently skip fetching and claim complete coverage.

```bash
# Remove an exact known path and its known former path from history.
git-filter-repo --sensitive-data-removal --invert-paths \
  --path 'path/to/exposed-file.csv' --path 'former/path/to/exposed-file.csv'

# Alternative: replace known text using a restricted file outside the repo.
git-filter-repo --sensitive-data-removal --replace-text ../restricted-replacements.txt
```

Use actual approved paths, never personal values in command arguments. Verify the replacement-file syntax; prefer literal matches unless a reviewed regex is necessary. Text replacement does not by itself clean ref names, filenames, author identities, messages or binary data. Plan those separately using the documented options, including explicit owner approval for renaming or deleting exposed refs. Do not provide a generic callback that silently rewrites every contributor identity. Path removal does not follow renames automatically.

## 5. Verify, then coordinate publication

1. Before publication, rescan rewritten ref names, metadata and blobs using the same scope and detectors. Verify known targets are absent, approved ref renames/deletions are complete, and unrelated files/refs remain. Run relevant repo checks offline without production credentials. Review the ref/commit map and changed-ref summary locally; protect these records.
2. Have the authorized maintainer publish only after the write freeze and explicit approval. Explain that commit SHAs change, signatures can be removed, open PRs need repair, links can break, and automated deployments may run. Stop if publication is blocked by a guard rail.
3. Audit a fresh clone from the server after publication. A successful push or clean working tree is not verification. GitHub's `refs/pull/*` are read-only to maintainers; force-pushing cannot clean them. Server-side PR refs and cached commit views may retain data: follow GitHub's support procedure with the owner and provide only the restricted evidence requested. Rewritten Git pointers do not delete stored LFS objects; arrange their removal separately with the host. Do not promise support will remove all copies.
4. Coordinate fork owners and collaborators. Prefer fresh clones; recover legitimate unpublished work only after checking it against the cleaned history. Never merge old branches back or push old tags. Address LFS objects, releases, logs, caches, artifacts, packages and external copies separately with their owners.
5. Close only when the owner accepts the coverage and remaining exposure. Record inaccessible copies and pending support work, assign owners and restricted tracking tasks, and expire audit material according to the agreed retention policy.

Prevent recurrence with synthetic fixtures, minimal exports, reviewed PII-specific checks plus secret scanning, and a documented incident path. Add checks in report-only mode first if necessary to classify existing findings; any temporary baseline needs an owner and expiry, not a permanent exemption.

## Sources

Research checked 2026-11-03. Recheck upstream before a destructive run.

- [GitHub: removing sensitive data](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository) — rewrite side effects, version requirement, clones/forks and support limits.
- [git-filter-repo manual](https://github.com/newren/git-filter-repo/blob/main/Documentation/git-filter-repo.txt) — path/text/metadata filtering and sensitive-data-removal workflow.
- [BFG Repo-Cleaner](https://rtyley.github.io/bfg-repo-cleaner/) — known-content cleanup and protected commits; [source](https://github.com/rtyley/bfg-repo-cleaner).
- [Gitleaks](https://github.com/gitleaks/gitleaks) — secret scanning, redaction and custom rules; verify reports do not disclose context.
- [Presidio](https://github.com/data-privacy-stack/presidio) — local PII detection and its accuracy limitations; originally Microsoft Presidio, now maintained under Data Privacy Stack.
