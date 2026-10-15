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
| Go | `go.sum` | `go get -u ./... && go mod tidy` (direct deps only: `go list -m -u -f '{{if and (not .Indirect) .Update}}{{.Path}}{{end}}' all`) | `go build ./... && go test ./...` |
| npm/pnpm/yarn | `package-lock.json` / `pnpm-lock.yaml` / `yarn.lock` | the repo's pinned manager: `npm update`, `pnpm update`, `yarn up` (minor/patch); majors one package at a time | `<pm> run build` (if defined) and `<pm> test` |
| Gradle | `gradle/libs.versions.toml` or `gradle.lockfile` | edit the catalog / build file versions after step 4 lookup | `./gradlew build` |
| .NET | `packages.lock.json` | `dotnet list package --outdated`, then `dotnet add package <id> --version <v>` | `dotnet build && dotnet test` |

## 2. Check the package manager version before installing (stop on mismatch)

Never run the machine's global package manager blindly; a global `pnpm = "latest"` in `mise` will rewrite a pnpm 9/10 lockfile to pnpm 11 and break CI. Run this check in each project directory before any install or update command:

```bash
# JS projects: compare repo pin and lockfileVersion against PATH
pin=$(jq -r '.packageManager // empty' package.json 2>/dev/null)
lock_ver=$(awk '/^lockfileVersion:/ { gsub(/['\''"]/, "", $2); print $2; exit }' pnpm-lock.yaml 2>/dev/null)
active_pnpm=$(pnpm --version 2>/dev/null)
active_npm=$(npm --version 2>/dev/null)
echo "packageManager=${pin:-none} pnpm-lock=${lock_ver:-none} pnpm=${active_pnpm:-none} npm=${active_npm:-none}"
mise ls --current 2>/dev/null || true
```

- **If `packageManager` is set in `package.json`** (for example `pnpm@9.15.0` or `pnpm@10.6.0`): run that exact version via `corepack pnpm ...` or `npx -y pnpm@<pinned-version> ...`, never bare `pnpm`.
- **If `pnpm-lock.yaml` is present without `packageManager`**: match the lockfile major (`lockfileVersion: '6.0'` → `npx -y pnpm@8`, `lockfileVersion: '9.0'` → `npx -y pnpm@9` or `pnpm@10`). If `pnpm --version` on `PATH` is a newer major (such as `11.x` from a `mise` `latest` override) and you cannot run the matching major via `corepack` or `npx`, **stop immediately** on that project and report: `Stopped: repo lockfile requires pnpm <expected>, but active pnpm is <actual>`.
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

# Maven Central / Gradle: must exit 0 (numFound == 1)
curl -sf "https://search.maven.org/solrsearch/select?q=g:<group>+AND+a:<artifact>+AND+v:<version>&rows=1&wt=json" \
  | jq -e '.response.numFound == 1'

# NuGet / .NET: package id and version in lowercase; must exit 0
curl -sf "https://api.nuget.org/v3-flatcontainer/<id-lowercase>/index.json" \
  | jq -e --arg v "<version-lowercase>" '.versions | index($v) != null'

# Container image tag (only when the user explicitly asked to bump an image)
docker buildx imagetools inspect "<image>:<tag>" >/dev/null

# GitHub Action tag (only when the user explicitly asked to bump a workflow action)
gh api "repos/<owner>/<repo>/git/ref/tags/<tag>" --jq '.ref'

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
3. Run `git diff --name-only` before finishing each project and verify that no held-back file (`Dockerfile*`, `.tool-versions`, `mise.toml`, `.nvmrc`, `global.json`, `gradle-wrapper.properties`, `.github/workflows/*`, `.nais/*`) and no `lockfileVersion` or `packageManager` field changed unrequested. Revert any unintended edit immediately.

## 7. Report

End with one table per project:

| Package | From | To | Status |
|---|---|---|---|
| github.com/foo/bar | v1.2.0 | v1.4.1 | upgraded |
| postgres (Dockerfile / compose / testcontainers) | 16 | — | held: DB major, not requested |
| pnpm (`packageManager` / `mise.toml`) | 9.15.0 | — | held: tool pin, not requested |
| eslint | 8.57.0 | — | held: 9.x broke config, tests red after one fix |

Follow the tables with each project's verify command and exit status, plus any package-manager version mismatch or `mise` `latest` override spotted in step 2.
