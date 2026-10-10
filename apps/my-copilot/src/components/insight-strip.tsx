import { Suspense } from "react";
import NextLink from "next/link";
import { Box, BodyShort, Heading, Skeleton } from "@navikt/ds-react";
import { ArrowRightIcon, LightBulbIcon } from "@navikt/aksel-icons";
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
      <Heading as="p" size="medium">
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
          className="group flex flex-1 items-start gap-2 text-inherit no-underline hover:underline"
        >
          <LightBulbIcon
            aria-hidden
            fontSize="1.25rem"
            className="shrink-0"
            style={{ marginTop: "var(--ax-space-2)" }}
          />
          <BodyShort>
            <span className="font-semibold">Dagens innsikt: </span>
            {fact.text}
            <ArrowRightIcon
              aria-hidden
              fontSize="1rem"
              style={{ marginLeft: "var(--ax-space-4)" }}
              className="inline align-[-0.15em] transition-transform group-hover:translate-x-0.5"
            />
          </BodyShort>
        </NextLink>
      )}
      <div className="flex gap-8">
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
    <Box background="neutral-soft" borderRadius="8" padding={{ xs: "space-12", md: "space-16" }}>
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:gap-8">
        <Suspense fallback={<Skeleton variant="text" width="100%" />}>
          <Strip />
        </Suspense>
      </div>
    </Box>
  );
}
