import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import { LinkableHeading } from "@/components/linkable-heading";
import { TestFlaskIcon, MagnifyingGlassIcon, CogIcon, TasklistIcon } from "@navikt/aksel-icons";

export default function Verification() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        Verifisering – Nøkkelen til Kvalitet
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        &quot;Gi Copilot en måte å verifisere arbeidet sitt – dette 2-3x kvaliteten.&quot; En god plan er viktig, men
        verifisering er det som sikrer at resultatet faktisk fungerer.
      </BodyShort>

      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="mb-6">
        <Box background="success-soft" padding="space-16" borderRadius="8" className="border-l-4 border-green-600">
          <div className="flex items-center gap-2 mb-5">
            <TestFlaskIcon className="text-green-700" aria-hidden />
            <Heading size="small" level="3" className="text-green-700">
              Be om tester i prompten
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Inkluder testing som del av oppgaven:</BodyShort>
          <Box background="default" padding="space-8" borderRadius="4">
            <code className="text-xs block whitespace-pre-wrap font-mono">
              {`Lag en funksjon som validerer
norske fødselsnumre.

Skriv enhetstester og kjør dem
før du anser oppgaven som ferdig.`}
            </code>
          </Box>
        </Box>

        <Box background="info-soft" padding="space-16" borderRadius="8" className="border-l-4 border-blue-600">
          <div className="flex items-center gap-2 mb-5">
            <MagnifyingGlassIcon className="text-blue-700" aria-hidden />
            <Heading size="small" level="3" className="text-blue-700">
              La Copilot reviewe seg selv
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-2">Etter implementering, be om selvreview:</BodyShort>
          <Box background="default" padding="space-8" borderRadius="4">
            <code className="text-xs block whitespace-pre-wrap font-mono">
              {`Review koden du nettopp skrev.
Sjekk for:
- Bugs og edge cases
- Sikkerhetsrisikoer
- Brudd på kodestil`}
            </code>
          </Box>
        </Box>
      </HGrid>

      {/* Knip tool */}
      <Box background="warning-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-center gap-2 mb-5">
          <CogIcon className="text-orange-700" aria-hidden />
          <Heading size="small" level="3" className="text-orange-700">
            Knip – Rydd opp etter Copilot
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm mb-5">
          Copilot kan etterlate ubrukt kode, avhengigheter og exports. Knip finner og fjerner dette automatisk. Brukes
          av Vercel, Anthropic, Cloudflare og TanStack.
        </BodyShort>
        <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Installer og kjør
            </BodyShort>
            <Box background="default" padding="space-8" borderRadius="4" className="mt-1">
              <code className="text-xs block font-mono">pnpm knip</code>
            </Box>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Hva Knip finner
            </BodyShort>
            <ul className="text-xs text-gray-600 mt-1 space-y-1">
              <li>• Ubrukte filer og exports</li>
              <li>• Ubrukte npm-avhengigheter</li>
              <li>• Ubrukte typer og interfaces</li>
            </ul>
          </div>
        </HGrid>
        <BodyShort className="text-gray-500 text-xs mt-3">
          &quot;Knip helped us delete ~300k lines of unused code at Vercel.&quot; –{" "}
          <a
            href="https://knip.dev/"
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            knip.dev
          </a>
        </BodyShort>
      </Box>

      {/* Verification checklist */}
      <Box background="accent-soft" padding="space-16" borderRadius="8">
        <div className="flex items-center gap-2 mb-5">
          <TasklistIcon className="text-blue-600" aria-hidden />
          <Heading size="small" level="3">
            Verifiseringssjekkliste
          </Heading>
        </div>
        <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-12">
          <div>
            <BodyShort weight="semibold" className="text-sm">
              1. Tester
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Kjør testsuiten, sjekk coverage</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              2. Linting
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">ESLint, TypeScript, Prettier</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              3. Knip
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Fjern ubrukt kode og deps</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              4. Manuell test
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Test i browser/preview</BodyShort>
          </div>
        </HGrid>
      </Box>
    </Box>
  );
}
