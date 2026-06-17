import { Box, VStack, Heading, BodyLong } from "@navikt/ds-react";
import { VariantShowcase } from "@/components/nav-pilot/picker-variants";

export const metadata = {
  title: "nav-pilot — picker prototyper",
  robots: { index: false, follow: false },
};

export default function PickerPrototypePage() {
  return (
    <Box
      as="main"
      paddingBlock={{ xs: "space-24", md: "space-48" }}
      paddingInline={{ xs: "space-16", md: "space-40" }}
      style={{ maxWidth: 900, margin: "0 auto" }}
    >
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
  );
}
