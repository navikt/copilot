import { BodyLong } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { DocPage, PageLinks, linkClass } from "@/components/nav-pilot/doc-page";
import { GUIDE_PAGES } from "@/components/nav-pilot/doc-pages";

export const metadata: Metadata = {
  title: "Guider — nav-pilot",
  description:
    "Korte oppskrifter for én oppgave om gangen: installere, tilpasse, synkronisere, lokal modell og feilsøking.",
};

export default function Guider() {
  return (
    <DocPage title="Guider" description="Hver guide tar én oppgave og forutsetter at nav-pilot er installert.">
      <BodyLong>
        Er du ny, start med{" "}
        <NextLink href="/kom-i-gang" className={linkClass}>
          Kom i gang
        </NextLink>
        . Kommandoer og konfignøkler står i{" "}
        <NextLink href="/nav-pilot/referanse" className={linkClass}>
          referansen
        </NextLink>
        .
      </BodyLong>
      <PageLinks pages={GUIDE_PAGES} />
    </DocPage>
  );
}
