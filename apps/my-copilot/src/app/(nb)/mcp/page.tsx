import { BodyLong, BodyShort, Box, Button, Heading, HGrid, HStack, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { Accordion, AccordionContent, AccordionHeader, AccordionItem } from "@/components/aksel-accordion";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { EntryCard, PilotServerCards, WhyGateway } from "@/components/mcp/blocks";
import { enableCommand } from "@/components/mcp/servers";
import { McpHeroDiagram } from "@/components/mcp-hero-diagram";
import { Bullets, code, linkClass } from "@/components/nav-pilot/doc-page";
import { getMcpServers } from "@/lib/mcp-registry";

export const metadata: Metadata = {
  title: "MCP i Nav",
  description:
    "Koble KI-assistenten din til MCP-servere fra Nav-team med én innlogging, eller tilby teamets egen server gjennom fleet-gateway.",
};

// Numbers from the MCP registry, as on /verktoy. getMcpServers returns [] when the registry is down.
async function registryStats() {
  const servers = await getMcpServers();
  if (servers.length === 0) return null;
  return {
    toolsPerServer: Object.fromEntries(servers.filter((s) => s.tools).map((s) => [s.serverId, s.tools!.length])),
    servers: servers.length,
    // Only a full total: while some servers have no tool list, the sum would undercount (#2604).
    tools: servers.every((s) => s.tools) ? servers.reduce((sum, s) => sum + s.tools!.length, 0) : 0,
    gateway: servers.filter((s) => s.remotes?.some((r) => r.url.includes("fleet-gateway."))).length,
  };
}

// The short label is what you see; screen readers get the long one.
function Stat({ value, label, long }: { value: number; label: string; long: string }) {
  return (
    <Box background="neutral-soft" borderRadius="8" paddingBlock="space-12" paddingInline="space-20">
      <span className="sr-only">{`${value} ${long}`}</span>
      <span aria-hidden>
        <BodyShort as="span" size="large" weight="semibold" className="mcp-stat-value">
          {value}
        </BodyShort>
        <BodyShort as="span" textColor="subtle">
          {label}
        </BodyShort>
      </span>
    </Box>
  );
}

const SECURITY: { title: string; text: string }[] = [
  {
    title: "Tilgang",
    text: "Gatewayen sjekker hvem du er og gir et eget token til akkurat denne serveren (OBO). Ditt eget token sendes aldri videre.",
  },
  {
    title: "Godkjente verktøy",
    text: "Bare verktøy eierteamet har godkjent, slipper gjennom. Andre kall stoppes i gatewayen.",
  },
  {
    title: "Ratelimit",
    text: "Gatewayen stopper når for mange kall kjører samtidig for en bruker. Grensen for kall per minutt setter serveren selv.",
  },
  { title: "Nødstopp", text: "Vi kan stenge en server med en gang hvis noe går galt." },
  {
    title: "Revisjonslogg",
    text: "Hvert kall logges med Nav-ident, klient, rute, MCP-metode, verktøy, utfall og beslutning.",
  },
  {
    title: "Innlogging som utløper",
    // fleet-gateway: SESSION_IDLE_TIMEOUT 12h (sliding refresh), SESSION_ABSOLUTE_MAX_AGE 7d.
    text: "Innloggingen fornyes automatisk mens du bruker den, i opptil 7 dager. Bruker du den ikke på 12 timer, må du logge inn på nytt. Godkjenningen av klienten huskes i 30 dager per verktøy og server.",
  },
];

const FLEET: { name: string; status: string; variant: "success" | "info" | "neutral"; text: string }[] = [
  {
    name: "fleet-gateway",
    status: "Pilot i prod",
    variant: "success",
    text: "Tar imot verktøykall, sjekker dem og sender dem videre til serveren.",
  },
  {
    name: "fleet-control",
    status: "I dev",
    variant: "info",
    text: "Servere, ruter, verktøy og agentprofiler. Jobb-API: jobb-ID inn, artefakt ut. Skrive-API og godkjenning i dev.",
  },
  {
    name: "fleet-runtime",
    status: "Ikke bygget",
    variant: "neutral",
    text: "Sandkasse per agentjobb, stengt for trafikk ut. Prøves ut først.",
  },
];

const VISION: { title: string; text: string }[] = [
  {
    title: "Ola starter en jobb i appen",
    text: "Appen starter en agentjobb gjennom jobb-API-et i fleet-control og får en jobb-ID. Jobben kjører i en egen sandkasse i fleet-runtime.",
  },
  {
    title: "Agenten spør teamets egen modell",
    text: "Først Vertex AI i teamets eget GCP-prosjekt. Nøkkelen legges på utenfor sandkassen, så agenten ser den aldri.",
  },
  {
    title: "Agenten søker i Dolly gjennom fleet-gateway",
    text: "fleet-control sier hva agentprofilen har lov til. Dolly lager bare syntetiske testpersoner.",
  },
  { title: "Ola godkjenner bestillingen", text: "Å bestille er skriving. Appen spør Ola, og jobben venter." },
  {
    title: "Agenten bestiller gjennom fleet-gateway",
    text: "fleet-gateway sjekker godkjenningen. Jobben avsluttes, og resultatet kommer som et artefakt.",
  },
];

function Section({ title, children, id }: { title: string; children: ReactNode; id?: string }) {
  return (
    <VStack as="section" gap="space-16" id={id}>
      <Heading size="large" level="2">
        {title}
      </Heading>
      {children}
    </VStack>
  );
}

export default async function McpPage() {
  const stats = await registryStats();
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <Box
        className="max-w-6xl mx-auto"
        paddingBlock={{ xs: "space-24", md: "space-48" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32" }}
      >
        <VStack gap={{ xs: "space-40", md: "space-64" }}>
          <VStack as="section" gap="space-28">
            <HGrid columns={{ xs: 1, lg: "3fr 2fr" }} gap="space-40" align="start">
              <VStack gap="space-16" align="start">
                <Tag variant="alt1" size="small">
                  Pilot
                </Tag>
                <Heading size="xlarge" level="1">
                  Navs MCP-servere, én innlogging
                </Heading>
                <BodyLong size="large">
                  Koble KI-assistenten din til verktøyene i Nav. Du logger inn én gang med Nav-kontoen. MCP er en åpen
                  standard for hvordan en KI-assistent kaller verktøy og henter data.
                </BodyLong>
                <HStack gap="space-12">
                  <Button as="a" href="/mcp/bruke">
                    Bruke
                  </Button>
                  <Button as="a" href="/mcp/tilby-server" variant="secondary">
                    Tilby en server
                  </Button>
                </HStack>
              </VStack>
              <VStack gap="space-12">
                {stats ? (
                  <VStack gap="space-8">
                    <HGrid
                      columns={stats.tools > 0 ? 3 : 2}
                      gap="space-12"
                      role="group"
                      aria-label="Tall fra MCP-registeret"
                    >
                      <Stat value={stats.servers} label="servere" long="MCP-servere i registeret" />
                      {/* Hidden while some servers have no tool list (#2604). */}
                      {stats.tools > 0 && <Stat value={stats.tools} label="verktøy" long="verktøy i registeret" />}
                      <Stat value={stats.gateway} label="via gateway" long="servere bak fleet-gateway" />
                    </HGrid>
                    <BodyShort size="small" textColor="subtle">
                      Fra MCP-registeret, oppdatert hver time.
                    </BodyShort>
                  </VStack>
                ) : (
                  <BodyShort textColor="subtle">
                    Fant ikke MCP-registeret akkurat nå. Se serverne i piloten i{" "}
                    <NextLink href="/mcp/fleet-gateway#servere-i-piloten" className={linkClass}>
                      oversikten over fleet-gateway
                    </NextLink>
                    .
                  </BodyShort>
                )}
                <Box background="neutral-soft" borderRadius="12" padding="space-20">
                  <VStack gap="space-8">
                    <Heading size="xsmall" level="2">
                      Kort fortalt
                    </Heading>
                    <Bullets>
                      <li>Kall til serverne i piloten går gjennom fleet-gateway.</li>
                      <li>Serveren får et token laget for akkurat den serveren, aldri ditt eget.</li>
                      <li>Agenten kan aldri mer enn brukeren, og modellen ser aldri tokenet.</li>
                      <li>Hvert kall logges.</li>
                    </Bullets>
                  </VStack>
                </Box>
              </VStack>
            </HGrid>
            <McpHeroDiagram />
          </VStack>

          <Section title="Hva du kan gjøre">
            <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
              <EntryCard
                title="Bruke MCP"
                intro="Bruk Navs MCP-servere fra klienten din."
                href="/mcp/bruke"
                cta="Koble til en klient"
                level="3"
              >
                <ol className="mcp-steps">
                  <li>
                    Koble til naisdevice, og velg en server i{" "}
                    <NextLink href="/verktoy" className={linkClass}>
                      verktøykatalogen
                    </NextLink>
                    .
                  </li>
                  <li>
                    Legg inn adressen i Copilot CLI eller OpenCode, så starter innloggingen selv. Eller kjør{" "}
                    <code className={code}>{enableCommand("kodeverk")}</code>.
                  </li>
                  <li>
                    Logg inn med Nav-kontoen i Entra ID og godkjenn klienten. Agenten ser verktøyene til serveren og
                    bruker dem med din tilgang.
                  </li>
                </ol>
                <NextLink href="/mcp/fleet-gateway" className={linkClass}>
                  Serverne i piloten
                </NextLink>
              </EntryCard>
              <EntryCard
                title="Tilby en MCP-server"
                intro="La resten av Nav bruke MCP-serveren til teamet ditt. Dere slipper å bygge innlogging selv."
                href="/mcp/tilby-server"
                cta="Les veiledningen"
                level="3"
              >
                <ol className="mcp-steps">
                  <li>
                    Følg{" "}
                    <NextLink href="/mcp/tilby-server/retningslinjer" className={linkClass}>
                      retningslinjene for MCP-servere
                    </NextLink>
                    : protokoll 2026-07-28, sjekk av tokenet med Texas.
                  </li>
                  <li>
                    Slipp inn fleet-gateway i <code className={code}>accessPolicy.inbound</code>.
                  </li>
                  <li>
                    Be om en rute i fleet-gateway, og test i dev før prod. Bruk{" "}
                    <NextLink href="/mcp/tilby-server#sjekkliste" className={linkClass}>
                      sjekklisten før dere ber om en rute
                    </NextLink>
                    .
                  </li>
                </ol>
                <NextLink href="/mcp/tilby-server/retningslinjer#eksempler" className={linkClass}>
                  Maler, samsvarstesten mcp-compliance og GitHub Action-en
                </NextLink>
              </EntryCard>
            </HGrid>
          </Section>

          <Section title="Hvorfor gateway">
            <WhyGateway />
          </Section>

          <Section title="MCP-servere i Nav">
            <PilotServerCards compact tools={stats?.toolsPerServer} />
            <BodyShort>
              <NextLink href="/mcp/fleet-gateway#servere-i-piloten" className={linkClass}>
                Oppsett for hver server
              </NextLink>{" "}
              ·{" "}
              <NextLink href="/verktoy" className={linkClass}>
                Alle MCP-servere i verktøykatalogen
              </NextLink>
            </BodyShort>
          </Section>

          <Section title="Slik er det sikret">
            <BodyLong className="max-w-2xl">
              Dette kjører i dag. Agenten kan aldri mer enn brukeren, og modellen ser aldri tokenet.
            </BodyLong>
            <Accordion>
              {SECURITY.map((s) => (
                <AccordionItem key={s.title}>
                  <AccordionHeader>{s.title}</AccordionHeader>
                  <AccordionContent>{s.text}</AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </Section>

          <Section title="Status for Fleet">
            <BodyLong className="max-w-2xl">
              Fleet er modulene Nav bygger for å kjøre og styre KI-agenter og verktøykall trygt.
            </BodyLong>
            <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
              {FLEET.map((f) => (
                <Box key={f.name} borderWidth="1" borderColor="neutral-subtle" borderRadius="12" padding="space-20">
                  <VStack gap="space-8" align="start">
                    <Tag variant={f.variant} size="small">
                      {f.status}
                    </Tag>
                    <Heading size="small" level="3">
                      {f.name}
                    </Heading>
                    <BodyShort>{f.text}</BodyShort>
                  </VStack>
                </Box>
              ))}
            </HGrid>
          </Section>

          <Section title="Dit vi vil">
            <HStack gap="space-8" align="center">
              <BodyShort>Ett spørsmål, fem steg.</BodyShort>
              <Tag variant="warning" size="small">
                Eksempel · forslag
              </Tag>
            </HStack>
            <Accordion>
              {VISION.map((v, i) => (
                <AccordionItem key={v.title}>
                  <AccordionHeader>{`${i + 1}. ${v.title}`}</AccordionHeader>
                  <AccordionContent>{v.text}</AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </Section>

          <BodyShort textColor="subtle">
            Adresser, protokollversjoner og feilkoder står i{" "}
            <NextLink href="/mcp/referanse" className={linkClass}>
              referansen
            </NextLink>
            . Mer om gatewayen:{" "}
            <NextLink href="/mcp/fleet-gateway" className={linkClass}>
              fleet-gateway
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </Box>
    </main>
  );
}
