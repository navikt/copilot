import { EnterIcon, FileTextIcon, LinkIcon, PadlockLockedIcon, PersonIcon, WrenchIcon } from "@navikt/aksel-icons";
import { BodyLong, BodyShort, Box, Button, Heading, HGrid, HStack, ReadMore, Tag, VStack } from "@navikt/ds-react";
import type { ReactNode } from "react";
import { CommandBlock } from "@/components/detail-drawer/shared";
import { Bullets, code, linkClass } from "@/components/nav-pilot/doc-page";
import { PILOT_SERVERS, enableCommand } from "./servers";

// The MCP content that more than one page shows: /mcp, /mcp/bruke, /mcp/fleet-gateway and the
// /verktoy drawer. Change it here, and every page follows.

const PROD_ROUTES_URL = "https://github.com/navikt/copilot/blob/main/apps/fleet-gateway/pilot-routes.prod.yaml";
const ALLOWLIST_URL = "https://github.com/navikt/copilot/blob/main/apps/mcp-registry/allowlist.json";

type Point = { title: string; text: string; icon: typeof LinkIcon };

export const WHY_GATEWAY: Point[] = [
  {
    title: "Én innlogging",
    text: "Logg inn én gang, og bruk alle serverne du har tilgang til.",
    icon: PadlockLockedIcon,
  },
  { title: "Logg", text: "Hvert kall logges med Nav-ident, rute og MCP-metode.", icon: FileTextIcon },
  {
    title: "På vegne av deg",
    text: "Serveren får et OBO-token laget for akkurat den appen, aldri ditt eget. Tilgangen din følger med.",
    icon: PersonIcon,
  },
];

export const HOW_IT_WORKS: Point[] = [
  {
    title: "Legg inn adressen",
    text: "Legg adressen til serveren inn i klienten, eller kjør nav-pilot mcp enable med navnet på serveren.",
    icon: LinkIcon,
  },
  {
    title: "Logg inn én gang",
    text: "Nettleseren åpnes. Du logger inn med Nav-kontoen i Entra ID og godkjenner klienten.",
    icon: EnterIcon,
  },
  {
    title: "Bruk verktøyene",
    text: "Agenten ser verktøyene til serveren og bruker dem med din tilgang.",
    icon: WrenchIcon,
  },
];

function PointBoxes({ points, numbered, level }: { points: Point[]; numbered?: boolean; level: "2" | "3" }) {
  const h = level === "2" ? "3" : "4";
  return (
    <HGrid as={numbered ? "ol" : "ul"} columns={{ xs: 1, md: 3 }} gap="space-16" className="list-none">
      {points.map(({ title, text, icon: Icon }, i) => (
        <Box as="li" key={title} background="neutral-soft" borderRadius="8" padding="space-20">
          <VStack gap="space-8">
            <HStack gap="space-8" align="center" wrap={false}>
              <Icon aria-hidden fontSize="1.75rem" />
              <Heading size="small" level={h}>
                {numbered ? `${i + 1}. ${title}` : title}
              </Heading>
            </HStack>
            <BodyShort>{text}</BodyShort>
          </VStack>
        </Box>
      ))}
    </HGrid>
  );
}

/** The three reasons to go through fleet-gateway. level is the heading level of the section around it. */
export const WhyGateway = ({ level = "2" }: { level?: "2" | "3" }) => <PointBoxes points={WHY_GATEWAY} level={level} />;

/** The three steps from address to tools. */
export const HowItWorks = ({ level = "2" }: { level?: "2" | "3" }) => (
  <PointBoxes points={HOW_IT_WORKS} numbered level={level} />
);

/** A card on /mcp that starts one of the two tracks. The button sits at the bottom, so two cards line up. */
export function EntryCard({
  title,
  intro,
  children,
  href,
  cta,
  level = "2",
}: {
  level?: "2" | "3";
  title: string;
  intro: string;
  children: ReactNode;
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
        <Heading size="medium" level={level}>
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

/**
 * The servers in the pilot. compact: icon, name, summary and tool count, for /mcp.
 * Otherwise the full card with example, command and setup, for /mcp/fleet-gateway.
 * tools maps a registry name to its tool count; a server the registry did not return has none.
 */
export function PilotServerCards({ compact, tools = {} }: { compact?: boolean; tools?: Record<string, number> }) {
  return (
    <div
      className={compact ? "mcp-server-cards" : "fleet-gateway-servers"}
      role="region"
      aria-label="Servere i piloten"
      tabIndex={compact ? undefined : 0}
    >
      {PILOT_SERVERS.map((server) => {
        const Icon = server.icon;
        const name = server.id.slice(server.id.lastIndexOf("/") + 1);
        const count = tools[server.id];
        return (
          <Box
            as="article"
            key={server.id}
            borderWidth="1"
            borderColor="neutral-subtle"
            borderRadius="12"
            padding="space-16"
          >
            <VStack gap="space-12" className="fleet-gateway-card">
              <HStack gap="space-8" align="center" wrap={false}>
                <Icon aria-hidden fontSize="1.75rem" />
                <Heading size="small" level="3">
                  {server.title}
                </Heading>
              </HStack>
              <BodyShort>{server.summary}</BodyShort>
              <HStack gap="space-4" wrap>
                {compact ? (
                  <Tag variant={server.readOnly ? "info" : "warning"} size="small">
                    {server.readOnly ? "Leser" : "Skriver"}
                  </Tag>
                ) : (
                  server.readOnly && (
                    <Tag variant="neutral" size="small">
                      Bare lesing
                    </Tag>
                  )
                )}
                {count !== undefined && (
                  <Tag variant="info" size="small">
                    {count === 1 ? "1 verktøy" : `${count} verktøy`}
                  </Tag>
                )}
              </HStack>
              {!compact && (
                <>
                  <Box
                    as="blockquote"
                    className="fleet-gateway-quote"
                    borderWidth="0 0 0 4"
                    borderColor="neutral-subtle"
                    paddingInline="space-12"
                  >
                    <BodyShort size="small">«{server.example}»</BodyShort>
                  </Box>
                  <CommandBlock command={enableCommand(name)} />
                  <ReadMore header={`Oppsett for ${server.title}`} size="small">
                    <Bullets>
                      <li>
                        Registernavn: <code className={code}>{server.id}</code>
                      </li>
                      <li>
                        MCP-serveren i{" "}
                        <a
                          href={`https://github.com/navikt/copilot/tree/main/${server.codeLabel}`}
                          className={linkClass}
                        >
                          {server.codeLabel}
                        </a>
                      </li>
                      <li>
                        Ruten <code className={code}>{server.route}</code> i{" "}
                        <a href={PROD_ROUTES_URL} className={linkClass}>
                          apps/fleet-gateway/pilot-routes.prod.yaml
                        </a>
                      </li>
                      <li>
                        Oppføringen i MCP-registeret i{" "}
                        <a href={ALLOWLIST_URL} className={linkClass}>
                          apps/mcp-registry/allowlist.json
                        </a>
                      </li>
                      {"inboundPrUrl" in server && (
                        <li>
                          Eksempel på PR til eierteamet for å be om tilgang i{" "}
                          <code className={code}>accessPolicy.inbound</code>:{" "}
                          <a href={server.inboundPrUrl} className={linkClass}>
                            {server.inboundPrLabel}
                          </a>
                        </li>
                      )}
                    </Bullets>
                  </ReadMore>
                </>
              )}
            </VStack>
          </Box>
        );
      })}
    </div>
  );
}
