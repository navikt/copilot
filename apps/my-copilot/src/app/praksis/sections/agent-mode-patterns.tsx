import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import {
  BookIcon,
  TestFlaskIcon,
  MagnifyingGlassIcon,
  LinkIcon,
  ShieldLockIcon,
  RocketIcon,
  FileTextIcon,
} from "@navikt/aksel-icons";

export default function AgentModePatterns() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        Vanlige mønstre for Agent Mode
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        Bygg spesialiserte agenter for repeterende oppgaver. Her er seks anbefalte agenter å starte med.
      </BodyShort>

      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <Box background="info-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <BookIcon className="text-blue-700" aria-hidden />
            <Heading size="small" level="3">
              @docs-agent
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Dokumentasjonsassistent</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Oppdater README ved API-endringer</li>
            <li>• Generer JSDoc/docstrings</li>
            <li>• Lag CHANGELOG-oppføringer</li>
          </ul>
        </Box>

        <Box background="success-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <TestFlaskIcon className="text-green-700" aria-hidden />
            <Heading size="small" level="3">
              @test-agent
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Testskriving</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Skriv enhetstester for ny kode</li>
            <li>• Øk testdekning på moduler</li>
            <li>• Fiks flaky tester</li>
          </ul>
        </Box>

        <Box background="warning-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <MagnifyingGlassIcon className="text-orange-700" aria-hidden />
            <Heading size="small" level="3">
              @lint-agent
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Kodeformatering</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Fiks linting-feil</li>
            <li>• Migrer til ny ESLint-config</li>
            <li>• Fjern ubrukt kode</li>
          </ul>
        </Box>

        <Box background="accent-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <LinkIcon className="text-blue-600" aria-hidden />
            <Heading size="small" level="3">
              @api-agent
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">API-utvikling</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Lag nye endepunkter</li>
            <li>• Generer OpenAPI-spec</li>
            <li>• Valider request/response</li>
          </ul>
        </Box>

        <Box background="danger-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <ShieldLockIcon className="text-red-700" aria-hidden />
            <Heading size="small" level="3">
              @security-agent
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Sikkerhetssjekk</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Audit avhengigheter</li>
            <li>• Finn sikkerhetshull</li>
            <li>• Foreslå fixes</li>
          </ul>
        </Box>

        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <RocketIcon className="text-gray-700" aria-hidden />
            <Heading size="small" level="3">
              @deploy-agent
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Dev/Deploy-hjelp</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Oppdater Dockerfile</li>
            <li>• Fiks CI-config</li>
            <li>• Miljøvariabler</li>
          </ul>
        </Box>
      </HGrid>

      {/* Example agent file */}
      <Box background="info-soft" padding="space-16" borderRadius="8" className="mt-4">
        <div className="flex items-center gap-2 mb-5">
          <FileTextIcon className="text-blue-700" aria-hidden />
          <Heading size="small" level="3" className="text-blue-700">
            Eksempel: .github/agents/test-agent.agent.md
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-xs mb-2">
          Følger GitHub sin anbefalte rekkefølge: Kommandoer → Testing → Prosjektstruktur → Kodestil → Git-workflow →
          Grenser
        </BodyShort>
        <CodeBlock filename=".github/agents/test-agent.agent.md">{`---
name: test-agent
description: Skriver tester for dette prosjektet
---

## Kommandoer
- Kjør tester: pnpm test
- Dekning: pnpm test --coverage
- Watch mode: pnpm test --watch

## Testing
- Testrammeverk: Jest + React Testing Library
- Mål: 80% coverage på nye filer

## Prosjektstruktur
- Tester: src/__tests__/ eller ved siden av fil som *.test.ts
- Mocks: src/__mocks__/

## Kodestil
- Bruk describe/it-blokker
- Test én ting per test
- Unngå implementasjonsdetaljer

## Git-workflow
- Commit-melding: "test: <beskrivelse>"
- Kjør tester før push

## Grenser
- ✅ Alltid: Kjør tester før commit
- ⚠️ Spør først: Endre eksisterende tester
- 🚫 Aldri: Slett tester uten godkjenning`}</CodeBlock>
      </Box>
    </Box>
  );
}
