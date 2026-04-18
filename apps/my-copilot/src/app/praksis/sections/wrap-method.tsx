import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import { LinkableHeading } from "@/components/linkable-heading";
import { BranchingIcon } from "@navikt/aksel-icons";

export default function WrapMethod() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        WRAP-metoden for Coding Agent
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        WRAP er en enkel huskeregel for å få mest mulig ut av Copilot coding agent. Tenk på det som å onboarde en ny
        kollega.
      </BodyShort>

      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
        <Box background="success-soft" padding="space-16" borderRadius="8" className="border-l-4 border-green-600">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-green-600 font-bold text-xl">W</span>
            <Heading size="small" level="3">
              Write
            </Heading>
          </div>
          <BodyShort className="text-gray-600 mb-3">
            Skriv issues som om du forklarer til en ny utvikler på teamet.
          </BodyShort>
          <Box background="default" padding="space-8" borderRadius="4">
            <code className="text-xs block">
              {`Legg til en "Slett bruker"-knapp på
/admin/users siden.

- Knappen skal vises ved hover på rad
- Vis bekreftelsesdialog før sletting
- Kall DELETE /api/users/{id}
- Vis toast ved suksess/feil`}
            </code>
          </Box>
        </Box>

        <Box background="info-soft" padding="space-16" borderRadius="8" className="border-l-4 border-blue-600">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-blue-600 font-bold text-xl">R</span>
            <Heading size="small" level="3">
              Refine
            </Heading>
          </div>
          <BodyShort className="text-gray-600 mb-3">
            Forbedre med copilot-instructions.md og agents.md for konsistente resultater.
          </BodyShort>
          <ul className="space-y-1 text-sm">
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <BodyShort className="text-sm">Definer tech stack og kodestil</BodyShort>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <BodyShort className="text-sm">Spesifiser testmønstre og kommandoer</BodyShort>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <BodyShort className="text-sm">Sett klare grenser (hva den aldri skal gjøre)</BodyShort>
            </li>
          </ul>
        </Box>

        <Box background="warning-soft" padding="space-16" borderRadius="8" className="border-l-4 border-orange-600">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-orange-600 font-bold text-xl">A</span>
            <Heading size="small" level="3">
              Atomic
            </Heading>
          </div>
          <BodyShort className="text-gray-600 mb-3">
            Bryt ned i små, uavhengige oppgaver som kan kjøres parallelt.
          </BodyShort>
          <ul className="space-y-1 text-sm">
            <li className="flex gap-2">
              <span className="text-orange-600">✗</span>
              <BodyShort className="text-sm">&quot;Bygg komplett autentiseringssystem&quot;</BodyShort>
            </li>
            <li className="flex gap-2">
              <span className="text-orange-600">✓</span>
              <BodyShort className="text-sm">&quot;Lag login-skjema med validering&quot;</BodyShort>
            </li>
            <li className="flex gap-2">
              <span className="text-orange-600">✓</span>
              <BodyShort className="text-sm">&quot;Lag JWT token-håndtering&quot;</BodyShort>
            </li>
            <li className="flex gap-2">
              <span className="text-orange-600">✓</span>
              <BodyShort className="text-sm">&quot;Lag protected route middleware&quot;</BodyShort>
            </li>
          </ul>
        </Box>

        <Box background="accent-soft" padding="space-16" borderRadius="8" className="border-l-4 border-purple-600">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-purple-600 font-bold text-xl">P</span>
            <Heading size="small" level="3">
              Pair
            </Heading>
          </div>
          <BodyShort className="text-gray-600 mb-3">
            Jobb sammen med agenten – du eier arkitekturen, den implementerer.
          </BodyShort>
          <ul className="space-y-1 text-sm">
            <li className="flex gap-2">
              <span className="text-purple-600">▪</span>
              <BodyShort className="text-sm">Les session logs for å forstå agentens tankegang</BodyShort>
            </li>
            <li className="flex gap-2">
              <span className="text-purple-600">▪</span>
              <BodyShort className="text-sm">Gi spesifikk tilbakemelding når den sporer av</BodyShort>
            </li>
            <li className="flex gap-2">
              <span className="text-purple-600">▪</span>
              <BodyShort className="text-sm">Bygg videre på PR-en manuelt ved behov</BodyShort>
            </li>
          </ul>
        </Box>
      </HGrid>

      {/* Real-world examples from GitHub */}
      <Box background="info-soft" padding="space-16" borderRadius="8" className="mt-6">
        <div className="flex items-center gap-2 mb-5">
          <BranchingIcon className="text-blue-700" aria-hidden />
          <Heading size="small" level="3" className="text-blue-700">
            Hva GitHub bruker Copilot til internt
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm mb-5">
          GitHub bruker Copilot coding agent aktivt på github.com-kodebasen:
        </BodyShort>
        <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-12">
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Opprydding
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Fjerne utdaterte feature flags, fikse 161 skrivefeil på tvers av 100 filer
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Refaktorering
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Gi nytt navn til klasser brukt overalt i kodebasen</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Feilretting
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Fikse flaky tester, produksjonsfeil, ytelsesproblemer
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Nye features
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Nye API-endepunkter, interne verktøy</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Migrasjoner
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Database-skjemaendringer, sikkerhetsgates</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Analyser
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Audit av feature flags, autorisasjonsanalyse</BodyShort>
          </div>
        </HGrid>
      </Box>
    </Box>
  );
}
