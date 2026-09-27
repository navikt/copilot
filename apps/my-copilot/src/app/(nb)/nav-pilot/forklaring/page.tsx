import { BodyLong } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { DocPage, PageLinks, linkClass } from "@/components/nav-pilot/doc-page";

export const metadata: Metadata = {
  title: "Forklaring — nav-pilot",
  description:
    "Hvorfor nav-pilot virker som det gjør: planleggingen, sandkassen, den lokale modellen, personvern og arkitektur.",
};

const PAGES = [
  {
    href: "/nav-pilot/forklaring/planlegging",
    title: "Planlegging",
    desc: "De fire fasene, skillene som driver dem, og hvorfor du skriver kjernelogikken selv.",
  },
  {
    href: "/nav-pilot/forklaring/sandkassen",
    title: "Sandkassen",
    desc: "Hvorfor agenten må kjøre isolert på Nav-utstyr, og hva sikkerhetsnivåene i cplt gjør.",
  },
  {
    href: "/nav-pilot/forklaring/lokal-modell",
    title: "Lokal modell",
    desc: "Hvorfor utsendingen er begrenset, og hva modellene klarer i målingene våre.",
  },
  {
    href: "/nav-pilot/forklaring/personvern",
    title: "Personvern og telemetri",
    desc: "Hva nav-pilot måler, hva som aldri er med, og hvordan du slår det av.",
  },
  {
    href: "/nav-pilot/forklaring/arkitektur",
    title: "Arkitektur",
    desc: "Hvorfor nav-pilot finnes, hva den vet som Copilot ikke vet, og prinsippene den er bygget på.",
  },
];

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
      <PageLinks pages={PAGES} />
    </DocPage>
  );
}
