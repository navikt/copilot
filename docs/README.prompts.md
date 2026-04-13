# 🎯 Reusable Prompts

Gjenbrukbare prompt-maler for vanlige Nav-utviklingsoppgaver. Bruk med `/prompt-name` i Copilot Chat.

📖 **Utforsk og installer:** [min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy)

### Installer

```bash
mkdir -p .github/prompts
curl -sO --output-dir .github/prompts \
  https://raw.githubusercontent.com/navikt/copilot/main/.github/prompts/<filename>.prompt.md
```

## Tilgjengelige prompts

<!-- BEGIN GENERATED TABLE -->
| Prompt | Description | VS Code |
| ------ | ----------- | ------- |
| **#aksel-component**<br/>[View File](../.github/prompts/aksel-component.prompt.md) | Scaffold en responsiv React-komponent med Aksel Design System og riktige spacing-tokens | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Faksel-component.prompt.md) |
| **#kafka-topic**<br/>[View File](../.github/prompts/kafka-topic.prompt.md) | Legg til Kafka-topic-konfigurasjon i Nais-manifest og lag Rapids & Rivers event handler | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fkafka-topic.prompt.md) |
| **#nais-manifest**<br/>[View File](../.github/prompts/nais-manifest.prompt.md) | Generer et produksjonsklart Nais-applikasjonsmanifest for Kubernetes-deployment | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fnais-manifest.prompt.md) |
| **#nextjs-api-route**<br/>[View File](../.github/prompts/nextjs-api-route.prompt.md) | Scaffold en Next.js App Router API-rute med validering, feilhåndtering, auth og test | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fnextjs-api-route.prompt.md) |
| **#spring-boot-endpoint**<br/>[View File](../.github/prompts/spring-boot-endpoint.prompt.md) | Scaffold et Spring Boot REST-endepunkt med Controller, Service, Repository, Test og Nais-konfig | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/prompt?url=vscode%3Achat-prompt%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fprompts%2Fspring-boot-endpoint.prompt.md) |
<!-- END GENERATED TABLE -->

## For bidragsytere

Legg til nye prompts som `.prompt.md`-filer i `.github/prompts/`. Se [AGENTS.md](../AGENTS.md) for retningslinjer.
