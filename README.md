# 🤖 Nav Copilot Customizations

A curated collection of GitHub Copilot customizations for building Nav applications following Nav's development standards, including Aksel design system, Nais platform patterns, and Security Playbook.

## 🚀 What is Nav Copilot Customizations?

This repository provides specialized GitHub Copilot customizations for the Nav ecosystem:

- **🤖 [6 Custom Agents](#-agents)** - Specialized AI assistants for Nav-specific domains (Nais, Aksel, Kafka, Auth, Observability, Security)
- **📋 [4 Instructions](#-instructions)** - File-pattern-based coding standards that apply automatically
- **⚡ [3 Prompts](#-prompts)** - Quick scaffolding templates for common Nav patterns
- **🎯 [5 Skills](#-skills)** - Production-proven patterns extracted from real Nav repositories

## 🔧 How to Use

### Quick Install (One-Click)

Install customizations directly in VS Code using install badges in the documentation:

- **[View all Agents →](docs/README.agents.md)** - Click to install individual agents
- **[View all Instructions →](docs/README.instructions.md)** - Click to install coding standards
- **[View all Prompts →](docs/README.prompts.md)** - Click to install scaffolding templates

### Discover & Search with MCP

Use the **Mcp Onboarding** server to browse customizations, assess agent readiness, and generate AGENTS.md — all from Copilot Chat.

#### Install from Nav MCP Registry

1. Open Command Palette (`Cmd+Shift+P`)
2. Run **MCP: Add Server**
3. Search for **Mcp Onboarding**
4. Sign in with GitHub when prompted (requires navikt org membership)

#### Use in Copilot Chat

Once installed, ask Copilot naturally:

```text
List all Nav agents
Search for kafka customizations
Check agent readiness for navikt/my-app
Generate AGENTS.md for navikt/my-app
Show agent readiness for the dagpenger team
```

**Available Tools:**

- `list_agents`, `list_instructions`, `list_prompts`, `list_skills` — browse customizations
- `search_customizations` — search by query, type, or tags
- `check_agent_readiness` — 14-point scorecard for agent readiness
- `suggest_customizations` — stack-aware recommendations
- `generate_agents_md`, `generate_setup_steps` — generate files for your repo
- `team_readiness` — scan all team repos

### Install with VS Code Tasks

Run the task: **"Install Nav Copilot Customizations"** from VS Code tasks menu (`Cmd+Shift+P` → "Tasks: Run Task")

Or install individually:

- **Install Copilot Instructions** - Main project instructions
- **Install All Agents** - All 6 specialized agents
- **Install All Instructions** - All 4 file-pattern rules
- **Install All Prompts** - All 3 scaffolding templates
- **Install All Skills** - All 5 production patterns

---

## 🤖 Agents

Specialized AI assistants for the Nav ecosystem. Use them by calling `@agent-name` in Copilot Chat or when assigning issues to Copilot coding agent.

**Available agents:** @nais-agent, @auth-agent, @kafka-agent, @aksel-agent, @observability-agent, @security-champion-agent

👉 **[View full agent documentation →](docs/README.agents.md)**

---

## 📋 Instructions

File-pattern-based rules that Copilot applies automatically when creating or modifying files matching specific patterns.

**Available instructions:** Testing, Kotlin/Ktor, Next.js/Aksel, Database migrations

👉 **[View full instructions documentation →](docs/README.instructions.md)**

---

## ⚡ Prompts

Quick scaffolding templates accessible via Copilot Chat using `/prompt-name` or `#prompt-name`.

**Available prompts:** #aksel-component, #kafka-topic, #nais-manifest

👉 **[View full prompts documentation →](docs/README.prompts.md)**

---

## 🎯 Skills

Production patterns extracted from real Nav repositories with bundled templates and resources.

**Available skills:** TokenX Auth, Observability Setup, Aksel Spacing, Kotlin App Config, Flyway Migration

👉 **[View full skills documentation →](docs/README.skills.md)**

---

## 🏗️ Nav Development Standards

These customizations enforce Nav's core principles:

### Principles

- **Team First** - Autonomous teams with circles of autonomy
- **Product Development** - Continuous development over ad hoc approaches
- **Essential Complexity** - Focus on essential, avoid accidental complexity
- **DORA Metrics** - Measure and improve team performance

### Tech Stack

- **Backend**: Kotlin, Ktor, PostgreSQL, Kafka
- **Frontend**: Next.js 16+, React, TypeScript, Aksel Design System
- **Platform**: Nais (Kubernetes on GCP)
- **Auth**: Azure AD, TokenX, ID-porten, Maskinporten
- **Observability**: Prometheus, Grafana Loki, Tempo (OpenTelemetry)

## 📖 Repository Structure

```plaintext
├── .github/
│   ├── agents/           # Custom GitHub Copilot agents (.agent.md)
│   ├── instructions/     # File-pattern-based coding standards (.instructions.md)
│   ├── prompts/          # Task-specific scaffolding templates (.prompt.md)
│   └── skills/           # Production patterns with bundled resources
├── docs/                 # Detailed documentation for each customization type
│   ├── README.agents.md
│   ├── README.instructions.md
│   ├── README.prompts.md
│   ├── README.skills.md
│   └── README.collections.md
└── apps/                 # Nav applications (my-copilot, mcp-registry, mcp-onboarding)
```

## 🎯 Why Use Nav Copilot Customizations?

- **Nav-Specific**: Pre-configured for Nais platform, Aksel Design System, and Nav tech stack
- **Production-Proven**: Patterns extracted from real Nav applications
- **Consistent Standards**: Enforces Nav development principles and best practices
- **Developer Productivity**: Reduces context-switching and repetitive setup work

---

## 📦 Applications

### my-copilot

Self-service tool for managing GitHub Copilot subscriptions at Nav.

- **Location**: `apps/my-copilot/`
- **Tech**: Next.js 16, TypeScript, Aksel Design System, Octokit
- **Auth**: Azure AD JWT validation via Nais sidecar proxy
- **Deployment**: Nais (dev-gcp, prod-gcp)

**Commands:**

```bash
cd apps/my-copilot
pnpm dev        # Start dev server
pnpm check      # Run all checks (ESLint, TypeScript, Prettier, Knip, Jest)
pnpm test       # Run Jest tests
pnpm build      # Production build
```

### mcp-registry

Public registry service for Nav-approved MCP servers.

- **Location**: `apps/mcp-registry/`
- **Tech**: Go 1.26, HTTP server implementing MCP Registry v0.1 spec
- **Public URL**: `https://mcp-registry.nav.no`
- **Purpose**: Enables GitHub Copilot enterprise to discover and use approved MCP servers

**Commands:**

```bash
cd apps/mcp-registry
mise run dev       # Run with DEBUG logging
mise run check     # Run all checks (fmt, vet, staticcheck, lint, test)
mise run validate  # Validate allowlist.json
```

### mcp-onboarding

MCP server for Nav Copilot onboarding — discover customizations, assess agent readiness, and generate setup files.

- **Location**: `apps/mcp-onboarding/`
- **Tech**: Go 1.26, OAuth 2.1 with PKCE, MCP JSON-RPC
- **Registry**: **Mcp Onboarding** (`io.github.navikt/mcp-onboarding`)
- **Features**:
  - 🔐 GitHub OAuth with Nav organization validation
  - 🔍 Browse and search customizations (agents, instructions, prompts, skills)
  - 📊 14-point agent readiness assessment (customizations + verification infrastructure)
  - 📝 Generate AGENTS.md and copilot-setup-steps.yml tailored to repo tech stack
  - 👥 Team-wide readiness scanning

**Commands:**

```bash
cd apps/mcp-onboarding
mise run generate  # Generate customizations manifest from .github files
mise run dev       # Run with DEBUG logging
mise run check     # Run all checks (fmt, vet, lint, test)
mise run build     # Build binary
```

---

## 🤝 Contributing

To add new customizations:

1. **Agents**: Add `*.agent.md` to `.github/agents/` following the [agent naming conventions](#-agents)
2. **Instructions**: Add `*.instructions.md` to `.github/instructions/`
3. **Prompts**: Add `*.prompt.md` to `.github/prompts/`
4. **Skills**: Add folder with `SKILL.md` to `.github/skills/`

For detailed contribution guidelines and development setup, see [AGENTS.md](AGENTS.md).

---

## 📄 License

See [LICENSE](LICENSE) file.

---

## 🔗 Resources

- [Nais Documentation](https://doc.nais.io)
- [Aksel Design System](https://aksel.Nav.no)
- [Nav GitHub](https://github.com/Navikt)
