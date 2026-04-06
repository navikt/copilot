# 📋 Custom Instructions

Team and project-specific instructions to enhance GitHub Copilot's behavior for Nav technologies and coding practices.

### How to Install

Instructions are `.instructions.md` files placed in your repo's `.github/instructions/` directory. They work across all Copilot-enabled editors.

| Editor | Install Method |
| ------ | -------------- |
| **VS Code** | Click the one-click install button below, or copy the file to `.github/instructions/` |
| **JetBrains** | Copy the file to `.github/instructions/` in your repo. Copilot picks it up automatically. |
| **Copilot CLI** | Copy the file to `.github/instructions/` in your repo. Supported out of the box. |
| **GitHub.com** | Works automatically when the file exists in the repo (Copilot coding agent + code review). |

> All editors read the same `.github/instructions/*.instructions.md` files. See [support matrix](https://docs.github.com/en/copilot/reference/custom-instructions-support) for details.

**Manual install (any editor):**

```bash
# From your project root
mkdir -p .github/instructions
curl -sO --output-dir .github/instructions \
  https://raw.githubusercontent.com/navikt/copilot/main/.github/instructions/<filename>.instructions.md
```

## Available Instructions

<!-- BEGIN GENERATED TABLE -->
| Instruction | Description | VS Code |
| ----------- | ----------- | ------- |
| **Accessibility (UU)**<br/>[View File](../.github/instructions/accessibility.instructions.md) | WCAG 2.1 AA, semantisk HTML, ARIA, Aksel a11y-patterns, keyboard-navigasjon. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Faccessibility.instructions.md) |
| **Database Development**<br/>[View File](../.github/instructions/database.instructions.md) | Flyway migration standards, PostgreSQL schema patterns, safe alterations. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fdatabase.instructions.md) |
| **Dockerfile Standards**<br/>[View File](../.github/instructions/docker.instructions.md) | Multi-stage builds, distroless base images, layer caching, non-root. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fdocker.instructions.md) |
| **GitHub Actions CI/CD**<br/>[View File](../.github/instructions/github-actions.instructions.md) | Action pinning, permissions, Nais deploy, caching, reusable workflows. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fgithub-actions.instructions.md) |
| **Kotlin/Ktor Development**<br/>[View File](../.github/instructions/kotlin-ktor.instructions.md) | ApplicationBuilder pattern, sealed classes, kotliquery/HikariCP, Rapids & Rivers. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fkotlin-ktor.instructions.md) |
| **Kotlin/Spring Development**<br/>[View File](../.github/instructions/kotlin-spring.instructions.md) | Spring Boot patterns: @RestController, @Service, Spring Data. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fkotlin-spring.instructions.md) |
| **Next.js/Aksel Development**<br/>[View File](../.github/instructions/nextjs-aksel.instructions.md) | Aksel spacing tokens (never Tailwind p-/m-), mobile-first responsive design. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fnextjs-aksel.instructions.md) |
| **Testing Standards**<br/>[View File](../.github/instructions/testing.instructions.md) | Kotlin (Kotest/JUnit) and TypeScript (Vitest) test patterns and coverage. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Ftesting.instructions.md) |
<!-- END GENERATED TABLE -->

## Creating Custom Instructions

When creating instructions for Nav projects:

1. **Technology-Specific**: Focus on specific technologies used in Nav (Kotlin, Next.js, PostgreSQL, Kafka)
2. **Nav Patterns**: Include Nav-specific patterns (Rapids & Rivers, ApplicationBuilder, etc.)
3. **Security Standards**: Always reference Nav security requirements
4. **Code Quality**: Enforce strict type checking and quality standards
5. **Norwegian Support**: Include Norwegian language/number formatting requirements
6. **NAIS Integration**: Consider NAIS platform requirements and constraints

## Instruction Categories

### Backend Development
- **Kotlin/Ktor**: ApplicationBuilder pattern, sealed classes for environment config, kotliquery/HikariCP
- **Kafka**: Rapids & Rivers event handling patterns
- **Database**: PostgreSQL migrations, query patterns, connection pooling
- **Testing**: JUnit 5, Mockk, testcontainers

### Frontend Development
- **Next.js**: App Router, Server Components, TypeScript strict mode
- **Aksel Design System**: Component usage, spacing tokens (never Tailwind p-/m-)
- **Responsive Design**: Mobile-first with `xs`, `sm`, `md`, `lg`, `xl` breakpoints
- **Formatting**: Norwegian number formatting (151 354), date/time patterns

### Platform & Deployment
- **NAIS Manifests**: Required endpoints (/isalive, /isready, /metrics)
- **Authentication**: Azure AD, TokenX, ID-porten patterns
- **Observability**: OpenTelemetry auto-instrumentation, Prometheus metrics
- **Security**: Secrets management, network policies, access control

### Code Quality
- **TypeScript**: Strict mode enabled, comprehensive type coverage
- **Kotlin**: Idiomatic patterns, null safety, coroutines best practices
- **Testing**: Unit tests, integration tests, end-to-end tests
- **Documentation**: Norwegian documentation where applicable

## Workspace Structure

```
.github/
├── copilot-instructions.md          # Main workspace instructions
├── instructions/                     # Additional instruction files
│   ├── kotlin-ktor.instructions.md
│   ├── kotlin-spring.instructions.md
│   ├── nextjs-aksel.instructions.md
│   ├── database.instructions.md
│   └── testing.instructions.md
├── agents/                          # Custom agents
├── prompts/                         # Reusable prompts
└── skills/                          # Agent skills
```

## Best Practices

- Keep instructions focused and technology-specific
- Reference official Nav documentation
- Include code examples following Nav patterns
- Consider both local development and NAIS deployment
- Support team autonomy and decision-making
- Maintain consistency across Nav projects
