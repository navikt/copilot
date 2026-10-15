---
name: dependency-upgrade
description: Oppgrader direkte avhengigheter i Go-, npm-, Gradle- og .NET-prosjekter med repoets låste pakkehåndterer, sjekk hver ny versjon mot pakkeregisteret, kjør bygg og tester, og hold tilbake verktøypinner og major-hopp på database og infrastruktur med mindre du ber om dem
license: MIT
compatibility: Repos with a lockfile and a test command (Go modules, npm/pnpm/yarn, Gradle, .NET). Written for local models in nav-pilot's local mode; works with any model.
metadata:
  domain: platform
  tags: dependencies upgrade go npm gradle dotnet local-model
---

# Dependency upgrade

A recipe for upgrading a repo's direct dependencies that a small local model can
follow without inventing versions. It is built from what went right and wrong in
the nais/examples runs (navikt/mlx-workspace `reports/2026-10-14-nais-examples-upgrade`
and `reports/2026-10-15-recipe-ab`): lockfile-backed dependency bumps with tests
went green across Go, npm, Gradle and .NET, while unguarded runs invented tool
pins, image tags and CI versions, rewrote lockfiles with the wrong package
manager major, and bumped Postgres across majors.

The rule behind every step: **only change what something in the repo will check.**

## When to Use

- "Upgrade the dependencies", "bump everything", "update npm packages"
- A Dependabot or Renovate backlog you want to clear in one pass

## When NOT to Use

- Framework migrations that require code changes (Spring Boot 3→4, Jackson 2→3): use the dedicated migration skill
- A project with no lockfile or no test command: report that it has no lockfile/tests and skip it

## 1. Find each project and its ecosystem

List every project directory (a monorepo has several). Skip any project directory that lacks either a lockfile or a build/test command, and list it as skipped in the report. For each eligible project, record:

| Ecosystem | Lockfile | Upgrade command | Verify |
|---|---|---|---|
| Go | `go.sum` | list direct updates with `go list -m -u -f '{{if and (not .Indirect) .Update}}{{.Path}}@{{.Update.Version}}{{end}}' all`, then `go get <module>@<version> && go mod tidy` (without `-u`, so only required transitive dependencies move) | `go build ./... && go test ./...` |
| npm/pnpm/yarn | `package-lock.json` / `pnpm-lock.yaml` / `yarn.lock` | the repo's pinned manager: `npm update`, `pnpm update`, or for Yarn an explicit verified same-major target (`yarn upgrade <pkg>@<version>` on Yarn 1, `yarn up <pkg>@<version>` on Yarn Berry); majors one package at a time | `<pm> run build` (if defined) and `<pm> test` |
| Gradle | `gradle/libs.versions.toml` or `gradle.lockfile` | edit the catalog / build file versions after step 4 lookup; if `gradle.lockfile` or `dependency-locks/` is present, run `./gradlew dependencies --write-locks` before verifying | `./gradlew build` |
| .NET | `packages.lock.json` | `dotnet list package --outdated`, then `dotnet add package <id> --version <v>` | `dotnet build && dotnet test` |

## 2. Check the package manager version before installing (stop on mismatch)

Never run the machine's global package manager blindly; a global `pnpm = "latest"` in `mise` will rewrite a pnpm 9/10 lockfile to pnpm 11 and break CI. Run this check in each project directory before any install or update command:

```bash
# JS projects: compare repo pin (stripping any Corepack +sha... suffix) and lockfile format against PATH
pin=$(jq -r '.packageManager // empty' package.json ../package.json ../../package.json 2>/dev/null | head -n1 | cut -d+ -f1)
pnpm_lock=$(awk '/^lockfileVersion:/ { gsub(/['\''"]/, "", $2); print $2; exit }' pnpm-lock.yaml 2>/dev/null)
npm_lock=$(jq -r '.lockfileVersion // empty' package-lock.json 2>/dev/null)
active_pnpm=$(pnpm --version 2>/dev/null)
active_npm=$(npm --version 2>/dev/null)
active_yarn=$(yarn --version 2>/dev/null)
echo "packageManager=${pin:-none} pnpm-lock=${pnpm_lock:-none} npm-lock=${npm_lock:-none} pnpm=${active_pnpm:-none} npm=${active_npm:-none} yarn=${active_yarn:-none}"
mise ls --current 2>/dev/null || true
```

- **If `packageManager` is set in `package.json` (or a parent workspace `package.json`)** (`pnpm@<v>`, `npm@<v>`, or `yarn@<v>` after stripping any `+sha...` suffix): dispatch to that declared manager and exact version — `corepack pnpm ...` / `npx -y pnpm@<v> ...` for `pnpm@...`, `npx -y npm@<v> ...` for `npm@...`, or `corepack yarn ...` for `yarn@...` — and verify `<launcher> --version` matches `<v>` before installing or updating.
- **If `pnpm-lock.yaml` is present without `packageManager`**: match the lockfile major (`lockfileVersion: '6.0'` → `npx -y pnpm@8`, `lockfileVersion: '9.0'` → `npx -y pnpm@9` or `pnpm@10`). If `pnpm --version` on `PATH` is a newer major (such as `11.x` from a `mise` `latest` override) and you cannot run the matching major via `corepack` or `npx`, **stop immediately** on that project and report: `Stopped: repo lockfile requires pnpm <expected>, but active pnpm is <actual>`.
- **If `package-lock.json` is present without `packageManager`**: match `lockfileVersion` (`1` → `npx -y npm@6`, `2` → `npx -y npm@8`, `3` → `npm >=9`) so a newer `npm` on `PATH` does not rewrite format `1`/`2` to `3`. If a compatible `npm` cannot be selected and verified with `--version`, **stop immediately** on that project and report: `Stopped: package-lock.json lockfileVersion <v> is incompatible with active npm <actual>`.
- **Never change `lockfileVersion`** in `pnpm-lock.yaml` or `package-lock.json`, and never edit `packageManager` in `package.json`.
- **Gradle**: always run `./gradlew`, never a global `gradle`.
- **Go and .NET**: compare `go version` against the `go` line in `go.mod`, and `dotnet --version` against `global.json` (if present) or `<TargetFramework>` in `.csproj`. Stop if the active toolchain cannot build the target version.

