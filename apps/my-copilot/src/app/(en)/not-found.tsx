import { Heading, BodyShort, Link, VStack, Box } from "@navikt/ds-react";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Page not found",
};

export default function NotFound() {
  return (
    <main className="flex flex-col items-center justify-center min-h-[60vh]">
      <Box paddingBlock="space-24" paddingInline="space-16">
        <VStack gap="space-16" align="center">
          <VStack gap="space-8" align="center">
            <Heading size="xlarge" level="1">
              Page not found
            </Heading>
            <BodyShort>The page you are looking for has moved, been deleted, or never existed.</BodyShort>
          </VStack>
          <Link href="/en/news">Go to the English articles</Link>
          <BodyShort size="small">
            <span lang="nb">
              <Link href="/">Til forsiden</Link>
            </span>
          </BodyShort>
        </VStack>
      </Box>
    </main>
  );
}
