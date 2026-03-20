# 🎯 Agent Skills

Agent Skills are self-contained folders with instructions and bundled resources that enhance AI capabilities for specialized Nav development tasks.

Based on the [Agent Skills specification](https://agentskills.io/specification), each skill contains a `SKILL.md` file with detailed instructions that agents load on-demand.

Skills differ from other primitives by supporting bundled assets (scripts, code samples, reference data) that agents can utilize when performing specialized tasks.

### How to Install

Skills are folders placed in your repo's `.github/skills/` directory.

| Editor          | Install Method                                                                                                                              |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| **VS Code**     | Copy the skill folder to `.github/skills/` in your repo. Agents discover skills automatically.                                              |
| **JetBrains**   | Copy the skill folder to `.github/skills/`. Supported in Agent Mode (public preview — enable via Settings > GitHub Copilot > Chat > Agent). |
| **Copilot CLI** | Copy the skill folder to `.github/skills/`. Full support via `/skills` commands (`/skills list`, `/skills info`, `/skills add`).            |
| **GitHub.com**  | Works with Copilot coding agent when the folder exists in the repo.                                                                         |

> Skills are supported in VS Code, JetBrains (Agent Mode preview), Copilot CLI (full `/skills` management), and GitHub.com (coding agent). Personal skills can be stored in `~/.copilot/skills/` for cross-project use.

**Manual install:**

```bash
# From your project root — install a single skill
mkdir -p .github/skills
# Clone the repo and copy the skill folder
git clone --depth 1 --filter=blob:none --sparse https://github.com/navikt/copilot.git /tmp/nav-copilot
cd /tmp/nav-copilot && git sparse-checkout set .github/skills/<skill-name>
cp -r .github/skills/<skill-name> /path/to/your/repo/.github/skills/
rm -rf /tmp/nav-copilot
```

**When to use skills vs instructions:** Skills are ideal for complex workflows that need bundled resources (templates, scripts, reference data). For simple coding guidelines, use instructions instead.

## Available Skills

<!-- BEGIN GENERATED TABLE -->
| Name | Description | Location |
| ---- | ----------- | -------- |
<!-- | **ai-news-research** | Skriv månedlige oppsummeringer av AI-nyheter for utviklere på norsk med fungerende kildelenker. Bruk for å skrive nyheter, oppsummere AI-trender, lage månedlig oppdatering, eller undersøke hva som er nytt i GitHub Copilot, coding agents, AGENTS.md, skills, memory, agentic workflows eller developer experience. | [`.github/skills/ai-news-research/`](../.github/skills/ai-news-research/SKILL.md) | -->
| **aksel-spacing** | Responsiv layout med Aksel spacing-tokens og Box, VStack, HStack og HGrid | [`.github/skills/aksel-spacing/`](../.github/skills/aksel-spacing/SKILL.md) |
| **api-design** | REST API-designmønstre, versjonering, feilhåndtering (RFC 7807) og OpenAPI-konvensjoner for Nav-tjenester | [`.github/skills/api-design/`](../.github/skills/api-design/SKILL.md) |
| **conventional-commit** | Generer conventional commit-meldinger med Nav-relevante scopes og breaking change-format | [`.github/skills/conventional-commit/`](../.github/skills/conventional-commit/SKILL.md) |
| **flyway-migration** | Databasemigrasjonsmønstre med Flyway og versjonerte SQL-skript | [`.github/skills/flyway-migration/`](../.github/skills/flyway-migration/SKILL.md) |
| **kotlin-app-config** | Sealed class-konfigurasjon for Kotlin-applikasjoner med miljøspesifikke innstillinger | [`.github/skills/kotlin-app-config/`](../.github/skills/kotlin-app-config/SKILL.md) |
| **observability-setup** | Sett opp Prometheus-metrikker, OpenTelemetry-tracing og health check-endepunkter for Nais-applikasjoner | [`.github/skills/observability-setup/`](../.github/skills/observability-setup/SKILL.md) |
| **playwright-testing** | Generer og kjør Playwright E2E-tester for webapplikasjoner med page objects, auth fixtures og tilgjengelighetstester | [`.github/skills/playwright-testing/`](../.github/skills/playwright-testing/SKILL.md) |
| **postgresql-review** | PostgreSQL query review, optimalisering og beste praksis for Nav-applikasjoner | [`.github/skills/postgresql-review/`](../.github/skills/postgresql-review/SKILL.md) |
| **security-review** | Bruk før commit, push eller pull request for å sjekke at koden er trygg å merge | [`.github/skills/security-review/`](../.github/skills/security-review/SKILL.md) |
| **spring-boot-scaffold** | Scaffold et nytt Spring Boot Kotlin-prosjekt med Nais-konfigurasjon, Flyway og standard Nav-mønstre | [`.github/skills/spring-boot-scaffold/`](../.github/skills/spring-boot-scaffold/SKILL.md) |
| **tokenx-auth** | Tjeneste-til-tjeneste-autentisering med TokenX token exchange i Nais | [`.github/skills/tokenx-auth/`](../.github/skills/tokenx-auth/SKILL.md) |
| **web-design-reviewer** | Visuell inspeksjon av nettsider for å identifisere og fikse designproblemer. Trigges av forespørsler som "sjekk designet", "gå gjennom UI-en", "fiks layouten", "finn designfeil". Finner problemer med responsivt design, tilgjengelighet, visuell konsistens og layout, og fikser dem i kildekoden. | [`.github/skills/web-design-reviewer/`](../.github/skills/web-design-reviewer/SKILL.md) |
<!-- END GENERATED TABLE -->

## Creating Nav Skills

When creating agent skills for Nav projects:

1. **Follow Specification**: Adhere to the [Agent Skills specification](https://agentskills.io/specification)
2. **Bundle Resources**: Include templates, scripts, and reference data
3. **Nav Context**: Include Nav-specific patterns and configurations
4. **Self-Contained**: Skills should be independent and reusable
5. **Progressive Disclosure**: Load only when needed for specific tasks

## Skill Structure

```
.github/skills/
└── skill-name/
    ├── SKILL.md              # Main instruction file
    ├── templates/            # Code templates
    ├── scripts/              # Helper scripts
    ├── examples/             # Example implementations
    └── reference/            # Reference documentation
```

## Best Practices

- Keep skills focused on specific domains
- Include practical examples from Nav projects
- Provide clear usage instructions
- Bundle only necessary resources
- Test skills in various Nav contexts
- Document dependencies and requirements
