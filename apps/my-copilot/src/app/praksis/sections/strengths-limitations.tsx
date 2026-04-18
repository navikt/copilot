import { Heading, BodyShort, Box, HGrid, VStack } from "@navikt/ds-react";
import { Carousel } from "@/components/carousel";
import { LinkableHeading } from "@/components/linkable-heading";
import {
  CheckmarkCircleIcon,
  XMarkOctagonIcon,
  ExclamationmarkTriangleIcon,
  ShieldLockIcon,
} from "@navikt/aksel-icons";

export default function StrengthsLimitations() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        Styrker, Begrensninger og Farer
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        Copilot er kraftig, men ikke magisk. Forstå hva det gjør best, hvor det svikter, og hvilke farer du må være
        oppmerksom på.
      </BodyShort>

      <Carousel showIndicators={true} showSwipeHint={true} className="mb-6">
        <VStack gap="space-16" className="md:min-w-100">
          <div className="flex items-center gap-2">
            <CheckmarkCircleIcon className="text-green-700" aria-hidden />
            <Heading size="small" level="3" className="text-green-700">
              Hva Copilot gjør best
            </Heading>
          </div>
          <ul className="space-y-3">
            <li className="flex gap-3">
              <span className="text-green-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Repetitivt arbeid i stor skala</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Fikse 161 skrivefeil på tvers av 100 filer, fjerne utdaterte feature flags, stor-skala refaktorering
                </BodyShort>
              </div>
            </li>
            <li className="flex gap-3">
              <span className="text-green-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Tester og dokumentasjon</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Genererer enhetstester, integrasjonstester, API-dokumentasjon og README-filer
                </BodyShort>
              </div>
            </li>
            <li className="flex gap-3">
              <span className="text-green-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Feilsøking og analyse</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Fikse flaky tester, debugge produksjonsfeil, finne ytelsesflaskehalser
                </BodyShort>
              </div>
            </li>
            <li className="flex gap-3">
              <span className="text-green-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Kodebase-analyser</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Audit av feature flags, autorisasjonsanalyse, finne forbedringsmuligheter
                </BodyShort>
              </div>
            </li>
          </ul>
        </VStack>

        <VStack gap="space-16" className="md:min-w-100">
          <div className="flex items-center gap-2">
            <XMarkOctagonIcon className="text-orange-700" aria-hidden />
            <Heading size="small" level="3" className="text-orange-700">
              Begrensninger
            </Heading>
          </div>
          <ul className="space-y-3">
            <li className="flex gap-3">
              <span className="text-orange-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Arkitektur og systemdesign</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Du må eie arkitekturen – Copilot implementerer, du designer
                </BodyShort>
              </div>
            </li>
            <li className="flex gap-3">
              <span className="text-orange-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Oppgaver med avhengigheter</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Komplekse oppgaver der steg 2 avhenger av resultatet fra steg 1
                </BodyShort>
              </div>
            </li>
            <li className="flex gap-3">
              <span className="text-orange-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Ukjent terreng</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Når du utforsker nye teknologier eller validerer antakelser
                </BodyShort>
              </div>
            </li>
            <li className="flex gap-3">
              <span className="text-orange-600 font-bold">•</span>
              <div>
                <BodyShort weight="semibold">Garantert sikker eller korrekt kode</BodyShort>
                <BodyShort className="text-gray-600 text-sm">
                  Du må alltid gjennomgå og teste – AI kan og vil gjøre feil
                </BodyShort>
              </div>
            </li>
          </ul>
        </VStack>
      </Carousel>

      {/* Dangers section */}
      <Box background="danger-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-center gap-2 mb-5">
          <ExclamationmarkTriangleIcon className="text-red-700" aria-hidden />
          <Heading size="small" level="3" className="text-red-700">
            Utfordringer du må kjenne til
          </Heading>
        </div>
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
          <div className="space-y-3">
            <div>
              <BodyShort weight="semibold">Scope creep</BodyShort>
              <BodyShort className="text-gray-600 text-sm">
                Agenten refaktorerer kode du ikke ba om, eller &quot;forbedrer&quot; ting utenfor oppgaven
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold">Sirkulær atferd</BodyShort>
              <BodyShort className="text-gray-600 text-sm">
                Agenten prøver samme feilende tilnærming flere ganger uten å justere
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold">Hallusinasjoner</BodyShort>
              <BodyShort className="text-gray-600 text-sm">
                Copilot kan finne på API-er, funksjoner eller biblioteker som ikke eksisterer
              </BodyShort>
            </div>
          </div>
          <div className="space-y-3">
            <div>
              <BodyShort weight="semibold">Prompt injection</BodyShort>
              <BodyShort className="text-gray-600 text-sm">
                Ondsinnet innhold i issues eller filer kan manipulere agentens oppførsel
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold">Konteksttap</BodyShort>
              <BodyShort className="text-gray-600 text-sm">
                Lange chat-sesjoner kan føre til at Copilot &quot;glemmer&quot; tidligere kontekst
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold">Over-engineering</BodyShort>
              <BodyShort className="text-gray-600 text-sm">
                Copilot kan generere unødvendig kompleks kode for enkle problemer
              </BodyShort>
            </div>
          </div>
        </HGrid>
      </Box>

      {/* Security principles */}
      <Box background="info-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
        <div className="flex items-center gap-2 mb-5">
          <ShieldLockIcon className="text-blue-700" aria-hidden />
          <Heading size="small" level="3" className="text-blue-700">
            GitHubs sikkerhetsprinsipper for agenter
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm mb-5">
          GitHub har bygget inn disse sikkerhetsprinsippene i Copilot coding agent:
        </BodyShort>
        <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-12">
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Synlig kontekst
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Kun synlig innhold sendes til agenten, usynlig Unicode/HTML fjernes
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Begrenset tilgang
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Agenten får ikke CI-hemmeligheter eller filer utenfor repo
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Ingen irreversible endringer
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">Kun PR-er, aldri direkte commits til main</BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Sporbarhet
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Alle handlinger attribueres til både bruker og agent
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Firewall
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Nettverkstilgang er begrenset, konfigurerbar per org
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Autoriserte brukere
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Kun brukere med write-tilgang kan tildele agenten issues
            </BodyShort>
          </div>
        </HGrid>
      </Box>
    </Box>
  );
}
