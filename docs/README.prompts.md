# 🎯 Reusable Prompts

Reusable prompt templates for Nav development scenarios and tasks, optimized for Norwegian public sector workflows.

### How to Install

Prompts are `.prompt.md` files placed in your repo's `.github/prompts/` directory.

| Editor            | Install Method                                                                     |
| ----------------- | ---------------------------------------------------------------------------------- |
| **VS Code**       | Click the one-click install button below, or copy the file to `.github/prompts/`   |
| **JetBrains**     | Copy the file to `.github/prompts/` in your repo. Use with `/prompt-name` in chat. |
| **Visual Studio** | Copy the file to `.github/prompts/` in your repo. Use with `/prompt-name` in chat. |
| **Copilot CLI**   | Not supported.                                                                     |
| **GitHub.com**    | Not supported.                                                                     |

> Prompt files are supported in VS Code, JetBrains, and Visual Studio. See [support matrix](https://docs.github.com/en/copilot/reference/custom-instructions-support) for details.

**Manual install (any editor):**

```bash
# From your project root
mkdir -p .github/prompts
curl -sO --output-dir .github/prompts \
  https://raw.githubusercontent.com/navikt/copilot/main/.github/prompts/<filename>.prompt.md
```

**To run:** Use `/prompt-name` in Copilot Chat, or run `Chat: Run Prompt` from the Command Palette.

## Available Prompts

<!-- BEGIN GENERATED TABLE -->
| Prompt | Description | VS Code |
| ------ | ----------- | ------- |
| **#aksel-component**<br/>[View File](../.github/prompts/aksel-component.prompt.md) | Scaffold en responsiv React-komponent med Aksel Design System og riktige spacing-tokens | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Faksel-component.prompt.md) |
| **#kafka-topic**<br/>[View File](../.github/prompts/kafka-topic.prompt.md) | Legg til Kafka-topic-konfigurasjon i Nais-manifest og lag Rapids & Rivers event handler | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fkafka-topic.prompt.md) |
| **#nais-manifest**<br/>[View File](../.github/prompts/nais-manifest.prompt.md) | Generer et produksjonsklart Nais-applikasjonsmanifest for Kubernetes-deployment | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fnais-manifest.prompt.md) |
| **#nextjs-api-route**<br/>[View File](../.github/prompts/nextjs-api-route.prompt.md) | Scaffold en Next.js App Router API-rute med validering, feilhåndtering, auth og test | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fnextjs-api-route.prompt.md) |
| **#spring-boot-endpoint**<br/>[View File](../.github/prompts/spring-boot-endpoint.prompt.md) | Scaffold et Spring Boot REST-endepunkt med Controller, Service, Repository, Test og Nais-konfig | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fspring-boot-endpoint.prompt.md) |
<!-- END GENERATED TABLE -->

## Creating Nav Prompts

When creating reusable prompts for Nav projects:

1. **Norwegian Context**: Support Norwegian language and formatting requirements
2. **NAIS Platform**: Include NAIS deployment and configuration patterns
3. **Security**: Always include security considerations for public sector
4. **Accessibility**: Follow UU (universal design) requirements
5. **Mobile-First**: Default to mobile-first responsive design patterns
6. **Team Autonomy**: Respect team decision-making and autonomous practices

## Prompt Categories

### Platform & Infrastructure
- NAIS manifest generation
- Kubernetes configuration
- Google Cloud Platform setup
- Network policies and security

### Authentication & Authorization
- Azure AD integration
- TokenX configuration
- ID-porten setup
- Maskinporten integration

### Frontend Development
- Aksel Design System implementation
- Next.js 16 patterns
- Norwegian number/date formatting
- Responsive design validation

### Backend Development
- Kotlin/Ktor application structure
- PostgreSQL schema design
- Kafka event handling
- Rapids & Rivers implementation

### Observability
- Prometheus metrics setup
- Grafana Loki configuration
- Tempo tracing implementation
- Alert rule creation

## Best Practices

- Keep prompts focused on single, well-defined tasks
- Include practical examples from Nav projects
- Reference official Nav documentation
- Consider both dev and prod environments
- Support automated testing and validation
