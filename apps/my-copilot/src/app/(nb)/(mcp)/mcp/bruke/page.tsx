import { BodyLong, ReadMore, VStack } from "@navikt/ds-react";
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
  { id: "tokenbruk", label: "Tokenforbruk" },
  { id: "agentverktoy", label: "Ser ikke modellen verktøyene?" },
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
          <li>
            Innloggingen fornyes automatisk mens du bruker den, i opptil 7 dager. Bruker du den ikke på 12 timer, må du
            logge inn på nytt.
          </li>
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
        <LinkableHeading id="tokenbruk" size="medium" level="2">
          Tokenforbruk
        </LinkableHeading>
        <BodyLong>
          <strong>Kort fortalt:</strong> Hver MCP-server du kobler til, tar plass i konteksten til modellen i hver tur.
          Slå bare på serverne og verktøyene du trenger i oppgaven.
        </BodyLong>
        <BodyLong>
          MCP er en åpen protokoll som lar agenten kalle verktøy og hente data fra systemer som Dolly og Teamkatalogen.
          Agenten slår opp selv, så du slipper å lime inn svar. Det er nyttig når oppgaven trenger data du ikke har for
          hånden.
        </BodyLong>
        <LinkableHeading id="hvorfor-spare" size="small" level="3">
          Hvorfor spare på MCP-servere
        </LinkableHeading>
        <BodyLong>
          Modellen har et begrenset kontekstvindu. Alt agenten skal vurdere i én tur, må få plass der: instruksjonene,
          samtalen så langt, filene den har lest og beskrivelsene av alle verktøyene den kan kalle.
        </BodyLong>
        <BodyLong>
          Mange klienter laster beskrivelsen av hvert verktøy inn i konteksten i hver samtale, også når agenten ikke
          bruker verktøyene. Mange servere gir da mindre plass til koden og oppgaven din. Alt som sendes til modellen,
          er tokens, så jo mer som ligger i konteksten, desto mer må modellen behandle i hver tur.
        </BodyLong>
        <BodyLong>
          Copilot CLI laster verktøy ved behov når du har rundt 30 verktøy eller flere og bruker en modell som støtter
          det. Da ligger bare de innebygde verktøyene i konteksten, og agenten søker etter resten. Under 30 verktøy, med
          andre modeller og i andre klienter kan alle beskrivelsene ligge i konteksten fra start (
          <a
            href="https://docs.github.com/en/copilot/concepts/agent-capabilities/agent-powered-workflows-and-operations/tool-search"
            className={linkClass}
          >
            GitHub Docs
          </a>
          ).
        </BodyLong>
        <BodyLong>
          Hvor mye det blir, avhenger av hvilke verktøy klienten gir til modellen. Med nav-pilot velger du enkeltverktøy
          når du slår på en server, og Copilot CLI har <code className={code}>--available-tools</code>. Verktøy du har
          filtrert bort, tar ikke plass. Gjennom fleet-gateway kan agenten bare kalle verktøyene som er godkjent for
          hver server. Det stopper kall, men fjerner ikke beskrivelsene fra konteksten. Vi planlegger også ett felles
          endpoint der agenten søker etter verktøy ved behov i stedet for å få alle beskrivelsene på forhånd (
          <a href="https://github.com/navikt/copilot/issues/2581" className={linkClass}>
            #2581
          </a>
          ).
        </BodyLong>
        <BodyLong>
          <a
            href="https://www.microsoft.com/en-us/research/blog/tokeneconomizer-learning-from-experience-to-make-ai-coding-agents-more-efficient/"
            className={linkClass}
          >
            Microsoft Research
          </a>{" "}
          profilerte Claude Code på 60 oppgaver fra SWE-Bench. Agenten fikk 23 verktøybeskrivelser i hver tur, og 16 av
          verktøyene ble aldri kalt. Tallene gjelder ikke Navs servere. Vi har ikke målt dem.
        </BodyLong>
        <BodyLong>
          Færre servere gir også mindre å holde styr på. Hver server du logger inn på, får tilgang på dine vegne, og
          hvert kall logges med Nav-identen din. Slår du av serverne du ikke bruker, er det færre steder agenten kan
          hente data eller gjøre endringer.
        </BodyLong>
        <LinkableHeading id="slik-sparer-du" size="small" level="3">
          Slik sparer du
        </LinkableHeading>
        <Bullets>
          <li>Slå på serverne du trenger i oppgaven, og slå dem av etterpå.</li>
          <li>
            Bruk Dolly når du skal bestille testpersoner og Teamkatalogen når du skal slå opp et team, ikke som fast
            oppsett.
          </li>
          <li>For oppslag du gjør sjelden: bruk et skript, en CLI eller en skill som lastes bare ved behov.</li>
          <li>
            Se hvor mange verktøy en server har før du kobler den til. Velg serveren med færrest verktøy som dekker
            behovet.
          </li>
          <li>Start en ny samtale når oppgaven er ferdig, så følger ikke gamle verktøyresultater med videre.</li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="agentverktoy" size="medium" level="2">
          Ser ikke modellen verktøyene?
        </LinkableHeading>
        <BodyLong>
          Serveren står som tilkoblet i <code className={code}>/mcp</code> i Copilot CLI eller i verktøyvelgeren, men
          modellen bruker aldri verktøyene. Da har agenten du bruker trolig en <code className={code}>tools:</code>
          -liste i frontmatter i <code className={code}>.agent.md</code>. Listen slipper bare gjennom verktøyene som
          står der. Alt annet er skjult, og ukjente navn ignoreres uten feilmelding.
        </BodyLong>
        <Bullets>
          <li>
            Fjern <code className={code}>tools:</code>, så får agenten alle verktøy. Det anbefaler vi for agenter som
            bruker MCP.
          </li>
          <li>
            Eller ta med <code className={code}>{"<server>/*"}</code> i listen.
          </li>
        </Bullets>
        <ReadMore header="Detaljer per klient" size="small">
          <Bullets>
            <li>
              Copilot CLI, GitHubs kodeagent og JetBrains-pluginen bruker samme format. Uten{" "}
              <code className={code}>tools:</code> eller med <code className={code}>{'["*"]'}</code> får agenten alle
              verktøy. MCP-verktøy skrives <code className={code}>{"<server>/*"}</code> eller{" "}
              <code className={code}>{"<server>/<verktøy>"}</code> (
              <a href="https://docs.github.com/en/copilot/reference/custom-agents-configuration" className={linkClass}>
                GitHub Docs
              </a>
              ). GitHub Copilot-appen er bygget på Copilot CLI.
            </li>
            <li>
              VS Code bruker også <code className={code}>.agent.md</code> (tidligere{" "}
              <code className={code}>.chatmode.md</code>) og <code className={code}>{"<server>/*"}</code>. Uten{" "}
              <code className={code}>tools:</code> bestemmer verktøyvelgeren (
              <a
                href="https://code.visualstudio.com/docs/agents/reference/custom-agent-file-reference"
                className={linkClass}
              >
                VS Code-dokumentasjonen
              </a>
              ).
            </li>
            <li>
              OpenCode styrer verktøy med tillatelser per agent. <code className={code}>{"<server>_*"}</code> treffer
              alle verktøyene til en server. Tegn utenom A–Z, a–z, 0–9, <code className={code}>_</code> og{" "}
              <code className={code}>-</code> i servernavnet blir <code className={code}>_</code>, så{" "}
              <code className={code}>com.figma/figma-mcp</code> blir <code className={code}>com_figma_figma-mcp_*</code>{" "}
              (
              <a href="https://opencode.ai/docs/agents" className={linkClass}>
                agenter
              </a>
              ,{" "}
              <a href="https://opencode.ai/docs/permissions" className={linkClass}>
                tillatelser
              </a>
              ,{" "}
              <a href="https://opencode.ai/docs/mcp-servers" className={linkClass}>
                MCP-servere
              </a>
              ).
            </li>
            <li>
              Testet i Copilot CLI 1.0.115: navn med <code className={code}>/</code> og <code className={code}>.</code>{" "}
              virker, for eksempel <code className={code}>com.figma/figma-mcp/*</code> og{" "}
              <code className={code}>io.github.navikt/aksel-mcp/*</code>. Vi har ikke bekreftet det samme for VS Code.
            </li>
          </Bullets>
        </ReadMore>
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
