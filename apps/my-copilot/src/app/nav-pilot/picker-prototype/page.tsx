import { Box, Page, VStack, Heading, BodyLong } from "@navikt/ds-react";
import { VariantShowcase } from "@/components/nav-pilot/picker-variants";

export const metadata = {
  title: "nav-pilot — picker prototyper",
  robots: { index: false, follow: false },
};

export default function PickerPrototypePage() {
  return (
    <Page>
      <Page.Block as="main" width="lg" gutters>
        <Box paddingBlock={{ xs: "space-24", md: "space-48" }}>
          <VStack gap="space-32">
            <VStack gap="space-8">
              <Heading size="large" level="1">
                Velg-din-klient — stilprototyper
              </Heading>
              <BodyLong textColor="subtle">
                Fire visuelle retninger for å vise at både Copilot og OpenCode er støttet av Nav og nav-pilot. Klikk deg
                gjennom hver variant og se hvilken som skiller seg ut uten å bli prangende.
              </BodyLong>
            </VStack>
            <VariantShowcase />
          </VStack>
        </Box>
      </Page.Block>
    </Page>
  );
}
