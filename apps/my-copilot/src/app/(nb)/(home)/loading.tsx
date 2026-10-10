import { BodyShort, Box, Heading, Skeleton, VStack } from "@navikt/ds-react";

// This skeleton is the front page's own, so it lives in the (home) group and
// its Suspense boundary covers only the front page. Above a page, a boundary
// means the shell streams with status 200 before notFound() can throw, so an
// unknown URL answered 200 with the not-found page drawn client side.
//
// It copies the hero of page.tsx (same section, padding and heading) and holds
// a full screen below it. A shorter skeleton let the footer show first and then
// jump away when the page arrived (CLS 0.13 at 390 px).
export default function Loading() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <section className="hero-gradient text-white overflow-hidden max-md:min-h-[19rem]">
        <Box
          paddingBlock={{ xs: "space-32", md: "space-40" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto relative"
        >
          <VStack gap="space-8">
            <Heading size="xlarge" level="1" className="hero-title">
              KI-utvikling i Nav
            </Heading>
            <BodyShort className="max-w-md opacity-70">
              Nyheter, beste praksis og verktøy for KI-drevet utvikling i Nav.
            </BodyShort>
            <div className="mt-2 min-h-14 md:min-h-9" />
          </VStack>
        </Box>
      </section>
      <section className="max-w-7xl mx-auto min-h-dvh">
        <Box
          paddingBlock={{ xs: "space-16", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <Skeleton variant="rectangle" height={160} />
        </Box>
      </section>
    </main>
  );
}
