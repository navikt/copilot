"use client";

import { VStack, BodyShort, HStack } from "@navikt/ds-react";
import { LightBulbIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";

interface Tip {
  text: string;
  href: string;
  label: string;
}

const TIPS: Tip[] = [
  {
    text: "Bruk norske domenebegreper, slik saksbehandlerne og regelverket bruker dem. Fest valget i AGENTS.md, så bruker agenten de samme ordene i koden.",
    href: "/praksis/guide/forberede-prosjektet",
    label: "Domenebegreper i AGENTS.md",
  },
  {
    text: "Tenk i to akser før du gir agenten fritt spillerom: hvor godt kjenner du koden, og hvor mye skade kan en feil gjøre? Lite kjent kode eller høy risiko betyr små steg og nøye gjennomgang.",
    href: "/praksis/guide/styrker-og-farer",
    label: "Læring og risiko",
  },
  {
    text: "Bruk Copilot CLI som hovedverktøy. I terminalen ser du hva agenten kjører, og du kan styre den i samme vindu som git og testene.",
    href: "/praksis/guide/velge-riktig-verktoy",
    label: "Copilot CLI",
  },
  {
    text: "Vær spesifikk. «Fiks auth-feilen» gir dårlige resultater. Beskriv heller symptom, fil og forventet oppførsel.",
    href: "/praksis/guide/skrive-presise-prompts",
    label: "Presise prompts",
  },
  {
    text: "Bryt ned oppgaver i små, uavhengige deler. «Lag login-skjema med validering» går bedre enn «bygg komplett auth-system».",
    href: "/praksis/guide/skrive-presise-prompts",
    label: "Små oppgaver",
  },
  {
    text: "Du eier arkitekturen, agenten implementerer. Ikke la den ta designbeslutninger. Gi den klare rammer i AGENTS.md.",
    href: "/praksis/guide/orkestrere-agenter",
    label: "Agentmønstre",
  },
  {
    text: "Pass på at agenten ikke gjør mer enn du ba om. Den refaktorerer gjerne kode utenfor oppgaven. Sett klare grenser i oppgavebeskrivelsen.",
    href: "/praksis/guide/styrker-og-farer",
    label: "Hold oppgaven avgrenset",
  },
  {
    text: "Kontekst betyr mer enn modellvalg. Gode instruksjoner i repoet gir bedre resultater enn å bytte til en dyrere modell.",
    href: "/praksis/guide/forberede-prosjektet",
    label: "Kontekst før modell",
  },
  {
    text: "KI kan dikte opp API-er og biblioteker som ikke finnes. Sjekk alltid at pakkene og funksjonene du importerer, faktisk finnes.",
    href: "/praksis/guide/styrker-og-farer",
    label: "Oppdiktede API-er",
  },
  {
    text: "Start en ny samtale når du bytter oppgave. Lange økter fyller konteksten, og agenten husker dårligere.",
    href: "/praksis/guide/styrker-og-farer",
    label: "Konteksthåndtering",
  },
];

function getWeekOfYear(): number {
  const now = new Date();
  const start = new Date(now.getFullYear(), 0, 1);
  const diff = now.getTime() - start.getTime();
  return Math.floor(diff / (7 * 24 * 60 * 60 * 1000));
}

export function WeeklyTip() {
  const week = getWeekOfYear();
  const tip = TIPS[week % TIPS.length];

  return (
    <VStack gap="space-8">
      <HStack gap="space-4" align="center">
        <LightBulbIcon aria-hidden fontSize="1rem" className="text-text-subtle" />
        <BodyShort size="small" weight="semibold" className="uppercase tracking-wide text-text-subtle">
          Tips denne uken
        </BodyShort>
      </HStack>
      <BodyShort size="small">{tip.text}</BodyShort>
      <NextLink href={tip.href} className="text-sm no-underline hover:underline">
        {tip.label} →
      </NextLink>
    </VStack>
  );
}
