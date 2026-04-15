"use client";

import React from "react";
import "@/lib/chart-utils"; // registers Chart.js components
import { SurveyBarChart } from "@/components/charts/survey/SurveyBarChart";
import { LikertChart } from "@/components/charts/survey/LikertChart";
import { VStack } from "@navikt/ds-react";

type Section = "tools" | "value" | "likert" | "change" | "themes";

const TOTAL = 163;

const toolLabels = [
  "Copilot (github.com)",
  "Copilot (IntelliJ)",
  "Copilot CLI",
  "Copilot (VS Code)",
  "Extensions / MCP",
  "Claude Code",
  "Ikke AI-verktøy",
  "OpenCode",
  "Andre",
];
const toolValues = [95, 88, 86, 54, 25, 22, 12, 9, 10];

const valueLabels = [
  "Forstå eksisterende kode",
  "Code completions",
  "Feilsøking",
  "Skrive tester",
  "Hjelp med code review",
  "Refaktorering",
  "Generere boilerplate",
  "Lære nye språk / API-er",
  "Delegere til autonom agent",
  "Dokumentasjon",
];
const valueValues = [78, 70, 66, 47, 40, 39, 28, 23, 21, 18];

const changeLabels = [
  "Bedre opplæring",
  "Forstå kodebase/rammeverk",
  "Flere AI-verktøy/miljøer",
  "Sikkerhet/personvern",
  "Fornøyd som det er",
  "Foretrekker uten AI",
  "Færre begrensninger",
  "Annet",
];
const changeValues = [50, 24, 21, 21, 16, 15, 5, 11];

const themeLabels = [
  "Dramatisk tidsbesparelse",
  "Kodeforståelse / lære",
  "Kvalitetsproblemer",
  "Ubehag / skepsis",
  "Gjennombruddsøyeblikk",
  "Dokumentasjon / tekst",
  "Eierskap / kompetansetap",
  "Code review-effekter",
  "Debugging / feilsøking",
  "Påtvunget teknologi",
  "Teamsamarbeid",
  "Informasjonssikkerhet",
  "Onboarding",
];
const themeValues = [12, 9, 9, 8, 7, 6, 6, 5, 4, 4, 3, 2, 2];

const likertItems = [
  {
    label: "Hjelper meg fullføre raskere",
    helt_enig: 46,
    enig: 76,
    noytral: 29,
    uenig: 9,
    helt_uenig: 3,
  },
  {
    label: "Fornøyd med verktøyene",
    helt_enig: 48,
    enig: 71,
    noytral: 32,
    uenig: 9,
    helt_uenig: 3,
  },
  {
    label: "Reduserer kognitiv belastning",
    helt_enig: 45,
    enig: 64,
    noytral: 42,
    uenig: 8,
    helt_uenig: 4,
  },
  {
    label: "Bekymret for kompetansetap",
    helt_enig: 36,
    enig: 60,
    noytral: 36,
    uenig: 25,
    helt_uenig: 6,
  },
  {
    label: "Eierskap til AI-generert kode",
    helt_enig: 43,
    enig: 58,
    noytral: 31,
    uenig: 20,
    helt_uenig: 11,
  },
  {
    label: "AI-kode god nok for review",
    helt_enig: 19,
    enig: 37,
    noytral: 70,
    uenig: 29,
    helt_uenig: 8,
  },
  {
    label: "Personvern hindrer bruk",
    helt_enig: 16,
    enig: 24,
    noytral: 42,
    uenig: 51,
    helt_uenig: 30,
  },
];

export const SurveyCharts: React.FC<{ section: Section }> = ({ section }) => {
  switch (section) {
    case "tools":
      return (
        <VStack gap="space-8" className="my-6">
          <SurveyBarChart
            title="Hvilke AI-kodeverktøy bruker du?"
            labels={toolLabels}
            values={toolValues}
            total={TOTAL}
            height={310}
          />
        </VStack>
      );

    case "value":
      return (
        <VStack gap="space-8" className="my-6">
          <SurveyBarChart
            title="Hvor gir AI mest verdi? (velg opptil 3)"
            labels={valueLabels}
            values={valueValues}
            total={TOTAL}
            height={340}
            color="rgba(16, 185, 129, 1)"
          />
        </VStack>
      );

    case "likert":
      return (
        <VStack gap="space-8" className="my-6">
          <LikertChart title="Hvor enig er du i følgende påstander?" items={likertItems} />
        </VStack>
      );

    case "change":
      return (
        <VStack gap="space-8" className="my-6">
          <SurveyBarChart
            title="Hva er det viktigste å endre?"
            labels={changeLabels}
            values={changeValues}
            total={TOTAL}
            height={280}
            color="rgba(139, 92, 246, 1)"
          />
        </VStack>
      );

    case "themes":
      return (
        <VStack gap="space-8" className="my-6">
          <SurveyBarChart
            title="Temaer i fritekstkommentarene (53 svar)"
            labels={themeLabels}
            values={themeValues}
            total={53}
            height={400}
            color="rgba(245, 158, 11, 1)"
          />
        </VStack>
      );
  }
};
