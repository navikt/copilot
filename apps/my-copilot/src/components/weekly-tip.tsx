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
    text: "Bruk WRAP-metoden: Write → Refine → Atomic → Pair. Tenk på det som å onboarde en ny kollega.",
    href: "/praksis#wrap-metoden-for-coding-agent",
    label: "WRAP-metoden",
  },
  {
    text: "Vær spesifikk i prompts. «Fix the auth bug» gir dårlige resultater — beskriv heller symptom, fil og forventet oppførsel.",
    href: "/praksis#prompt-engineering",
    label: "Prompt engineering",
  },
  {
    text: "Bryt ned oppgaver i små, uavhengige deler. Copilot håndterer «lag login-skjema med validering» bedre enn «bygg komplett auth-system».",
    href: "/praksis#wrap-metoden-for-coding-agent",
    label: "Atomiske oppgaver",
  },
  {
    text: "Gjennomgå alltid session logs i Copilot-PR-er. De avslører om agenten forsto oppgaven, sporet av, eller ga opp.",
    href: "/praksis#gjennomgå-copilots-arbeid",
    label: "Code review",
  },
  {
    text: "Copilot er best på repetitivt arbeid i stor skala — refaktorering, fjerne feature flags, fikse skrivefeil på tvers av mange filer.",
    href: "/praksis#styrker-begrensninger-og-farer",
    label: "Styrker og begrensninger",
  },
  {
    text: "Du eier arkitekturen, Copilot implementerer. Ikke la agenten ta designbeslutninger — gi den klare rammer i AGENTS.md.",
    href: "/praksis#vanlige-mønstre-for-agent-mode",
    label: "Agent-mønstre",
  },
  {
    text: "Gi eksempler i prompts. Vis Copilot ett konkret eksempel på ønsket output, og den matcher stilen mye bedre.",
    href: "/praksis#prompt-engineering",
    label: "Eksempler i prompts",
  },
  {
    text: "PR-er fra Copilot coding agent utløser ikke CI automatisk. Du må starte workflows manuelt — dette er en sikkerhetsfunksjon.",
    href: "/praksis#gjennomgå-copilots-arbeid",
    label: "CI og sikkerhet",
  },
  {
    text: "Pass på scope creep: Copilot refaktorerer gjerne kode du ikke ba om. Sett klare grenser i oppgavebeskrivelsen.",
    href: "/praksis#styrker-begrensninger-og-farer",
    label: "Scope creep",
  },
  {
    text: "Bruk copilot-instructions.md for å definere tech stack, kodestil og testmønstre. Det gir konsistente resultater på tvers av teamet.",
    href: "/praksis#effektive-tilpasninger",
    label: "Tilpasninger",
  },
  {
    text: "Kontekst er viktigere enn modellvalg. Gode instruksjoner i repoet gir bedre resultater enn å bytte til en dyrere modell.",
    href: "/praksis#forbered-for-suksess",
    label: "Kontekst vs. modell",
  },
  {
    text: "Copilot kan hallusinere API-er og biblioteker som ikke finnes. Verifiser alltid at importerte pakker og funksjoner eksisterer.",
    href: "/praksis#styrker-begrensninger-og-farer",
    label: "Hallusinasjoner",
  },
  {
    text: "Definer klare grenser med «Always / Ask First / Never»-mønsteret i AGENTS.md. Det hindrer agenten i å gjøre ting den ikke burde.",
    href: "/praksis#effektive-tilpasninger",
    label: "Boundaries-mønsteret",
  },
  {
    text: "Lange chat-sesjoner fører til konteksttap. Start ny samtale når du bytter oppgave — da husker Copilot bedre.",
    href: "/praksis#styrker-begrensninger-og-farer",
    label: "Konteksthåndtering",
  },
  {
    text: "Be Copilot gjennomgå sin egen PR: «Review this PR for bugs, security issues, and code style violations.» Nyttig som første sjekk.",
    href: "/praksis#gjennomgå-copilots-arbeid",
    label: "Selv-review",
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
