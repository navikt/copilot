import type { ReactNode } from "react";
import { BodyLong, Box, Heading, HStack, Stack, VStack } from "@navikt/ds-react";
import { Breadcrumbs, FleetNav, type FleetNavProps } from "./nav";

export const fleetRobots = { index: false, follow: false };

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
  children,
}: {
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
            <Box as="aside" className="w-full md:w-56 shrink-0">
              <FleetNav {...nav} />
            </Box>
            <VStack gap="space-32" className="min-w-0 flex-1 break-words">
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
                  {action && <div className="shrink-0">{action}</div>}
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
