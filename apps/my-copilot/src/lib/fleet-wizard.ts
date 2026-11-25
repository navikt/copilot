// The /fleet registration wizard's state, per-step validation and the spec it
// submits. No server imports: the wizard (a client component) and /api/fleet use it.
import type { MCPServerSpec } from "@/lib/fleet-control";
import { isName } from "@/lib/fleet-names";

export type Risk = "read" | "propose" | "write";
export const RISKS: Risk[] = ["read", "propose", "write"];
export const DATA_CATEGORIES = ["Aapen", "Intern", "Fortrolig"] as const;
/** fleet-control's toolNameRegex (seed.go), the charset the gateway routes on. */
export const TOOL_NAME = /^[A-Za-z0-9_.-]{1,128}$/;
export const MAX_TOOLS = 256;
const MAX_ROS_BYTES = 512;
const MAX_README_BYTES = 64 << 10;

export type WizardTool = {
  name: string;
  description?: string;
  allowed: boolean;
  /** "" follows defaultRisk. */
  risk: Risk | "";
  /** From annotations: hints for the labels, never a decision. */
  destructive?: boolean;
  suggested?: Risk;
};

export type WizardState = {
  team: string;
  name: string;
  app: string;
  url: string;
  tools: WizardTool[];
  defaultRisk: Risk | "";
  rosRef: string;
  dataCategory: string;
  readme: string;
};

export const emptyState = (team: string): WizardState => ({
  team,
  name: "",
  app: "",
  url: "",
  tools: [],
  defaultRisk: "",
  rosRef: "",
  dataCategory: "",
  readme: "",
});

export const STEPS = ["Navn", "Mål-app", "MCP-endpoint", "Verktøy og risiko", "Data og beskrivelse", "Se over"];

/** The level a tool ends up on: its own, else defaultRisk, else write. */
export const effectiveRisk = (t: WizardTool, d: Risk | ""): Risk => t.risk || d || "write";

const bytes = (s: string) => new TextEncoder().encode(s).length;

/**
 * Field errors for one step (1-based); empty means the step is done. Field
 * keys match the form fields and /api/fleet's error fields. fleet-control
 * checks everything again.
 */
export function validateStep(step: number, s: WizardState, opts: { requireRos: boolean }): Record<string, string> {
  const e: Record<string, string> = {};
  switch (step) {
    case 1:
      if (!isName(s.team)) e.team = "Velg et team.";
      if (!s.name) e.name = "Skriv et navn.";
      else if (!isName(s.name)) e.name = "Navnet kan bare ha små bokstaver, tall og bindestrek, høyst 63 tegn.";
      break;
    case 2:
      if (!s.app) e.app = "Velg appen som kjører MCP-serveren.";
      else if (!isName(s.app)) e.app = "Appnavnet kan bare ha små bokstaver, tall og bindestrek.";
      break;
    case 3: {
      let u: URL | null = null;
      try {
        u = new URL(s.url);
      } catch {
        // handled below
      }
      if (!s.url) e.url = "Skriv den interne adressen.";
      else if (!u || u.protocol !== "http:" || u.search || u.hash || s.url.includes("?") || s.url.includes("#"))
        e.url = `Adressen må begynne med http:// og gå til appen i ${s.team || "teamet"}, uten ? eller #.`;
      break;
    }
    case 4: {
      const names = s.tools.map((t) => t.name);
      if (s.tools.some((t) => !TOOL_NAME.test(t.name)))
        e.tools = "Verktøynavn kan bare ha bokstaver, tall, _, . og -, høyst 128 tegn.";
      else if (new Set(names).size !== names.length) e.tools = "Hvert verktøy kan bare stå én gang.";
      else if (!s.tools.some((t) => t.allowed)) e.tools = "Kryss av for minst ett verktøy.";
      else if (s.tools.filter((t) => t.allowed).length > MAX_TOOLS)
        e.tools = `Du kan tillate høyst ${MAX_TOOLS} verktøy.`;
      if (!s.defaultRisk) e.defaultRisk = "Velg hva verktøyene gjør som standard.";
      if (s.rosRef !== s.rosRef.trim() || bytes(s.rosRef) > MAX_ROS_BYTES)
        e.rosRef = "Lenken til ROS-en kan være høyst 512 tegn, uten mellomrom først eller sist.";
      else if (
        opts.requireRos &&
        !s.rosRef &&
        s.tools.some((t) => t.allowed && effectiveRisk(t, s.defaultRisk) === "write")
      )
        e.rosRef =
          "Verktøy som skriver krever en ROS i prod. Lim inn lenken, eller sett verktøy som ikke skriver til «Leser» eller «Foreslår».";
      break;
    }
    case 5:
      if (!(DATA_CATEGORIES as readonly string[]).includes(s.dataCategory)) e.dataCategory = "Velg en datakategori.";
      if (bytes(s.readme) > MAX_README_BYTES) e.readme = "Beskrivelsen kan være høyst 64 KiB.";
      break;
  }
  return e;
}

/** The first step with an error, or 0 when every step is done. */
export function firstInvalidStep(s: WizardState, opts: { requireRos: boolean }): number {
  for (let i = 1; i <= 5; i++) if (Object.keys(validateStep(i, s, opts)).length) return i;
  return 0;
}

/** The spec for fleet-control: only allowed tools, and toolRisk only where a tool differs from defaultRisk. */
export function toSpec(s: WizardState): MCPServerSpec {
  const allowed = s.tools.filter((t) => t.allowed);
  const toolRisk = Object.fromEntries(
    allowed.filter((t) => t.risk && t.risk !== s.defaultRisk).map((t) => [t.name, t.risk])
  );
  const docs = allowed.filter((t) => t.description).map((t) => ({ name: t.name, description: t.description }));
  return {
    dataCategory: s.dataCategory,
    upstream: { app: s.app, url: s.url.trim() },
    allowedTools: allowed.map((t) => t.name),
    ...(docs.length && { tools: docs }),
    ...(s.defaultRisk && { defaultRisk: s.defaultRisk }),
    ...(Object.keys(toolRisk).length && { toolRisk }),
    ...(s.rosRef && { rosRef: s.rosRef }),
    ...(s.readme.trim() && { readme: s.readme }),
  };
}

export const RISK_LABEL: Record<Risk, string> = { read: "Leser", propose: "Foreslår", write: "Skriver" };
export const DATA_LABEL: Record<string, [string, string]> = {
  Aapen: ["Åpne data", "Kan deles med alle, også utenfor Nav. For eksempel offentlige kodeverk og regelverk."],
  Intern: [
    "Interne data",
    "For ansatte i Nav. Ikke personopplysninger og ikke taushetsbelagt. For eksempel interne rutiner og systemdokumentasjon.",
  ],
  Fortrolig: [
    "Fortrolige data",
    "Kan skade personer eller Nav hvis det kommer på avveie, for eksempel personopplysninger. Velg Fortrolige data hvis du er i tvil.",
  ],
};
