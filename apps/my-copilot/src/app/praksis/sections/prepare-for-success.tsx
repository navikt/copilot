import { Heading, BodyShort, Box, HGrid, HelpText, VStack, Label } from "@navikt/ds-react";
import { Carousel } from "@/components/carousel";
import { CodeBlock } from "@/components/code-block";
import {
  GlobeIcon,
  RocketIcon,
  InformationIcon,
  LightBulbIcon,
  FileTextIcon,
  BulletListIcon,
  CogIcon,
  PencilWritingIcon,
  TasklistIcon,
} from "@navikt/aksel-icons";

export default function PrepareForSuccess() {
  return (
    <div className="space-y-8">
      {/* Language guidance */}
      <Box background="info-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-start gap-2">
          <GlobeIcon className="text-blue-700 mt-0.5" aria-hidden />
          <Heading size="small" level="3" className="text-blue-700">
            Norsk vs. Engelsk
          </Heading>
          <HelpText title="Når bruke hvilket språk?">
            Copilot forstår begge språk godt, men konsistens er viktig for at agenten skal følge mønstrene i koden din.
          </HelpText>
        </div>
        <BodyShort className="text-gray-600 text-sm mt-2">
          <strong>Anbefaling:</strong> Skriv beskrivelser og kommentarer på norsk hvis det passer teamet. Hold kode,
          kommandoer, variabelnavn og tekniske termer på engelsk. Dette matcher vanlig praksis i norske
          utviklingsmiljøer og sikrer at Copilot forstår koden din korrekt.
        </BodyShort>
      </Box>

      {/* Start here: AGENTS.md + copilot-setup-steps */}
      <Box background="success-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-center gap-2 mb-5">
          <RocketIcon className="text-green-700" aria-hidden />
          <Heading size="small" level="3" className="text-green-700">
            Start her – to filer som gir mest effekt
          </Heading>
        </div>
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
          <Box background="default" padding="space-12" borderRadius="4">
            <BodyShort weight="semibold" className="text-sm text-green-700 mb-2">
              1. AGENTS.md (repo-rot)
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs mb-2">
              En universell kontekstfil som fungerer med Copilot, Claude Code, Codex og andre agenter. Beskriv tech
              stack, bygg-kommandoer, kodestil og grenser.
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Tenk på det som onboarding-dokumentet for en ny utvikler – det er nøyaktig det AI-agenter trenger for å
              forstå prosjektet ditt.
            </BodyShort>
          </Box>
          <Box background="default" padding="space-12" borderRadius="4">
            <BodyShort weight="semibold" className="text-sm text-green-700 mb-2">
              2. copilot-setup-steps.yml
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs mb-2">
              GitHub Actions workflow som klargjør miljøet for Copilot coding agent. Uten denne filen kan ikke coding
              agent bygge eller teste koden din.
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Plasseres i <code className="bg-gray-100 px-1 rounded">.github/workflows/copilot-setup-steps.yml</code>.
            </BodyShort>
          </Box>
        </HGrid>
        <BodyShort className="text-gray-600 text-xs mt-3">
          Bruk{" "}
          <a href="/verktoy?item=mcp-io.github.navikt%2Fmcp-onboarding" className="text-blue-600 hover:underline">
            MCP onboarding-serveren
          </a>{" "}
          for å sjekke repoets agent-beredskap og generere begge filene automatisk.
        </BodyShort>
      </Box>

      {/* Comparison table: Prompts vs Instructions vs Agents vs Skills */}
      <Box
        background="warning-soft"
        padding={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        borderRadius="8"
        className="mb-6"
      >
        <div className="flex items-center gap-2 mb-3">
          <InformationIcon className="text-orange-600" aria-hidden />
          <Heading size="small" level="3" className="text-orange-700">
            Fire typer tilpasninger
          </Heading>
        </div>
        <HGrid columns={{ xs: 1, md: 2, lg: 4 }} gap="space-20">
          <VStack gap="space-8">
            <Label size="small" className="text-blue-700">
              <a href="/verktoy?type=prompt" className="hover:underline">
                Prompts
              </a>
            </Label>
            <VStack gap="space-4">
              <BodyShort size="small" className="text-gray-600">
                <strong>Når:</strong> Engangsoppgaver
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Aktivering:</strong> /prompt-name i chat
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Eksempel:</strong> &quot;Lag README for denne modulen&quot;
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Filformat:</strong>{" "}
                <code className="text-xs bg-white/50 px-1 py-0.5 rounded">.github/prompts/*.prompt.md</code>
              </BodyShort>
            </VStack>
          </VStack>
          <VStack gap="space-8">
            <Label size="small" className="text-green-700">
              Instructions
            </Label>
            <VStack gap="space-4">
              <BodyShort size="small" className="text-gray-600">
                <strong>Når:</strong> Alltid aktiv
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Aktivering:</strong> Automatisk på matchende filer
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Eksempel:</strong> TypeScript kodestil, navnekonvensjoner
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Filformat:</strong>{" "}
                <code className="text-xs bg-white/50 px-1 py-0.5 rounded">.github/instructions/*.instructions.md</code>
              </BodyShort>
            </VStack>
          </VStack>
          <VStack gap="space-8">
            <Label size="small" className="text-orange-700">
              <a href="/verktoy?type=agent" className="hover:underline">
                Agents
              </a>
            </Label>
            <VStack gap="space-4">
              <BodyShort size="small" className="text-gray-600">
                <strong>Når:</strong> Spesialiserte oppgaver
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Aktivering:</strong> @agent-name
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Eksempel:</strong> @nais-agent, @aksel-agent, @kafka-agent
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Filformat:</strong>{" "}
                <code className="text-xs bg-white/50 px-1 py-0.5 rounded">.github/agents/*.agent.md</code>
              </BodyShort>
            </VStack>
          </VStack>
          <VStack gap="space-8">
            <Label size="small" className="text-purple-700">
              <a href="/verktoy?type=skill" className="hover:underline">
                Skills
              </a>
            </Label>
            <VStack gap="space-4">
              <BodyShort size="small" className="text-gray-600">
                <strong>Når:</strong> Automatisk ved behov
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Aktivering:</strong> Automatisk når relevant
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Eksempel:</strong> PDF-ekstraksjon, API-testing
              </BodyShort>
              <BodyShort size="small" className="text-gray-600">
                <strong>Filformat:</strong>{" "}
                <code className="text-xs bg-white/50 px-1 py-0.5 rounded">.github/skills/*/SKILL.md</code>
              </BodyShort>
            </VStack>
          </VStack>
        </HGrid>
      </Box>

      <VStack gap="space-32">
        {/* Horizontal scrollable instruction files */}
        <Carousel>
          <VStack gap="space-16" className="w-100">
            <div className="flex items-center gap-2">
              <LightBulbIcon className="text-blue-600" aria-hidden />
              <Heading size="small" level="3">
                Prompts (Engangsoppgaver)
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-xs">Aktiver med /prompt-name i chat.</BodyShort>
            <CodeBlock filename=".github/prompts/create-readme.prompt.md" maxHeight="350px">{`---
name: create-readme
description: Generates a comprehensive README for a project
---

You are a technical documentation expert.

Generate a comprehensive README.md that includes:

1. Project title and description
2. Installation instructions
3. Usage examples with code blocks
4. API documentation (if applicable)
5. Contributing guidelines
6. License information

**Style:**
- Use clear, concise language
- Include code examples in relevant languages
- Use badges for build status, coverage, etc.
- Add a table of contents for long READMEs

**Format:**
Follow the structure used in popular open-source projects.`}</CodeBlock>
          </VStack>

          <VStack gap="space-16" className="w-100">
            <div className="flex items-center gap-2">
              <FileTextIcon className="text-blue-600" aria-hidden />
              <Heading size="small" level="3">
                Repository Instructions
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-xs">Gjelder hele repoet, leses automatisk.</BodyShort>
            <CodeBlock
              filename=".github/copilot-instructions.md"
              maxHeight="350px"
            >{`# Prosjektinstruksjoner for Copilot

## Teknisk stack
- Next.js 15 med App Router
- TypeScript strict mode
- Nav Design System (@navikt/ds-react)
- Tailwind CSS for utilities

## Kodestil
- Bruk funksjonelle komponenter med hooks
- Unngå \`any\`-typer, definer eksplisitte interfaces
- Norske kommentarer, engelsk kode

## Kommandoer
- Test: \`pnpm test\`
- Lint: \`pnpm lint\`
- Build: \`pnpm build\`
- Typecheck: \`pnpm check\``}</CodeBlock>
          </VStack>

          <VStack gap="space-16" className="w-100">
            <div className="flex items-center gap-2">
              <BulletListIcon className="text-green-600" aria-hidden />
              <Heading size="small" level="3">
                AGENTS.md (universell)
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-xs">
              Fungerer med Copilot, Claude Code, Codex og andre agenter.
            </BodyShort>
            <CodeBlock filename="AGENTS.md" maxHeight="350px">{`# AGENTS.md — mitt-prosjekt

## Repository Overview
Backend-tjeneste for brukeradministrasjon.

## Tech Stack
- Kotlin 2.0 med Ktor
- PostgreSQL med Flyway-migrasjoner
- Apache Kafka (Rapids & Rivers)

## Build & Test Commands
\`\`\`bash
./gradlew build   # Build
./gradlew test    # Run tests
\`\`\`

## Code Standards
- Sealed classes for konfigrasjon
- Kotliquery for database-tilgang
- Skriv tester for alle public APIs

## Deployment
- Platform: Nais (Kubernetes on GCP)
- Manifester i \`.nais/\`
- Påkrevde endepunkter: /isalive, /isready, /metrics

## Boundaries
### ✅ Always
- Kjør tester før commit
- Bruk parameteriserte queries
### ⚠️ Ask First
- Endre autentiseringsmekanismer
### 🚫 Never
- Commit secrets til git`}</CodeBlock>
          </VStack>

          <VStack gap="space-16" className="w-100">
            <div className="flex items-center gap-2">
              <RocketIcon className="text-orange-600" aria-hidden />
              <Heading size="small" level="3">
                Copilot Setup Steps
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-xs">Klargjør miljøet for Copilot coding agent.</BodyShort>
            <CodeBlock
              filename=".github/workflows/copilot-setup-steps.yml"
              maxHeight="350px"
            >{`name: Copilot Setup Steps
on: workflow_dispatch

jobs:
  setup:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 22

      - uses: pnpm/action-setup@v4

      - run: pnpm install --frozen-lockfile`}</CodeBlock>
          </VStack>

          <VStack gap="space-16" className="w-100">
            <div className="flex items-center gap-2">
              <CogIcon className="text-green-600" aria-hidden />
              <Heading size="small" level="3">
                Custom Agents
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-xs">Spesialiserte agenter med YAML frontmatter.</BodyShort>
            <CodeBlock filename=".github/agents/test-agent.agent.md" maxHeight="350px">{`---
name: test-agent
description: Skriver tester for dette prosjektet
---

Du er en erfaren QA-ingeniør som skriver tester.

## Din rolle
- Skriv enhetstester og integrasjonstester
- Følg eksisterende testmønstre i prosjektet
- Sikre god testdekning for edge cases

## Kommandoer
- Kjør tester: \`pnpm test\`
- Dekning: \`pnpm test --coverage\`

## Prosjektstruktur
- Tester ligger i \`__tests__/\` eller \`*.test.ts\`
- Bruk Jest og React Testing Library

## Grenser
✅ **Alltid:** Skriv til test-filer, kjør tester før commit
⚠️ **Spør først:** Endre eksisterende tester
🚫 **Aldri:** Slett tester, endre kildekode, commit secrets`}</CodeBlock>
          </VStack>

          <VStack gap="space-16" className="w-100">
            <div className="flex items-center gap-2">
              <PencilWritingIcon className="text-orange-600" aria-hidden />
              <Heading size="small" level="3">
                Path-Specific Instructions
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-xs">Brukes av Copilot Code Review.</BodyShort>
            <CodeBlock filename=".github/instructions/ts.instructions.md" maxHeight="350px">{`---
applyTo: "**/*.ts"
---
# TypeScript Coding Standards

## Naming Conventions
- Variables/functions: \`camelCase\` (getUserData, calculateTotal)
- Classes/interfaces: \`PascalCase\` (UserService, DataController)
- Constants: \`UPPER_SNAKE_CASE\` (API_KEY, MAX_RETRIES)

## Code Style
- Prefer \`const\` over \`let\` when not reassigning
- Use arrow functions for callbacks
- Avoid \`any\` – specify precise types
- Handle all promise rejections with try/catch

## Example
\`\`\`typescript
// ✅ Good
const fetchUser = async (id: string): Promise<User> => {
  if (!id) throw new Error('User ID required');
  return await api.get(\`/users/\${id}\`);
};

// ❌ Bad
async function get(x) {
  return await api.get('/users/' + x).data;
}
\`\`\``}</CodeBlock>
          </VStack>

          <VStack gap="space-16" className="w-100">
            <div className="flex items-center gap-2">
              <CogIcon className="text-purple-600" aria-hidden />
              <Heading size="small" level="3">
                Agent Skills
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-xs">
              Automatisk lastet når relevant. Støtter skript og ressurser.
            </BodyShort>
            <CodeBlock filename=".github/skills/pdf-extractor/SKILL.md" maxHeight="350px">{`---
name: pdf-extractor
description: Extracts text and form fields from PDF files
---

You are an expert at extracting information from PDF documents.

## Your role
- Extract text content from PDF files
- Identify and extract form fields
- Preserve document structure and formatting

## Tools available
This skill includes a Python script for PDF extraction:
\`\`\`bash
python scripts/extract_pdf.py <path-to-pdf>
\`\`\`

## Output format
Return extracted data as structured JSON:
\`\`\`json
{
  "text": "Full document text...",
  "fields": [
    {"name": "field1", "value": "...", "type": "text"}
  ]
}
\`\`\`

## Guidelines
- Maintain original text formatting
- Preserve table structures
- Extract metadata (author, dates, etc.)

## Boundaries
✅ **Always:** Validate PDF file exists, handle errors gracefully
⚠️ **Ask first:** Processing PDFs larger than 50MB
🚫 **Never:** Modify source PDF files`}</CodeBlock>
          </VStack>
        </Carousel>

        {/* Six core areas */}
        <Box background="info-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
          <div className="flex items-center gap-2 mb-5">
            <BulletListIcon className="text-blue-700" aria-hidden />
            <Heading size="small" level="3" className="text-blue-700">
              Seks kjerneområder (fra 2500+ repos)
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-5">
            Analyse av over 2500 agents.md-filer viser at de beste dekker disse områdene:
          </BodyShort>
          <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-12">
            <div>
              <BodyShort weight="semibold" className="text-sm">
                1. Kommandoer
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">Kjørbare kommandoer tidlig: npm test, pnpm build</BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                2. Testing
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">Testrammeverk, hvor tester ligger, coverage</BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                3. Prosjektstruktur
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">Mappestruktur, hvor kode hører hjemme</BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                4. Kodestil
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">Kodeeksempler over forklaringer</BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                5. Git-workflow
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">Branch-strategi, commit-meldinger</BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                6. Grenser
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">Hva agenten aldri skal gjøre</BodyShort>
            </div>
          </HGrid>
        </Box>
      </VStack>

      {/* Readiness checklist */}
      <Box background="accent-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mt-6">
        <div className="flex items-center gap-2 mb-5">
          <TasklistIcon className="text-blue-600" aria-hidden />
          <Heading size="small" level="3">
            Er repoet ditt agent-klart?
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-xs mb-3">
          Sjekk disse punktene for å gjøre repoet ditt klart for AI-agenter. Tilpasninger + verifikasjon = 14 poeng
          totalt.
        </BodyShort>
        <BodyShort weight="semibold" className="text-xs mb-2">
          Tilpasninger (8 poeng)
        </BodyShort>
        <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-12" className="mb-4">
          <div>
            <BodyShort weight="semibold" className="text-xs">
              AGENTS.md
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Universell kontekst (alle agenter)</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              copilot-setup-steps.yml
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Coding agent miljøoppsett</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              copilot-instructions.md
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Copilot-spesifikke instruksjoner</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              .github/instructions/
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Filtype-spesifikke regler</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              .github/agents/
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Spesialiserte agenter</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              .github/prompts/
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Gjenbrukbare prompts</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              .github/skills/
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Kapabiliteter med skript</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              .github/hooks/
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Automatisk lint/format</BodyShort>
          </div>
        </HGrid>
        <BodyShort weight="semibold" className="text-xs mb-2">
          Verifikasjonsinfrastruktur (6 poeng)
        </BodyShort>
        <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-12">
          <div>
            <BodyShort weight="semibold" className="text-xs">
              CI/CD workflows
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Automatisert bygg og test i GitHub Actions</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              Linter-konfigurasjon
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">ESLint, golangci-lint, detekt</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              Typesjekking
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">tsconfig.json, Go/Kotlin/Rust (innebygd)</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              Testkonfigurasjon
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Jest, Vitest, eller innebygd (Go, JVM)</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              Dependabot
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Automatiske avhengighetsoppdateringer</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-xs">
              README.md
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Dokumentasjon agenter leser først</BodyShort>
          </div>
        </HGrid>
      </Box>
    </div>
  );
}
