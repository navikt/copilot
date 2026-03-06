# 📋 Custom Instructions

Team and project-specific instructions to enhance GitHub Copilot's behavior for NAV technologies and coding practices.

### How to Use Custom Instructions

**To Install:**
- Click the **VS Code** or **VS Code Insiders** install button for the instruction you want to use
- Download the `*.instructions.md` file and manually add it to your project's instruction collection

**To Use/Apply:**
- Copy these instructions to your `.github/copilot-instructions.md` file in your workspace
- Create task-specific `.github/instructions/*.instructions.md` files in your workspace's `.github/instructions` folder
- Instructions automatically apply to Copilot behavior once installed in your workspace

## Available Instructions

| Instruction | Description | Install |
| ----------- | ----------- | ------- |
| **Kotlin/Ktor Development**<br/>[View File](../.github/instructions/kotlin-ktor.instructions.md) | Guidelines for Kotlin/Ktor backend development following NAV patterns. ApplicationBuilder pattern, sealed classes for environment config, kotliquery/HikariCP, Rapids & Rivers. | [![Install in VS Code](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fkotlin-ktor.instructions.md)<br/>[![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode-insiders%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fkotlin-ktor.instructions.md) |
| **Kotlin/Spring Development**<br/>[View File](../.github/instructions/kotlin-spring.instructions.md) | Spring Boot patterns for NAV backends. @RestController, @Service, Spring Data. Use for Spring apps — for Ktor/Rapids & Rivers, use Kotlin/Ktor instead. | [![Install in VS Code](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fkotlin-spring.instructions.md)<br/>[![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode-insiders%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fkotlin-spring.instructions.md) |
| **Next.js/Aksel Development**<br/>[View File](../.github/instructions/nextjs-aksel.instructions.md) | Next.js 16+ with Aksel Design System patterns. **CRITICAL**: Enforces Aksel spacing tokens instead of Tailwind padding/margin. Mobile-first responsive design, loading states, component patterns. | [![Install in VS Code](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fnextjs-aksel.instructions.md)<br/>[![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode-insiders%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fnextjs-aksel.instructions.md) |
| **Database Development**<br/>[View File](../.github/instructions/database.instructions.md) | PostgreSQL best practices and migration patterns for NAV. Flyway migration standards: naming conventions, schema patterns, safe alterations. | [![Install in VS Code](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fdatabase.instructions.md)<br/>[![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode-insiders%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Fdatabase.instructions.md) |
| **Testing Standards**<br/>[View File](../.github/instructions/testing.instructions.md) | Testing guidelines for Kotlin (JUnit), TypeScript (Jest), and integration tests. Standards for Kotlin (Kotest) and TypeScript (Jest) tests with coverage requirements. | [![Install in VS Code](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Ftesting.instructions.md)<br/>[![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode-insiders%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Ftesting.instructions.md) |

## Creating Custom Instructions

When creating instructions for NAV projects:

1. **Technology-Specific**: Focus on specific technologies used in NAV (Kotlin, Next.js, PostgreSQL, Kafka)
2. **NAV Patterns**: Include NAV-specific patterns (Rapids & Rivers, ApplicationBuilder, etc.)
3. **Security Standards**: Always reference NAV security requirements
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
- Reference official NAV documentation
- Include code examples following NAV patterns
- Consider both local development and NAIS deployment
- Support team autonomy and decision-making
- Maintain consistency across NAV projects
