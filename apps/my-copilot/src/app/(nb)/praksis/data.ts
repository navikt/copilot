import React from "react";
import StrengthsLimitations from "./sections/strengths-limitations";
import ToolsAndModes from "./sections/tools-and-modes";
import PrepareForSuccess from "./sections/prepare-for-success";
import EffectiveCustomizations from "./sections/effective-customizations";
import PromptEngineering from "./sections/prompt-engineering";
import CostOptimization from "./sections/cost-optimization";
import OrchestrateAgents from "./sections/orchestrate-agents";
import ReviewCopilotWork from "./sections/review-copilot-work";
import Verification from "./sections/verification";
import AgentModePatterns from "./sections/agent-mode-patterns";
import Resources from "./sections/resources";
import GettingStarted from "./sections/getting-started";
import UnderstandingCode from "./sections/understanding-code";
import Troubleshooting from "./sections/troubleshooting";

export type GuideMeta = {
  id: string;
  title: string;
  description: string;
  keywords?: string[];
  iconName: string;
};

export type Guide = GuideMeta & {
  components: React.ComponentType<unknown>[];
};

export type CategoryMeta = {
  title: string;
  description: string;
  guides: GuideMeta[];
};

export type Category = CategoryMeta & {
  guides: Guide[];
};

export const categories: Category[] = [
  {
    title: "Skrive kode og løse oppgaver",
    description: "Slik får du Copilot til å bygge nye funksjoner og løse oppgavene du gir den.",
    guides: [
      {
        id: "kom-i-gang",
        title: "Kom i gang med Copilot",
        description: "Hvordan få tilgang, og hvordan installere Copilot i din editor.",
        keywords: ["kom i gang", "installasjon", "tilgang", "lisens", "auth"],
        iconName: "PlayIcon",
        components: [GettingStarted],
      },
      {
        id: "skrive-presise-prompts",
        title: "Skriv presise prompts for koding",
        description: "Lær teknikkene for å få nøyaktig den koden du ber om (WRAP).",
        keywords: ["skrive prompts", "instruksjoner", "prompting", "ai chat", "wrap"],
        iconName: "ChatIcon",
        components: [PromptEngineering],
      },
      {
        id: "lese-og-forsta-kode",
        title: "Forstå eksisterende og legacy kode",
        description: "Bruk Copilot til å lese kode, forklare stack traces og dokumentere legacy.",
        keywords: ["legacy", "forståelse", "stack trace", "dokumentasjon", "lese kode"],
        iconName: "GlassesIcon",
        components: [UnderstandingCode],
      },
      {
        id: "styrker-og-farer",
        title: "Forstå styrkene og fellene ved AI",
        description: "Forstå begrensninger, og lær om personvern og .copilotignore.",
        keywords: ["sikkerhet", "begrensninger", "personvern", "pii", "copilotignore", "trening"],
        iconName: "ShieldLockIcon",
        components: [StrengthsLimitations],
      },
    ],
  },
  {
    title: "Test, verifiser og feilsøk",
    description: "Metoder for å sikre at koden Copilot genererer faktisk fungerer og er trygg.",
    guides: [
      {
        id: "skrive-og-kjore-tester",
        title: "Skriv og kjør tester trygt",
        description: "Slik bruker du Copilot til å etablere testdekning og verifisere kode.",
        keywords: ["feilsøking", "testing", "kvalitetssikring", "unit testing", "playwright"],
        iconName: "FileCheckmarkIcon",
        components: [Verification],
      },
      {
        id: "gjennomfore-code-review",
        title: "Gjennomfør Code Review med Copilot",
        description: "Bruk agenter til å kvalitetssikre andres kode, eller se over Copilots arbeid.",
        keywords: ["code review", "pr", "pull request", "kvalitetssikring"],
        iconName: "MagnifyingGlassIcon",
        components: [ReviewCopilotWork],
      },
      {
        id: "feilsoking",
        title: "Når Copilot stopper opp (Feilsøking)",
        description: "Løsninger på de vanligste problemene med Copilot og Agenter.",
        keywords: ["feil", "krasj", "stoppet", "hjelp", "support", "troubleshooting"],
        iconName: "WrenchIcon",
        components: [Troubleshooting],
      },
    ],
  },
  {
    title: "Agenter: Oppsett og Orkestrering",
    description: "Hvordan delegere arbeidet til autonome agenter i terminalen og editoren.",
    guides: [
      {
        id: "forberede-prosjektet",
        title: "Gjør prosjektet klart for agenter",
        description: "Grunnlaget du må ha på plass før du slipper agenter løs på kodebasen.",
        keywords: ["sette opp agenter", "arkitektur", "planlegging", "forberedelser"],
        iconName: "WrenchIcon",
        components: [PrepareForSuccess],
      },
      {
        id: "skreddersy-med-skills-og-rules",
        title: "Skreddersy med egne Skills og Rules",
        description: "Lær å bygge effektive tilpasninger (Customizations) for ditt team.",
        keywords: ["skills", "rules", "konfigurasjon", "tilpasninger"],
        iconName: "PuzzlePieceIcon",
        components: [EffectiveCustomizations],
      },
      {
        id: "orkestrere-agenter",
        title: "Orkestrer flere agenter for store endringer",
        description: "Hvordan delegere oppgaver og bruke vanlige mønstre som Test-Driven Development.",
        keywords: ["mønstre", "patterns", "delegere", "sette opp agenter", "arkitektur"],
        iconName: "RobotIcon",
        components: [OrchestrateAgents, AgentModePatterns],
      },
    ],
  },
  {
    title: "Innsikt og Optimalisering",
    description: "Få dypere innsikt i hvordan Copilot fungerer under panseret.",
    guides: [
      {
        id: "velge-riktig-verktoy",
        title: "Spar tid med riktige verktøy og moduser",
        description: "Oversikt over de ulike modusene og verktøyene Copilot tilbyr.",
        keywords: ["moduser", "verktoy", "copilot chat", "agent mode"],
        iconName: "TerminalIcon",
        components: [ToolsAndModes],
      },
      {
        id: "redusere-token-bruk",
        title: "Reduser token-bruk og spar kostnader",
        description: "Hvordan bruke Copilot effektivt uten å sprenge token-budsjettet med RTK.",
        keywords: ["kostnadsoptimalisering", "token", "pris", "rtk", "økonomi"],
        iconName: "BarChartIcon",
        components: [CostOptimization],
      },
      {
        id: "nyttige-ressurser",
        title: "Finn nyttige ressurser og lenker",
        description: "Lenker til videre lesning, verktøy, slack-kanaler og fellesskap.",
        keywords: ["hjelp", "ressurser", "slack", "støtte"],
        iconName: "BookIcon",
        components: [Resources],
      },
    ],
  },
];

export const allGuides = categories.flatMap((c) => c.guides);
