# Evidence collection

Each `[section]` below is cited by the rules in [checks.md](checks.md). Collect
only the sections the applicable rules need.

Run commands from the repository root, one command per call. They use only
`gh`, `git`, `zizmor` and `gitleaks`, with no pipes, redirects or embedded
double quotes, so they behave the same in bash, zsh and PowerShell. Replace
`OWNER/REPO` with the slug and `BRANCH` with the default branch. For local file
checks, use your own file search and read tools instead of shell `grep`, `find`
or `cat`.

Record failures instead of retrying. A `gh api` call that returns 403 or 404
means `Not verifiable` for the rules that depend on it; a missing tool means
the same for its section. Never print or quote a secret value.

## [meta]

```
git rev-parse --show-toplevel
git rev-parse --short HEAD
gh repo view --json nameWithOwner --jq .nameWithOwner
gh --version
zizmor --version
gitleaks version
```

## [repo]

```
gh api repos/OWNER/REPO --jq '{visibility, default_branch, archived, license: .license.spdx_id, delete_branch_on_merge, security_and_analysis}'
gh api repos/OWNER/REPO/private-vulnerability-reporting --jq .enabled
gh api repos/OWNER/REPO/vulnerability-alerts -i
```

The last call answers `204` when Dependabot alerts are enabled and `404` when
they are disabled or the caller lacks access.

## [branch-protection]

```
gh api repos/OWNER/REPO/rules/branches/BRANCH --jq '[.[].type]'
gh api repos/OWNER/REPO/branches/BRANCH/protection --jq '{require_pr: (.required_pull_request_reviews != null), checks: .required_status_checks.contexts, force_push: .allow_force_pushes.enabled, deletions: .allow_deletions.enabled}'
```

Rulesets count. `Branch not protected` (404) from the second call is not a gap
when the first lists `deletion`, `non_fast_forward` and `pull_request`.

## [access]

```
gh api repos/OWNER/REPO/teams --jq '[.[] | {slug, permission}]'
gh api 'repos/OWNER/REPO/collaborators?affiliation=direct&per_page=100' --jq '[.[] | {login, role_name}]'
gh api repos/OWNER/REPO/keys --jq '[.[] | {title, read_only}]'
```

## [code-scanning]

```
gh api repos/OWNER/REPO/code-scanning/default-setup --jq .state
gh api 'repos/OWNER/REPO/code-scanning/alerts?state=open&per_page=100' --paginate --jq '.[] | .rule.security_severity_level // .rule.severity'
```

The second call prints one severity per open alert; count them.

## [dependabot-alerts]

```
gh api 'repos/OWNER/REPO/dependabot/alerts?state=open&per_page=100' --paginate --jq '.[].security_advisory.severity'
```

## [secret-scanning]

```
gh api 'repos/OWNER/REPO/secret-scanning/alerts?state=open&per_page=100' --paginate --jq '.[].secret_type_display_name'
```

The filter returns alert types only. Do not fetch alert details: they contain
the secret.

## [files]

```
git ls-files -- 'LICENSE*' 'README*' 'CONTRIBUTING*' CODEOWNERS .github/CODEOWNERS docs/CODEOWNERS .gitignore '.github/dependabot.y*ml' zizmor.yml .github/zizmor.yml .gitleaksignore
```

Read the first line of the license file to identify it.

## [codeowners]

Read the `CODEOWNERS` file from `[files]` and note the owners of `*`.

## [dependabot]

Read `.github/dependabot.yml` or `.yaml`. List each `package-ecosystem`, its
`directory` or `directories`, and whether it has a `cooldown` block.

## [workflows]

```
git ls-files -- '.github/workflows/*.yml' '.github/workflows/*.yaml' '.github/actions/*/action.y*ml' 'action.y*ml'
```

Search those files for:

- `uses:` references. Classify each as `local` (`./…`, `docker://…`),
  `github-owned` (`actions/*`, `github/*`), `sha-pinned` (ref after `@` is a
  40-character hex SHA) or `unpinned` (anything else). Cite `file:line`.
- `pull_request_target` and `workflow_run` triggers.
- Workflows without a top-level `permissions:` key, and whether every job sets
  its own. Any `write-all`.
- `secrets.NAME` references (names only).
- `schedule:` triggers.
- `nais/docker-build-push`, SBOM or attestation settings, `codeql`, `zizmor`,
  `dependency-submission`, and package installs (`npm ci`, `npm install`,
  `pnpm install`, `yarn install`, `bun install`).

## [docker]

Find tracked Dockerfiles with `git ls-files -- '*Dockerfile' '*Dockerfile.*' '*.Dockerfile'`.
Read the `FROM`, `USER`, `COPY` and `ADD` lines, noting which belong to the final
stage.

## [nais]

Find manifests with `git ls-files -- '.nais/*' '*/.nais/*' 'nais.y*ml' 'nais/*'`.
Look for `azure`, `tokenx`, `idporten`, `maskinporten`, `accessPolicy`,
`inbound`, `outbound`, `ingresses`, `envFrom`, `filesFrom` and `autoLogin`.

## [js]

Find `package.json` files and lockfiles (`pnpm-lock.yaml`,
`package-lock.json`, `yarn.lock`, `bun.lock`, `bun.lockb`, `deno.lock`) with
`git ls-files`. Note each `packageManager` field.

For committed `.npmrc` files, report only whether auth keys (`_auth`,
`_authToken`, `password`) use a `${VAR}` reference. Never reproduce the value.

Check `pnpm-workspace.yaml`, `.yarnrc.yml` and `bunfig.toml` for
`minimumReleaseAge`, `onlyBuiltDependencies`, `allowBuilds`,
`blockExoticSubdeps`, `enableScripts`, `npmMinimalAgeGate` and
`minimum-release-age`.

## [lockfiles]

```
git ls-files -- go.mod go.sum pyproject.toml uv.lock poetry.lock Cargo.toml Cargo.lock build.gradle.kts build.gradle gradle.lockfile gradle/verification-metadata.xml pom.xml
```

Pair each manifest with its expected lockfile: `go.mod` with `go.sum`,
`pyproject.toml` with `uv.lock` or `poetry.lock`, `Cargo.toml` with
`Cargo.lock`, Gradle with `gradle.lockfile` or `verification-metadata.xml`.

## [gitignore]

```
git check-ignore --no-index .env .env.local .env.production key.pem server.key keystore.jks cert.p12 .idea/x .vscode/x .env.example
```

Paths that are printed are ignored; paths that are missing are not. Exit code
1 means none are ignored. `.env.example` should not be printed.

## [tracked-sensitive]

```
git ls-files -- '*.env' '*.env.*' '*.pem' '*.key' '*.p12' '*.pfx' '*.jks' '*.keystore' '*.tfstate' '*id_rsa' '*id_ed25519' '*kubeconfig' '*credentials' '*credentials.json' '*.netrc' '*.pgpass'
```

Ignore templates (`.example`, `.sample`, `.template`, `.dist`). For the rest,
count lines of the form `KEY=value` without reproducing them.

## [zizmor]

Only when workflows exist:

```
zizmor --offline --format github .
```

Each line is `::level file=…,line=…,title=rule::message`. Group by rule.

## [gitleaks]

```
gitleaks git --redact --no-banner --log-level error --report-format csv --report-path - .
```

Use `gitleaks dir` instead of `gitleaks git` when the repository has no
commits. Exit code 1 means findings, not failure. Report only `RuleID`, `File`,
`StartLine` and the first 8 characters of `Commit`. Leave out `Author`, `Email`
and `Message`. `Secret` and `Match` are already redacted.
