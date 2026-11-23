/**
 * Read client for fleet-control's admin API (apps/fleet-control, #2736). Every
 * call carries the person's OBO token, and fleet-control decides per team what
 * the person sees. The /fleet pages exist only where FLEET_CONTROL_URL is set.
 */
import { notFound, redirect } from "next/navigation";
import { exchangeTokenFor, fetchWithTimeout, isLocalDev } from "@/lib/backend-api";
import { getUserToken } from "@/lib/auth";

export type Me = { ident: string; teams: string[] };

export type ServerSummary = {
  namespace: string;
  name: string;
  status: "draft" | "published" | "revoked";
  published_version: number | null;
  pending_version: number | null;
};

export type AgentProfile = {
  namespace: string;
  name: string;
  identity: { cluster: string; namespace: string; app: string; entra_client_id: string };
  mode: string;
  grants: { route: string; tools: string[]; max_risk: string }[];
  quota: { max_in_flight: number; rpm: number; daily_calls_per_user: number; daily_calls: number };
  dpia: { owner: string; reference: string };
  status?: "draft" | "approved" | "expired" | "revoked";
  expires_at?: string;
};

export type AgentDetail = {
  id: number;
  status: AgentProfile["status"];
  profile: AgentProfile;
  approvals: Record<string, { approved_by: string; approved_at: string } | null>;
};

export type PendingAgent = { id: number; profile: AgentProfile; awaiting: string[] };

export type AuditEvent = { at: string; actor: string; action: string; version?: number; detail: unknown };

/** What a page shows instead of data: no access, or fleet-control is unavailable. */
export type FleetProblem = "forbidden" | "unavailable";

export class FleetControlError extends Error {
  constructor(public readonly status: number) {
    super(`fleet-control error (${status})`);
    this.name = "FleetControlError";
  }
}

/** The pages are 404 unless FLEET_CONTROL_URL is set (dev only for now). */
export function requireFleet(): string {
  const url = process.env.FLEET_CONTROL_URL;
  if (!url) notFound();
  return url;
}

export async function fleetGet<T>(path: string): Promise<T> {
  const url = requireFleet();
  const headers: Record<string, string> = { Accept: "application/json" };
  if (!isLocalDev()) {
    const token = await getUserToken();
    if (!token) throw new FleetControlError(401);
    headers.Authorization = `Bearer ${await exchangeTokenFor(token, "fleet-control")}`;
  }
  const res = await fetchWithTimeout(`${url}${path}`, { headers, cache: "no-store" }, 10000, "fleet-control timed out");
  if (!res.ok) throw new FleetControlError(res.status);
  return (await res.json()) as T;
}

/**
 * Turns a failed call into what the page does: 401 logs in again, 404 is
 * «Fant ikke siden», 403 and everything else (503, timeouts) become an Alert.
 */
export function fleetProblem(err: unknown): FleetProblem {
  const status = err instanceof FleetControlError ? err.status : 0;
  if (status === 401) redirect("/oauth2/login");
  if (status === 404) notFound();
  return status === 403 ? "forbidden" : "unavailable";
}

/** Runs the calls and returns their data, or the problem that stops the page. */
export async function fleetLoad<T>(load: () => Promise<T>): Promise<{ data: T } | { problem: FleetProblem }> {
  try {
    return { data: await load() };
  } catch (err) {
    return { problem: fleetProblem(err) };
  }
}

const seg = encodeURIComponent;

export const fleetApi = {
  me: () => fleetGet<Me>("/admin/v1/me"),
  servers: (ns: string) => fleetGet<{ servers: ServerSummary[] }>(`/admin/v1/servers?namespace=${seg(ns)}`),
  agents: (ns: string) => fleetGet<{ agents: AgentProfile[] }>(`/admin/v1/agents?namespace=${seg(ns)}`),
  agent: (ns: string, name: string) => fleetGet<AgentDetail>(`/admin/v1/agents/${seg(ns)}/${seg(name)}`),
  pending: () => fleetGet<{ agents: PendingAgent[] }>("/admin/v1/agents/pending-approval"),
  audit: (kind: "servers" | "agents", ns: string, name: string) =>
    fleetGet<{ events: AuditEvent[] }>(`/admin/v1/${kind}/${seg(ns)}/${seg(name)}/audit`),
};
