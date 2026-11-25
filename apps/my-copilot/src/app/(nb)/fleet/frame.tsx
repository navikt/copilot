import type { ReactNode } from "react";
import { Box, HStack, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { FleetNav, type FleetNavProps } from "./nav";

export const fleetRobots = { index: false, follow: false };

export function FleetFrame({
  title,
  description,
  nav = {},
  children,
}: {
  title: string;
  description: string;
  nav?: FleetNavProps;
  children: ReactNode;
}) {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title={title} description={description} />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <HStack gap="space-40" align="start">
            <aside className="w-56 shrink-0">
              <FleetNav {...nav} />
            </aside>
            <VStack gap="space-24" className="min-w-0 flex-1 basis-80">
              {children}
            </VStack>
          </HStack>
        </Box>
      </div>
    </main>
  );
}
