import { Heading, BodyShort, Link, VStack, Box } from "@navikt/ds-react";
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Siden finnes ikke",
};

// Next renders a global not-found inside a built-in layout, not inside either
// route group's layout, so this page carries no site chrome. It exists because
// the alternative for an unmatched URL is Next's own unstyled English page. A
// notFound() call inside (nb) or (en) still renders that group's not-found.
export default function NotFound() {
  return (
    <html lang="nb">
      <body>
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
                  Page not found. <Link href="/en/news">English articles</Link>
                </span>
              </BodyShort>
            </VStack>
          </Box>
        </main>
      </body>
    </html>
  );
}
