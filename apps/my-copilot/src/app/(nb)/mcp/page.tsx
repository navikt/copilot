import { BodyLong, BodyShort, Box, Button, Heading, HGrid, HStack, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { PILOT_SERVERS } from "@/app/(nb)/(mcp)/mcp/fleet-gateway/servers";
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

function Entry({
  title,
  intro,
  children,
  href,
  cta,
}: {
  title: string;
  intro: string;
  children: React.ReactNode;
  href: string;
  cta: string;
}) {
  return (
    <Box
      as="section"
      borderWidth="1"
      borderColor="neutral-subtle"
      borderRadius="12"
      padding="space-24"
      className="mcp-entry"
    >
      <VStack gap="space-12" align="start" className="h-full">
        <Heading size="medium" level="2">
          {title}
        </Heading>
        <BodyLong>{intro}</BodyLong>
        {children}
        <Button as="a" href={href} variant="secondary" className="mcp-entry-cta">
          {cta}
        </Button>
      </VStack>
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
          <VStack gap="space-16" align="center" className="text-center">
            <Tag variant="info" size="small">
              MCP i Nav
            </Tag>
            <Heading size="xlarge" level="1">
              Navs MCP-servere, én innlogging
            </Heading>
            <BodyLong size="large" className="max-w-2xl">
              Koble KI-assistenten din til verktøyene i Nav. Du logger inn én gang med Nav-kontoen.
            </BodyLong>
          </VStack>

          <McpHeroDiagram servers={PILOT_SERVERS.map((s) => s.title)} />

          <VStack gap="space-20" align="center">
            <HStack gap="space-12" justify="center">
              <Button as="a" href="/mcp/bruke">
                Bruke
              </Button>
              <Button as="a" href="/mcp/tilby-server" variant="secondary">
                Tilby en server
              </Button>
            </HStack>
            {stats ? (
              <VStack gap="space-8" align="center">
                <HGrid
                  columns={3}
                  gap="space-12"
                  role="group"
                  aria-label="Tall fra MCP-registeret"
                  className="mcp-stats"
                >
                  <Stat value={stats.servers} label="servere" long="MCP-servere i registeret" />
                  <Stat value={stats.tools} label="verktøy" long="verktøy i registeret" />
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
            <Entry
              title="Bruke"
              intro="Bruk Navs MCP-servere fra klienten din."
              href="/mcp/bruke"
              cta="Koble til en klient"
            >
              <Bullets>
                <li>
                  Med nav-pilot: <code className={code}>nav-pilot mcp enable kodeverk</code>
                </li>
                <li>Copilot CLI og OpenCode: legg inn adressen til serveren, så starter innloggingen selv.</li>
                <li>
                  <NextLink href="/mcp/fleet-gateway" className={linkClass}>
                    Serverne i piloten
                  </NextLink>
                </li>
              </Bullets>
            </Entry>
            <Entry
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
            </Entry>
          </HGrid>

          <VStack gap="space-16">
            <Heading size="medium" level="2">
              Hvorfor gå gjennom fleet-gateway
            </Heading>
            <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
              {[
                ["Én innlogging", "Logg inn én gang, og bruk alle serverne du har tilgang til."],
                [
                  "Logg",
                  "Hvert kall logges med NAV-ident, rute og MCP-metode. En rute kan stenges raskt hvis noe går galt.",
                ],
                ["OBO", "Serveren får et token på vegne av deg, aldri ditt eget. Tilgangen din følger med."],
              ].map(([title, text]) => (
                <Box key={title} background="neutral-soft" borderRadius="8" padding="space-20">
                  <VStack gap="space-8">
                    <Heading size="small" level="3">
                      {title}
                    </Heading>
                    <BodyShort>{text}</BodyShort>
                  </VStack>
                </Box>
              ))}
            </HGrid>
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
