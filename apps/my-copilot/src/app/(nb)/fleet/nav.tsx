"use client";

import { useEffect } from "react";
import NextLink from "next/link";
import { usePathname } from "next/navigation";
import { TEAM_COOKIE } from "@/lib/fleet-names";
import { Button, Detail, HStack, Label, Link, Select, VStack } from "@navikt/ds-react";

export type FleetNavProps = {
  /** The team the overview shows. Remembered in a cookie for the next visit. */
  team?: string;
  teams?: string[];
  pending?: number;
  hasAgents?: boolean;
};

function NavLink({ href, children }: { href: string; children: React.ReactNode }) {
  const current = usePathname() === href;
  return (
    <Link as={NextLink} href={href} aria-current={current ? "page" : undefined} underline={false}>
      {current ? <strong>{children}</strong> : children}
    </Link>
  );
}

export function FleetNav({ team, teams = [], pending = 0, hasAgents = false }: FleetNavProps) {
  useEffect(() => {
    if (team) document.cookie = `${TEAM_COOKIE}=${team}; path=/; max-age=31536000; samesite=lax`;
  }, [team]);
  const base = team ? `/fleet?team=${team}` : "/fleet";
  return (
    <nav aria-label="Fleet">
      <VStack gap="space-24">
        {team && teams.length > 1 && (
          <form method="get" action="/fleet">
            <VStack gap="space-4">
              <HStack gap="space-8" align="end" wrap={false}>
                <Select label="Team" name="team" defaultValue={team} size="small">
                  {teams.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </Select>
                <Button size="small" variant="secondary">
                  Bytt
                </Button>
              </HStack>
              <Detail>Vi husker teamet til neste gang.</Detail>
            </VStack>
          </form>
        )}
        {team && (
          <VStack gap="space-8" as="ul">
            <li>
              <NavLink href="/fleet">Oversikt</NavLink>
            </li>
            <li>
              <Link as={NextLink} href={`${base}#mcp-servere`} underline={false}>
                MCP-servere
              </Link>
            </li>
            {hasAgents && (
              <li>
                <Link as={NextLink} href={`${base}#agentprofiler`} underline={false}>
                  Agentprofiler
                </Link>
              </li>
            )}
            {pending > 0 && (
              <li>
                <Link as={NextLink} href={`${base}#til-godkjenning`} underline={false}>
                  Til godkjenning ({pending})
                </Link>
              </li>
            )}
          </VStack>
        )}
        <VStack gap="space-8">
          <Label as="p" size="small">
            For alle team
          </Label>
          <VStack gap="space-8" as="ul">
            {!team && (
              <li>
                <NavLink href="/fleet">Oversikt</NavLink>
              </li>
            )}
            <li>
              <NavLink href="/fleet/katalog">Katalog</NavLink>
            </li>
            <li>
              <NavLink href="/fleet/registrer">Registrer MCP-server</NavLink>
            </li>
          </VStack>
        </VStack>
      </VStack>
    </nav>
  );
}
