# 🤖 Custom Agents

Custom agents for Nav's GitHub Copilot ecosystem, specialized for Norwegian public sector development patterns.

### How to Install

Agents are `.agent.md` files placed in your repo's `.github/agents/` directory.

| Editor          | Install Method                                                                                    |
| --------------- | ------------------------------------------------------------------------------------------------- |
| **VS Code**     | Click the one-click install button below, or copy the file to `.github/agents/`                   |
| **JetBrains**   | Copy the file to `.github/agents/`. Supported in Copilot Chat and coding agent.                   |
| **Copilot CLI** | Copy the file to `.github/agents/`. Select with `/agent` command in CLI sessions.                 |
| **GitHub.com**  | Create and manage agents at github.com/copilot/agents, or add files to `.github/agents/` in repo. |

> Custom agents (`.agent.md`) are supported in VS Code, JetBrains, Copilot CLI and GitHub.com. See [support matrix](https://docs.github.com/en/copilot/reference/custom-instructions-support) for details.

**Manual install (any editor):**

```bash
# From your project root
mkdir -p .github/agents
curl -sO --output-dir .github/agents \
  https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/<filename>.agent.md
```

**To use in VS Code:** Type `@agent-name` in Copilot Chat after installing.

## Available Agents

<!-- BEGIN GENERATED TABLE -->
| Agent | Description | VS Code |
| ----- | ----------- | ------- |
| **Accessibility Agent**<br/>[`@accessibility-agent`](../.github/agents/accessibility.agent.md) | Ekspert på WCAG 2.1/2.2, universell utforming, Aksel accessibility-mønstre og automatisert UU-testing | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Faccessibility.agent.md) |
| **Aksel Agent**<br/>[`@aksel-agent`](../.github/agents/aksel.agent.md) | Ekspert på Navs Aksel Design System, spacing-tokens, responsiv layout og komponentmønstre | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Faksel.agent.md) |
| **Auth Agent**<br/>[`@auth-agent`](../.github/agents/auth.agent.md) | Ekspert på Azure AD, TokenX, ID-porten, Maskinporten og JWT-validering for Nav-applikasjoner | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fauth.agent.md) |
| **Code Review Agent**<br/>[`@code-review-agent`](../.github/agents/code-review.agent.md) | Kodegjennomgang for Nav-applikasjoner — finner feil, sikkerhetsproblemer og brudd på Nav-konvensjoner | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fcode-review.agent.md) |
| **Forfatter**<br/>[`@forfatter`](../.github/agents/forfatter.agent.md) | Norsk teknisk redaktør: klarspråk, AI-markører, anglismer, fagtermer, mikrotekst. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fforfatter.agent.md) |
| **Kafka Agent**<br/>[`@kafka-agent`](../.github/agents/kafka.agent.md) | Ekspert på Rapids & Rivers eventdrevet arkitektur, Kafka-mønstre og event schema-design | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fkafka.agent.md) |
| **Nais Agent**<br/>[`@nais-agent`](../.github/agents/nais.agent.md) | Ekspert på Nais-deployment, GCP-ressurser, Kafka-topics og plattform-feilsøking | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fnais.agent.md) |
| **Observability Agent**<br/>[`@observability-agent`](../.github/agents/observability.agent.md) | Ekspert på Prometheus-metrikker, OpenTelemetry-tracing, Grafana-dashboards og varsling | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fobservability.agent.md) |
| **Research Agent**<br/>[`@research-agent`](../.github/agents/research.agent.md) | Ekspert på å utforske kodebaser, undersøke problemer, analysere mønstre og samle kontekst før implementering | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fresearch.agent.md) |
| **Security Champion Agent**<br/>[`@security-champion-agent`](../.github/agents/security-champion.agent.md) | Ekspert på Navs sikkerhetsarkitektur, trusselmodellering, compliance og helhetlig sikkerhetspraksis | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/agent?url=vscode%3Achat-agent%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Fagents%2Fsecurity-champion.agent.md) |
<!-- END GENERATED TABLE -->

## Creating Custom Agents

When creating new agents for Nav projects:

1. **Follow Nav Standards**: Align with Nav development principles (Team First, Essential Complexity, DORA Metrics)
2. **Include Context**: Reference Nav tech stack (Kotlin/Ktor, Next.js, NAIS)
3. **Security First**: Always consider security implications and Nav security policies
4. **Norwegian Language**: Support Norwegian text and number formatting where applicable
5. **Platform Integration**: Ensure compatibility with NAIS deployment patterns

## Agent Guidelines

- Agents should be self-contained and focused on specific domains
- Include clear examples and common use cases
- Reference relevant Nav documentation and standards
- Support both local development and NAIS-deployed scenarios
- Consider mobile-first design for frontend-related agents
