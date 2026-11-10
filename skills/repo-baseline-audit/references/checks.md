# Rule catalogue

Each rule has an ID, a tier, an *Applies when* condition, what to check, and a
source. Evidence sections from [evidence.md](evidence.md) are named in
`[brackets]`. Rules marked *judgment* need code reading; sample entry points
and cite `file:line`.

Tiers: **Must** comes from requirements for all `navikt` repositories.
**Should** comes from Golden Path and the security playbook. **Recommended** is
general GitHub practice that no Nav source states yet; label it as such.

## Must — requirements for all navikt repositories

Source for this tier, unless stated otherwise:
[navikt/offentlig — Krav til repo](https://github.com/navikt/offentlig#krav-til-%C3%A5pne-repo-under-githubcomnavikt).
It applies regardless of visibility.

### REQ-LICENSE
- Applies when: always.
- Check: `LICENSE` or `LICENSE.md` exists and is MIT; the API `license` field
  says `MIT`. `[repo]` `[files]`

### REQ-README
- Applies when: always.
- Check: `README` or `README.md` explains purpose, how to build and test, and
  how to contact the team. A public repository must give a contact route that
  works for outsiders (e-mail, person, GitHub issues); a Slack channel alone is
  internal-only. *judgment*
- Source: [nye-repo.md](https://github.com/navikt/offentlig/blob/main/guider/nye-repo.md#readme)

### REQ-CODEOWNERS
- Applies when: always.
- Check: `CODEOWNERS` (root, `.github/` or `docs/`) assigns `*` to a
  `@navikt/<team>`. An owner without a slash (`@jdoe`) is an individual and is
  a gap. `[codeowners]`

### REQ-CLOSED-JUSTIFIED
- Applies when: visibility is `private` or `internal`.
- Check: README has a section that justifies concretely why the code is
  closed (credentials, fraud detection, unannounced legislation). *judgment*
- Source: [Kode som ikke kan åpnes](https://github.com/navikt/offentlig#kode-som-ikke-kan-%C3%A5pnes)

### REQ-NO-SECRETS
- Applies when: always.
- Check: no credentials or personal data (fødselsnummer, names, addresses,
  aktør-ID, NAV-ident) in code or history. Use `[gitleaks]` and
  `[secret-scanning]`; triage hits as described in `SKILL.md`. Synthetic test
  identities are fine; real-looking ones outside tests are a question for the
  team.
- Source: [sikkerhetsvask.md](https://github.com/navikt/offentlig/blob/main/guider/sikkerhetsvask.md),
  [Hemmeligheter](https://sikkerhet.nav.no/docs/sikker-utvikling/hemmeligheter#n%C3%A5r-hemmeligheter-havner-i-git)

## Should — Golden Path

Source: [Golden Path](https://sikkerhet.nav.no/docs/goldenpath/). IDs follow
`src/data/golden-path.js` in `navikt/security-playbook`.

### GP-NAIS-DEFAULTS
- Applies when: `.nais/`, `nais.yaml` or another Nais manifest exists.
- Check: authentication uses the platform (`azure`, `tokenx`, `idporten`,
  `maskinporten` in the manifest) rather than a custom flow; `accessPolicy` is
  declared; no secret values in manifests. `[nais]` *judgment*

### GP-NPM-HARDENING
- Applies when: `package.json` exists.
- Check: a lockfile is committed; CI installs from it (`npm ci`,
  `--frozen-lockfile`); npm and yarn projects disable lifecycle scripts in the
  repository config; a minimum release age is set (playbook recommends 3 days);
  committed `.npmrc` uses `${NPM_TOKEN}`, never a literal token. pnpm disables
  lifecycle scripts by default. `[js]` `[workflows]`
- Source: [Supply chain](https://sikkerhet.nav.no/docs/sikker-utvikling/supply-chain)

### GP-GITHUB-PIN
- Applies when: `.github/workflows/` exists.
- Check: third-party actions are pinned to a full commit SHA, ideally with a
  `# vX.Y.Z` comment. GitHub-owned actions (`actions/*`, `github/*`) may use
  tags. Pinning only through an `actions.lock` file is `Partly`: the playbook
  does not mention it. `[workflows]` `[zizmor]`
- Fix hint: `gh extension install navikt/gh-act && gh act pin`. To align
  zizmor with the playbook, set a `ref-pin` policy for `actions/*` and
  `github/*` in `zizmor.yml`.
- Source: [Pin tredjeparts-actions](https://sikkerhet.nav.no/docs/sikker-utvikling/github#pin-tredjeparts-actions)

### GP-GITHUB-PERMISSIONS
- Applies when: `.github/workflows/` exists.
- Check: every workflow sets `permissions` explicitly (top level or every
  job), grants only what the job needs, and never uses `write-all`.
  `[workflows]` `[zizmor]`
- Source: [Use minimum permissions](https://sikkerhet.nav.no/docs/sikker-utvikling/github#use-minimum-permissions)

### GP-GITHUB-ZIZMOR
- Applies when: `.github/workflows/` exists.
- Check: zizmor runs in CI or the repository is clean after triage. Security
  scans that run in CI have a `schedule` trigger as well. `[workflows]`
  `[zizmor]`
- Source: [Kjør zizmor](https://sikkerhet.nav.no/docs/sikker-utvikling/github#kj%C3%B8r-zizmor)

### GP-DEPENDABOT
- Applies when: always.
- Check: `.github/dependabot.yml` or `.yaml` covers every ecosystem present,
  including `github-actions`, plus `docker` when a Dockerfile exists UNLESS the
  repository uses hardened images. Every
  entry has `cooldown`. `[dependabot]`
- Source: [Dependabot](https://sikkerhet.nav.no/docs/verktoy/dependabot)

### GP-LOCAL-SECRETS
- Applies when: always.
- Check: no tracked env files with values, private keys, keystores,
  kubeconfigs or credential files. `.env.example` and similar templates are
  fine when they hold placeholders only. Local start scripts read secrets from a
  vault or the platform, not from files in the repository. `[tracked-sensitive]`
- Source: [Hemmeligheter](https://sikkerhet.nav.no/docs/sikker-utvikling/hemmeligheter#lokal-utvikling)

### GP-DOCKER-BUILD-PUSH
- Applies when: a workflow builds a container image.
- Check: the build uses `nais/docker-build-push` and does not disable SBOM or
  attestation. `[workflows]`
- Source: [Nais build](https://doc.nais.io/build/)

### GP-DISTROLESS
- Applies when: a `Dockerfile` exists.
- Check: the final stage uses a minimal image (Chainguard via
  `europe-north1-docker.pkg.dev/cgr-nav/…` or `cgr.dev/chainguard/…`, or
  `gcr.io/distroless/…`), and no `USER root` remains in it. `[docker]`
- Source: [Baseimages](https://sikkerhet.nav.no/docs/sikker-utvikling/baseimages),
  [Containere](https://sikkerhet.nav.no/docs/sikker-utvikling/containere)

### GP-VALIDATE-INPUT
- Applies when: the repository exposes HTTP endpoints or renders a UI.
- Check: backend validates request input with types and allow-lists, even
  when the frontend validates too; no `dangerouslySetInnerHTML`, `v-html` or
  `innerHTML` with user-controlled data. *judgment*
- Source: [Inputvalidering](https://sikkerhet.nav.no/docs/sikker-utvikling/inputvalidering)

### GP-CLIENT-CREDENTIALS
- Applies when: the service calls other services.
- Check: machine-to-machine calls use Entra ID client credentials,
  on-behalf-of or TokenX; no service users with long-lived passwords, no STS
  (`security-token-service`, `/rest/v1/sts`). *judgment*
- Source: [Maskin-til-maskin](https://sikkerhet.nav.no/docs/sikker-utvikling/m2m)

### GP-SENSITIVE-LOGGING
- Applies when: the application logs.
- Check: ordinary logs do not include fødselsnummer, aktør-ID, IP addresses,
  JWTs or credentials; sensitive data goes to team logs; sensitive identifiers
  travel in POST bodies, not URL paths or query strings. *judgment*
- Source: [Logging](https://sikkerhet.nav.no/docs/sikker-utvikling/logging)

## Should — security playbook pages

Source: [Github best practices](https://sikkerhet.nav.no/docs/sikker-utvikling/github)
unless stated otherwise.

### PB-BRANCH-PROTECTION
- Applies when: always.
- Check: the default branch has a ruleset or classic protection that
  restricts deletion, blocks force pushes and requires. `[branch-protection]`

### PB-OWNERSHIP
- Applies when: always.
- Check: a team holds `admin`; individuals do not hold direct admin access.
  `[access]`
- Source: [Eierskap](https://sikkerhet.nav.no/docs/sikker-utvikling/github#eierskap)

### PB-WORKFLOW-TRIGGERS
- Applies when: `.github/workflows/` exists.
- Check: no `pull_request_target` or `workflow_run` workflow checks out or
  runs code from the pull request. `[workflows]` `[zizmor]`
- Source: [Workflow triggers](https://sikkerhet.nav.no/docs/sikker-utvikling/github#workflow-triggers)

### PB-INTERMEDIATE-VARS
- Applies when: `.github/workflows/` exists.
- Check: untrusted `${{ github.event.* }}` values and inputs reach `run:`
  through `env:`, not inline. zizmor's `template-injection` covers it.
  `[zizmor]`
- Source: [Bruk intermediate variables](https://sikkerhet.nav.no/docs/sikker-utvikling/github#bruk-intermediate-variables)

### PB-TOKENS
- Applies when: `.github/workflows/` exists.
- Check: workflows use `GITHUB_TOKEN` or a GitHub App installation token; a
  secret that looks like a PAT (`*PAT*`, `*_TOKEN` passed to checkout or `gh`)
  needs a reason. *judgment* `[workflows]`
- Source: [Tokens](https://sikkerhet.nav.no/docs/sikker-utvikling/github#tokens)

### PB-SECRET-SCANNING
- Applies when: always.
- Check: secret scanning and push protection are enabled; open alerts are
  triaged. `[repo]` `[secret-scanning]`
- Source: [Secret scanning](https://sikkerhet.nav.no/docs/sikker-utvikling/github#secret-scanning)

### PB-DEPENDENCY-GRAPH
- Applies when: always.
- Check: Dependabot alerts are enabled and the dependency graph is
  populated; Gradle projects submit dependencies with
  `gradle/actions/dependency-submission`. Report open critical and high alerts.
  `[repo]` `[dependabot-alerts]` `[workflows]`
- Source: [Dependabot](https://sikkerhet.nav.no/docs/verktoy/dependabot#sjekk-om-dependabot-finner-avhengigheter-og-versjoner-automatisk)

### PB-CODE-SCANNING
- Applies when: the repository contains a CodeQL-supported language.
- Check: CodeQL default setup or a CodeQL workflow is active; report open
  alerts by severity. `[code-scanning]`
- Source: [Statisk kodeanalyse](https://sikkerhet.nav.no/docs/sikker-utvikling/kodeanalyse)

### PB-DOCKER-COPY
- Applies when: a `Dockerfile` exists.
- Check: no `COPY . .` into the final stage; copy named files or build
  output. `[docker]`
- Source: [Hemmeligheter — GitHub og build](https://sikkerhet.nav.no/docs/sikker-utvikling/hemmeligheter#github-og-build)

## Recommended — general GitHub practice

No Nav source states these yet. Report them as general practice.

### HYG-GITIGNORE
- Applies when: always.
- Check: `.gitignore` ignores `.env` and `.env.*` (keeping `!.env.example`),
  private keys (`*.pem`, `*.key`, `*.p12`, `*.jks`) and the build output and
  IDE folders of the detected stacks. `[gitignore]`

### HYG-LOCKFILES
- Applies when: an ecosystem with a lockfile exists outside npm.
- Check: `go.sum`, `uv.lock` or `poetry.lock`, and `Cargo.lock` for
  applications are committed. `[lockfiles]`

### HYG-VULN-REPORTING
- Applies when: always.
- Check: private vulnerability reporting is enabled. The org-wide
  `SECURITY.md` sends reporters there, so the route must exist in every
  repository. `[repo]`
- Source: [navikt/.github SECURITY.md](https://github.com/navikt/.github/blob/main/SECURITY.md)

### HYG-DEPLOY-KEYS
- Applies when: always.
- Check: no deploy keys, or each one is read-only and has a known purpose.
  `[access]`

### HYG-BRANCH-CLEANUP
- Applies when: always.
- Check: `delete_branch_on_merge` is enabled. `[repo]`

### HYG-CONTRIBUTING
- Applies when: visibility is `public`.
- Check: `CONTRIBUTING.md` explains how outsiders can contribute.
  `[files]`
- Source: [nye-repo.md](https://github.com/navikt/offentlig/blob/main/guider/nye-repo.md#contributing)
  (suggested, not required)
