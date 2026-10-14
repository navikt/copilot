---
name: dependency-upgrade
description: Oppgrader direkte avhengigheter i Go-, npm-, Gradle- og .NET-prosjekter med innebygde sperrer: repoets egen pakkehåndterer, bygg og tester etterpå, hver ny versjon sjekket mot registeret, og ingen major-hopp for database eller infrastruktur uten at du ber om det
license: MIT
compatibility: Repos with a lockfile and a test command (Go modules, npm/pnpm/yarn, Gradle, .NET). Written for local models in nav-pilot's local mode; works with any model.
metadata:
  domain: platform
  tags: dependencies upgrade go npm gradle dotnet local-model
---

# Dependency upgrade

A recipe for upgrading a repo's direct dependencies that a small local model can
follow without inventing versions. It is built from what went right and wrong in
the nais/examples run (navikt/mlx-workspace `reports/2026-10-14-nais-examples-upgrade`):
lockfile-backed dependency bumps with tests went green in every ecosystem;
tool pins, image tags and CI versions got invented versions, a package manager
that no longer matched the lockfile, and a breaking postgres major.

The rule behind every step: **only change what something in the repo will check.**

## When to Use

- "Upgrade the dependencies", "bump everything", "update npm packages"
- A Dependabot/Renovate backlog you want to clear in one pass

## When NOT to Use

- Framework migrations with code changes (Spring Boot 3→4, Jackson 2→3): use the dedicated skill
- The repo has no lockfile or no test command: say so and stop; there is nothing to verify against

## 1. Find each project and its ecosystem

List every project directory (a monorepo has several). For each, record:

| Ecosystem | Lockfile | Upgrade command | Verify |
|---|---|---|---|
| Go | `go.sum` | `go get -u ./... && go mod tidy` (direct deps only: `go list -m -u -f '{{if and (not .Indirect) .Update}}{{.Path}}{{end}}' all`) | `go build ./... && go test ./...` |
| npm/pnpm/yarn | `package-lock.json` / `pnpm-lock.yaml` / `yarn.lock` | the repo's manager: `npm update`, `pnpm update`, `yarn up` (minor/patch); majors one package at a time | `<pm> run build` (if defined) and `<pm> test` |
| Gradle | `gradle/libs.versions.toml` or `gradle.lockfile` | edit the catalog / build file versions | `./gradlew build` |
| .NET | `packages.lock.json` | `dotnet list package --outdated`, then `dotnet add package <id> --version <v>` | `dotnet build && dotnet test` |

## 2. Use the repo's package manager, not the machine's

- npm: read `packageManager` in package.json (e.g. `pnpm@9.12.0`) and run that version (`corepack enable` or `npx pnpm@9.12.0`). The lockfile format must not change major.
- Check for global overrides that would win silently: `mise ls --current`, `.tool-versions`, `~/.config/mise/config.toml`. **Warn** if one says `latest` (e.g. `pnpm = "latest"`) and differs from the repo pin; do not proceed with the override.
- Gradle: always `./gradlew`, never a global `gradle`.

## 3. Upgrade direct dependencies only

- Do not edit transitive versions by hand; let the lockfile resolve them.
- Do one ecosystem (one project) at a time, verify, then move on.

## 4. Verify every new version exists and is stable

Before accepting a version you wrote by hand (Gradle catalog, a pinned version in package.json), look it up:

- npm: `npm view <pkg>@<version> version`
- Go: `go list -m <module>@<version>`
- Maven Central: `curl -s "https://search.maven.org/solrsearch/select?q=g:<group>+AND+a:<artifact>+AND+v:<version>&rows=1&wt=json"` (or the repo's mirror)
- NuGet: `curl -s https://api.nuget.org/v3-flatcontainer/<id-lowercase>/index.json`

Reject any version that does not resolve, and any pre-release (`-alpha`, `-beta`, `-rc`, `-M1`, `-SNAPSHOT`, `-next`, `-canary`) unless the current version is already a pre-release.

## 5. Hold back by default

Leave these unchanged unless the user asked for them by name, and list them in the report:

- **Database and infrastructure majors**: postgres, redis, kafka, elasticsearch images or drivers whose major is tied to a server version (e.g. testcontainers `postgres:15` → `16`).
- **Tool pins**: `.tool-versions`, `mise.toml`, `.nvmrc`, `global.json`, `gradle-wrapper.properties`, `packageManager`.
- **Container image tags** in Dockerfiles and compose files.
- **CI action versions** in `.github/workflows` and `.github/actions`.
- **nais manifests**.
- A major bump whose tests fail after one fix attempt: revert that package, keep the rest.

If the user does ask for one of these, verify the new tag/version upstream (step 4; for images `docker manifest inspect <image>:<tag>`; for actions `gh api repos/<owner>/<repo>/git/ref/tags/<tag>`) before writing it.

## 6. Build and test after each project

Run the verify command from step 1. Red tests: fix if the cause is the upgrade and the fix is small; otherwise revert that package and record it. Never disable or delete a test to get green.

## 7. Report

End with one table per project:

| Package | From | To | Status |
|---|---|---|---|
| github.com/foo/bar | v1.2.0 | v1.4.1 | upgraded |
| postgres (testcontainers) | 15 | — | held: DB major, not requested |
| eslint | 8.57.0 | — | held: 9.x broke config, tests red after one fix |

Then the verify command and its result, and any package-manager override warning from step 2.
