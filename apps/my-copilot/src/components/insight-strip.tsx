import { Suspense } from "react";
import NextLink from "next/link";
import { Box, BodyShort, Heading, HStack, Skeleton, VStack } from "@navikt/ds-react";
import { LightBulbIcon } from "@navikt/aksel-icons";
import { getDailyFact } from "@/lib/daily-fact";
import { getAllCustomizations } from "@/lib/customizations";
import { formatNumber } from "@/lib/format";

// The front page's insight strip, shown to everyone. Every number is public:
// the API rounds people to the nearest 50 and hides groups under 20
// (copilot-intern underlag/offentlige-tall.md). The /innsikt links may ask for login.

// A stat as text, not a heading: the numbers sit before the page's H2s (#1193).
function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div className="min-w-0">
      <Heading as="p" size="medium" className="tabular-nums">
        {value}
      </Heading>
      <BodyShort size="small" textColor="subtle">
        {label}
      </BodyShort>
    </div>
  );
}

async function Strip() {
  const fact = await getDailyFact();
  const customizations = getAllCustomizations().length;
  return (
    <>
      {fact && (
        <NextLink
          href={fact.href}
          prefetch={false}
          className="flex-1 min-w-0 text-inherit no-underline hover:underline"
        >
          <VStack gap="space-4">
            <HStack gap="space-8" align="center">
              <LightBulbIcon aria-hidden fontSize="1rem" className="text-text-subtle shrink-0" />
              <BodyShort size="small" weight="semibold" className="uppercase tracking-wide text-text-subtle">
                Dagens innsikt
              </BodyShort>
            </HStack>
            <BodyShort size="small" textColor="subtle">
              {fact.text} →
            </BodyShort>
          </VStack>
        </NextLink>
      )}
      <div className="flex gap-8 shrink-0">
        {fact?.weekUsers && <Stat value={`ca. ${formatNumber(fact.weekUsers)}`} label="brukere siste uke" />}
        <NextLink href="/innsikt/tilpasninger" prefetch={false} className="text-inherit underline hover:no-underline">
          <Stat value={formatNumber(customizations)} label="tilpasninger →" />
        </NextLink>
      </div>
    </>
  );
}

export function InsightStrip() {
  return (
    <Box
      background="default"
      borderColor="neutral-subtleA"
      borderWidth="1"
      borderRadius="8"
      padding={{ xs: "space-12", md: "space-16" }}
    >
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between md:gap-8">
        <Suspense fallback={<Skeleton variant="text" width="100%" />}>
          <Strip />
        </Suspense>
      </div>
    </Box>
  );
}
