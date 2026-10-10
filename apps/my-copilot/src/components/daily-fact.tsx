import NextLink from "next/link";
import { BodyShort } from "@navikt/ds-react";
import { ArrowRightIcon, LightBulbIcon } from "@navikt/aksel-icons";
import { getDailyFact } from "@/lib/daily-fact";

// One Nav-wide fact a day, shown to everyone. The /innsikt link may ask for login.
export async function DailyFact() {
  const fact = await getDailyFact();
  if (!fact) return null;
  return (
    <NextLink
      href={fact.href}
      prefetch={false}
      className="group inline-flex max-w-xl items-start gap-2 rounded-lg bg-white/10 px-3 py-2 text-white no-underline backdrop-blur-sm hover:bg-white/15 focus-visible:outline-2 focus-visible:outline-white"
    >
      <LightBulbIcon aria-hidden fontSize="1.25rem" className="mt-0.5 shrink-0" />
      <BodyShort size="small">
        <span className="font-semibold">Dagens innsikt: </span>
        {fact.text}
        <ArrowRightIcon
          aria-hidden
          fontSize="1rem"
          className="ml-1 inline align-[-0.15em] transition-transform group-hover:translate-x-0.5"
        />
      </BodyShort>
    </NextLink>
  );
}
