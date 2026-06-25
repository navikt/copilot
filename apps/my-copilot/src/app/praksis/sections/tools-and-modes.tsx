import { Heading, BodyShort, Box, HGrid, Label } from "@navikt/ds-react";
import { Carousel } from "@/components/carousel";
import { LaptopIcon, GlobeIcon, TerminalIcon, CpuIcon, CogIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";

export default function ToolsAndModes() {
  return (
    <div className="space-y-8">
      {/* Video showcase */}
      <Box background="default" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
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
        <Box background="info-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="max-w-lg">
          <div className="flex items-center gap-2 mb-5">
            <LaptopIcon className="text-blue-700" aria-hidden />
            <Heading size="small" level="3" className="text-blue-700">
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
                1. Ghost Text (Inline Autocomplete)
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Den svake teksten som dukker opp mens du skriver. Trykk Tab for å godta, Esc for å avvise. Copilot
                prøver å gjette din neste linje basert på filene du har åpne.
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                2. Copilot Chat (Cmd+I eller sidepanel)
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Assisterende AI. Du kan stille spørsmål om koden din, be om forklaringer, eller generere nye funksjoner.
                Vær obs på at den ikke alltid forstår hele prosjektet uten at du eksplisitt nevner filene.
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                3. Copilot Edits / Agent Mode (Cmd+Shift+I)
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Autonom AI. Dette er den nye "agent-modusen". Du gir et stort mål ("Bytt ut alle fetch-kall med axios"),
                og Copilot åpner flere filer, endrer dem, og ber deg godkjenne diff-en til slutt.
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
            <div>
              <BodyShort weight="semibold" className="text-sm text-blue-700">
                ⚠️ Forskjeller mellom IDE-er
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Vær oppmerksom på at IntelliJ og Visual Studio ofte ligger flere måneder bak VS Code i
                Copilot-funksjonalitet. Agent Mode / Copilot Edits og avanserte slash-kommandoer fungerer ofte best
                (eller kun) i VS Code.
              </BodyShort>
            </div>
          </div>
        </Box>

        {/* GitHub.com */}
        <Box
          background="success-soft"
          padding={{ xs: "space-12", sm: "space-16" }}
          borderRadius="8"
          className="max-w-lg"
        >
          <div className="flex items-center gap-2 mb-5">
            <GlobeIcon className="text-green-700" aria-hidden />
            <Heading size="small" level="3" className="text-green-700">
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
                Dashboard for å spore Copilot-oppgaver på tvers av repoer. Se fremdrift, session logs, og styr agenten
                underveis. Tilgjengelig via{" "}
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
          background="warning-soft"
          padding={{ xs: "space-12", sm: "space-16" }}
          borderRadius="8"
          className="max-w-lg"
        >
          <div className="flex items-center gap-2 mb-5">
            <TerminalIcon className="text-orange-700" aria-hidden />
            <Heading size="small" level="3" className="text-orange-700">
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
            <Box background="default" padding="space-8" borderRadius="4">
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
      <Box background="success-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-center gap-2 mb-2">
          <CogIcon className="text-green-700" aria-hidden />
          <Heading size="small" level="3" className="text-green-700">
            MCP (Model Context Protocol)
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm mb-2">
          Utvid Copilot med eksterne verktøy via MCP-servere. Tilgjengelig i Agent Mode (VS Code), Copilot CLI og Coding
          Agent på GitHub.com.
        </BodyShort>
        <BodyShort className="text-gray-600 text-sm mb-2">
          Navs{" "}
          <a
            href="https://mcp-registry.nav.no"
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            MCP-registry
          </a>{" "}
          er allerede konfigurert for alle brukere. Se tilgjengelige MCP-servere på{" "}
          <a href="/verktoy?type=mcp" className="text-blue-600 hover:underline">
            verktøy-siden
          </a>
          .
        </BodyShort>
        <BodyShort className="text-gray-600 text-xs mt-3">
          Nav har også en{" "}
          <a href="/verktoy?item=mcp-io.github.navikt%2Fmcp-onboarding" className="text-blue-600 hover:underline">
            MCP onboarding-server
          </a>{" "}
          som hjelper deg å sjekke hvor «agent-klar» repoet ditt er, og generere tilpasningsfiler.
        </BodyShort>
      </Box>

      {/* Model selection */}
      <Box background="accent-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
        <div className="flex items-center gap-2 mb-2">
          <CpuIcon className="text-blue-600" aria-hidden />
          <Heading size="small" level="3">
            Modellvalg og kostnader
          </Heading>
        </div>
        <Box background="warning-soft" padding="space-12" borderRadius="8" className="mb-4">
          <BodyShort size="small">
            <strong>Nytt kostnadsregime:</strong> GitHub går over til bruksbasert prising. Vi forventer omtrent 3×
            kostnadsøkning for organisasjonen. Bruk modellene bevisst — velg <strong>Auto</strong> eller inkluderte
            modeller der det er tilstrekkelig.{" "}
            <NextLink href="/priser" className="underline">
              Se fullstendig pristabell →
            </NextLink>
          </BodyShort>
        </Box>
        <BodyShort className="text-gray-600 text-sm mb-5">
          Du har <strong>300 premium requests</strong> per måned. Inkluderte modeller (GPT-4.1, GPT-4o og lignende)
          bruker ingen premium requests. <strong>Auto</strong> gir 10 % rabatt og velger beste modell automatisk.
        </BodyShort>
        <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-12">
          <div>
            <Label size="small" className="text-green-700">
              Inkluderte (0x)
            </Label>
            <BodyShort className="text-gray-600 text-xs">
              GPT-4.1, GPT-4o og tilsvarende — ingen premium requests
            </BodyShort>
          </div>
          <div>
            <Label size="small" className="text-green-700">
              Auto (10 % rabatt)
            </Label>
            <BodyShort className="text-gray-600 text-xs">
              Anbefalt – velger optimal modell, 0.9× multiplikator
            </BodyShort>
          </div>
          <div>
            <Label size="small">Claude Sonnet (siste)</Label>
            <BodyShort className="text-gray-600 text-xs">Balansert – god til de fleste oppgaver (1×)</BodyShort>
          </div>
          <div>
            <Label size="small">GPT-4o / nyeste OpenAI</Label>
            <BodyShort className="text-gray-600 text-xs">OpenAI premium – bred kunnskap (1×)</BodyShort>
          </div>
          <div>
            <Label size="small">Claude Opus (siste)</Label>
            <BodyShort className="text-gray-600 text-xs">Kraftigst – komplekse oppgaver (3×)</BodyShort>
          </div>
          <div>
            <Label size="small">Haiku / mini-modeller</Label>
            <BodyShort className="text-gray-600 text-xs">Raske – enklere oppgaver (0.33×)</BodyShort>
          </div>
        </HGrid>
        <Box background="info-soft" padding="space-12" borderRadius="8" className="mt-5">
          <Heading size="xsmall" level="4" className="mb-1 text-blue-700">
            Forvirret over "Context Window" måleren? (Reserved Output)
          </Heading>
          <BodyShort className="text-gray-700 text-xs">
            Nyere modeller reserverer automatisk en stor del av kontekstvinduet (ofte tusenvis av tokens) til sin
            interne "chain-of-thought" (resonnering). Derfor vil kontekstmåleren din kunne se nesten full ut selv om du
            bare har lagt ved et par filer. Dette er normalt og nødvendig for at modellen skal tenke seg om.
          </BodyShort>
        </Box>
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
    </div>
  );
}
