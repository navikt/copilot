export interface NavPilotModelChoice {
  purpose: string;
  primary: string;
  fallbacks: string[];
  /** One line on why, condensed from docs/modellvalg.md. Shown on /modeller. */
  reason: string;
}

/**
 * @nav-pilot has no model: pin in its frontmatter; the agent pack sets this
 * default (cli/nav-pilot/internal/agentpakke). Every other agent and prompt is
 * listed on /modeller from its frontmatter pin, via copilot-manifest.json.
 */
export const NAV_PILOT_DEFAULT_MODEL = "GPT-6 Sol";

export const NAV_PILOT_MODEL_CHOICES: NavPilotModelChoice[] = [
  {
    purpose: "Daglig agentisk koding",
    primary: "GPT-6 Sol",
    fallbacks: ["GPT-5.6 Sol", "GPT-5.3-Codex"],
    reason:
      "Standard for verktøytunge kodeagenter. Fant alle sikkerhets- og auth-kravene i fem av fem kjøringer, men hoppet over intervjuet i én.",
  },
  {
    purpose: "Research og faste maler",
    primary: "GPT-6 Luna",
    fallbacks: ["GPT-5.6 Luna", "GPT-5.3-Codex"],
    reason: "Besto ti av ti avgrensede krav og brukte omtrent 45 prosent færre credits enn GPT-5.6 Luna.",
  },
  {
    purpose: "Høyrisikoplanlegging og kodegjennomgang",
    primary: "Claude Opus 5.5",
    fallbacks: ["Claude Opus 5", "GPT-5.3-Codex"],
    reason:
      "Med high effort pekte den på riktige linjer i fem av fem gjennomganger. Med medium var linjenumrene feil i to av fem.",
  },
  {
    purpose: "Aksel, tilgjengelighet og norsk tekst",
    primary: "Claude Sonnet 5.5",
    fallbacks: ["Claude Sonnet 5"],
    reason: "God på komponentstruktur, WCAG og norsk klarspråk. Ikke målt i den siste modelltesten.",
  },
  {
    purpose: "Rask Aksel-scaffolding",
    primary: "Gemini 3.8 Flash",
    fallbacks: [],
    reason: "Rask og billig til å lage Aksel-komponenter fra en fast mal.",
  },
];

const NAV_ALLOWED_MODELS = new Set([
  "GPT-5.3-Codex",
  "GPT-5.4 nano",
  "GPT-5.6 Luna",
  "GPT-5.6 Sol",
  "GPT-5.6 Terra",
  "GPT-6 Astra",
  "GPT-6 Luna",
  "GPT-6 Sol",
  "Claude Sonnet 4",
  "Claude Sonnet 4.6",
  "Claude Opus 4.8",
  "Claude Opus 5.5",
  "Claude Sonnet 5",
  "Claude Sonnet 5.5",
  "Gemini 3.8 Flash",
  "MAI-Code-1.1-Flash",
  "Kimi K2.7 Code",
  "Kimi K3",
]);

export function normalizeModelName(model: string): string {
  return model.replace(/ \((?:Default|Long context)[^)]*\)$/, "").replace(/ \(preview\)$/, "");
}

export function isNavAllowedModel(model: string): boolean {
  return NAV_ALLOWED_MODELS.has(normalizeModelName(model));
}

export function navPilotPurposesFor(model: string): string[] {
  const normalized = normalizeModelName(model);
  return NAV_PILOT_MODEL_CHOICES.filter(
    (choice) => choice.primary === normalized || choice.fallbacks.includes(normalized)
  ).map((choice) => choice.purpose);
}
