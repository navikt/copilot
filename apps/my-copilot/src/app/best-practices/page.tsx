import { Heading, BodyShort, Box, HGrid, HelpText, Label, VStack } from "@navikt/ds-react";
import { Carousel } from "@/components/carousel";
import { CodeBlock } from "@/components/code-block";
import {
  CheckmarkCircleIcon,
  XMarkOctagonIcon,
  ExclamationmarkTriangleIcon,
  LaptopIcon,
  GlobeIcon,
  TerminalIcon,
  CpuIcon,
  FileTextIcon,
  BranchingIcon,
  ShieldLockIcon,
  RocketIcon,
  BookIcon,
  TestFlaskIcon,
  MagnifyingGlassIcon,
  LinkIcon,
  LightBulbIcon,
  InformationIcon,
  TasklistIcon,
  CogIcon,
  BulletListIcon,
  PencilWritingIcon,
  StarIcon,
} from "@navikt/aksel-icons";

export default async function BestPractices() {
  return (
    <main className="max-w-7xl mx-auto">
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          {/* Header */}
          <div className="relative">
            <div className="absolute inset-0 overflow-hidden rounded-xl -z-10">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src="/images/cli-headerimage.jpeg" alt="" className="w-full h-full object-cover opacity-15" />
              <div className="absolute inset-0 bg-linear-to-r from-white via-white/90 to-transparent" />
            </div>
            <div className="py-2 sm:py-4">
              <Heading size="xlarge" level="1" className="mb-2">
                Beste Praksis og Læring
              </Heading>
              <BodyShort className="text-gray-600 max-w-2xl">
                En praktisk guide til GitHub Copilot – fra kodeforslag i editoren til autonome agenter som jobber i
                bakgrunnen. Lær å bruke verktøyet effektivt og trygt.
              </BodyShort>
            </div>
          </div>

          {/* 1. Styrker, Begrensninger og Farer */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Styrker, Begrensninger og Farer
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Copilot er kraftig, men ikke magisk. Forstå hva det gjør best, hvor det svikter, og hvilke farer du må
              være oppmerksom på.
            </BodyShort>

            <Carousel showIndicators={true} showSwipeHint={true} className="mb-6">
              <VStack gap="space-16" className="md:min-w-100">
                <div className="flex items-center gap-2">
                  <CheckmarkCircleIcon className="text-green-700" aria-hidden />
                  <Heading size="medium" level="3" className="text-green-700">
                    Hva Copilot gjør best
                  </Heading>
                </div>
                <ul className="space-y-3">
                  <li className="flex gap-3">
                    <span className="text-green-600 font-bold">•</span>
                    <div>
                      <BodyShort weight="semibold">Repetitivt arbeid i stor skala</BodyShort>
                      <BodyShort className="text-gray-600 text-sm">
                        Fikse 161 skrivefeil på tvers av 100 filer, fjerne utdaterte feature flags, stor-skala
                        refaktorering
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
                  <Heading size="medium" level="3" className="text-orange-700">
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
            <Box
              background="surface-danger-subtle"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="medium"
              className="mb-6"
            >
              <div className="flex items-center gap-2 mb-4">
                <ExclamationmarkTriangleIcon className="text-red-700" aria-hidden />
                <Heading size="medium" level="3" className="text-red-700">
                  Utfordringer du må kjenne til
                </Heading>
              </div>
              <HGrid columns={{ xs: 1, md: 2 }} gap="4">
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
            <Box background="surface-info-subtle" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="medium">
              <div className="flex items-center gap-2 mb-3">
                <ShieldLockIcon className="text-blue-700" aria-hidden />
                <Heading size="small" level="3" className="text-blue-700">
                  GitHubs sikkerhetsprinsipper for agenter
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-3">
                GitHub har bygget inn disse sikkerhetsprinsippene i Copilot coding agent:
              </BodyShort>
              <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="3">
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

          {/* 2. Verktøy og Moduser */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Verktøy og Moduser
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              GitHub Copilot er ikke bare kodeforslag i editoren. Det er et økosystem av verktøy som spenner fra
              sanntidsforslag til autonome agenter som jobber i bakgrunnen.
            </BodyShort>

            {/* Video showcase */}
            <Box
              background="surface-default"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="medium"
              className="mb-6"
            >
              <video
                autoPlay
                loop
                muted
                playsInline
                className="w-full rounded-lg"
                aria-label="GitHub Copilot demonstrasjon"
                poster="/videos/hero-poster-lg.jpeg"
              >
                <source src="/videos/hero-animation-lg.mp4" type="video/mp4" media="(min-width: 768px)" />
                <source src="/videos/hero-animation-sm.mp4" type="video/mp4" />
              </video>
            </Box>

            <Carousel showIndicators={true} showSwipeHint={true} className="mb-6">
              {/* IDE */}
              <Box
                background="surface-info-subtle"
                padding={{ xs: "space-12", sm: "space-16" }}
                borderRadius="medium"
                className="max-w-lg"
              >
                <div className="flex items-center gap-2 mb-3">
                  <LaptopIcon className="text-blue-700" aria-hidden />
                  <Heading size="medium" level="3" className="text-blue-700">
                    I editoren (IDE)
                  </Heading>
                </div>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src="/images/github-copilot-agent-mode.jpeg"
                  alt="Copilot Agent Mode i VS Code"
                  className="w-full rounded-md mb-3 border border-blue-200"
                />
                <div className="space-y-3">
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Kodeforslag (Completions)
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Sanntidsforslag mens du skriver. Tab for å godta, Esc for å avvise.
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Chat (⌘+I / Ctrl+I)
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Still spørsmål, generér kode, få forklaringer. Bruk @workspace for prosjektkontekst.
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Agent Mode
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Multifil-endringer, kjør kommandoer, iterér på feil. Mer autonomt enn chat.
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Godkjenninger
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Agent spør om godkjenning før terminalkommandoer og nettsidefetching.
                    </BodyShort>
                  </div>
                </div>
              </Box>

              {/* GitHub.com */}
              <Box
                background="surface-success-subtle"
                padding={{ xs: "space-12", sm: "space-16" }}
                borderRadius="medium"
                className="max-w-lg"
              >
                <div className="flex items-center gap-2 mb-3">
                  <GlobeIcon className="text-green-700" aria-hidden />
                  <Heading size="medium" level="3" className="text-green-700">
                    På GitHub.com
                  </Heading>
                </div>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src="/images/github-copilot-coding-agent.jpeg"
                  alt="Copilot Coding Agent på GitHub"
                  className="w-full rounded-md mb-3 border border-green-200"
                />
                <div className="space-y-3">
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Coding Agent
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Tildel en issue til @copilot, agenten lager PR i bakgrunnen. Perfekt for backlog.
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Mission Control
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Dashboard for å spore Copilot-oppgaver på tvers av repoer. Se fremdrift, session logs, og styr
                      agenten underveis. Tilgjengelig via{" "}
                      <a
                        href="https://github.com/copilot/tasks"
                        className="text-blue-600 hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        github.com/copilot/tasks
                      </a>
                      .
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Code Review
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Legg til @copilot som reviewer på PR-er. Tilpass med instructions-filer.
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Copilot Spaces
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Del kontekst med teamet for raskere debugging og samarbeid.
                    </BodyShort>
                  </div>
                </div>
              </Box>

              {/* CLI */}
              <Box
                background="surface-warning-subtle"
                padding={{ xs: "space-12", sm: "space-16" }}
                borderRadius="medium"
                className="max-w-lg"
              >
                <div className="flex items-center gap-2 mb-3">
                  <TerminalIcon className="text-orange-700" aria-hidden />
                  <Heading size="medium" level="3" className="text-orange-700">
                    I terminalen (CLI)
                  </Heading>
                </div>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src="/images/github-copilot-cli.jpeg"
                  alt="Copilot i terminalen"
                  className="w-full rounded-md mb-3 border border-orange-200"
                />
                <div className="space-y-3">
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      copilot
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Agentic CLI – build, debug, refactor kode med naturlig språk direkte i terminalen.
                    </BodyShort>
                  </div>
                  <Box background="surface-default" padding="space-8" borderRadius="small">
                    <code className="text-xs block">copilot</code>
                    <code className="text-xs block mt-1 text-gray-500"># Åpner interaktiv agent-modus</code>
                  </Box>
                  <BodyShort className="text-gray-500 text-xs">
                    Installer: <code className="bg-gray-100 px-1 rounded">brew install copilot-cli</code>{" "}
                    <code className="bg-gray-100 px-1 rounded">winget install GitHub.Copilot</code>{" "}
                    <code className="bg-gray-100 px-1 rounded">npm install -g @github/copilot</code>
                  </BodyShort>
                </div>
              </Box>
            </Carousel>

            {/* MCP section - separate from CLI */}
            <Box
              background="surface-warning-subtle"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="medium"
              className="mb-6"
            >
              <div className="flex items-center gap-2 mb-2">
                <CogIcon className="text-orange-700" aria-hidden />
                <Heading size="small" level="3" className="text-orange-700">
                  MCP (Model Context Protocol)
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-2">
                Utvid Copilot med eksterne verktøy via MCP-servere. Tilgjengelig i Agent Mode (VS Code), Copilot CLI og
                Coding Agent på GitHub.com.
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Godkjenning for bruk i Nav pågår – ventes tilgjengelig snart.
              </BodyShort>
            </Box>

            {/* Model selection */}
            <Box background="surface-action-subtle" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="medium">
              <div className="flex items-center gap-2 mb-2">
                <CpuIcon className="text-blue-600" aria-hidden />
                <Heading size="small" level="3">
                  Modellvalg og kostnader
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-3">
                Du har <strong>300 premium requests</strong> per måned. <strong>Auto</strong> gir 10 % rabatt og velger
                beste modell automatisk. Multiplikatoren (1x, 3x, 0.33x) viser hvor mange requests som trekkes per
                forespørsel.
              </BodyShort>
              <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="3">
                <div>
                  <Label size="small" className="text-green-700">
                    Auto (10 % rabatt)
                  </Label>
                  <BodyShort className="text-gray-600 text-xs">Anbefalt – velger optimal modell automatisk</BodyShort>
                </div>
                <div>
                  <Label size="small">Claude Sonnet 4 / 4.5</Label>
                  <BodyShort className="text-gray-600 text-xs">Balansert – god til de fleste oppgaver (1x)</BodyShort>
                </div>
                <div>
                  <Label size="small">Claude Opus 4.5</Label>
                  <BodyShort className="text-gray-600 text-xs">Kraftigst – komplekse oppgaver (3x)</BodyShort>
                </div>
                <div>
                  <Label size="small">GPT-5.1 / 5.2</Label>
                  <BodyShort className="text-gray-600 text-xs">OpenAI – bred kunnskap (1x)</BodyShort>
                </div>
                <div>
                  <Label size="small">Gemini 2.5 Pro / 3 Pro</Label>
                  <BodyShort className="text-gray-600 text-xs">Google – stor kontekst (1x)</BodyShort>
                </div>
                <div>
                  <Label size="small">Haiku 4.5 / Gemini Flash</Label>
                  <BodyShort className="text-gray-600 text-xs">Raske – enklere oppgaver (0.33x)</BodyShort>
                </div>
              </HGrid>
              <BodyShort className="text-gray-500 text-xs mt-3">
                Se{" "}
                <a
                  href="https://docs.github.com/en/copilot/managing-copilot/monitoring-usage-and-entitlements/about-premium-requests"
                  className="text-blue-600 hover:underline"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Premium requests dokumentasjon
                </a>{" "}
                for detaljer.
              </BodyShort>
            </Box>
          </Box>

          {/* 3. Forbered for Suksess */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Forbered for Suksess
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Tilpass Copilot til ditt prosjekt med instruksjonsfiler. Jo bedre kontekst du gir, jo bedre resultater får
              du.
            </BodyShort>

            {/* Language guidance */}
            <Box
              background="surface-info-subtle"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="medium"
              className="mb-6"
            >
              <div className="flex items-start gap-2">
                <GlobeIcon className="text-blue-700 mt-0.5" aria-hidden />
                <Heading size="small" level="3" className="text-blue-700">
                  Norsk vs. Engelsk
                </Heading>
                <HelpText title="Når bruke hvilket språk?">
                  Copilot forstår begge språk godt, men konsistens er viktig for at agenten skal følge mønstrene i koden
                  din.
                </HelpText>
              </div>
              <BodyShort className="text-gray-600 text-sm mt-2">
                <strong>Anbefaling:</strong> Skriv beskrivelser og kommentarer på norsk hvis det passer teamet. Hold
                kode, kommandoer, variabelnavn og tekniske termer på engelsk. Dette matcher vanlig praksis i norske
                utviklingsmiljøer og sikrer at Copilot forstår koden din korrekt.
              </BodyShort>
            </Box>

            {/* Comparison table: Prompts vs Instructions vs Agents vs Skills */}
            <Box
              background="surface-warning-subtle"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="medium"
              className="mb-6"
            >
              <div className="flex items-center gap-2 mb-3">
                <InformationIcon className="text-orange-600" aria-hidden />
                <Heading size="small" level="3" className="text-orange-700">
                  Fire typer tilpasninger
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-xs mb-3">
                GitHub Copilot kan tilpasses på fire måter. Se{" "}
                <a
                  href="https://github.com/github/awesome-copilot"
                  className="text-blue-600 hover:underline"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  github/awesome-copilot
                </a>{" "}
                for fellesskapets kuraterte eksempler.
              </BodyShort>
              <HGrid columns={{ xs: 1, md: 2, lg: 4 }} gap="4">
                <div>
                  <Label size="small" className="text-blue-700">
                    Prompts
                  </Label>
                  <BodyShort className="text-gray-600 text-xs mt-1">
                    <strong>Når:</strong> Engangsoppgaver
                    <br />
                    <strong>Aktivering:</strong> /prompt-name i chat
                    <br />
                    <strong>Eksempel:</strong> "Lag README for denne modulen"
                    <br />
                    <strong>Filformat:</strong> .github/prompts/*.prompt.md
                  </BodyShort>
                </div>
                <div>
                  <Label size="small" className="text-green-700">
                    Instructions
                  </Label>
                  <BodyShort className="text-gray-600 text-xs mt-1">
                    <strong>Når:</strong> Alltid aktiv
                    <br />
                    <strong>Aktivering:</strong> Automatisk på matchende filer
                    <br />
                    <strong>Eksempel:</strong> TypeScript kodestil, navnekonvensjoner
                    <br />
                    <strong>Filformat:</strong> .github/copilot-instructions.md eller
                    .github/instructions/*.instructions.md
                  </BodyShort>
                </div>
                <div>
                  <Label size="small" className="text-orange-700">
                    Agents
                  </Label>
                  <BodyShort className="text-gray-600 text-xs mt-1">
                    <strong>Når:</strong> Spesialiserte oppgaver
                    <br />
                    <strong>Aktivering:</strong> @agent-name
                    <br />
                    <strong>Eksempel:</strong> @nais-agent, @aksel-agent, @kafka-agent
                    <br />
                    <strong>Filformat:</strong> .github/agents/*.agent.md
                  </BodyShort>
                </div>
                <div>
                  <Label size="small" className="text-purple-700">
                    Skills
                  </Label>
                  <BodyShort className="text-gray-600 text-xs mt-1">
                    <strong>Når:</strong> Automatisk ved behov
                    <br />
                    <strong>Aktivering:</strong> Automatisk når relevant
                    <br />
                    <strong>Eksempel:</strong> PDF-ekstraksjon, API-testing
                    <br />
                    <strong>Filformat:</strong> .github/skills/*/SKILL.md med scripts/
                  </BodyShort>
                </div>
              </HGrid>
            </Box>

            <VStack gap="space-24">
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
- Next.js 16 med App Router
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
              <Box background="surface-info-subtle" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="medium">
                <div className="flex items-center gap-2 mb-3">
                  <BulletListIcon className="text-blue-700" aria-hidden />
                  <Heading size="small" level="3" className="text-blue-700">
                    Seks kjerneområder (fra 2500+ repos)
                  </Heading>
                </div>
                <BodyShort className="text-gray-600 text-sm mb-3">
                  Analyse av over 2500 agents.md-filer viser at de beste dekker disse områdene:
                </BodyShort>
                <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="3">
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      1. Kommandoer
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Kjørbare kommandoer tidlig: npm test, pnpm build
                    </BodyShort>
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
          </Box>

          {/* Skriv Effektive Tilpasninger */}
          <Box
            background="neutral-soft"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="12"
          >
            <Heading size="large" level="2" className="mb-4">
              Skriv Effektive Tilpasninger
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Nå som du vet hvilke tilpasningstyper som finnes, her er konkrete råd for å skrive dem godt. Kilde:{" "}
              <a
                href="https://code.visualstudio.com/docs/copilot/customization/overview"
                className="text-blue-600 hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                VS Code Docs
              </a>
              ,{" "}
              <a
                href="https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/"
                className="text-blue-600 hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                GitHub Blog (2500+ repos)
              </a>
              ,{" "}
              <a
                href="https://agentskills.io/specification"
                className="text-blue-600 hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                agentskills.io
              </a>
            </BodyShort>

            {/* Instructions */}
            <Box
              background="success-soft"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="8"
              className="mb-6"
            >
              <div className="flex items-center gap-2 mb-3">
                <FileTextIcon className="text-green-700" aria-hidden />
                <Heading size="medium" level="3" className="text-green-700">
                  Instructions
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-4">
                Instructions definerer kodestil og regler som alltid gjelder. Tenk på dem som teamets stilguide – korte,
                konkrete og sjelden i endring. Se{" "}
                <a
                  href="https://code.visualstudio.com/docs/copilot/customization/custom-instructions"
                  className="text-blue-600 hover:underline"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  VS Code: Custom Instructions
                </a>
              </BodyShort>
              <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
                <Box background="default" padding="space-12" borderRadius="4">
                  <BodyShort weight="semibold" className="text-sm text-green-700 mb-2">
                    Gode mønstre
                  </BodyShort>
                  <ul className="space-y-2 text-xs text-gray-600">
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Hold instruksjoner korte og selvstendige – én regel per punkt</span>
                    </li>
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>
                        Forklar <em>hvorfor</em> – &quot;Bruk date-fns i stedet for moment.js – moment er deprecated og
                        øker bundle size&quot;
                      </span>
                    </li>
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Vis konkrete kodeeksempler (✅ Good / ❌ Bad)</span>
                    </li>
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Bruk applyTo-glob for filtype-spesifikke regler</span>
                    </li>
                  </ul>
                </Box>
                <Box background="default" padding="space-12" borderRadius="4">
                  <BodyShort weight="semibold" className="text-sm text-red-700 mb-2">
                    Vanlige feil
                  </BodyShort>
                  <ul className="space-y-2 text-xs text-gray-600">
                    <li className="flex gap-2">
                      <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>For lange filer – hold det fokusert, hopp over ting linteren allerede sjekker</span>
                    </li>
                    <li className="flex gap-2">
                      <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Vage direktiver som &quot;skriv ren kode&quot; – vær konkret</span>
                    </li>
                    <li className="flex gap-2">
                      <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Duplikate regler på tvers av filer – bruk Markdown-lenker for gjenbruk</span>
                    </li>
                    <li className="flex gap-2">
                      <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Alt i én fil – splitt i *.instructions.md per språk/rammeverk</span>
                    </li>
                  </ul>
                </Box>
              </HGrid>
              <Box background="default" padding="space-12" borderRadius="4" className="mt-4">
                <BodyShort weight="semibold" className="text-sm mb-2">
                  Prioritet (ved konflikt)
                </BodyShort>
                <BodyShort className="text-gray-600 text-xs">
                  1. Personlige instruksjoner (bruker-nivå) → 2. Repository-instruksjoner (copilot-instructions.md /
                  AGENTS.md) → 3. Organisasjons-instruksjoner. Høyere prioritet vinner.
                </BodyShort>
              </Box>
            </Box>

            {/* Custom Agents */}
            <Box
              background="info-soft"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="8"
              className="mb-6"
            >
              <div className="flex items-center gap-2 mb-3">
                <CogIcon className="text-blue-700" aria-hidden />
                <Heading size="medium" level="3" className="text-blue-700">
                  Custom Agents
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-4">
                Agenter er spesialiserte roller med eget verktøysett og instruksjoner. Nøkkelen er spesifisitet – en
                god agent har én jobb, ikke ti. Se{" "}
                <a
                  href="https://code.visualstudio.com/docs/copilot/customization/custom-agents"
                  className="text-blue-600 hover:underline"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  VS Code: Custom Agents
                </a>
              </BodyShort>

              <BodyShort weight="semibold" className="text-sm mb-2">
                Anbefalt struktur i agent-filen (fra{" "}
                <a
                  href="https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/"
                  className="text-blue-600 hover:underline"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  analyse av 2500+ repos
                </a>
                ):
              </BodyShort>
              <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="mb-4">
                <Box background="default" padding="space-12" borderRadius="4">
                  <ol className="space-y-2 text-xs text-gray-600 list-decimal list-inside">
                    <li>
                      <strong>YAML-frontmatter</strong> – name, description, tools
                    </li>
                    <li>
                      <strong>Persona</strong> – én setning: hvem du er og hva du gjør
                    </li>
                    <li>
                      <strong>Kommandoer</strong> – kjørbare kommandoer tidlig, med flagg
                    </li>
                    <li>
                      <strong>Relaterte agenter</strong> – eventuelle handoffs
                    </li>
                    <li>
                      <strong>Kodeeksempler</strong> – vis, ikke forklar
                    </li>
                    <li>
                      <strong>Tre-trinns grenser</strong> – ✅ Alltid / ⚠️ Spør først / 🚫 Aldri
                    </li>
                  </ol>
                </Box>
                <Box background="default" padding="space-12" borderRadius="4">
                  <BodyShort weight="semibold" className="text-sm mb-2">
                    YAML-frontmatter felter
                  </BodyShort>
                  <ul className="space-y-1 text-xs text-gray-600">
                    <li>
                      <strong>description</strong> – kort beskrivelse (vises som placeholder i chat)
                    </li>
                    <li>
                      <strong>tools</strong> – liste over tilgjengelige verktøy (f.eks. search, fetch, editFiles)
                    </li>
                    <li>
                      <strong>model</strong> – valgfri AI-modell (én eller prioritert liste)
                    </li>
                    <li>
                      <strong>handoffs</strong> – sekvensielle workflows mellom agenter
                    </li>
                    <li>
                      <strong>agents</strong> – tillatte sub-agenter (bruk * for alle)
                    </li>
                  </ul>
                </Box>
              </HGrid>

              <Box
                background="danger-soft"
                padding="space-12"
                borderRadius="4"
              >
                <BodyShort weight="semibold" className="text-sm text-red-700 mb-1">
                  Vanligste feilen
                </BodyShort>
                <BodyShort className="text-gray-600 text-xs">
                  &quot;You are a helpful coding assistant&quot; fungerer ikke. &quot;You are a test engineer who
                  writes tests for React components, follows these examples, and never modifies source code&quot;
                  fungerer. Spesifisitet slår generalitet.
                </BodyShort>
              </Box>
            </Box>

            {/* Agent Skills */}
            <Box
              background="accent-soft"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="8"
              className="mb-6"
            >
              <div className="flex items-center gap-2 mb-3">
                <PencilWritingIcon className="text-blue-600" aria-hidden />
                <Heading size="medium" level="3">
                  Agent Skills
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-4">
                Skills er gjenbrukbare kapabiliteter med skript og ressurser som Copilot laster automatisk når de er
                relevante. Åpen standard via{" "}
                <a
                  href="https://agentskills.io/specification"
                  className="text-blue-600 hover:underline"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  agentskills.io
                </a>
                . Fungerer i VS Code, Copilot CLI og Coding Agent. Se{" "}
                <a
                  href="https://code.visualstudio.com/docs/copilot/customization/agent-skills"
                  className="text-blue-600 hover:underline"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  VS Code: Agent Skills
                </a>
              </BodyShort>

              <HGrid columns={{ xs: 1, md: 3 }} gap="space-16" className="mb-4">
                <Box background="default" padding="space-12" borderRadius="4">
                  <BodyShort weight="semibold" className="text-sm text-blue-700 mb-2">
                    Progressive disclosure
                  </BodyShort>
                  <BodyShort className="text-gray-600 text-xs">
                    Copilot laster kun det som trengs i tre nivåer: 1) name + description (alltid synlig), 2)
                    SKILL.md body (ved match), 3) scripts/resources (ved referanse). Installer mange skills uten å
                    bruke kontekst.
                  </BodyShort>
                </Box>
                <Box background="default" padding="space-12" borderRadius="4">
                  <BodyShort weight="semibold" className="text-sm text-blue-700 mb-2">
                    Mappestruktur
                  </BodyShort>
                  <CodeBlock filename=".github/skills/my-skill/">{`.github/skills/my-skill/
├── SKILL.md          # Påkrevd
├── scripts/          # Valgfri
│   └── run-tests.sh
├── references/       # Valgfri
│   └── FORMAT.md
└── examples/         # Valgfri`}</CodeBlock>
                </Box>
                <Box background="default" padding="space-12" borderRadius="4">
                  <BodyShort weight="semibold" className="text-sm text-blue-700 mb-2">
                    Invokering
                  </BodyShort>
                  <BodyShort className="text-gray-600 text-xs">
                    Skills kan både brukes som /slash-commands og lastes automatisk basert på description-match.
                    Kontroller med user-invokable og disable-model-invocation i frontmatter.
                  </BodyShort>
                </Box>
              </HGrid>

              <Box background="default" padding="space-12" borderRadius="4">
                <BodyShort weight="semibold" className="text-sm mb-2">
                  Tips for gode skills
                </BodyShort>
                <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
                  <ul className="space-y-2 text-xs text-gray-600">
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Skriv en presis description med trigger-ord som brukere faktisk sier</span>
                    </li>
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Hold SKILL.md body under 500 linjer – flytt detaljer til references/</span>
                    </li>
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>name i frontmatter må matche mappenavnet</span>
                    </li>
                  </ul>
                  <ul className="space-y-2 text-xs text-gray-600">
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Inkluder skript som agenten kan kjøre for å verifisere arbeidet</span>
                    </li>
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>Skills er portable – fungerer i VS Code, CLI og Coding Agent</span>
                    </li>
                    <li className="flex gap-2">
                      <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                      <span>
                        Se{" "}
                        <a
                          href="https://github.com/anthropics/skills"
                          className="text-blue-600 hover:underline"
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          anthropics/skills
                        </a>{" "}
                        og{" "}
                        <a
                          href="https://github.com/github/awesome-copilot"
                          className="text-blue-600 hover:underline"
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          awesome-copilot
                        </a>{" "}
                        for eksempler
                      </span>
                    </li>
                  </ul>
                </HGrid>
              </Box>
            </Box>

            {/* Quick reference: when to use what */}
            <Box
              background="warning-soft"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="8"
            >
              <div className="flex items-center gap-2 mb-3">
                <LightBulbIcon className="text-orange-700" aria-hidden />
                <Heading size="small" level="3" className="text-orange-700">
                  Når bruker du hva?
                </Heading>
              </div>
              <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
                <Box background="default" padding="space-12" borderRadius="4">
                  <Label size="small" className="text-green-700">
                    Instructions
                  </Label>
                  <BodyShort className="text-gray-600 text-xs mt-1">
                    Kodestil, navnekonvensjoner, sikkerhetsregler. Start med én copilot-instructions.md, utvid med
                    *.instructions.md per språk.
                  </BodyShort>
                </Box>
                <Box background="default" padding="space-12" borderRadius="4">
                  <Label size="small" className="text-blue-700">
                    Agents
                  </Label>
                  <BodyShort className="text-gray-600 text-xs mt-1">
                    Spesialiserte roller som @test-agent, @docs-agent. Når du trenger eget verktøysett og persona.
                    Støtter handoffs mellom agenter.
                  </BodyShort>
                </Box>
                <Box background="default" padding="space-12" borderRadius="4">
                  <Label size="small" className="text-purple-700">
                    Skills
                  </Label>
                  <BodyShort className="text-gray-600 text-xs mt-1">
                    Gjenbrukbare kapabiliteter med skript. Når du trenger portabilitet på tvers av VS Code, CLI og
                    Coding Agent.
                  </BodyShort>
                </Box>
              </HGrid>
            </Box>
          </Box>

          {/* 4. Prompt Engineering */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Prompt Engineering
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Hvordan du formulerer forespørselen påvirker kvaliteten på Copilots svar. Spesifisitet er nøkkelen.
            </BodyShort>

            <div className="space-y-6">
              {/* Strategy 1: Specific prompts */}
              <div>
                <Heading size="medium" level="3" className="mb-4 flex items-center gap-2">
                  <span className="text-blue-600">1.</span>
                  Vær spesifikk, ikke vag
                </Heading>

                <Carousel showIndicators={true} showSwipeHint={true}>
                  <Box
                    background="surface-danger-subtle"
                    padding="space-16"
                    borderRadius="medium"
                    className="border-l-4 border-red-600"
                  >
                    <BodyShort weight="semibold" className="text-red-700 mb-2">
                      ❌ Vag
                    </BodyShort>
                    <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                      {`Fix the authentication bug.`}
                    </code>
                  </Box>

                  <Box
                    background="surface-success-subtle"
                    padding="space-16"
                    borderRadius="medium"
                    className="border-l-4 border-green-600"
                  >
                    <BodyShort weight="semibold" className="text-green-700 mb-2">
                      ✓ Spesifikk
                    </BodyShort>
                    <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                      {`Users report 'Invalid token' errors
after 30 minutes. JWT tokens are
configured with 1-hour expiration
in auth.config.ts. Investigate why
tokens expire early and fix the
validation logic in middleware/auth.ts`}
                    </code>
                  </Box>
                </Carousel>
              </div>

              {/* Strategy 2: Examples */}
              <div>
                <Heading size="medium" level="3" className="mb-4 flex items-center gap-2">
                  <span className="text-blue-600">2.</span>
                  Gi eksempler på forventet output
                </Heading>

                <Carousel showIndicators={true} showSwipeHint={true}>
                  <Box
                    background="surface-danger-subtle"
                    padding="space-16"
                    borderRadius="medium"
                    className="border-l-4 border-red-600"
                  >
                    <BodyShort weight="semibold" className="text-red-700 mb-2">
                      ❌ Uten eksempel
                    </BodyShort>
                    <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                      {`Write a function that formats
currency in Norwegian style`}
                    </code>
                  </Box>

                  <Box
                    background="surface-success-subtle"
                    padding="space-16"
                    borderRadius="medium"
                    className="border-l-4 border-green-600"
                  >
                    <BodyShort weight="semibold" className="text-green-700 mb-2">
                      ✓ Med eksempel
                    </BodyShort>
                    <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                      {`Write a TypeScript function that
formats numbers as Norwegian currency.

Example:
formatNOK(1234.5) → "1 234,50 kr"
formatNOK(1000000) → "1 000 000,00 kr"`}
                    </code>
                  </Box>
                </Carousel>
              </div>

              {/* Strategy 3: Break down */}
              <div>
                <Heading size="medium" level="3" className="mb-4 flex items-center gap-2">
                  <span className="text-blue-600">3.</span>
                  Bryt ned komplekse oppgaver
                </Heading>
                <BodyShort className="text-gray-600 mb-4">
                  Store oppgaver bør deles i mindre steg. Bruk <strong>Plan Mode</strong> for å la Copilot analysere
                  oppgaven og foreslå en plan før implementering.
                </BodyShort>

                {/* Plan Mode Image */}
                <div className="mb-4 rounded-lg overflow-hidden border border-gray-200 shadow-sm relative aspect-video">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src="/images/copilot-in-vs-code-hero-plan-mode.jpeg"
                    alt="Plan Mode i VS Code - Copilot analyserer og planlegger oppgaven"
                    className="w-full h-full object-cover"
                  />
                </div>

                <HGrid columns={{ xs: 1, md: 2 }} gap="4">
                  <Box
                    background="surface-info-subtle"
                    padding={{ xs: "space-12", sm: "space-16" }}
                    borderRadius="medium"
                  >
                    <div className="flex items-center gap-2 mb-2">
                      <TasklistIcon className="text-blue-600" aria-hidden />
                      <BodyShort weight="semibold">Plan Mode</BodyShort>
                    </div>
                    <BodyShort className="text-gray-600 text-sm mb-2">
                      Aktiver med &quot;/plan&quot; eller velg Plan i modusvelgeren. Copilot vil:
                    </BodyShort>
                    <ol className="space-y-1 list-decimal list-inside text-xs text-gray-600">
                      <li>Analysere oppgaven og konteksten</li>
                      <li>Foreslå en detaljert plan med steg</li>
                      <li>La deg godkjenne eller justere planen</li>
                      <li>Implementere steg for steg</li>
                    </ol>
                  </Box>

                  <Box
                    background="surface-success-subtle"
                    padding={{ xs: "space-12", sm: "space-16" }}
                    borderRadius="medium"
                  >
                    <BodyShort weight="semibold" className="mb-2">
                      Eksempel: Legg til autentisering
                    </BodyShort>
                    <ol className="space-y-1 list-decimal list-inside text-sm">
                      <li>Lag en AuthContext med login/logout</li>
                      <li>Lag en useAuth-hook</li>
                      <li>Lag ProtectedRoute-komponent</li>
                      <li>Integrer i app layout</li>
                    </ol>
                  </Box>
                </HGrid>

                <Box background="surface-warning-subtle" padding="space-12" borderRadius="medium" className="mt-3">
                  <BodyShort className="text-gray-600 text-xs">
                    <strong>Tips:</strong> For coding agent på GitHub.com, skriv issues med klare akseptkriterier og
                    bruk sub-issues for store oppgaver. Se{" "}
                    <a
                      href="https://docs.github.com/en/copilot/tutorials/coding-agent/get-the-best-results"
                      className="text-blue-600 hover:underline"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Get the best results from the coding agent
                    </a>
                    .
                  </BodyShort>
                </Box>

                {/* Spec Kit */}
                <Box background="surface-success-subtle" padding="space-12" borderRadius="medium" className="mt-3">
                  <div className="flex items-center gap-2 mb-2">
                    <FileTextIcon className="text-green-700" aria-hidden />
                    <BodyShort weight="semibold" className="text-green-700 text-sm">
                      Spec Kit – Strukturert planlegging
                    </BodyShort>
                  </div>
                  <BodyShort className="text-gray-600 text-xs mb-2">
                    GitHubs offisielle verktøy for &quot;Spec-Driven Development&quot;. Skriv spesifikasjoner først, la
                    Copilot implementere. Støtter slash-commands:
                  </BodyShort>
                  <div className="flex flex-wrap gap-2 mb-2">
                    <code className="text-xs bg-white px-2 py-1 rounded">/speckit.specify</code>
                    <code className="text-xs bg-white px-2 py-1 rounded">/speckit.plan</code>
                    <code className="text-xs bg-white px-2 py-1 rounded">/speckit.tasks</code>
                    <code className="text-xs bg-white px-2 py-1 rounded">/speckit.implement</code>
                  </div>
                  <BodyShort className="text-gray-500 text-xs">
                    Installer: <code className="bg-white px-1 rounded">specify init my-project --ai copilot</code> –{" "}
                    <a
                      href="https://github.com/github/spec-kit"
                      className="text-blue-600 hover:underline"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      github/spec-kit
                    </a>
                  </BodyShort>
                </Box>
              </div>

              {/* Strategy 4: Context */}
              <div>
                <Heading size="medium" level="3" className="mb-4 flex items-center gap-2">
                  <span className="text-blue-600">4.</span>
                  Gi relevant kontekst
                </Heading>
                <Box
                  background="surface-info-subtle"
                  padding={{ xs: "space-12", sm: "space-16" }}
                  borderRadius="medium"
                >
                  <ul className="space-y-2">
                    <li className="flex gap-2">
                      <span className="text-blue-600">▪</span>
                      <BodyShort className="text-sm">Åpne relevante filer, lukk irrelevante</BodyShort>
                    </li>
                    <li className="flex gap-2">
                      <span className="text-blue-600">▪</span>
                      <BodyShort className="text-sm">Bruk @workspace for prosjektkontekst i chat</BodyShort>
                    </li>
                    <li className="flex gap-2">
                      <span className="text-blue-600">▪</span>
                      <BodyShort className="text-sm">Merk opp koden du vil referere til</BodyShort>
                    </li>
                    <li className="flex gap-2">
                      <span className="text-blue-600">▪</span>
                      <BodyShort className="text-sm">Start ny chat når du bytter tema</BodyShort>
                    </li>
                  </ul>
                </Box>
              </div>

              {/* Anti-patterns */}
              <Box
                background="surface-danger-subtle"
                padding={{ xs: "space-12", sm: "space-16" }}
                borderRadius="medium"
              >
                <div className="flex items-center gap-2 mb-3">
                  <XMarkOctagonIcon className="text-red-700" aria-hidden />
                  <Heading size="small" level="3" className="text-red-700">
                    Anti-mønstre å unngå
                  </Heading>
                </div>
                <HGrid columns={{ xs: 1, md: 2 }} gap="4">
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Vage direktiver
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      &quot;Be more accurate&quot; eller &quot;Identify all issues&quot; – Copilot gjør allerede sitt
                      beste
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Eksterne lenker
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Copilot følger ikke lenker – kopier relevant innhold inn i prompten
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      Tvetydige referanser
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      &quot;Fix this&quot; eller &quot;What does it do?&quot; – vær eksplisitt om hva du refererer til
                    </BodyShort>
                  </div>
                  <div>
                    <BodyShort weight="semibold" className="text-sm">
                      UX-endringer
                    </BodyShort>
                    <BodyShort className="text-gray-600 text-xs">
                      Du kan ikke endre fonter eller formatering på Copilot-kommentarer
                    </BodyShort>
                  </div>
                </HGrid>
              </Box>
            </div>
          </Box>

          {/* 5. WRAP-metoden */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              WRAP-metoden for Coding Agent
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              WRAP er en enkel huskeregel for å få mest mulig ut av Copilot coding agent. Tenk på det som å onboarde en
              ny kollega.
            </BodyShort>

            <HGrid columns={{ xs: 1, md: 2 }} gap="6">
              <Box
                background="surface-success-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-green-600"
              >
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-green-600 font-bold text-xl">W</span>
                  <Heading size="medium" level="3">
                    Write
                  </Heading>
                </div>
                <BodyShort className="text-gray-600 mb-3">
                  Skriv issues som om du forklarer til en ny utvikler på teamet.
                </BodyShort>
                <Box background="surface-default" padding="space-8" borderRadius="small">
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

              <Box
                background="surface-info-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-blue-600"
              >
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-blue-600 font-bold text-xl">R</span>
                  <Heading size="medium" level="3">
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

              <Box
                background="surface-warning-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-orange-600"
              >
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-orange-600 font-bold text-xl">A</span>
                  <Heading size="medium" level="3">
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

              <Box
                background="surface-action-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-purple-600"
              >
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-purple-600 font-bold text-xl">P</span>
                  <Heading size="medium" level="3">
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
            <Box background="surface-info-subtle" padding="space-16" borderRadius="medium" className="mt-6">
              <div className="flex items-center gap-2 mb-3">
                <BranchingIcon className="text-blue-700" aria-hidden />
                <Heading size="small" level="3" className="text-blue-700">
                  Hva GitHub bruker Copilot til internt
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-3">
                GitHub bruker Copilot coding agent aktivt på github.com-kodebasen:
              </BodyShort>
              <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="3">
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
                  <BodyShort className="text-gray-600 text-xs">
                    Gi nytt navn til klasser brukt overalt i kodebasen
                  </BodyShort>
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

          {/* 6. Orkestrer og Styr Agenter */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Orkestrer og Styr Agenter
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Med Copilot coding agent jobber du som en &quot;mission control&quot; – du styrer oppgaver, overvåker
              fremdrift og griper inn ved behov.
            </BodyShort>

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
              <HGrid columns={{ xs: 1, md: 2 }} gap="4">
                <Box background="surface-success-subtle" padding="space-16" borderRadius="medium">
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

                <Box background="surface-warning-subtle" padding="space-16" borderRadius="medium">
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
              <Box background="surface-info-subtle" padding="space-16" borderRadius="medium">
                <div className="flex items-center gap-2 mb-3">
                  <InformationIcon className="text-blue-700" aria-hidden />
                  <Heading size="small" level="3" className="text-blue-700">
                    Les agentens signaler
                  </Heading>
                </div>
                <BodyShort className="text-gray-600 text-sm mb-3">
                  Session logs viser agentens tankegang. Se etter disse tegnene:
                </BodyShort>
                <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="3">
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
              <Box background="surface-action-subtle" padding="space-16" borderRadius="medium">
                <div className="flex items-center gap-2 mb-3">
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
            </div>
          </Box>

          {/* 7. Gjennomgå Copilots Arbeid */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Gjennomgå Copilots Arbeid
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Copilot coding agent lager PR-er som trenger grundig gjennomgang. Bruk en tre-trinns sjekkliste.
            </BodyShort>

            {/* Code Review Image */}
            <div className="mb-6 rounded-lg overflow-hidden border border-gray-200 shadow-sm relative aspect-video">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src="/images/github-copilot-code-review-updated.jpeg"
                alt="Copilot Code Review på GitHub"
                className="w-full h-full object-cover"
              />
            </div>

            <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="4">
              <Box
                background="surface-info-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-blue-600"
              >
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-blue-600 font-bold text-lg">1</span>
                  <Heading size="small" level="3">
                    Session logs
                  </Heading>
                </div>
                <ul className="space-y-2 text-sm">
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <span>Forstod agenten oppgaven?</span>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <span>Var det feil den ga opp på?</span>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <span>Gikk den i loop eller hallusinerte?</span>
                  </li>
                </ul>
              </Box>

              <Box
                background="surface-success-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-green-600"
              >
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-green-600 font-bold text-lg">2</span>
                  <Heading size="small" level="3">
                    Files changed
                  </Heading>
                </div>
                <ul className="space-y-2 text-sm">
                  <li className="flex gap-2">
                    <span className="text-green-600">▪</span>
                    <span>Kun relevante filer endret?</span>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-green-600">▪</span>
                    <span>Følger koden prosjektets stil?</span>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-green-600">▪</span>
                    <span>Er det hardkodet/generert kode?</span>
                  </li>
                </ul>
              </Box>

              <Box
                background="surface-warning-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-orange-600"
              >
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-orange-600 font-bold text-lg">3</span>
                  <Heading size="small" level="3">
                    Checks
                  </Heading>
                </div>
                <ul className="space-y-2 text-sm">
                  <li className="flex gap-2">
                    <span className="text-orange-600">▪</span>
                    <span>Kjør CI manuelt (ikke auto på Copilot PR)</span>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-orange-600">▪</span>
                    <span>Sjekk at alle tester passerer</span>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-orange-600">▪</span>
                    <span>Verifiser i preview/staging</span>
                  </li>
                </ul>
              </Box>
            </HGrid>

            <Box background="surface-danger-subtle" padding="space-16" borderRadius="medium" className="mt-4">
              <div className="flex items-center gap-2 mb-2">
                <ExclamationmarkTriangleIcon className="text-red-700" aria-hidden />
                <Heading size="small" level="3" className="text-red-700">
                  Viktig: CI kjører ikke automatisk
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm">
                PR-er fra Copilot coding agent utløser ikke CI-workflows automatisk. Du må starte dem manuelt eller
                approve workflow run. Dette er en sikkerhetsfunksjon.
              </BodyShort>
            </Box>

            {/* Pro tips */}
            <Box background="surface-action-subtle" padding="space-16" borderRadius="medium" className="mt-4">
              <div className="flex items-center gap-2 mb-2">
                <LightBulbIcon className="text-blue-600" aria-hidden />
                <Heading size="small" level="3">
                  Pro-tips for effektiv gjennomgang
                </Heading>
              </div>
              <HGrid columns={{ xs: 1, md: 2 }} gap="4">
                <div>
                  <BodyShort weight="semibold" className="text-sm">
                    Be Copilot gjennomgå seg selv
                  </BodyShort>
                  <BodyShort className="text-gray-600 text-xs">
                    &quot;Review this PR for bugs, security issues, and code style violations&quot;
                  </BodyShort>
                </div>
                <div>
                  <BodyShort weight="semibold" className="text-sm">
                    Grupper lignende PR-er
                  </BodyShort>
                  <BodyShort className="text-gray-600 text-xs">
                    Gjennomgå flere dokumentasjons-PR-er sammen for konsistens
                  </BodyShort>
                </div>
              </HGrid>
            </Box>
          </Box>

          {/* 8. Verifisering */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Verifisering – Nøkkelen til Kvalitet
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              &quot;Gi Copilot en måte å verifisere arbeidet sitt – dette 2-3x kvaliteten.&quot; En god plan er viktig,
              men verifisering er det som sikrer at resultatet faktisk fungerer.
            </BodyShort>

            <HGrid columns={{ xs: 1, md: 2 }} gap="4" className="mb-6">
              <Box
                background="surface-success-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-green-600"
              >
                <div className="flex items-center gap-2 mb-3">
                  <TestFlaskIcon className="text-green-700" aria-hidden />
                  <Heading size="small" level="3" className="text-green-700">
                    Be om tester i prompten
                  </Heading>
                </div>
                <BodyShort className="text-gray-600 text-sm mb-2">Inkluder testing som del av oppgaven:</BodyShort>
                <Box background="surface-default" padding="space-8" borderRadius="small">
                  <code className="text-xs block whitespace-pre-wrap">
                    {`Lag en funksjon som validerer
norske fødselsnumre.

Skriv enhetstester og kjør dem
før du anser oppgaven som ferdig.`}
                  </code>
                </Box>
              </Box>

              <Box
                background="surface-info-subtle"
                padding="space-16"
                borderRadius="medium"
                className="border-l-4 border-blue-600"
              >
                <div className="flex items-center gap-2 mb-3">
                  <MagnifyingGlassIcon className="text-blue-700" aria-hidden />
                  <Heading size="small" level="3" className="text-blue-700">
                    La Copilot reviewe seg selv
                  </Heading>
                </div>
                <BodyShort className="text-gray-600 text-sm mb-2">Etter implementering, be om selvreview:</BodyShort>
                <Box background="surface-default" padding="space-8" borderRadius="small">
                  <code className="text-xs block whitespace-pre-wrap">
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
            <Box
              background="surface-warning-subtle"
              padding={{ xs: "space-12", sm: "space-16" }}
              borderRadius="medium"
              className="mb-6"
            >
              <div className="flex items-center gap-2 mb-3">
                <CogIcon className="text-orange-700" aria-hidden />
                <Heading size="small" level="3" className="text-orange-700">
                  Knip – Rydd opp etter Copilot
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-3">
                Copilot kan etterlate ubrukt kode, avhengigheter og exports. Knip finner og fjerner dette automatisk.
                Brukes av Vercel, Anthropic, Cloudflare og TanStack.
              </BodyShort>
              <HGrid columns={{ xs: 1, sm: 2 }} gap="4">
                <div>
                  <BodyShort weight="semibold" className="text-sm">
                    Installer og kjør
                  </BodyShort>
                  <Box background="surface-default" padding="space-8" borderRadius="small" className="mt-1">
                    <code className="text-xs block">npx knip</code>
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
            <Box background="surface-action-subtle" padding="space-16" borderRadius="medium">
              <div className="flex items-center gap-2 mb-3">
                <TasklistIcon className="text-blue-600" aria-hidden />
                <Heading size="small" level="3">
                  Verifiseringssjekkliste
                </Heading>
              </div>
              <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="3">
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

          {/* 9. Vanlige mønstre */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Vanlige mønstre for Agent Mode
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Bygg spesialiserte agenter for repeterende oppgaver. Her er seks anbefalte agenter å starte med.
            </BodyShort>

            <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="4">
              <Box background="surface-info-subtle" padding="space-16" borderRadius="medium">
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

              <Box background="surface-success-subtle" padding="space-16" borderRadius="medium">
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

              <Box background="surface-warning-subtle" padding="space-16" borderRadius="medium">
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

              <Box background="surface-action-subtle" padding="space-16" borderRadius="medium">
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

              <Box background="surface-danger-subtle" padding="space-16" borderRadius="medium">
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

              <Box background="surface-neutral-subtle" padding="space-16" borderRadius="medium">
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
            <Box background="surface-info-subtle" padding="space-16" borderRadius="medium" className="mt-4">
              <div className="flex items-center gap-2 mb-3">
                <FileTextIcon className="text-blue-700" aria-hidden />
                <Heading size="small" level="3" className="text-blue-700">
                  Eksempel: .github/agents/test-agent.agent.md
                </Heading>
              </div>
              <BodyShort className="text-gray-600 text-xs mb-2">
                Følger GitHub sin anbefalte rekkefølge: Kommandoer → Testing → Prosjektstruktur → Kodestil →
                Git-workflow → Grenser
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

          {/* 9. Ressurser */}
          <Box
            background="surface-subtle"
            padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
            borderRadius="large"
          >
            <Heading size="large" level="2" className="mb-4">
              Ressurser
            </Heading>
            <BodyShort className="text-gray-600 mb-6">
              Offisielle kilder, fellesskapsressurser og Nav-spesifikk dokumentasjon.
            </BodyShort>

            <HGrid columns={{ xs: 1, md: 2 }} gap="4">
              <Box background="surface-info-subtle" padding="space-16" borderRadius="medium">
                <div className="flex items-center gap-2 mb-2">
                  <BookIcon className="text-blue-600" aria-hidden />
                  <Heading size="small" level="3">
                    Offisiell dokumentasjon
                  </Heading>
                </div>
                <ul className="space-y-2">
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <a
                      href="https://docs.github.com/en/copilot"
                      className="text-blue-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      GitHub Copilot Docs
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <a
                      href="https://docs.github.com/en/copilot/get-started/best-practices"
                      className="text-blue-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Best Practices (Official)
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <a
                      href="https://docs.github.com/en/copilot/concepts/prompting/prompt-engineering"
                      className="text-blue-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Prompt Engineering Guide
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <a
                      href="https://docs.github.com/en/copilot/managing-copilot/monitoring-usage-and-entitlements/about-premium-requests"
                      className="text-blue-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Premium Requests & Limits
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <a
                      href="https://github.blog/changelog/2025-10-28-a-mission-control-to-assign-steer-and-track-copilot-coding-agent-tasks/"
                      className="text-blue-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Mission Control Changelog
                    </a>
                  </li>
                </ul>
              </Box>

              <Box background="surface-success-subtle" padding="space-16" borderRadius="medium">
                <div className="flex items-center gap-2 mb-2">
                  <StarIcon className="text-green-600" aria-hidden />
                  <Heading size="small" level="3">
                    Fellesskapsressurser (offisielle)
                  </Heading>
                </div>
                <ul className="space-y-2">
                  <li className="flex gap-2">
                    <span className="text-green-600">▪</span>
                    <a
                      href="https://github.com/github/awesome-copilot"
                      className="text-green-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Awesome Copilot – offisiell kuratert samling av prompts, instructions, agents og skills
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-green-600">▪</span>
                    <a
                      href="https://github.com/github/spec-kit"
                      className="text-green-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Spec Kit – GitHubs offisielle verktøy for Spec-Driven Development (60k+ ⭐)
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-green-600">▪</span>
                    <a
                      href="https://github.com/anthropics/skills"
                      className="text-green-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Anthropic Skills – offisielle eksempler på Agent Skills
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-green-600">▪</span>
                    <a
                      href="https://github.blog/tag/github-copilot/"
                      className="text-green-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      GitHub Blog – Copilot-artikler
                    </a>
                  </li>
                </ul>
              </Box>

              <Box background="surface-neutral-subtle" padding="space-16" borderRadius="medium">
                <div className="flex items-center gap-2 mb-2">
                  <CogIcon className="text-gray-600" aria-hidden />
                  <Heading size="small" level="3">
                    Verifiseringsverktøy
                  </Heading>
                </div>
                <ul className="space-y-2">
                  <li className="flex gap-2">
                    <span className="text-gray-600">▪</span>
                    <a
                      href="https://knip.dev/"
                      className="text-gray-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Knip – Finn ubrukt kode, deps og exports i JS/TS-prosjekter
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-gray-600">▪</span>
                    <a
                      href="https://knip.dev/blog/for-editors-and-agents"
                      className="text-gray-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Knip for Editors & Agents – Integrasjon med AI-verktøy
                    </a>
                  </li>
                </ul>
              </Box>

              <Box background="surface-warning-subtle" padding="space-16" borderRadius="medium">
                <div className="flex items-center gap-2 mb-2">
                  <ShieldLockIcon className="text-orange-600" aria-hidden />
                  <Heading size="small" level="3">
                    Sikkerhet og tillit
                  </Heading>
                </div>
                <ul className="space-y-2">
                  <li className="flex gap-2">
                    <span className="text-orange-600">▪</span>
                    <a
                      href="https://copilot.github.trust.page/"
                      className="text-orange-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      GitHub Copilot Trust Center
                    </a>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-orange-600">▪</span>
                    <a
                      href="https://docs.github.com/en/copilot/managing-copilot"
                      className="text-orange-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Copilot Policy & Security
                    </a>
                  </li>
                </ul>
              </Box>

              <Box background="surface-action-subtle" padding="space-16" borderRadius="medium">
                <div className="flex items-center gap-2 mb-2">
                  <BranchingIcon className="text-blue-600" aria-hidden />
                  <Heading size="small" level="3">
                    Nav-spesifikk
                  </Heading>
                </div>
                <ul className="space-y-2">
                  <li className="flex gap-2">
                    <span className="text-blue-600">▪</span>
                    <a
                      href="https://utvikling.intern.nav.no/teknisk/github-copilot.html"
                      className="text-blue-600 hover:underline text-sm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Om GitHub Copilot i Nav
                    </a>
                  </li>
                </ul>
              </Box>
            </HGrid>
          </Box>

          {/* Footer tip */}
          <Box background="surface-info-subtle" padding="space-16" borderRadius="medium">
            <div className="flex items-center gap-2 mb-2">
              <LightBulbIcon className="text-blue-700" aria-hidden />
              <Heading size="small" level="3" className="text-blue-700">
                Tips
              </Heading>
            </div>
            <BodyShort className="text-gray-700 text-sm">
              Copilot utvikles raskt – hold deg oppdatert via GitHub Blog og awesome-copilot. Husk at agenten er et
              verktøy: du eier arkitekturen, den implementerer.
            </BodyShort>
          </Box>
        </VStack>
      </Box>
    </main>
  );
}
