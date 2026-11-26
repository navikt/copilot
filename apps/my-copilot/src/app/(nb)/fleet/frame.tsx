import type { ReactNode } from "react";
import { cookies } from "next/headers";
import { TEAM_COOKIE } from "@/lib/fleet-names";
import { fleetApi } from "@/lib/fleet-control";
import { BodyLong, Box, Heading, HStack, Stack, VStack } from "@navikt/ds-react";
import { Breadcrumbs, FleetNav, type FleetNavProps } from "./nav";

export const fleetRobots = { index: false, follow: false };

/** The badge call may take this long; after that the page renders without the count. */
export const BADGE_TIMEOUT_MS = 2000;

/** Versions waiting in the team, as on the overview; undefined on error or after BADGE_TIMEOUT_MS. */
function pendingBadge(team: string): Promise<number | undefined> {
  return Promise.race([
    fleetApi.servers(team).then(
      (l) => l.items.filter((s) => s.status.pending).length,
      () => undefined
    ),
    new Promise<undefined>((r) => setTimeout(() => r(undefined), BADGE_TIMEOUT_MS)),
  ]);
}

/**
 * The side navigation for a page outside one team's overview: the remembered team, else the first.
 * Call it before the page's own fetches: the badge call for the likely team starts at once and runs alongside them.
 */
export function teamNavLoader(prefer?: string) {
  const start = cookies().then((c) => {
    const last = c.get(TEAM_COOKIE)?.value;
    // Both likely teams: the page's own (for example the MCP server's) and the remembered one.
    const badges = new Map([prefer, last].filter((t): t is string => !!t).map((t) => [t, pendingBadge(t)]));
    return { last, badges };
  });
  /** pick: the team the page itself settled on, when it has one. */
  return async (
    teams: string[],
    extra: Partial<FleetNavProps> = {},
    pick?: string
  ): Promise<FleetNavProps | undefined> => {
    const { last, badges } = await start;
    const team = [pick, prefer, last].find((t) => t && teams.includes(t)) ?? teams[0];
    if (!team) return undefined;
    const pending = await (badges.get(team) ?? pendingBadge(team));
    return { team, teams, pending, ...extra };
  };
}

import type { Crumb } from "./nav";
export type { Crumb };

/** /fleet layout from the sketches: side navigation, breadcrumb, H1 and lead on the light page, action to the right. */
export function FleetFrame({
  title,
  description,
  nav = {},
  crumbs = [],
  action,
  titleTag,
  bare = false,
  children,
}: {
  /** No side navigation, one centred column (the team picker). */
  bare?: boolean;
  title: string;
  description: ReactNode;
  nav?: FleetNavProps;
  /** Before the current page; «Fleet» comes first on its own. */
  crumbs?: Crumb[] | false;
  action?: ReactNode;
  /** A status tag next to the H1. */
  titleTag?: ReactNode;
  children: ReactNode;
}) {
  const trail: Crumb[] = [{ label: "Fleet", href: "/fleet" }, ...(crumbs || [])];
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-32" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <Stack
            direction={{ xs: "column", md: "row" }}
            gap={{ xs: "space-24", md: "space-40" }}
            align={{ xs: "stretch", md: "start" }}
            wrap={false}
          >
            {!bare && (
              <Box as="aside" className="w-full md:w-56 shrink-0">
                <FleetNav {...nav} />
              </Box>
            )}
            <VStack gap="space-32" className={`min-w-0 flex-1 break-words ${bare ? "max-w-2xl mx-auto" : ""}`}>
              <VStack gap="space-12">
                {crumbs !== false && title !== "Fleet" && <Breadcrumbs trail={trail} current={title} />}
                {/* The action sits on the H1 row and wraps below it on a narrow screen. */}
                <HStack gap="space-16" justify="space-between" align="start">
                  <VStack gap="space-8" className="min-w-0 flex-1 basis-80">
                    <HStack gap="space-12" align="center">
                      <Heading level="1" size="xlarge" className="[overflow-wrap:anywhere]">
                        {title}
                      </Heading>
                      {titleTag}
                    </HStack>
                    {typeof description === "string" ? <BodyLong>{description}</BodyLong> : description}
                  </VStack>
                  {action && <div className="w-full shrink-0 sm:w-auto">{action}</div>}
                </HStack>
              </VStack>
              {children}
            </VStack>
          </Stack>
        </Box>
      </div>
    </main>
  );
}
