import type { ReactNode } from "react";
import { Box, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";

export const fleetRobots = { index: false, follow: false };

export function FleetFrame({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
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
          <VStack gap="space-24">{children}</VStack>
        </Box>
      </div>
    </main>
  );
}
