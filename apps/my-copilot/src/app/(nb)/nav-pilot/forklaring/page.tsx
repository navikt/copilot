import { BodyLong } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { DocPage, PageLinks, linkClass } from "@/components/nav-pilot/doc-page";
import { EXPLANATION_PAGES } from "@/components/nav-pilot/doc-pages";

export const metadata: Metadata = {
  title: "Forklaring — nav-pilot",
  description:
    "Hvorfor nav-pilot virker som det gjør: planleggingen, sandkassen, den lokale modellen, personvern og arkitektur.",
};

export default function Forklaring() {
  return (
    <DocPage
      title="Forklaring"
      description="Bakgrunnen for valgene i nav-pilot. Les dette når du vil forstå, ikke når du skal få noe gjort."
    >
      <BodyLong>
        Oppskriftene står i{" "}
        <NextLink href="/nav-pilot/guider" className={linkClass}>
          guidene
        </NextLink>
        , og tabellene i{" "}
        <NextLink href="/nav-pilot/referanse" className={linkClass}>
          referansen
        </NextLink>
        .
      </BodyLong>
      <PageLinks pages={EXPLANATION_PAGES} />
    </DocPage>
  );
}
