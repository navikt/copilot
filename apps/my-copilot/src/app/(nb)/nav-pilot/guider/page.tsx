import { BodyLong } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { DocPage, PageLinks, linkClass } from "@/components/nav-pilot/doc-page";

export const metadata: Metadata = {
  title: "Guider — nav-pilot",
  description:
    "Korte oppskrifter for én oppgave om gangen: installere, tilpasse, synkronisere, lokal modell og feilsøking.",
};

const PAGES = [
  {
    href: "/nav-pilot/guider/installere-og-oppgradere",
    title: "Installere og oppgradere",
    desc: "Velg hvor agentpakka skal ligge, installer i CI, oppgrader og avinstaller.",
  },
  {
    href: "/nav-pilot/guider/tilpasse",
    title: "Tilpasse",
    desc: "Endre innstillinger, legg til teamets egne instruksjoner, overstyr og ignorer komponenter, og slå hooks av og på.",
  },
  {
    href: "/nav-pilot/guider/synkronisere",
    title: "Synkronisere",
    desc: "Hold agentpakka oppdatert med en ukentlig pull request eller med nav-pilot sync.",
  },
  {
    href: "/nav-pilot/guider/lokal",
    title: "Lokal modell",
    desc: "Styr utsendingen, bytt modell, bruk egen server og bruk alpha decide i hooks og skript.",
  },
  {
    href: "/nav-pilot/guider/feilsoking",
    title: "Feilsøking",
    desc: "Sjekk maskinen med doctor, se hva cplt blokkerer, og få en lokal modell som henger, i gang igjen.",
  },
];

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
      <PageLinks pages={PAGES} />
    </DocPage>
  );
}
