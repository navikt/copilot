// The /fleet registration wizard's state, per-step validation and the spec it
// submits. No server imports: the wizard (a client component) and /api/fleet use it.
import type { MCPServerSpec } from "@/lib/fleet-control";
import { isName } from "@/lib/fleet-names";
import { EFFECT } from "@/lib/mcp-labels";
import type { DiscoveredTool } from "@/lib/fleet-tools";

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

// Servers behind fleet-gateway run remotely, so write is «Utfører», never «Endrer filer».
export const RISK_LABEL: Record<Risk, string> = {
  read: EFFECT.read.label,
  propose: EFFECT.propose.label,
  write: EFFECT.acts.label,
};
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

const isRisk = (r: unknown): r is Risk => typeof r === "string" && (RISKS as string[]).includes(r);

/** The edit form's state from the spec fleet-control holds. No defaultRisk means write (#2968), so say so. */
export function fromSpec(team: string, name: string, spec: MCPServerSpec = {}): WizardState {
  const docs = new Map((spec.tools ?? []).map((d) => [d.name, d.description]));
  return {
    team,
    name,
    app: spec.upstream?.app ?? "",
    url: spec.upstream?.url ?? "",
    tools: (spec.allowedTools ?? []).map((t) => {
      const r = spec.toolRisk?.[t];
      return { name: t, description: docs.get(t), allowed: true, risk: isRisk(r) ? r : "" };
    }),
    defaultRisk: isRisk(spec.defaultRisk) ? spec.defaultRisk : "write",
    rosRef: spec.rosRef ?? "",
    dataCategory: spec.dataCategory ?? "",
    readme: spec.readme ?? "",
  };
}

/**
 * «Hent verktøy» into the list: a tool already there keeps its tick and level and gets the new hints; a new
 * tool gets the server's level and starts unticked if it can change something. Tools the server no longer
 * lists stay, so nothing allowed disappears without the team seeing it.
 */
export function mergeDiscovered(tools: WizardTool[], found: DiscoveredTool[]): WizardTool[] {
  const known = new Map(tools.map((t) => [t.name, t]));
  const seen = new Set(found.map((f) => f.name));
  return [
    ...found.map((f): WizardTool => {
      const hints = { suggested: f.suggestedRisk, destructive: f.destructive };
      const old = known.get(f.name);
      if (old) return { ...old, description: f.description ?? old.description, ...hints };
      const writes = f.destructive || f.suggestedRisk === "write";
      return { name: f.name, description: f.description, allowed: !writes, risk: f.suggestedRisk, ...hints };
    }),
    ...tools.filter((t) => !seen.has(t.name)),
  ];
}

export type SpecChange = { text: string; approval: boolean };

/** Each tool's level as fleet-control resolves it: toolRisk, else defaultRisk, else write. */
const levels = (s: MCPServerSpec) =>
  new Map((s.allowedTools ?? []).map((t) => [t, s.toolRisk?.[t] ?? (s.defaultRisk || "write")]));
const label = (r: string) => (isRisk(r) ? RISK_LABEL[r] : r);
const dataLabel = (c?: string) => (c && DATA_LABEL[c]?.[0]) || c || "ingen";

/**
 * What an edit changes against the published spec, and which changes team copilot must approve. Mirrors
 * Classify in apps/fleet-control/internal/model/escalation.go; fleet-control decides, this only previews.
 * published undefined: no published version (or revoked), so everything waits for approval.
 */
export function specChanges(published: MCPServerSpec | undefined, next: MCPServerSpec): SpecChange[] {
  if (!published) return [{ text: "Ingen versjon er publisert ennå", approval: true }];
  const out: SpecChange[] = [];
  const [p, n] = [published.upstream, next.upstream];
  if (p?.app !== n?.app) out.push({ text: `App: ${p?.app ?? "ingen"} til ${n?.app}`, approval: true });
  if (p?.url !== n?.url) out.push({ text: `Intern adresse: ${p?.url ?? "ingen"} til ${n?.url}`, approval: true });
  const [pl, nl] = [levels(published), levels(next)];
  for (const [t, r] of nl) {
    const old = pl.get(t);
    if (old === undefined) out.push({ text: `Nytt verktøy: ${t} (${label(r)})`, approval: true });
    else if (old !== r) out.push({ text: `${t}: ${label(old)} til ${label(r)}`, approval: true });
  }
  for (const t of pl.keys()) if (!nl.has(t)) out.push({ text: `Fjernet verktøy: ${t}`, approval: false });
  if ((published.dataCategory ?? "") !== (next.dataCategory ?? ""))
    out.push({
      text: `Datakategori: ${dataLabel(published.dataCategory)} til ${dataLabel(next.dataCategory)}`,
      approval: true,
    });
  if ((published.rosRef ?? "") !== (next.rosRef ?? ""))
    // Adding a ROS gives no access; changing or removing one does.
    out.push({ text: next.rosRef ? "Ny lenke til ROS" : "Lenken til ROS er fjernet", approval: !!published.rosRef });
  if ((published.readme ?? "") !== (next.readme ?? "")) out.push({ text: "Beskrivelsen", approval: false });
  // Only tools both versions allow: a removed tool's description going with it is not news.
  const docs = (x: MCPServerSpec) => JSON.stringify((x.tools ?? []).filter((d) => pl.has(d.name) && nl.has(d.name)));
  if (docs(published) !== docs(next)) out.push({ text: "Beskrivelser av verktøy", approval: false });
  return out;
}
