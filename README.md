# 🤖 Nav Copilot Customizations

![Nav Copilot tools page](docs/assets/my-copilot-hero.png)

Agenter, instruksjoner, ferdigheter og MCP-servere som gjør GitHub Copilot smartere for Navs stack. Alt samlet i én verktøyskatalog.

## Hva er dette?

En kuratert samling Copilot-tilpasninger for Nav-økosystemet:

- **🤖 [6 Agenter](#-agenter)** — Spesialiserte AI-assistenter for Nais, Aksel, Kafka, Auth, Observability og Sikkerhet
- **📋 [4 Instruksjoner](#-instruksjoner)** — Kodestandarder som aktiveres automatisk basert på filmønster
- **⚡ [3 Prompts](#-prompts)** — Scaffolding-maler for vanlige Nav-mønstre
- **🎯 [5 Ferdigheter](#-ferdigheter)** — Produksjonsmønstre fra ekte Nav-repoer
- **🔌 [6 MCP-servere](#-mcp-servere)** — GitHub, Figma, Next.js, Svelte, Playwright og Nav Onboarding

Alle tilpasninger er tilgjengelige fra **[Verktøy-siden](https://min-copilot.ansatt.nav.no/customizations)** med søk, domenefiltrering og installasjonsinstruksjoner.

## Kom i gang

### Fra verktøysiden (anbefalt)

Gå til **[min-copilot.ansatt.nav.no/customizations](https://min-copilot.ansatt.nav.no/customizations)**, finn det du trenger, og følg installasjonsinstruksjonene. MCP-servere har ferdig `code --add-mcp`-kommandoer du kan kopiere rett inn i terminalen.

### Fra dokumentasjonen

- **[Agenter →](docs/README.agents.md)** — VS Code one-click, JetBrains via coding agent
- **[Instruksjoner →](docs/README.instructions.md)** — Alle editorer
- **[Prompts →](docs/README.prompts.md)** — VS Code, JetBrains
- **[Ferdigheter →](docs/README.skills.md)** — VS Code
- **[MCP-servere →](docs/README.mcp.md)** — Alle editorer

### Med MCP Onboarding

Bruk **MCP Onboarding**-serveren for å utforske tilpasninger, sjekke agent-readiness og generere AGENTS.md — direkte fra Copilot Chat.

---

## 🤖 Agenter

Spesialiserte AI-assistenter for Nav-domener. Bruk med `@agent-name` i Copilot Chat eller ved tildeling av issues til Copilot coding agent.

**Tilgjengelige:** @nais-agent, @auth-agent, @kafka-agent, @aksel-agent, @observability-agent, @security-champion-agent

👉 **[Full dokumentasjon →](docs/README.agents.md)**

---

## 📋 Instruksjoner

Regler som Copilot aktiverer automatisk basert på filmønster (f.eks. `*.kt`, `*.tsx`, `*.sql`).

**Tilgjengelige:** Testing, Kotlin/Ktor, Next.js/Aksel, Database-migrasjoner

👉 **[Full dokumentasjon →](docs/README.instructions.md)**

---

## ⚡ Prompts

Scaffolding-maler tilgjengelig via `/prompt-name` eller `#prompt-name` i Copilot Chat.

**Tilgjengelige:** #aksel-component, #kafka-topic, #nais-manifest

👉 **[Full dokumentasjon →](docs/README.prompts.md)**

---

## 🎯 Ferdigheter

Produksjonsmønstre med innebygde maler og referanser.

**Tilgjengelige:** TokenX Auth, Observability Setup, Aksel Spacing, Kotlin App Config, Flyway Migration

👉 **[Full dokumentasjon →](docs/README.skills.md)**

---

## 🔌 MCP-servere

Nav-godkjente MCP-servere fra [MCP-registeret](https://mcp-registry.nav.no). Serverne dukker automatisk opp i VS Code og JetBrains når registeret er konfigurert på organisasjonsnivå.

| Server                 | Beskrivelse                                              | Type         |
| ---------------------- | -------------------------------------------------------- | ------------ |
| **GitHub MCP**         | Repos, issues, PRs via GitHub API                        | Remote       |
| **Nav MCP Onboarding** | Utforsk tilpasninger, agent-readiness, generer AGENTS.md | Remote       |
| **Figma MCP**          | Designkontekst fra Figma til kode                        | Remote       |
| **Next.js DevTools**   | Diagnostikk og dokumentasjon fra Next.js dev-server      | npm-pakke    |
| **Svelte MCP**         | Søk i Svelte 5/SvelteKit-dokumentasjon                   | Remote + npm |
| **Playwright MCP**     | Browser-automatisering for testing (Nav-sikret)          | npm-pakke    |

👉 **[Full dokumentasjon →](docs/README.mcp.md)**

---

## 🛠️ Applikasjoner

Monorepo med fire applikasjoner:

### My Copilot — Selvbetjeningsportal

Administrer Copilot-abonnement, se bruksanalyse, og utforsk alle tilpasninger fra verktøykatalogen.

**URL:** [min-copilot.ansatt.nav.no](https://min-copilot.ansatt.nav.no)

### Copilot Metrics — BigQuery-datapipeline

Naisjob som henter daglige Copilot-bruksmetrikker fra GitHub API og lagrer i BigQuery.

### MCP Registry — Serveroppdagelse

Offentlig register over Nav-godkjente MCP-servere, implementerer [MCP Registry v0.1-spesifikasjonen](https://github.com/modelcontextprotocol/registry).

**URL:** [mcp-registry.nav.no](https://mcp-registry.nav.no)

#### For Enterprise/Organization Admins

**Enterprise Settings** → **AI Controls** → **MCP**:

1. Enable **MCP servers in Copilot**
2. Set **MCP Registry URL**: `https://mcp-registry.nav.no`
3. Choose policy: **Allow all** (discoverable) or **Registry only** (enforced)

> **Important:** Use the base URL without any path suffix. The Copilot client appends `/v0.1/servers` automatically.

#### For IDE-brukere

Registry-servere dukker automatisk opp i MCP-panelet i VS Code og JetBrains når registeret er konfigurert på organisasjonsnivå. Ingen oppsett per bruker.

#### For Copilot CLI

```bash
# Bla gjennom tilgjengelige servere
curl -s https://mcp-registry.nav.no/v0.1/servers | jq

# Legg til en server
gh copilot mcp add --url https://mcp-onboarding.nav.no/mcp
```

### MCP Onboarding — Agent Readiness

MCP-server for å utforske Nav Copilot-tilpasninger, vurdere agent-readiness og generere AGENTS.md.

**URL:** [mcp-onboarding.nav.no](https://mcp-onboarding.nav.no)

#### Installer

1. Åpne Command Palette i VS Code (`Cmd+Shift+P`)
2. Kjør **MCP: Add Server**
3. Søk etter **Mcp Onboarding** i Nav MCP-registeret
4. Logg inn med GitHub (krever navikt-medlemskap)

#### Bruk i Copilot Chat

```text
List all Nav agents
Search for kafka customizations
Check agent readiness for navikt/fp-sak
Generate AGENTS.md for navikt/fp-sak
Show agent readiness for repos with prefix fp
```

> **Tips:**
>
> - Erstatt `fp-sak` med ditt reponavn i `navikt/`.
> - For `team_readiness`, bruk **repo-prefiks** teamet bruker (f.eks. `fp` for foreldrepenger), ikke fullt teamnavn.

---

## 🏗️ Nav Tech Stack

Tilpasningene dekker Navs kjernestack:

- **Backend**: Kotlin, Ktor, PostgreSQL, Kafka
- **Frontend**: Next.js 16+, React, TypeScript, Aksel Design System
- **Plattform**: Nais (Kubernetes on GCP)
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
│   ├── README.mcp.md
│   ├── README.prompts.md
│   ├── README.skills.md
│   └── README.collections.md
├── apps/                 # Nav applications (my-copilot, copilot-metrics, mcp-registry, mcp-onboarding)
└── dashboards/           # Grafana dashboard definitions
```

## 🎯 Why Use Nav Copilot Customizations?

- **Nav-Specific**: Pre-configured for Nais platform, Aksel Design System, and Nav tech stack
- **Production-Proven**: Patterns extracted from real Nav applications
- **Consistent Standards**: Enforces Nav development principles and best practices

---

## 🤝 Contributing

To add new customizations:

1. **Agents**: Add `*.agent.md` to `.github/agents/` following the [agent naming conventions](#-agenter)
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
