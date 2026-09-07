import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import {
  CheckmarkCircleIcon,
  XMarkOctagonIcon,
  ExclamationmarkTriangleIcon,
  LinkIcon,
  InformationIcon,
  CogIcon,
} from "@navikt/aksel-icons";

export default function OrchestrateAgents() {
  return (
    <div className="space-y-8">
      {/* Mission Control Hero Image */}
      <div className="mb-6 rounded-lg overflow-hidden border border-gray-200 shadow-sm relative aspect-video">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src="/images/agents-on-github-hero-mission-control.jpeg"
          alt="Mission Control dashboard for Copilot agenter"
          className="w-full h-full object-cover"
        />
      </div>

      <div className="space-y-6">
        {/* Parallel vs Sequential */}
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
          <Box background="success-soft" padding="space-16" borderRadius="8">
            <div className="flex items-center gap-2 mb-2">
              <CheckmarkCircleIcon className="text-green-700" aria-hidden />
              <Heading size="small" level="3" className="text-green-700">
                Parallelt (uavhengige oppgaver)
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-sm mb-2">
              Start flere agenter samtidig når oppgavene ikke påvirker hverandre:
            </BodyShort>
            <ul className="space-y-1 text-xs">
              <li className="flex gap-2">
                <span className="text-green-600">✓</span>
                <span>Dokumentasjon for ulike moduler</span>
              </li>
              <li className="flex gap-2">
                <span className="text-green-600">✓</span>
                <span>Tester for forskjellige features</span>
              </li>
              <li className="flex gap-2">
                <span className="text-green-600">✓</span>
                <span>Code review av separate PR-er</span>
              </li>
              <li className="flex gap-2">
                <span className="text-green-600">✓</span>
                <span>Research på ulike teknologier</span>
              </li>
            </ul>
          </Box>

          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <div className="flex items-center gap-2 mb-2">
              <LinkIcon className="text-orange-700" aria-hidden />
              <Heading size="small" level="3" className="text-orange-700">
                Sekvensielt (avhengige oppgaver)
              </Heading>
            </div>
            <BodyShort className="text-gray-600 text-sm mb-2">Vent på én agent før du starter neste:</BodyShort>
            <ul className="space-y-1 text-xs">
              <li className="flex gap-2">
                <span className="text-orange-600">→</span>
                <span>1. Lag database-schema</span>
              </li>
              <li className="flex gap-2">
                <span className="text-orange-600">→</span>
                <span>2. Lag API som bruker schema</span>
              </li>
              <li className="flex gap-2">
                <span className="text-orange-600">→</span>
                <span>3. Lag frontend som kaller API</span>
              </li>
              <li className="flex gap-2">
                <span className="text-orange-600">→</span>
                <span>4. Lag tester for hele stacken</span>
              </li>
            </ul>
          </Box>
        </HGrid>

        {/* Reading Signals */}
        <Box background="info-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-5">
            <InformationIcon className="text-blue-700" aria-hidden />
            <Heading size="small" level="3" className="text-blue-700">
              Les agentens signaler
            </Heading>
          </div>
          <BodyShort className="text-gray-600 text-sm mb-5">
            Session logs viser agentens tankegang. Se etter disse tegnene:
          </BodyShort>
          <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-12">
            <div>
              <div className="flex items-center gap-1">
                <CheckmarkCircleIcon className="text-green-700" fontSize="1rem" aria-hidden />
                <BodyShort weight="semibold" className="text-sm text-green-700">
                  På rett spor
                </BodyShort>
              </div>
              <BodyShort className="text-gray-600 text-xs">
                Bruker riktige filer, følger kodestil, kjører tester
              </BodyShort>
            </div>
            <div>
              <div className="flex items-center gap-1">
                <ExclamationmarkTriangleIcon className="text-orange-700" fontSize="1rem" aria-hidden />
                <BodyShort weight="semibold" className="text-sm text-orange-700">
                  Sporet av
                </BodyShort>
              </div>
              <BodyShort className="text-gray-600 text-xs">
                Gjør mer enn oppgaven, redigerer irrelevante filer, går i loops
              </BodyShort>
            </div>
            <div>
              <div className="flex items-center gap-1">
                <XMarkOctagonIcon className="text-red-700" fontSize="1rem" aria-hidden />
                <BodyShort weight="semibold" className="text-sm text-red-700">
                  Stopp og ta over
                </BodyShort>
              </div>
              <BodyShort className="text-gray-600 text-xs">
                Feil etter feil, hallusinerer APIs, trenger domenekunnskap
              </BodyShort>
            </div>
          </HGrid>
        </Box>

        {/* Steering Techniques */}
        <Box background="accent-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-5">
            <CogIcon className="text-blue-600" aria-hidden />
            <Heading size="small" level="3">
              Korrigeringsteknikker
            </Heading>
          </div>
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <div className="space-y-2">
              <div className="flex gap-3 items-start">
                <span className="text-blue-600 font-bold">1</span>
                <div>
                  <BodyShort weight="semibold" className="text-sm">
                    Kommenter på PR-en
                  </BodyShort>
                  <BodyShort className="text-gray-600 text-xs">
                    &quot;Ikke endre config.ts – fokuser kun på UserService&quot;
                  </BodyShort>
                </div>
              </div>
              <div className="flex gap-3 items-start">
                <span className="text-blue-600 font-bold">2</span>
                <div>
                  <BodyShort weight="semibold" className="text-sm">
                    Gjør manuell endring + be om å fortsette
                  </BodyShort>
                  <BodyShort className="text-gray-600 text-xs">
                    Fiks feil selv, push, og skriv &quot;Fikset typen, vennligst fortsett med resten&quot;
                  </BodyShort>
                </div>
              </div>
              <div className="flex gap-3 items-start">
                <span className="text-blue-600 font-bold">3</span>
                <div>
                  <BodyShort weight="semibold" className="text-sm">
                    Bryt opp oppgaven
                  </BodyShort>
                  <BodyShort className="text-gray-600 text-xs">
                    Lukk issue, lag flere mindre issues, tildel på nytt
                  </BodyShort>
                </div>
              </div>
            </div>
            {/* Session log screenshot */}
            <div className="rounded-lg overflow-hidden border border-gray-200 shadow-sm relative aspect-video">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src="/images/chat-reasoning.png"
                alt="Session log med agentens resonnering og verktøykall"
                className="w-full h-full object-cover"
              />
            </div>
          </HGrid>
        </Box>

        {/* Copilot App + Terminal-First - June 2026 */}
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <Heading size="small" level="3" className="mb-3">
            🆕 Copilot App og Terminal-First (juni 2026)
          </Heading>
          <BodyShort className="text-gray-700 text-sm mb-4">
            GitHub lanserte en frittstående <strong>Copilot App</strong> (macOS/Windows/Linux) som lar deg kjøre agenter
            på tvers av flere repoer og skydrift via «Canvases» – en toveiskommunikasjon der du kan justere oppgaven
            underveis uten å starte på nytt.
          </BodyShort>
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <Box background="default" padding="space-12" borderRadius="4">
              <BodyShort weight="semibold" className="text-sm mb-2">
                Terminal-First på Nav
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs mb-2">
                Nav-utviklere er allerede i kjernen av «Terminal-First»-trenden med CLI-verktøy som <code>gh</code>,{" "}
                <code>nais</code>, <code>mise</code> og <code>rtk</code>. Den naturlige neste steget er å la agenter
                orkestrere disse verktøyene automatisk.
              </BodyShort>
              <pre className="text-xs font-mono bg-gray-900 text-green-400 p-3 rounded-md">{`# La agenten kjøre standardsjekkene
rtk mise check        # Typesjekk + lint + test
rtk gh pr create      # Opprett PR automatisk
rtk go test ./...     # Verifiser endringer`}</pre>
            </Box>
            <Box background="default" padding="space-12" borderRadius="4">
              <BodyShort weight="semibold" className="text-sm mb-2">
                Din rolle som AI-orkestrator
              </BodyShort>
              <ul className="space-y-2 text-xs text-gray-600">
                <li className="flex gap-2">
                  <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                  <span>Definer oppgaven klart i et issue eller prompt</span>
                </li>
                <li className="flex gap-2">
                  <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                  <span>Verifiser shell-kommandoer agenten ønsker å kjøre</span>
                </li>
                <li className="flex gap-2">
                  <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                  <span>Gjennomgå diff og logg før PR merges</span>
                </li>
                <li className="flex gap-2">
                  <ExclamationmarkTriangleIcon
                    className="text-orange-600 shrink-0 mt-0.5"
                    fontSize="1rem"
                    aria-hidden
                  />
                  <span>Godkjenn aldri agent-kjøringer blindt – les session logs</span>
                </li>
              </ul>
            </Box>
          </HGrid>
        </Box>
      </div>
    </div>
  );
}
