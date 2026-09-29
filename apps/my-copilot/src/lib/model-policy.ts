export const NAV_MODEL_POLICY_LAST_UPDATED = "2026-09-29";

const NAV_DISABLED_MODELS = new Set([
  "Claude Haiku 4.5",
  "GPT-5 mini",
  "Claude Fable 5",
  "Claude Fable 5.1",
  "Claude Opus 4.7",
  "Claude Opus 4.8 (fast mode)",
  "Claude Opus 5",
  "Gemini 3.5 Flash",
  "Gemini 3.6 Flash",
  "Gemini 3.7 Flash",
  "GPT-5.4",
  "GPT-5.4 mini",
  "GPT-5.5",
  "Grok 4.5",
  "Grok 4.6",
  "Grok 4.7",
]);

export interface NavPilotModelChoice {
  purpose: string;
  primary: string;
  fallbacks: string[];
  usedBy: string;
}

export const NAV_PILOT_MODEL_CHOICES: NavPilotModelChoice[] = [
  {
    purpose: "Daglig agentisk koding",
    primary: "GPT-6 Sol",
    fallbacks: ["GPT-5.6 Sol", "GPT-5.3-Codex"],
    usedBy: "@nav-pilot, @kafka, @rust og @security-champion",
  },
  {
    purpose: "Research og faste maler",
    primary: "GPT-6 Luna",
    fallbacks: ["GPT-5.6 Luna", "GPT-5.3-Codex"],
    usedBy: "@research og enkle scaffold-prompts",
  },
  {
    purpose: "Høyrisikoplanlegging og kodegjennomgang",
    primary: "Claude Opus 5.5",
    fallbacks: ["Claude Opus 5", "GPT-5.3-Codex"],
    usedBy: "@nav-pilot-opus og @code-review",
  },
  {
    purpose: "Aksel, tilgjengelighet og norsk tekst",
    primary: "Claude Sonnet 5.5",
    fallbacks: ["Claude Sonnet 5"],
    usedBy: "@aksel, @accessibility og @forfatter",
  },
  {
    purpose: "Rask Aksel-scaffolding",
    primary: "Gemini 3.8 Flash",
    fallbacks: [],
    usedBy: "aksel-component-prompten",
  },
];

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
