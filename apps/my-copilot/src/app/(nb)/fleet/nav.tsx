"use client";

import { useEffect, useRef, type ReactNode } from "react";
import NextLink from "next/link";
import { usePathname } from "next/navigation";
import { TEAM_COOKIE } from "@/lib/fleet-names";
import { BodyShort, Box, Button, Detail, Hide, HStack, Link, Select, Show, Tag, VStack } from "@navikt/ds-react";

/** A Button that is a link, for server components (they cannot pass NextLink themselves). */
export const LinkButton = ({ href, children }: { href: string; children: ReactNode }) => (
  <Button as={NextLink} href={href} className="w-full sm:w-auto">
    {children}
  </Button>
);

export const LinkText = ({ href, children }: { href: string; children: ReactNode }) => (
  <Link as={NextLink} href={href}>
    {children}
  </Link>
);

export type Crumb = { label: string; href?: string };

/** Client side, since a server component cannot pass NextLink to Link. */
export function Breadcrumbs({ trail, current }: { trail: Crumb[]; current: string }) {
  return (
    <nav aria-label="Brødsmuler">
      <HStack as="ol" gap="space-8" align="center">
        {trail.map((c) => (
          <HStack as="li" key={c.label} gap="space-8" align="center">
            {c.href ? (
              <Link as={NextLink} href={c.href}>
                {c.label}
              </Link>
            ) : (
              c.label
            )}
            <span aria-hidden>›</span>
          </HStack>
        ))}
        <li aria-current="page" className="[overflow-wrap:anywhere]">
          {current}
        </li>
      </HStack>
    </nav>
  );
}

export type FleetNavProps = {
  /** The team the overview shows. Remembered in a cookie for the next visit. */
  team?: string;
  teams?: string[];
  pending?: number;
  /** Another team's MCP server is a catalogue page: «Katalog» is the current section. */
  inCatalog?: boolean;
};

function NavItem({ href, current, children }: { href: string; current?: boolean; children: ReactNode }) {
  return (
    <li>
      <Box
        as={NextLink}
        href={href}
        aria-current={current ? "page" : undefined}
        paddingBlock="space-8"
        paddingInline="space-16"
        borderRadius="4"
        background={current ? "accent-soft" : undefined}
        className={`block no-underline hover:underline ${current ? "font-semibold" : ""}`}
        style={{
          color: "var(--ax-text-neutral)",
          borderLeft: `4px solid ${current ? "var(--ax-border-accent)" : "transparent"}`,
        }}
      >
        {children}
      </Box>
    </li>
  );
}

export function FleetNav({ team, teams = [], pending = 0, inCatalog = false }: FleetNavProps) {
  const path = usePathname();
  const form = useRef<HTMLFormElement>(null);
  useEffect(() => {
    if (team) document.cookie = `${TEAM_COOKIE}=${team}; path=/; max-age=31536000; samesite=lax; secure`;
  }, [team]);
  const base = team ? `/fleet?team=${team}` : "/fleet";
  const links: { label: string; href: string; current?: boolean; count?: number; team?: boolean }[] = team
    ? [
        { label: "Oversikt", href: base, current: path === "/fleet", team: true },
        {
          label: "MCP-servere",
          href: `${base}#mcp-servere`,
          current: !inCatalog && path.startsWith("/fleet/mcp-servere"),
          team: true,
        },
        {
          label: "Agentprofiler",
          href: `/fleet/agenter?team=${team}`,
          current: path.startsWith("/fleet/agenter"),
          team: true,
        },
        {
          label: "Til godkjenning",
          href: `/fleet/godkjenning?team=${team}`,
          current: path.startsWith("/fleet/godkjenning"),
          count: pending,
          team: true,
        },
        { label: "Katalog", href: "/fleet/katalog", current: inCatalog || path === "/fleet/katalog" },
      ]
    : [
        { label: "Oversikt", href: "/fleet", current: path === "/fleet" },
        { label: "Katalog", href: "/fleet/katalog", current: inCatalog || path === "/fleet/katalog" },
      ];
  return (
    <nav aria-label="Fleet">
      <VStack gap="space-16">
        {team && teams.length > 1 && (
          <form method="get" action="/fleet" ref={form}>
            <VStack gap="space-4">
              <Select
                label="Team"
                hideLabel
                name="team"
                defaultValue={team}
                onChange={() => form.current?.requestSubmit()}
              >
                {teams.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </Select>
              <noscript>
                <Button variant="secondary" size="small">
                  Bytt
                </Button>
              </noscript>
              <Detail>Sist brukte team huskes</Detail>
            </VStack>
          </form>
        )}
        {team && teams.length === 1 && <BodyShort weight="semibold">{team}</BodyShort>}
        {/* On a phone: a short link list under the team picker instead of the side menu. */}
        <Show below="md">
          <HStack as="ul" role="list" gap="space-8 space-16" wrap aria-label="Fleet-sider">
            {links.map((l) => (
              <li key={l.label}>
                <Link as={NextLink} href={l.href} aria-current={l.current ? "page" : undefined}>
                  {l.current ? <strong>{l.label}</strong> : l.label}
                  {l.count ? ` (${l.count})` : ""}
                </Link>
              </li>
            ))}
          </HStack>
        </Show>
        <Hide below="md">
          <VStack gap="space-16">
            {team && (
              <VStack as="ul" gap="space-4">
                {links
                  .filter((l) => l.team)
                  .map((l) => (
                    <NavItem key={l.label} href={l.href} current={l.current}>
                      {l.count ? (
                        <HStack gap="space-8" align="center" justify="space-between">
                          {l.label}
                          <Tag size="xsmall" variant="warning" aria-label={`${l.count} venter`}>
                            {l.count}
                          </Tag>
                        </HStack>
                      ) : (
                        l.label
                      )}
                    </NavItem>
                  ))}
              </VStack>
            )}
            <Box
              borderWidth={team ? "1 0 0 0" : "0"}
              borderColor="neutral-subtle"
              paddingBlock={team ? "space-12 space-0" : "space-0"}
            >
              <VStack gap="space-4">
                <Detail as="p">For alle team</Detail>
                <VStack as="ul" gap="space-4">
                  {links
                    .filter((l) => !l.team)
                    .map((l) => (
                      <NavItem key={l.label} href={l.href} current={l.current}>
                        {l.label}
                      </NavItem>
                    ))}
                </VStack>
              </VStack>
            </Box>
            <Button
              as={NextLink}
              href={team ? `/fleet/registrer?team=${team}` : "/fleet/registrer"}
              variant="secondary"
            >
              Registrer MCP-server
            </Button>
          </VStack>
        </Hide>
      </VStack>
    </nav>
  );
}
