export interface NavPilotModelChoice {
  purpose: string;
  primary: string;
  fallbacks: string[];
  /** One line on why, condensed from docs/modellvalg.md. Shown on /modeller. */
  reason: string;
  /** Limits «Brukes av» to these agents when two choices share a primary model. */
  users?: string[];
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
    reason: "Besto blokkeringsskjermen 23. september mot GPT-5.6 Sol på samme oppgaver. Ett fasebrudd følges.",
  },
  {
    purpose: "Kodeagenter for Kafka og Rust",
    primary: "GPT-6 Luna",
    fallbacks: ["GPT-6 Sol", "GPT-5.3-Codex"],
    reason:
      "Besto alle 30 sjekker i kodesuiten på Medium for omtrent 1,7 credits, mot omtrent 28 med GPT-6 Sol. Oppgavene var små, så Sol er reserve.",
    users: ["@kafka", "@rust"],
  },
  {
    purpose: "Research og faste maler",
    primary: "GPT-6 Luna",
    fallbacks: ["GPT-5.6 Luna", "GPT-5.3-Codex"],
    reason: "Besto samme krav som GPT-5.6 Luna i blokkeringsskjermen 23. september, til lavere pris.",
  },
  {
    purpose: "Høyrisikoplanlegging og kodegjennomgang",
    primary: "Claude Opus 5.5",
    fallbacks: ["GPT-6.1 Sol", "GPT-5.3-Codex"],
    reason: "Fant alle plantede feil på riktig linje i ti av ti kodegjennomganger 30. september. Low holder.",
  },
  {
    purpose: "Aksel, tilgjengelighet og norsk tekst",
    primary: "Claude Sonnet 5.5",
    fallbacks: ["Claude Sonnet 5"],
    reason: "God på komponentstruktur, WCAG og norsk klarspråk. Ikke målt i den siste modelltesten.",
  },
  {
    purpose: "Rask oppretting av Aksel-komponenter",
    primary: "Gemini 3.8 Flash",
    fallbacks: [],
    reason: "Rask og billig til å lage Aksel-komponenter fra en fast mal.",
  },
];

/**
 * Models Nav has turned off in GitHub Copilot. Every other model counts as
 * enabled, so a new model shows up on /priser without a code change. Add a model
 * here when Nav turns it off. Names are without the price-tier suffix, as
 * normalizeModelName returns them.
 */
const NAV_DISABLED_MODELS = new Set([
  "GPT-5 mini",
  "GPT-5.4",
  "GPT-5.4 mini",
  "GPT-5.5",
  "Claude Haiku 4.5",
  "Claude Sonnet 4",
  "Claude Sonnet 4.6",
  "Claude Opus 4.7",
  "Claude Opus 4.8 (fast mode)",
  "Claude Opus 5",
  "Claude Fable 5",
  "Claude Fable 5.1",
  "Gemini 3.5 Flash",
  "Gemini 3.6 Flash",
  "Gemini 3.7 Flash",
]);

export function normalizeModelName(model: string): string {
  return model.replace(/ \((?:Default|Long context)[^)]*\)$/, "").replace(/ \(preview\)$/, "");
}

export function isNavAllowedModel(model: string): boolean {
  return !NAV_DISABLED_MODELS.has(normalizeModelName(model));
}

export function navPilotPurposesFor(model: string): string[] {
  const normalized = normalizeModelName(model);
  return NAV_PILOT_MODEL_CHOICES.filter(
    (choice) => choice.primary === normalized || choice.fallbacks.includes(normalized)
  ).map((choice) => choice.purpose);
}
