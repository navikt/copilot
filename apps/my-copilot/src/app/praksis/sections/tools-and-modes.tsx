import { Heading, BodyShort, Box, HGrid, Label } from "@navikt/ds-react";
import { Carousel } from "@/components/carousel";
import { LinkableHeading } from "@/components/linkable-heading";
import { LaptopIcon, GlobeIcon, TerminalIcon, CpuIcon, CogIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";

export default function ToolsAndModes() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        Verktøy og Moduser
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        GitHub Copilot er ikke bare kodeforslag i editoren. Det er et økosystem av verktøy som spenner fra
        sanntidsforslag til autonome agenter som jobber i bakgrunnen.
      </BodyShort>

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
          Du har <strong>300 premium requests</strong> per måned. Inkluderte modeller (GPT-5 mini, GPT-4.1, GPT-4o)
          bruker ingen premium requests. <strong>Auto</strong> gir 10 % rabatt og velger beste modell automatisk.
        </BodyShort>
        <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-12">
          <div>
            <Label size="small" className="text-green-700">
              Inkluderte (0x)
            </Label>
            <BodyShort className="text-gray-600 text-xs">
              GPT-5 mini, GPT-4.1, GPT-4o — ingen premium requests
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
            <Label size="small">Claude Sonnet 4 / 4.5 / 4.6</Label>
            <BodyShort className="text-gray-600 text-xs">Balansert – god til de fleste oppgaver (1×)</BodyShort>
          </div>
          <div>
            <Label size="small">GPT-5.2 / 5.4</Label>
            <BodyShort className="text-gray-600 text-xs">OpenAI premium – bred kunnskap (1×)</BodyShort>
          </div>
          <div>
            <Label size="small">Claude Opus 4.5 / 4.6</Label>
            <BodyShort className="text-gray-600 text-xs">Kraftigst – komplekse oppgaver (3×)</BodyShort>
          </div>
          <div>
            <Label size="small">Haiku 4.5 / GPT-5.4 mini</Label>
            <BodyShort className="text-gray-600 text-xs">Raske – enklere oppgaver (0.33×)</BodyShort>
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
  );
}
