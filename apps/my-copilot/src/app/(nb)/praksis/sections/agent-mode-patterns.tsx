import NextLink from "next/link";
import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import { CodeBlock } from "@/components/code-block";
import {
  BookIcon,
  TestFlaskIcon,
  MagnifyingGlassIcon,
  LinkIcon,
  ShieldLockIcon,
  FileTextIcon,
} from "@navikt/aksel-icons";

export default function AgentModePatterns() {
  return (
    <div className="space-y-8">
      <BodyShort className="text-gray-600 text-sm">
        Nav har egne agenter i organisasjonen. Se alle på{" "}
        <NextLink href="/verktoy?type=agent" className="text-blue-600 hover:underline">
          verktøy-siden
        </NextLink>
        .
      </BodyShort>
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <Box background="info-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <BookIcon className="text-blue-700" aria-hidden />
            <Heading size="small" level="3">
              <NextLink href="/verktoy?item=aksel-agent" className="hover:underline">
                @aksel-agent
              </NextLink>
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Aksel og frontend</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Bygg og refaktorer UI med Aksel</li>
            <li>• Bruk tokens og layout-primitiver</li>
            <li>• Migrer til nye Aksel-versjoner</li>
          </ul>
        </Box>

        <Box background="success-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <LinkIcon className="text-green-700" aria-hidden />
            <Heading size="small" level="3">
              <NextLink href="/verktoy?item=kafka-agent" className="hover:underline">
                @kafka-agent
              </NextLink>
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Kafka og hendelser</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Rapids & Rivers-mønstre</li>
            <li>• Hendelsesdrevet arkitektur</li>
            <li>• Schema-design</li>
          </ul>
        </Box>

        <Box background="warning-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <MagnifyingGlassIcon className="text-orange-700" aria-hidden />
            <Heading size="small" level="3">
              <NextLink href="/verktoy?item=code-review" className="hover:underline">
                @code-review
              </NextLink>
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Kodegjennomgang</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Finn feil og sikkerhetsproblemer</li>
            <li>• Sjekk Nav-konvensjoner</li>
            <li>• Se over før du ber en kollega om review</li>
          </ul>
        </Box>

        <Box background="accent-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <FileTextIcon className="text-blue-600" aria-hidden />
            <Heading size="small" level="3">
              <NextLink href="/verktoy?item=research-agent" className="hover:underline">
                @research-agent
              </NextLink>
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Undersøk før du endrer</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Utforsk ukjente kodebaser</li>
            <li>• Samle kontekst før implementering</li>
            <li>• Finn årsaken til et problem</li>
          </ul>
        </Box>

        <Box background="danger-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <ShieldLockIcon className="text-red-700" aria-hidden />
            <Heading size="small" level="3">
              <NextLink href="/verktoy?item=security-champion-agent" className="hover:underline">
                @security-champion-agent
              </NextLink>
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Sikkerhet</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Trusselmodellering</li>
            <li>• Navs sikkerhetsarkitektur</li>
            <li>• Sikkerhetspraksis og compliance</li>
          </ul>
        </Box>

        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <TestFlaskIcon className="text-gray-700" aria-hidden />
            <Heading size="small" level="3">
              <NextLink href="/verktoy?item=accessibility-agent" className="hover:underline">
                @accessibility-agent
              </NextLink>
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Universell utforming</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• WCAG 2.1 og 2.2</li>
            <li>• Tilgjengelighet i Aksel</li>
            <li>• Automatisert UU-testing</li>
          </ul>
        </Box>

        <Box background="info-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <BookIcon className="text-blue-700" aria-hidden />
            <Heading size="small" level="3">
              <NextLink href="/verktoy?item=forfatter" className="hover:underline">
                @forfatter
              </NextLink>
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Norsk tekst</BodyShort>
          <ul className="space-y-1 text-xs">
            <li>• Klarspråk og mikrotekst</li>
            <li>• Fjern KI-markører og anglisismer</li>
            <li>• Riktige fagtermer</li>
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
          Følger GitHubs anbefalte rekkefølge: Kommandoer → Testing → Prosjektstruktur → Kodestil → Git-arbeidsflyt →
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
    </div>
  );
}
