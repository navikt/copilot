# 📦 Copilot Collections

Collections are curated bundles of agents, skills, instructions, and prompts organized by team archetype. Instead of browsing 15+ skills to find the right ones, pick your stack and get a complete, tested package.

All collections include **nav-pilot** — the planning and architecture agent that ties everything together.

## Quick Start

```bash
# Install the nav-pilot CLI
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash

# List available collections
nav-pilot list

# Preview what will be installed
nav-pilot install --dry-run kotlin-backend

# Install into your repo
cd /path/to/your/repo
nav-pilot install kotlin-backend
```

After installing, use `@nav-pilot` in Copilot to start planning your application.

## Available Collections

| Collection | Description | Agents | Skills | Best for |
| --- | --- | --- | --- | --- |
| **kotlin-backend** | Kotlin/Ktor and Spring Boot on Nais | 6 | 10 | Backend API and event consumers |
| **nextjs-frontend** | Next.js with Aksel Design System | 4 | 7 | Innbygger- and saksbehandler-frontends |
| **fullstack** | Complete stack (backend + frontend) | 10 | 13 | Teams that own the full stack |
| **platform** | Nais, observability, security | 4 | 7 | Platform and DevOps teams |

## What's in Each Collection

### kotlin-backend

| Type | Included |
| --- | --- |
| **Agents** | auth, kafka, nais, observability, security-champion, nav-pilot |
| **Skills** | api-design, flyway-migration, kotlin-app-config, observability-setup, security-review, tokenx-auth, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot |
| **Instructions** | kotlin-ktor, kotlin-spring, testing, testing-kotlin, github-actions, docker, database |
| **Prompts** | spring-boot-endpoint, kafka-topic, nais-manifest |

### nextjs-frontend

| Type | Included |
| --- | --- |
| **Agents** | accessibility, aksel, forfatter, nav-pilot |
| **Skills** | aksel-spacing, playwright-testing, web-design-reviewer, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot |
| **Instructions** | nextjs-aksel, testing, testing-typescript, accessibility, github-actions, docker |
| **Prompts** | aksel-component, nextjs-api-route, nais-manifest |

### fullstack

| Type | Included |
| --- | --- |
| **Agents** | accessibility, aksel, auth, code-review, forfatter, kafka, nais, observability, security-champion, nav-pilot |
| **Skills** | aksel-spacing, api-design, flyway-migration, kotlin-app-config, observability-setup, playwright-testing, security-review, tokenx-auth, web-design-reviewer, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot |
| **Instructions** | kotlin-ktor, kotlin-spring, nextjs-aksel, testing, testing-kotlin, testing-typescript, accessibility, github-actions, docker, database |
| **Prompts** | spring-boot-endpoint, kafka-topic, nais-manifest, aksel-component, nextjs-api-route |

### platform

| Type | Included |
| --- | --- |
| **Agents** | nais, observability, security-champion, nav-pilot |
| **Skills** | observability-setup, security-review, workstation-security, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot |
| **Instructions** | github-actions, docker |
| **Prompts** | nais-manifest |

## nav-pilot Planning Skills

Every collection includes four planning skills that form the **nav-pilot pipeline**:

| Skill | Purpose |
| --- | --- |
| `$nav-deep-interview` | Structured interview that exposes blind spots (privacy, auth, dependencies) |
| `$nav-plan` | Architecture decision trees → concrete Nais manifest, CI/CD, project structure |
| `$nav-architecture-review` | Multi-perspective review → Architecture Decision Record (ADR) |
| `$nav-troubleshoot` | Diagnostic trees for pod crashes, auth failures, Kafka lag, DB issues |

Use them via `@nav-pilot` (recommended) or invoke individually.

## Collection Structure

```
.github/collections/
├── kotlin-backend/
│   └── manifest.json       # Lists all agents, skills, instructions, prompts
├── nextjs-frontend/
│   └── manifest.json
├── fullstack/
│   └── manifest.json
└── platform/
    └── manifest.json
```

Each `manifest.json` references items by name. The install script resolves these to actual files from the repository.

## Install Tool

The `nav-pilot` CLI installs collections into your repo. Install it with:

```bash
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash
```

Then use it:

```bash
nav-pilot install <collection>              # Install a collection
nav-pilot install --dry-run <collection>    # Preview what would be installed
nav-pilot install --force <collection>      # Overwrite locally modified files
nav-pilot install --target /path/to/repo <collection>  # Install into a specific repo
nav-pilot list                              # List available collections
nav-pilot status                            # Show what's installed and file integrity
nav-pilot uninstall                         # Remove all installed files
nav-pilot uninstall --dry-run               # Preview what would be removed
nav-pilot version                           # Show version information
```

The tool:
1. Reads `manifest.json` for the chosen collection
2. Copies agents (`.agent.md` + `.metadata.json`) to `.github/agents/`
3. Copies skills (directory with `SKILL.md` + `metadata.json` + `references/`) to `.github/skills/`
4. Copies instructions to `.github/instructions/`
5. Copies prompts to `.github/prompts/`
6. Copies `copilot-instructions.md` if not already present
7. Writes `.github/.nav-pilot-state.json` to track installed files, version, and source SHA

Features:
- **Conflict detection** — warns when target files differ from source; use `--force` to overwrite
- **Install state** — tracks what was installed for safe updates and uninstall
- **Stale file cleanup** — skill directories are replaced entirely, removing old reference files
- **Integrity checks** — `status` command verifies all installed files are intact
- **No external dependencies** — pure Go, no bash/jq required; works on stock macOS

## Keeping Collections Updated

Collections are versioned by date (e.g., `2025.07`). To update:

1. Re-run the install tool with `--force` — it overwrites existing files and updates the state file
2. Or set up the [copilot-customization-sync workflow](https://github.com/navikt/copilot-customization-sync) for automatic weekly PRs

## Creating New Collections

1. Create a directory in `.github/collections/`
2. Add a `manifest.json` listing the items to include
3. Test with `nav-pilot install --dry-run <name>`
4. Update this documentation