## 3. Upgrade direct dependencies only

- Do not edit transitive versions by hand; let the package manager and lockfile resolve them.
- Upgrade one project at a time, run its verify command, and only then move to the next project.

## 4. Verify every new version exists and is stable

Every version you write by hand — in `gradle/libs.versions.toml`, `build.gradle.kts`, `package.json`, `.csproj`, `go.mod`, or in a pinned file the user explicitly asked to bump — **must** be checked against the upstream registry before you keep the edit:

```bash
# npm / pnpm / yarn: must print "<version>"
npm view "<pkg>@<version>" version

# Go module proxy: must print "<module> <version>"
go list -m "<module>@<version>"

# Maven Central / Gradle: check the POM directly (<group-path> replaces '.' with '/'); for Gradle plugins try https://plugins.gradle.org/m2/ too, or verify via ./gradlew dependencies --configuration compileClasspath
curl -sfI "https://repo1.maven.org/maven2/<group-path>/<artifact>/<version>/<artifact>-<version>.pom" >/dev/null

# NuGet / .NET: package id and version in lowercase; must exit 0
curl -sf "https://api.nuget.org/v3-flatcontainer/<id-lowercase>/index.json" \
  | jq -e --arg v "<version-lowercase>" '.versions | index($v) != null'

# Container image tag (only when the user explicitly asked to bump an image)
docker buildx imagetools inspect "<image>:<tag>" >/dev/null

# GitHub Action tag (only when the user explicitly asked to bump a workflow action): resolve commit SHA and keep `@<sha> # <tag>` pinning when the workflow already pins by SHA
gh api "repos/<owner>/<repo>/commits/<tag>" --jq '.sha'

# Tool pin in mise / .tool-versions (only when the user explicitly asked to bump a tool)
mise ls-remote "<tool>" | grep -Fx "<version>"
```

- If the lookup command fails or returns empty, **the version does not exist**: revert it immediately and record it in the report. Never guess a version number.
- Reject any pre-release (`-alpha`, `-beta`, `-rc`, `-M1`, `-SNAPSHOT`, `-next`, `-canary`, `-preview`) unless the current version in the repo is already a pre-release.

## 5. Hold back by default

Leave these untouched unless the user asked for them by name, and list them in the report as held back:

- **Database and infrastructure majors**: `postgres`, `redis`, `kafka`, `elasticsearch`, `opensearch`, `mongodb`, `rabbitmq` images, migration containers, Helm charts, or drivers/testcontainers whose major is tied to a server version (for example `postgres:16` → `18`). Keep the current major; only apply patch bumps within the same major if requested.
- **Tool pins**: `.tool-versions`, `mise.toml`, `.mise.toml`, `.nvmrc`, `.node-version`, `.python-version`, `global.json`, `gradle-wrapper.properties`, and `packageManager` in `package.json`.
- **Container image tags** in `Dockerfile*` and `docker-compose*.yml` / `compose*.yml`.
- **CI action versions** in `.github/workflows/` and `.github/actions/`.
- **nais manifests** (`.nais/`, `nais.yaml`, `nais.yml`).
- **Breaking major bumps**: if a major dependency bump fails build or tests after one small fix attempt, revert that package and keep the rest.

If the user explicitly asks to bump any held-back item, you **must** run the corresponding registry/tag check in step 4 before writing the new version.

## 6. Build, test and audit diff after each project

1. Run the project's verify command from step 1.
2. If tests fail: fix only if the upgrade caused it and the fix is small; otherwise revert that package and record it as held back. Never delete, skip or weaken a test to get green.
3. Run `git diff --name-only` before finishing each project and verify that no held-back file (`Dockerfile*`, `docker-compose*.yml`, `compose*.yml`, `.tool-versions`, `mise.toml`, `.mise.toml`, `.nvmrc`, `.node-version`, `.python-version`, `global.json`, `gradle-wrapper.properties`, `.github/workflows/*`, `.github/actions/*`, `.nais/*`, `nais.yaml`, `nais.yml`) and no `lockfileVersion` or `packageManager` field changed unrequested. Revert any unintended edit immediately.

## 7. Report

End with one table per project:

| Package | From | To | Status |
|---|---|---|---|
| github.com/foo/bar | v1.2.0 | v1.4.1 | upgraded |
| postgres (Dockerfile / compose / testcontainers) | 16 | — | held: DB major, not requested |
| pnpm (`packageManager` / `mise.toml`) | 9.15.0 | — | held: tool pin, not requested |
| eslint | 8.57.0 | — | held: 9.x broke config, tests red after one fix |

Follow the tables with each project's verify command and exit status, plus any package-manager version mismatch or `mise` `latest` override spotted in step 2.
