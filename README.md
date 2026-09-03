# 🤖 Nav Copilot Customizations

![Nav Copilot tools page](docs/assets/my-copilot-hero.png)

Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack. Alt samlet i én verktøyskatalog.

## Hva er dette?

En samling Copilot-tilpasninger for Nav-økosystemet:

<!-- BEGIN GENERATED COUNTS -->
- **🤖 [10 Agenter](docs/README.agents.md)** — Spesialiserte AI-assistenter for Nav-domener
- **📋 [17 Instruksjoner](docs/README.instructions.md)** — Kodestandarder som aktiveres automatisk basert på filmønster
- **⚡ [7 Prompts](docs/README.prompts.md)** — Scaffolding-maler for vanlige Nav-mønstre
- **🎯 [31 Skills](docs/README.skills.md)** — Produksjonsmønstre fra ekte Nav-repoer
- **🔌 [MCP-servere](docs/README.mcp.md)** — Nav-godkjente MCP-servere fra registeret
<!-- END GENERATED COUNTS -->

Alle tilpasninger finnes på **[verktøysida](https://min-copilot.ansatt.nav.no/verktoy)** med søk, filtrering og installeringshjelp.

### Innhold

- [Hva er dette?](#hva-er-dette) (oversikt og nav-pilot)
- [Kom i gang](#kom-i-gang) (installer tilpasninger)
- [Tilpasninger](#tilpasninger) (agenter, instruksjoner, skills, prompts, MCP)
- [Applikasjoner](#️-applikasjoner) (portalen, metrikker, MCP-register, onboarding)
- [Nav tech stack](#️-nav-tech-stack) (stacken tilpasningene dekker)
- [Mappestruktur](#-mappestruktur) (hva ligger hvor)
- [Bidra](#-bidra) (legg til tilpasninger)
- [Team](#-team)
- [Lisens](#-lisens)
- [Ressurser](#-ressurser)

### 🧭 nav-pilot (nytt)

**[nav-pilot](docs/README.nav-pilot.md)** er både et CLI-verktøy og en AI-agent. CLI-et klargjør repoet ditt med riktige agenter, skills og instruksjoner, og setter opp en optimalisert integrasjon med token-optimalisering. Agenten `@nav-pilot` tar deg gjennom fire faser i Copilot Chat: Intervju, Plan, Review og Lever.

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

Enkleste vei er **[min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy)**. Finn det du trenger og følg installeringsstega. MCP-servere har ferdige `code --add-mcp`-kommandoer du kan kopiere rett inn i terminalen.

Vil du lese deg opp først, har hver type sin egen doc i tabellen under.

## Tilpasninger

| Type                | Beskrivelse                                                                                                                 | Dokumentasjon                                      |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| 🤖 **Agenter**       | Spesialiserte AI-assistenter for Nav-domener, kalles med `@agent-name` i Copilot Chat. VS Code, JetBrains, GitHub.com og Copilot CLI | **[Agenter →](docs/README.agents.md)**             |
| 📋 **Instruksjoner** | Kodestandarder som aktiveres automatisk basert på filmønster (`*.kt`, `*.tsx`, `*.sql`). Alle editorer                       | **[Instruksjoner →](docs/README.instructions.md)** |
| ⚡ **Prompts**       | Scaffolding-maler tilgjengelig via `/prompt-name` i Copilot Chat. VS Code, JetBrains og Visual Studio                        | **[Prompts →](docs/README.prompts.md)**            |
| 🎯 **Skills**        | Produksjonsmønstre med innebygde maler og referanser. VS Code, Copilot CLI og GitHub.com, JetBrains i preview. `security-owasp` dekker OWASP Top 10:2025 for Kotlin, Go, Java og Node.js | **[Skills →](docs/README.skills.md)**              |
| 🔌 **MCP-servere**   | Nav-godkjente servere fra [MCP-registeret](https://mcp-registry.nav.no). VS Code, JetBrains, Visual Studio, GitHub.com og Copilot CLI | **[MCP-servere →](docs/README.mcp.md)**            |
| 🔄 **Sync**          | Hold tilpasninger oppdatert automatisk, som Dependabot                                                                      | **[Sync →](docs/README.sync.md)**                  |
| 📦 **Samlinger**     | Installer en hel pakke med agenter, skills og instruksjoner på én gang                                                      | **[Samlinger →](docs/README.collections.md)**      |
| 🧳 **Agentpakke**    | Teamets eget innholdsrepo med manifest, installeres med `nav-pilot install --source`                                        | **[Agentpakke →](docs/README.agentpakke.md)**      |
| 🧭 **nav-pilot**     | CLI-verktøy og AI-agent som installerer og bruker Nav-tilpasninger i Copilot Chat                                           | **[nav-pilot →](docs/README.nav-pilot.md)**        |
| 🧪 **Testing**       | Strukturelle og E2E-tester for nav-pilot                                                                                    | **[Testing →](docs/README.testing.md)**            |

## 🛠️ Applikasjoner

Monorepoet inneholder seks applikasjoner. cplt bor i sitt eget repo.

### cplt

Kernel-level sandbox for AI-agenter. Sandboxer AI-kodingsagenter med OS-primitiver (macOS Seatbelt, Linux Landlock + seccomp-BPF) og blokkerer filsystemtilgang, nettverkstrafikk og credential-exfiltration.

**Repo:** [navikt/cplt](https://github.com/navikt/cplt) · **Docs:** [min-copilot.ansatt.nav.no/cplt](https://min-copilot.ansatt.nav.no/cplt)

```bash
brew install navikt/tap/cplt
```

**Windows (WSL2):** kjør alt inne i Ubuntu, ikke i PowerShell, og bruk installasjonsskriptene i stedet for brew:

```bash
curl -fsSL https://gh.io/copilot-install | bash                                   # Copilot CLI
curl -fsSL https://raw.githubusercontent.com/navikt/cplt/main/install.sh | bash   # cplt
export PATH="$HOME/.local/bin:$PATH"   # skriptene installerer hit
which -a copilot cplt   # ingen treff skal starte med /mnt/c
```

WSL2 arver Windows-PATH. Mangler et verktøy i Ubuntu, plukker terminalen Windows-varianten i stedet, og den kjører via interop som en Windows-prosess — den installerer til Windows-siden og ser ikke Linux-filsystemet slik du forventer. Vanligste fella: `apt install nodejs` gir `node` i Ubuntu, men ikke `npm`, så `npm install -g` havner i Windows-prefixet og henter win32-pakken. Symptomet er `no platform package found` fra `copilot --version`. Jobb også fra Linux-filsystemet (`~/git/...`), ikke `/mnt/c/...` — 9p-I/O er tregt.

### My Copilot

Selvbetjeningsportalen. Administrer Copilot-abonnement, se bruksstatistikk og utforsk tilpasninger fra verktøykatalogen. Har også offentlige sider for [cplt](https://min-copilot.ansatt.nav.no/cplt), [nav-pilot](https://min-copilot.ansatt.nav.no/nav-pilot) og [kom i gang](https://min-copilot.ansatt.nav.no/kom-i-gang).

**URL:** [min-copilot.ansatt.nav.no](https://min-copilot.ansatt.nav.no)

### Copilot API

Go-backend for my-copilot. Håndterer BigQuery-analyser, GitHub API-operasjoner og seat-administrasjon, og bruker Azure AD On-Behalf-Of (OBO) token exchange for sikker kommunikasjon.

- API-endepunkter for bruksdata, adopsjon, tilpasninger, fakturering og seat-administrasjon
- In-memory cache (1t TTL) for BigQuery-data
- Bakgrunnsinnsamling av metrikker hvert 5. minutt
- Audit logging av alle seat-endringer

**Arkitektur:** Se [ARCHITECTURE.md](./ARCHITECTURE.md)

### Copilot Metrics

Naisjob som henter daglige Copilot-bruksmetrikker fra GitHub API og lagrer dem i BigQuery.

### MCP Registry

Offentlig register over Nav-godkjente MCP-servere. Implementerer [MCP Registry v0.1-spesifikasjonen](https://github.com/modelcontextprotocol/registry).

**URL:** [mcp-registry.nav.no](https://mcp-registry.nav.no)

#### Organisasjonsoppsett (allerede konfigurert)

MCP-policyen er satt på organisasjonsnivå og håndheves automatisk for alle med Copilot-sete i navikt. Enkeltbrukere kan ikke endre den.

- **MCP servers in Copilot**: Enabled
- **MCP Registry URL**: `https://mcp-registry.nav.no`
- **Policy**: Registry only (kun servere fra registeret kan brukes)

> **Håndhevelse**: Basert på server name/ID-matching. Lokale servere (som IntelliJ MCP) må ha en oppføring i registeret med ID som matcher nøyaktig det installerte server-ID-et. Se [GitHub docs: MCP allowlist enforcement](https://docs.github.com/en/copilot/reference/mcp-allowlist-enforcement).

#### For IDE-brukere

Registry-servere dukker automatisk opp i MCP-panelet i VS Code og JetBrains, uten oppsett per bruker.

#### For Copilot CLI

```bash
# Bla gjennom tilgjengelige servere
curl -s https://mcp-registry.nav.no/v0.1/servers | jq

# Legg til en server
gh copilot mcp add --url https://mcp-onboarding.nav.no/mcp
```

### MCP Onboarding

MCP-server for å utforske Nav Copilot-tilpasninger, vurdere agent-readiness og generere AGENTS.md rett fra Copilot Chat.

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

## 🏗️ Nav tech stack

Tilpasningene dekker Navs kjernestack:

- **Backend**: Kotlin, Ktor, PostgreSQL, Kafka
- **Frontend**: Next.js 16+, React, TypeScript, Aksel Design System
- **Plattform**: Nais (Kubernetes på GCP)
- **Auth**: Azure AD, TokenX, ID-porten, Maskinporten
- **Observability**: Prometheus, Grafana Loki, Tempo (OpenTelemetry)

## 📖 Mappestruktur

```plaintext
├── agents/               # Custom GitHub Copilot agents (.agent.md)
├── instructions/         # File-pattern-based coding standards (.instructions.md)
├── prompts/              # Task-specific scaffolding templates (.prompt.md)
├── skills/               # Production patterns with bundled resources
├── docs/                 # Detailed documentation for each customization type
│   ├── README.agents.md
│   ├── README.instructions.md
│   ├── README.mcp.md
│   ├── README.prompts.md
│   ├── README.skills.md
│   ├── README.testing.md
│   └── README.collections.md
├── apps/                 # Nav applications
│   ├── copilot-adoption/ # Naisjob that scans navikt repos for customization files
│   ├── copilot-api/      # Go backend API (BigQuery, GitHub API, seat management)
│   ├── copilot-metrics/  # BigQuery data pipeline (Naisjob)
│   ├── mcp-onboarding/   # MCP server for agent readiness
│   ├── mcp-registry/     # MCP server registry
│   └── my-copilot/       # Next.js frontend portal
└── dashboards/           # Grafana dashboard definitions
```

## 🤝 Bidra

### Legg til tilpasninger

1. **Agenter**: Legg til `*.agent.md` i `agents/`, se [agent-dokumentasjonen](docs/README.agents.md)
2. **Instruksjoner**: Legg til `*.instructions.md` i `instructions/`
3. **Prompts**: Legg til `*.prompt.md` i `prompts/`
4. **Skills**: Legg til mappe med `SKILL.md` i `skills/`

Kjør `mise check` etter endringer for å validere alt.

### CI og merge queue

Alle pull requests og merge queue-oppføringer kjører [`ci.yaml`](.github/workflows/ci.yaml). Den produserer én sjekk, **`ci-ok`**, og det er den eneste required status check i branch-rulesettet. Blir den grønn, kan PR-en merges via køen.

- Jobben `changes` bruker [dorny/paths-filter](https://github.com/dorny/paths-filter) til å finne hvilke komponenter endringen berører, med de samme path-listene som komponent-workflowene. Bare de berørte komponentenes sjekker kjøres, så endrer du ingenting relevant blir `ci-ok` grønn med en gang.
- gitleaks-skann av hele historikken kjører alltid, uansett hvilke filer som er endret.
- `ci-ok` feiler hvis en kjørt sjekk feiler eller avbrytes, og lykkes når alt som trengtes er grønt. Hoppede jobber teller som OK.

Komponent-workflowene (`copilot-api.yaml`, `my-copilot.yaml`, osv.) kjører fortsatt på pull requests for PR-preview-deploy til dev, og på push til `main` for deploy til produksjon. De er bevisst *ikke* required checks og kjører ikke i merge-køen, siden deploys ikke skal skje per køoppføring.

### Unngå å committe hemmeligheter

CI skanner hele historikken med [gitleaks](https://github.com/gitleaks/gitleaks). Vil du fange lekkasjer før commit, installer gitleaks lokalt (`brew install gitleaks`) og legg til en valgfri pre-commit-sjekk:

```bash
gitleaks git --pre-commit --staged    # Skanner kun stagede endringer
```

Falske positiver håndteres i [.gitleaks.toml](.gitleaks.toml).

<details>
<summary>Utvikleroppsett for applikasjonene</summary>

**Forutsetninger:** [mise](https://mise.jdx.dev) og [fnox](https://fnox.jdx.dev)

```bash
mise install          # Installer verktøy
lefthook install      # Aktiver pre-commit og commit-msg hooks
```

Hemmeligheter håndteres med fnox + macOS Keychain, ingen `.env`-filer. Hver app har en `fnox.toml` med nødvendige hemmeligheter:

| Service | Apper | Hemmeligheter |
|---|---|---|
| `copilot-portal` | my-copilot | GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, GITHUB_APP_INSTALLATION_ID |
| `copilot-jobs` | copilot-adoption, copilot-metrics, mcp-onboarding | GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, GITHUB_APP_INSTALLATION_ID, SLACK_WEBHOOK_URL |

```bash
cd apps/my-copilot
fnox set GITHUB_APP_ID              # Ber om verdi, lagrer i Keychain
fnox set GITHUB_APP_PRIVATE_KEY
fnox set GITHUB_APP_INSTALLATION_ID
```

Ikke-hemmelig konfig (org-navn, BigQuery-datasett osv.) ligger i `.mise.toml` under `[env]` per app.

**Annen secrets-backend?** `fnox.toml` bruker macOS Keychain som standard, men du kan overstyre med 1Password, GCP Secret Manager osv. i en gitignored `fnox.local.toml`. Se [fnox providers](https://fnox.jdx.dev/providers/).

```bash
cd apps/my-copilot && mise dev      # Starter med hemmeligheter via fnox
```

Se [AGENTS.md](AGENTS.md) for fullstendig utviklerguide.

</details>

## 👥 Team

Vedlikeholdes av **Team Copilot** i Nav IT.

## 📄 Lisens

[MIT](LICENSE)

## 🔗 Ressurser

- [Nais Documentation](https://doc.nais.io)
- [Aksel Design System](https://aksel.Nav.no)
- [Nav GitHub](https://github.com/Navikt)
