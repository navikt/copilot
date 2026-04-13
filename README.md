# 🤖 Nav Copilot Customizations

![Nav Copilot tools page](docs/assets/my-copilot-hero.png)

Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack. Alt samlet i én verktøyskatalog.

## Hva er dette?

En samling Copilot-tilpasninger for Nav-økosystemet:

<!-- BEGIN GENERATED COUNTS -->
- **🤖 [12 Agenter](docs/README.agents.md)** — Spesialiserte AI-assistenter for Nav-domener
- **📋 [13 Instruksjoner](docs/README.instructions.md)** — Kodestandarder som aktiveres automatisk basert på filmønster
- **⚡ [7 Prompts](docs/README.prompts.md)** — Scaffolding-maler for vanlige Nav-mønstre
- **🎯 [21 Skills](docs/README.skills.md)** — Produksjonsmønstre fra ekte Nav-repoer
- **🔌 [MCP-servere](docs/README.mcp.md)** — Nav-godkjente MCP-servere fra registeret
<!-- END GENERATED COUNTS -->

Alle tilpasninger finnes på **[verktøysida](https://min-copilot.ansatt.nav.no/verktoy)** med søk, filtrering og installeringshjelp.

### 🧭 nav-pilot — Nytt!

**[nav-pilot](docs/README.nav-pilot.md)** er Navs AI-utviklerverktøy — én agent med en 4-fase modell (Intervju → Plan → Review → Lever) som koder inn Navs institusjonelle kunnskap. Installer en samling, bruk `@nav-pilot`, og gå fra idé til Nav-kompatibel arkitekturplan.

```bash
# Installer nav-pilot CLI
brew install navikt/tap/nav-pilot

# Installer Kotlin-backend-samlingen i repoet ditt
cd /path/to/your/repo
nav-pilot install kotlin-backend

# Bruk i Copilot
@nav-pilot Jeg trenger en ny tjeneste som behandler dagpengesøknader
```

**[Les mer →](docs/README.nav-pilot.md)** · **[Samlinger →](docs/README.collections.md)**

## Kom i gang

### Fra verktøysiden (anbefalt)

Gå til **[min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy)**, finn det du trenger og følg installeringsstega. MCP-servere har ferdige `code --add-mcp`-kommandoer du kan kopiere rett inn i terminalen.

### Fra dokumentasjonen

- **[Agenter →](docs/README.agents.md)** — VS Code one-click, JetBrains via coding agent
- **[Instruksjoner →](docs/README.instructions.md)** — Alle editorer
- **[Prompts →](docs/README.prompts.md)** — VS Code, JetBrains
- **[Skills →](docs/README.skills.md)** — VS Code
- **[MCP-servere →](docs/README.mcp.md)** — Alle editorer
- **[Samlinger →](docs/README.collections.md)** — Installer alt på én gang
- **[nav-pilot →](docs/README.nav-pilot.md)** — Navs AI-utviklerverktøy
- **[Testing →](docs/README.testing.md)** — Strukturelle og E2E-tester for nav-pilot
- **[Hold tilpasninger oppdatert →](docs/README.sync.md)** — Automatisk sync-workflow (som Dependabot)

### Med MCP Onboarding

Bruk **MCP Onboarding**-serveren for å utforske tilpasninger, sjekke agent-readiness og generere AGENTS.md — direkte fra Copilot Chat.

---

## Tilpasninger

| Type                | Beskrivelse                                                                             | Dokumentasjon                                      |
| ------------------- | --------------------------------------------------------------------------------------- | -------------------------------------------------- |
| 🤖 **Agenter**       | Spesialiserte AI-assistenter for Nav-domener — bruk med `@agent-name` i Copilot Chat    | **[Agenter →](docs/README.agents.md)**             |
| 📋 **Instruksjoner** | Kodestandarder som aktiveres automatisk basert på filmønster (`*.kt`, `*.tsx`, `*.sql`) | **[Instruksjoner →](docs/README.instructions.md)** |
| ⚡ **Prompts**       | Scaffolding-maler tilgjengelig via `#prompt-name` i Copilot Chat                        | **[Prompts →](docs/README.prompts.md)**            |
| 🎯 **Skills**        | Produksjonsmønstre med innebygde maler og referanser                                    | **[Skills →](docs/README.skills.md)**              |
| 🔌 **MCP-servere**   | Nav-godkjente servere fra [MCP-registeret](https://mcp-registry.nav.no)                 | **[MCP-servere →](docs/README.mcp.md)**            |
| 🔄 **Sync**          | Hold tilpasninger oppdatert automatisk (som Dependabot)                                 | **[Sync →](docs/README.sync.md)**                  |
| 📦 **Samlinger**     | Installer en hel pakke med agenter, skills og instruksjoner på én gang                  | **[Samlinger →](docs/README.collections.md)**      |
| 🧭 **nav-pilot**     | Planleggingsagent som koder inn Navs institusjonelle kunnskap                           | **[nav-pilot →](docs/README.nav-pilot.md)**        |

---

## 🛠️ Applikasjoner

Monorepo med fire applikasjoner:

### My Copilot — Selvbetjeningsportal

Administrer Copilot-abonnement, se bruksstatistikk og utforsk tilpasninger fra verktøykatalogen.

**URL:** [min-copilot.ansatt.nav.no](https://min-copilot.ansatt.nav.no)

### Copilot Metrics — BigQuery-datapipeline

Naisjob som henter daglige Copilot-bruksmetrikker fra GitHub API og lagrer i BigQuery.

### MCP Registry — MCP-register

Offentlig register over Nav-godkjente MCP-servere. Implementerer [MCP Registry v0.1-spesifikasjonen](https://github.com/modelcontextprotocol/registry).

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
- **Plattform**: Nais (Kubernetes på GCP)
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
│   ├── README.testing.md
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

### Development Setup

**Prerequisites:** [mise](https://mise.jdx.dev) and [fnox](https://fnox.jdx.dev)

```bash
mise install          # Install all tools
```

**Secrets** are managed with fnox + macOS Keychain — no `.env` files needed. Each app has a `fnox.toml` listing required secrets. Two Keychain services are used:

| Service | Apps | Secrets |
|---|---|---|
| `copilot-portal` | my-copilot | GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, GITHUB_APP_INSTALLATION_ID |
| `copilot-jobs` | copilot-adoption, copilot-metrics, mcp-onboarding | GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, GITHUB_APP_INSTALLATION_ID, SLACK_WEBHOOK_URL |

```bash
cd apps/my-copilot
fnox set GITHUB_APP_ID              # Prompts for value, stores in Keychain
fnox set GITHUB_APP_PRIVATE_KEY
fnox set GITHUB_APP_INSTALLATION_ID

cd ../copilot-adoption
fnox set GITHUB_APP_ID              # Different GitHub App — different credentials
fnox set GITHUB_APP_PRIVATE_KEY
fnox set GITHUB_APP_INSTALLATION_ID
fnox set SLACK_WEBHOOK_URL
```

Non-secret config (org names, BigQuery datasets, etc.) is in each app's `.mise.toml` under `[env]`.

**Using a different secret backend?** The committed `fnox.toml` defaults to macOS Keychain, but you can override with any provider (1Password, GCP Secret Manager, etc.) in a gitignored `fnox.local.toml`:

```toml
# fnox.local.toml — your personal override
[providers]
op = { type = "1password", vault = "Nav Dev" }

[secrets]
GITHUB_APP_ID = { provider = "op", value = "copilot-portal/GITHUB_APP_ID" }
```

See [fnox providers](https://fnox.jdx.dev/providers/) for all supported backends.

**Run an app:**

```bash
cd apps/my-copilot && mise dev      # Injects secrets via fnox automatically
cd apps/copilot-adoption && mise dev
```

### Adding Customizations

To add new customizations:

1. **Agents**: Add `*.agent.md` to `.github/agents/` following the [agent docs](docs/README.agents.md)
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
