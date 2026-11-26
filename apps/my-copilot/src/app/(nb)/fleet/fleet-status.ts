import { hasCondition, refKey } from "@/lib/fleet-names";
import type { AgentProfile, MCPServer } from "@/lib/fleet-control";
import { APPROVER } from "@/lib/fleet-approval";
import { DATA_LABEL } from "@/lib/fleet-wizard";

export type TagVariant = "success" | "warning" | "info" | "neutral" | "error";

// Status comes from the conditions; a missing or unknown one reads as false.
export function serverStatus(s: MCPServer): { label: string; variant: TagVariant } {
  const pending = s.status.pending?.generation;
  // A revoked MCP server can get a new draft, and approving it publishes the server again.
  if (hasCondition(s, "Revoked"))
    return pending
      ? { label: "Avpublisert, ny versjon venter", variant: "warning" }
      : { label: "Avpublisert", variant: "neutral" };
  if (!hasCondition(s, "Published")) return { label: "Venter på godkjenning", variant: "warning" };
  if (pending) return { label: "Ny versjon venter", variant: "info" };
  return { label: "Publisert", variant: "success" };
}

export function agentStatus(p: AgentProfile): { label: string; variant: TagVariant } {
  if (hasCondition(p, "Revoked")) return { label: "Avpublisert", variant: "neutral" };
  if (hasCondition(p, "Expired")) return { label: "Utløpt", variant: "neutral" };
  if (hasCondition(p, "Published")) return { label: "I bruk", variant: "success" };
  return { label: "Venter på eierteamene", variant: "warning" };
}

export const date = (iso: string) =>
  new Date(iso).toLocaleString("nb-NO", { dateStyle: "medium", timeStyle: "short", timeZone: "Europe/Oslo" });

export const n = (v: number) => v.toLocaleString("nb-NO");

/** One short follow-up per MCP server for the overview list, empty when nothing waits. */
export function serverShortNext(s: MCPServer, approver = false): string {
  const p = s.status.pending;
  if (p) return approver ? "Godkjenn versjon " + p.generation : "Venter på " + APPROVER;
  if (hasCondition(s, "Revoked")) return "Send inn en ny versjon";
  return "";
}

/** The MCP servers whose owner team has not granted access yet. A revoked grant keeps grantedAt and is not waiting. */
export const awaiting = (p: AgentProfile) =>
  (p.status.grants ?? []).filter((g) => !g.granted && !g.grantedAt).map((g) => `MCP-server ${refKey(g.serverRef)}`);

export function serverNextStep(s: MCPServer): string {
  const pending = s.status.pending?.generation;
  const published = s.status.publishedGeneration;
  if (hasCondition(s, "Revoked"))
    return pending
      ? `Team copilot må godkjenne versjon ${pending} før MCP-serveren blir publisert igjen.`
      : "Bruk «Publiser igjen» nederst på siden og send inn en ny versjon. Når team copilot har godkjent den, blir MCP-serveren publisert igjen.";
  if (!hasCondition(s, "Published")) return "Team copilot må godkjenne utkastet før MCP-serveren blir publisert.";
  if (pending) return `Team copilot må godkjenne versjon ${pending}. Til da gjelder versjon ${published}.`;
  return `Ingen. Versjon ${published} er publisert.`;
}

export const categoryLabel = (c: string) => DATA_LABEL[c]?.[0] ?? c;

export function agentNextStep(p: AgentProfile): string {
  if (hasCondition(p, "Revoked") || hasCondition(p, "Expired")) return "Ingen.";
  const left = awaiting(p);
  if (left.length) return `Eierteamet til hver MCP-server må gi tilgang. Venter på: ${left.join(", ")}.`;
  if (hasCondition(p, "Published"))
    return p.status.expiresAt
      ? `Ingen. Profilen er godkjent til ${date(p.status.expiresAt)}.`
      : "Ingen. Profilen er godkjent.";
  return "Ingen. Profilen blir godkjent når alle har gitt tilgang.";
}
