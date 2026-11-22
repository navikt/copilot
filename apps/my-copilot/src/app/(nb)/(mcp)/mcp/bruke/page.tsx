import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { HowItWorks } from "@/components/mcp/blocks";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Bruke Navs MCP-servere",
  description:
    "Slik kobler du Copilot CLI, OpenCode eller nav-pilot til en MCP-server som et Nav-team tilbyr. Du logger inn med Nav-kontoen.",
};

const HELP_URL = "https://nav-it.slack.com/archives/C0ASJSZK8P8";

const TOC: TocItem[] = [
  { id: "slik-fungerer-det", label: "Slik fungerer det" },
  { id: "for-du-starter", label: "Før du starter" },
  { id: "copilot-cli", label: "Copilot CLI" },
  { id: "opencode", label: "OpenCode" },
  { id: "nav-pilot", label: "nav-pilot" },
  { id: "protokollversjon", label: "Protokollversjon" },
  { id: "innlogging", label: "Innlogging" },
  { id: "hjelp", label: "Hjelp" },
];

const OPENCODE_JSON = `{
  "mcp": {
    "onboarding": {
      "type": "remote",
      "url": "https://fleet-gateway.intern.nav.no/mcp/copilot/onboarding"
    }
  }
}`;

export default function NavMcpServerePage() {
  return (
    <DocPage label="MCP" title="Bruke Navs MCP-servere" description={metadata.description as string} toc={TOC}>
      <VStack gap="space-12">
        <LinkableHeading id="slik-fungerer-det" size="medium" level="2">
          Slik fungerer det
        </LinkableHeading>
        <HowItWorks />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="for-du-starter" size="medium" level="2">
          Før du starter
        </LinkableHeading>
        <Bullets>
          <li>
            Foreløpig bare internt: du må være på naisdevice for å nå{" "}
            <code className={code}>fleet-gateway.intern.nav.no</code>. En ekstern gateway med offentlig ingress er{" "}
            <a href="https://github.com/navikt/copilot/issues/2297" className={linkClass}>
              planlagt (#2297)
            </a>
            . Den vil kreve en Nav-administrert enhet, og DCR blir slått av eksternt.
          </li>
          <li>
            Finn serveren i{" "}
            <NextLink href="/verktoy" className={linkClass}>
              verktøykatalogen
            </NextLink>
            . Der står adressen, for eksempel{" "}
            <code className={code}>https://fleet-gateway.intern.nav.no/mcp/copilot/onboarding</code>.
          </li>
          <li>
            Eksemplene under bruker <code className={code}>onboarding</code>. Bytt ut navn og adresse med serveren du
            vil bruke.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="copilot-cli" size="medium" level="2">
          Copilot CLI
        </LinkableHeading>
        <BodyLong>
          Testet i prod. Nav tillater bare MCP-servere som står i MCP-registeret. Skriv{" "}
          <code className={code}>/mcp add</code> i Copilot CLI, velg type <code className={code}>http</code> og lim inn
          adressen fra katalogen. Innloggingen starter første gang agenten bruker serveren.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="opencode" size="medium" level="2">
          OpenCode
        </LinkableHeading>
        <BodyLong>
          Testet i prod. Legg serveren til i <code className={code}>opencode.json</code>:
        </BodyLong>
        <CodeBlock filename="opencode.json">{OPENCODE_JSON}</CodeBlock>
        <BodyLong>Logg inn, og start OpenCode:</BodyLong>
        <CodeBlock filename="Terminal">{"opencode mcp auth onboarding\nopencode"}</CodeBlock>
        <BodyLong>
          Logg ut med <code className={code}>opencode mcp logout onboarding</code>.
        </BodyLong>
        <BodyLong>
          nav-pilot setter <code className={code}>protocol</code> selv, ut fra protokollversjonene i MCP-registeret.
          Konfigurerer du OpenCode 2.x for hånd, bruker du{" "}
          <code className={code}>&quot;protocol&quot;: &quot;auto&quot;</code>. Bruk{" "}
          <code className={code}>&quot;2026-07-28&quot;</code> bare når registeret oppgir den som eneste versjon. Se{" "}
          <a href="#protokollversjon" className={linkClass}>
            Protokollversjon
          </a>
          .
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="nav-pilot" size="medium" level="2">
          nav-pilot
        </LinkableHeading>
        <BodyLong>
          <code className={code}>nav-pilot mcp enable &lt;navn&gt;</code> slår på en server fra registeret. Vi har ikke
          testet dette mot Navs egne MCP-servere ennå. Virker det ikke, bruk Copilot CLI eller OpenCode direkte.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="protokollversjon" size="medium" level="2">
          Protokollversjon
        </LinkableHeading>
        <Bullets>
          <li>
            OpenCode 2.x: <code className={code}>&quot;protocol&quot;: &quot;2026-07-28&quot;</code> virker bare mot
            servere som bare støtter revisjonen. <code className={code}>onboarding</code> støtter både{" "}
            <code className={code}>2026-07-28</code> og eldre protokoll, så bruk{" "}
            <code className={code}>&quot;protocol&quot;: &quot;auto&quot;</code> der.
          </li>
          <li>
            OpenCode 1.19 støtter ikke <code className={code}>2026-07-28</code> og bruker eldre protokoll.
          </li>
          <li>
            Copilot CLI prøver <code className={code}>2026-07-28</code> først og faller tilbake til eldre protokoll.
            Bruk versjon <code className={code}>1.0.107</code> eller nyere.
          </li>
          <li>
            <code className={code}>nav-pilot</code> setter <code className={code}>protocol</code> selv på nye
            OpenCode-oppføringer, ut fra <code className={code}>protocolVersions</code> i MCP-registeret.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="innlogging" size="medium" level="2">
          Innlogging
        </LinkableHeading>
        <Bullets>
          <li>Første gang åpnes nettleseren. Du logger inn med Nav-kontoen og godkjenner klienten.</li>
          <li>Godkjenningen huskes i 30 dager per verktøy og server.</li>
          <li>En innlogging varer i omtrent 12 timer. Etter det logger du inn på nytt.</li>
          <li>
            Står det «Unverified client» på godkjenningssiden, fortsett bare hvis du nettopp startet innloggingen selv.
          </li>
          <li>
            Teamet som eier serveren, bestemmer hva du får gjøre. Innloggingen går gjennom{" "}
            <NextLink href="/mcp/fleet-gateway" className={linkClass}>
              fleet-gateway
            </NextLink>
            , men det trenger du ikke tenke på.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hjelp" size="medium" level="2">
          Hjelp
        </LinkableHeading>
        <BodyLong>
          Spør i{" "}
          <a href={HELP_URL} className={linkClass}>
            #ki-produktutvikling-diskusjon
          </a>
          .
        </BodyLong>
      </VStack>
    </DocPage>
  );
}
