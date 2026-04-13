# 🤖 Custom Agents

Spesialiserte AI-assistenter for Nav-domener. Bruk med `@agent-name` i Copilot Chat.

📖 **Utforsk og installer:** [min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy)

### Installer

```bash
mkdir -p .github/agents
curl -sO --output-dir .github/agents \
  https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/<filename>.agent.md
```

Eller bruk one-click install-knappene nedenfor (VS Code).

## Tilgjengelige agenter

<!-- BEGIN GENERATED TABLE -->
| Agent | Description | VS Code |
| ----- | ----------- | ------- |
| **Accessibility Agent**<br/>[`@accessibility-agent`](../.github/agents/accessibility.agent.md) | WCAG 2.1/2.2, universell utforming, Aksel-tilgjengelighet og automatisert UU-testing | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Faccessibility.agent.md) |
| **Aksel Agent**<br/>[`@aksel-agent`](../.github/agents/aksel.agent.md) | Navs Aksel Design System — spacing-tokens, responsiv layout og komponentmønstre | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Faksel.agent.md) |
| **Auth Agent**<br/>[`@auth-agent`](../.github/agents/auth.agent.md) | Azure AD, TokenX, ID-porten, Maskinporten og JWT-validering for Nav-apper | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fauth.agent.md) |
| **Code Review Agent**<br/>[`@code-review-agent`](../.github/agents/code-review.agent.md) | Kodegjennomgang for Nav-applikasjoner — finner feil, sikkerhetsproblemer og brudd på Nav-konvensjoner | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fcode-review.agent.md) |
| **Forfatter**<br/>[`@forfatter`](../.github/agents/forfatter.agent.md) | Norsk teknisk redaktør: klarspråk, AI-markører, anglismer, fagtermer, mikrotekst. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fforfatter.agent.md) |
| **Kafka Agent**<br/>[`@kafka-agent`](../.github/agents/kafka.agent.md) | Rapids & Rivers, eventdrevet arkitektur, Kafka-mønstre og schema-design | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fkafka.agent.md) |
| **Nais Agent**<br/>[`@nais-agent`](../.github/agents/nais.agent.md) | Nais-deployment, GCP-ressurser, Kafka-topics og feilsøking på plattformen | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fnais.agent.md) |
| **Nav Pilot**<br/>[`@nav-pilot`](../.github/agents/nav-pilot.agent.md) | Planlegg, arkitekturer og bygg Nav-applikasjoner med innebygd kjennskap til Nais, auth, Kafka, sikkerhet og Nav-mønstre | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fnav-pilot.agent.md) |
| **Observability Agent**<br/>[`@observability-agent`](../.github/agents/observability.agent.md) | Prometheus-metrikker, OpenTelemetry-tracing, Grafana-dashboards og varsling | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fobservability.agent.md) |
| **Research Agent**<br/>[`@research-agent`](../.github/agents/research.agent.md) | Utforsker kodebaser, undersøker problemer og samler kontekst før implementering | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fresearch.agent.md) |
| **Rust Agent**<br/>[`@rust-agent`](../.github/agents/rust.agent.md) | Idiomatisk Rust-utvikling med cargo, clippy, error handling, async/tokio, unsafe og testing | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Frust.agent.md) |
| **Security Champion Agent**<br/>[`@security-champion-agent`](../.github/agents/security-champion.agent.md) | Navs sikkerhetsarkitektur, trusselmodellering, compliance og sikkerhetspraksis | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fsecurity-champion.agent.md) |
<!-- END GENERATED TABLE -->

## For bidragsytere

Legg til nye agenter som `.agent.md`-filer i `.github/agents/`. Se [AGENTS.md](../AGENTS.md) for retningslinjer.
