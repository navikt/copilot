import type { ReactNode } from "react";
import { cookies } from "next/headers";
import { TEAM_COOKIE } from "@/lib/fleet-names";
import { BodyLong, Box, Heading, HStack, Stack, VStack } from "@navikt/ds-react";
import { Breadcrumbs, FleetNav, type FleetNavProps } from "./nav";

export const fleetRobots = { index: false, follow: false };

/** The side navigation for a page outside one team's overview: the remembered team, else the first. */
export async function teamNav(teams: string[], prefer?: string): Promise<FleetNavProps | undefined> {
  const last = (await cookies()).get(TEAM_COOKIE)?.value;
  const team = [prefer, last].find((t) => t && teams.includes(t)) ?? teams[0];
  return team ? { team, teams } : undefined;
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
