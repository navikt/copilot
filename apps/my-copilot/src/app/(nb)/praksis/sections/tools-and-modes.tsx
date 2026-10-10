import NextLink from "next/link";
import { Heading, BodyShort, Box } from "@navikt/ds-react";
import { Carousel } from "@/components/carousel";
import { LaptopIcon, GlobeIcon, TerminalIcon, CpuIcon, CogIcon } from "@navikt/aksel-icons";

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
                Assisterende KI. Du kan stille spørsmål om koden din, be om forklaringer, eller generere nye funksjoner.
                Vær obs på at den ikke alltid forstår hele prosjektet uten at du eksplisitt nevner filene.
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                3. Copilot Edits / Agent Mode (Cmd+Shift+I)
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Autonom KI. Du gir et stort mål ("Bytt ut alle fetch-kall med axios"), og Copilot åpner flere filer,
                endrer dem, og ber deg godkjenne diff-en til slutt.
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
                Agent mode finnes også i JetBrains-IDE-ene. Nye funksjoner kommer ofte først til VS Code, så noen
                slash-kommandoer og innstillinger kan mangle i IntelliJ og Visual Studio en stund.
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
            alt="Copilot cloud agent på GitHub"
            className="w-full rounded-md mb-3 border border-green-200"
          />
          <div className="space-y-3">
            <div>
              <BodyShort weight="semibold" className="text-sm">
                Copilot cloud agent
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
                Copilot CLI: en agent i terminalen som bygger, feilsøker og refaktorerer kode. På Nav-utstyr skal den
                kjøre i{" "}
                <NextLink href="/cplt" className="text-blue-600 hover:underline">
                  cplt
                </NextLink>
                . Installer med{" "}
                <NextLink href="/nav-pilot/guider/kom-i-gang" className="text-blue-600 hover:underline">
                  nav-pilot
                </NextLink>
                , som også setter opp opencode.
              </BodyShort>
            </div>
            <Box background="default" padding="space-8" borderRadius="4">
              <code className="text-xs block">copilot</code>
              <code className="text-xs block mt-1 text-gray-500"># Åpner interaktiv agent-modus</code>
            </Box>
            <BodyShort className="text-gray-500 text-xs">
              Installer: <code className="bg-gray-100 px-1 rounded">brew install copilot-cli</code>{" "}
              <code className="bg-gray-100 px-1 rounded">winget install GitHub.Copilot</code>{" "}
              <code className="bg-gray-100 px-1 rounded">curl -fsSL https://gh.io/copilot-install | bash</code>
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
          Utvid Copilot med eksterne verktøy via MCP-servere. Tilgjengelig i agent mode i editoren, Copilot CLI,
          opencode og Copilot cloud agent på GitHub.com.
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
          <NextLink href="/verktoy?type=mcp" className="text-blue-600 hover:underline">
            verktøy-siden
          </NextLink>
          .
        </BodyShort>
        <BodyShort className="text-gray-600 text-xs mt-3">
          Nav har også en{" "}
          <NextLink
            href="/verktoy?item=mcp-io.github.navikt%2Fmcp-onboarding"
            className="text-blue-600 hover:underline"
          >
            MCP onboarding-server
          </NextLink>{" "}
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
        <BodyShort className="text-gray-600 text-sm" style={{ marginBottom: "var(--ax-space-8)" }}>
          <strong>Start med Auto.</strong> Da velger Copilot modell for deg. Bytt modell bare når du har en grunn, for
          eksempel at agenten står fast på en vanskelig oppgave.
        </BodyShort>
        <BodyShort className="text-gray-600 text-sm">
          Bruken måles i <strong>AI Credits</strong>. Hver bruker har en kvote som samles i en felles pott for Nav, og
          modellene koster ulikt. Noen modeller har Nav slått av. Se{" "}
          <NextLink href="/priser" className="text-blue-600 hover:underline">
            priser
          </NextLink>{" "}
          og{" "}
          <NextLink href="/modeller" className="text-blue-600 hover:underline">
            modeller
          </NextLink>{" "}
          for hva som finnes og hva det koster.
        </BodyShort>
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
      </Box>
    </div>
  );
}
