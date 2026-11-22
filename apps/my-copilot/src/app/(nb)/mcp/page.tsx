import { BodyLong, BodyShort, Box, Button, Heading, HGrid, HStack, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { EntryCard, HowItWorks, PilotServerCards, WhyGateway } from "@/components/mcp/blocks";
import { PILOT_SERVERS, enableCommand } from "@/components/mcp/servers";
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
    tools: servers.reduce((sum, s) => sum + (s.tools?.length ?? 0), 0),
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

export default async function McpPage() {
  const stats = await registryStats();
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <Box
        className="max-w-6xl mx-auto"
        paddingBlock={{ xs: "space-24", md: "space-48" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32" }}
      >
        <VStack gap={{ xs: "space-32", md: "space-48" }}>
          <VStack gap="space-16" align="start">
            <Tag variant="alt1" size="small">
              Pilot
            </Tag>
            <Heading size="xlarge" level="1">
              Navs MCP-servere, én innlogging
            </Heading>
            <BodyLong size="large" className="max-w-2xl">
              Koble KI-assistenten din til verktøyene i Nav. Du logger inn én gang med Nav-kontoen.
            </BodyLong>
          </VStack>

          <McpHeroDiagram servers={PILOT_SERVERS.map((s) => s.title)} />

          <VStack gap="space-20" align="start">
            <HStack gap="space-12">
              <Button as="a" href="/mcp/bruke">
                Bruke
              </Button>
              <Button as="a" href="/mcp/tilby-server" variant="secondary">
                Tilby en server
              </Button>
            </HStack>
            {stats ? (
              <VStack gap="space-8" align="start">
                <HGrid
                  columns={stats.tools > 0 ? 3 : 2}
                  gap="space-12"
                  role="group"
                  aria-label="Tall fra MCP-registeret"
                  className="mcp-stats"
                >
                  <Stat value={stats.servers} label="servere" long="MCP-servere i registeret" />
                  {/* Hidden while the registry has no tool lists (#2604). */}
                  {stats.tools > 0 && <Stat value={stats.tools} label="verktøy" long="verktøy i registeret" />}
                  <Stat value={stats.gateway} label="via gateway" long="servere bak fleet-gateway" />
                </HGrid>
                <BodyShort size="small" textColor="subtle">
                  Fra MCP-registeret, oppdatert hver time.
                </BodyShort>
              </VStack>
            ) : (
              <BodyShort textColor="subtle">
                Fant ikke MCP-registeret akkurat nå. Se serverne i{" "}
                <NextLink href="/verktoy" className={linkClass}>
                  verktøykatalogen
                </NextLink>
                .
              </BodyShort>
            )}
          </VStack>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
            <EntryCard
              title="Bruke"
              intro="Bruk Navs MCP-servere fra klienten din."
              href="/mcp/bruke"
              cta="Koble til en klient"
            >
              <Bullets>
                <li>
                  Med nav-pilot: <code className={code}>{enableCommand("kodeverk")}</code>
                </li>
                <li>Copilot CLI og OpenCode: legg inn adressen til serveren, så starter innloggingen selv.</li>
                <li>
                  <NextLink href="/mcp/fleet-gateway" className={linkClass}>
                    Serverne i piloten
                  </NextLink>
                </li>
              </Bullets>
            </EntryCard>
            <EntryCard
              title="Tilby en server"
              intro="La resten av Nav bruke MCP-serveren til teamet ditt. Dere slipper å bygge innlogging selv."
              href="/mcp/tilby-server"
              cta="Les veiledningen"
            >
              <Bullets>
                <li>
                  <NextLink href="/mcp/tilby-server/retningslinjer" className={linkClass}>
                    Retningslinjer for MCP-servere
                  </NextLink>
                </li>
                <li>
                  <NextLink href="/mcp/tilby-server/retningslinjer#eksempler" className={linkClass}>
                    Maler, samsvarstesten mcp-compliance og GitHub Action-en
                  </NextLink>
                </li>
                <li>
                  <NextLink href="/mcp/tilby-server#sjekkliste" className={linkClass}>
                    Sjekkliste før dere ber om en rute
                  </NextLink>
                </li>
              </Bullets>
            </EntryCard>
          </HGrid>

          <VStack gap="space-16">
            <Heading size="medium" level="2">
              Hvorfor gateway
            </Heading>
            <WhyGateway />
          </VStack>

          <VStack gap="space-16">
            <Heading size="medium" level="2">
              Slik fungerer det
            </Heading>
            <HowItWorks />
          </VStack>

          <VStack gap="space-16">
            <Heading size="medium" level="2">
              Serverne i piloten
            </Heading>
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
          </VStack>

          <BodyShort textColor="subtle">
            Adresser, protokollversjoner og feilkoder står i{" "}
            <NextLink href="/mcp/referanse" className={linkClass}>
              referansen
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </Box>
    </main>
  );
}
