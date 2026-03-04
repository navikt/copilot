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

See [MCP Onboarding](#mcp-onboarding--agent-readiness--customization-discovery) below for installation and usage.

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

---

## 🛠️ Applications

This monorepo contains three applications:

### My Copilot — Self-Service Portal

Self-service portal for managing your GitHub Copilot subscription (activate/deactivate, view usage analytics, billing details).

**URL:** [`https://my-copilot.intern.nav.no`](https://my-copilot.intern.nav.no)

### MCP Registry — Server Discovery

Public registry of Nav-approved MCP servers, implementing the [MCP Registry v0.1 specification](https://github.com/modelcontextprotocol/registry).

**URL:** [`https://mcp-registry.nav.no`](https://mcp-registry.nav.no)

#### For Enterprise/Organization Admins

**Enterprise Settings** → **AI Controls** → **MCP**:

1. Enable **MCP servers in Copilot**
2. Set **MCP Registry URL**: `https://mcp-registry.nav.no`
3. Choose policy: **Allow all** (discoverable) or **Registry only** (enforced)

> **Important:** Use the base URL without any path suffix. The Copilot client appends `/v0.1/servers` automatically. Including route suffixes like `/v0.1/servers` or `/allowlist` will cause the registry to error out.

#### For IDE Users (VS Code, JetBrains, Xcode, Eclipse)

Registry servers appear automatically in the MCP servers sidebar panel when configured at the enterprise/organization level. No per-user setup is needed.

#### For Copilot CLI Users

Copilot CLI does not have a built-in registry browser. Add servers from the registry manually:

1. Browse available servers: `curl -s https://mcp-registry.nav.no/v0.1/servers | jq`
2. Add a server interactively: `/mcp add` in the CLI
3. Or edit `~/.copilot/mcp-config.json` directly:

```json
{
  "mcpServers": {
    "mcp-onboarding": {
      "type": "http",
      "url": "https://mcp-onboarding.nav.no/mcp",
      "tools": ["*"]
    }
  }
}
```

> **Note:** Copilot CLI does not support OAuth authentication. MCP servers requiring OAuth (like mcp-onboarding) will not work from the CLI. Servers using PAT or no authentication work fine.

Enterprise allowlist policies still apply to Copilot CLI — if "Registry only" is set, only servers listed in the registry can be used.

### MCP Onboarding — Agent Readiness & Customization Discovery

MCP server for browsing Nav Copilot customizations, assessing agent readiness, and generating AGENTS.md — all from Copilot Chat.

**URL:** [`https://mcp-onboarding.nav.no`](https://mcp-onboarding.nav.no)

#### Install

1. Open Command Palette in VS Code (`Cmd+Shift+P`)
2. Run **MCP: Add Server**
3. Search for **Mcp Onboarding** in the Nav MCP registry
4. Sign in with GitHub when prompted (requires navikt org membership)

#### Use in Copilot Chat

Once installed, ask Copilot naturally:

```text
List all Nav agents
Search for kafka customizations
Check agent readiness for navikt/fp-sak
Generate AGENTS.md for navikt/fp-sak
Show agent readiness for repos with prefix fp
```

> **Tips:**
> - Replace `fp-sak` with your actual repo name in `navikt/`.
> - For `team_readiness`, use the **repo name prefix** your team uses (e.g. `fp` for foreldrepenger, `dp` for dagpenger), not the full team name. Most Nav teams use short prefixes for their repos.

#### Typical Onboarding Workflow

1. **Assess** — `check_agent_readiness` → see what's missing
2. **Generate** — `generate_agents_md` + `generate_setup_steps` → get tailored files
3. **Customize** — `suggest_customizations` → discover Nav-specific agents, instructions, and skills for your stack
4. **Track** — `team_readiness` → monitor adoption across your team's repos

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
├── apps/                 # Nav applications (my-copilot, mcp-registry, mcp-onboarding)
└── dashboards/           # Grafana dashboard definitions
```

## 🎯 Why Use Nav Copilot Customizations?

- **Nav-Specific**: Pre-configured for Nais platform, Aksel Design System, and Nav tech stack
- **Production-Proven**: Patterns extracted from real Nav applications
- **Consistent Standards**: Enforces Nav development principles and best practices

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
