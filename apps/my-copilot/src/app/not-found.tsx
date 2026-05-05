import { Heading, BodyShort, Link, VStack, Box } from "@navikt/ds-react";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Siden finnes ikke",
};

export default function NotFound() {
  return (
    <main className="flex flex-col items-center justify-center min-h-[60vh]">
      <Box paddingBlock="space-24" paddingInline="space-16">
        <VStack gap="space-16" align="center">
          <VStack gap="space-8" align="center">
            <Heading size="xlarge" level="1">
              Siden finnes ikke
            </Heading>
            <BodyShort>Siden du leter etter er flyttet, slettet, eller har aldri eksistert.</BodyShort>
          </VStack>
          <Link href="/">Gå til forsiden</Link>
          <BodyShort size="small">
            <span lang="en">
              Page not found — <Link href="/">Go to front page</Link>
            </span>
          </BodyShort>
        </VStack>
      </Box>
    </main>
  );
}
