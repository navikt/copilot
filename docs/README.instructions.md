# 📋 Custom Instructions

Kodestandarder og teknologispesifikke retningslinjer som påvirker Copilots oppførsel i ditt repo.

📖 **Utforsk og installer:** [min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy)

### Installer

```bash
mkdir -p .github/instructions
curl -sO --output-dir .github/instructions \
  https://raw.githubusercontent.com/navikt/copilot/main/.github/instructions/<filename>.instructions.md
```

## Tilgjengelige instruksjoner

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
| **Testing Kotlin**<br/>[View File](../.github/instructions/testing-kotlin.instructions.md) |  | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Ftesting-kotlin.instructions.md) |
| **Testing Typescript**<br/>[View File](../.github/instructions/testing-typescript.instructions.md) |  | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Ftesting-typescript.instructions.md) |
| **Testing Standards**<br/>[View File](../.github/instructions/testing.instructions.md) | Kotlin (Kotest/JUnit) and TypeScript (Vitest) test patterns and coverage. | [![Install](https://img.shields.io/badge/VS_Code-Install-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://min-copilot.ansatt.nav.no/install/instructions?url=vscode%3Achat-instructions%2Finstall%3Furl%3Dhttps%3A%2F%2Fraw.githubusercontent.com%2Fnavikt%2Fcopilot%2Fmain%2F.github%2Finstructions%2Ftesting.instructions.md) |
<!-- END GENERATED TABLE -->

## For bidragsytere

Legg til nye instruksjoner som `.instructions.md`-filer i `.github/instructions/`. Se [AGENTS.md](../AGENTS.md) for retningslinjer.
