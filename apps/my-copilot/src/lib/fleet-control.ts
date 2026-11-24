/**
 * Read client for fleet-control's admin API (apps/fleet-control, #2736). Every
 * call carries the person's OBO token, and fleet-control decides per team what
 * the person sees. The /fleet pages exist only where FLEET_CONTROL_URL is set.
 */
import { notFound, redirect } from "next/navigation";
import { exchangeTokenFor, fetchWithTimeout, isLocalDev } from "@/lib/backend-api";
import { getUserToken } from "@/lib/auth";
import { isName } from "@/lib/fleet-names";

export type Me = { name: string; teams: string[] };

// v1alpha1 shapes (fleet-control apis*.go). Empty fields are left out, and
// enum-like strings stay string so an unknown value never breaks the page.
type Metadata = { name: string; namespace: string; resourceVersion?: string; generation?: number };
export type Condition = { type: string; status: string; reason: string; message?: string };
type Actor = { kind?: string; name: string };
export type ServerRef = { group: string; kind: string; namespace: string; name: string };

export type MCPServerSpec = {
  dataCategory?: string;
  upstream?: { app: string; url: string };
  allowedTools?: string[];
};

export type MCPServer = {
  metadata: Metadata;
  spec?: MCPServerSpec;
  status: {
    publishedGeneration?: number;
    route?: { path: string };
    // spec and contentHash are for the owner team only; approval sends contentHash back.
    pending?: { generation: number; contentHash?: string; spec?: MCPServerSpec; submittedBy?: Actor };
    conditions?: Condition[];
  };
};

export type AgentProfileSpec = {
  identity: { app: string; cluster: string; entraClientID: string };
  mode: string;
  grants?: { serverRef: ServerRef; allowedTools: string[]; maxRisk: string }[];
  quota: { maxInFlight: number; requestsPerMinute: number; dailyCallsPerUser: number; dailyCalls: number };
  dpia?: { owner: string; reference: string };
};

export type AgentProfile = {
  metadata: Metadata;
  spec: AgentProfileSpec;
  status: {
    expiresAt?: string;
    // spec here is for the owner team only; status.grants describe this generation.
    pending?: { generation: number; spec?: AgentProfileSpec };
    grants?: { serverRef: ServerRef; granted: boolean; grantedBy?: Actor; grantedAt?: string; reason?: string }[];
    conditions?: Condition[];
  };
};

export type List<T> = { items: T[] };

export type AuditEvent = { at: string; actor: Actor; action: string; generation?: number };

/** What a page shows instead of data: no access, or fleet-control is unavailable. */
export type FleetProblem = "forbidden" | "conflict" | "unavailable";

export class FleetControlError extends Error {
  /** reason from fleet-control's metav1.Status body, when it sent one. */
  constructor(
    public readonly status: number,
    public readonly reason?: string,
    public readonly detail?: string
  ) {
    super(`fleet-control error (${status}${reason ? ` ${reason}` : ""})`);
    this.name = "FleetControlError";
  }
}

/** The pages are 404 unless FLEET_CONTROL_URL is set (dev only for now). */
export function requireFleet(): string {
  const url = process.env.FLEET_CONTROL_URL;
  if (!url) notFound();
  return url;
}

export async function fleetGet<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const url = requireFleet();
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (!isLocalDev()) {
    const token = await getUserToken();
    if (!token) throw new FleetControlError(401);
    headers.Authorization = `Bearer ${await exchangeTokenFor(token, "fleet-control")}`;
  }
  const res = await fetchWithTimeout(
    `${url}${path}`,
    { method, headers, cache: "no-store", body: body === undefined ? undefined : JSON.stringify(body) },
    10000,
    "fleet-control timed out"
  );
  if (!res.ok) {
    const st = (await res.json().catch(() => null)) as { kind?: string; reason?: unknown; message?: unknown } | null;
    const isStatus = st?.kind === "Status";
    const reason = isStatus && typeof st.reason === "string" ? st.reason : undefined;
    const detail = isStatus && typeof st.message === "string" ? st.message.slice(0, 300) : undefined;
    throw new FleetControlError(res.status, reason, detail);
  }
  return (await res.json()) as T;
}

/**
 * Turns a failed call into what the page does: 401 logs in again, 404 is
 * «Fant ikke siden», 403, 409 (a stale resourceVersion) and everything else
 * (503, timeouts) become an Alert. Only the status code decides.
 */
export function fleetProblem(err: unknown): FleetProblem {
  const fce = err instanceof FleetControlError ? err : null;
  const status = fce?.status ?? 0;
  if (status === 401) redirect("/oauth2/login");
  if (status === 404) notFound();
  if (status === 403) return "forbidden";
  return status === 409 ? "conflict" : "unavailable";
}

/** Runs the calls and returns their data, or the problem that stops the page. */
export async function fleetLoad<T>(load: () => Promise<T>): Promise<{ data: T } | { problem: FleetProblem }> {
  try {
    return { data: await load() };
  } catch (err) {
    return { problem: fleetProblem(err) };
  }
}

const seg = (s: string) => {
  if (!isName(s)) throw new FleetControlError(404);
  return s;
};
const API = "/apis/fleet.nav.no/v1alpha1";
const ns = (namespace: string) => `${API}/namespaces/${seg(namespace)}`;

// Only /apis: the old /admin/v1 routes are never a fallback (#2748).
export const fleetApi = {
  whoami: () => fleetGet<Me>(`${API}/whoami`),
  servers: (namespace: string) => fleetGet<List<MCPServer>>(`${ns(namespace)}/mcpservers`),
  server: (namespace: string, name: string) => fleetGet<MCPServer>(`${ns(namespace)}/mcpservers/${seg(name)}`),
  agents: (namespace: string) => fleetGet<List<AgentProfile>>(`${ns(namespace)}/agentprofiles`),
  agent: (namespace: string, name: string) => fleetGet<AgentProfile>(`${ns(namespace)}/agentprofiles/${seg(name)}`),
  pending: () => fleetGet<List<AgentProfile>>(`${API}/agentprofiles?awaitingApprovalBy=me`),
  history: (kind: "mcpservers" | "agentprofiles", namespace: string, name: string) =>
    fleetGet<List<AuditEvent>>(`${ns(namespace)}/${kind}/${seg(name)}/history`),
  createServer: (namespace: string, name: string, spec: MCPServerSpec) =>
    fleetGet<MCPServer>(`${ns(namespace)}/mcpservers`, "POST", {
      apiVersion: "fleet.nav.no/v1alpha1",
      kind: "MCPServer",
      metadata: { namespace: seg(namespace), name: seg(name) },
      spec,
    }),
  approveServer: (namespace: string, name: string, generation: number, contentHash: string) =>
    fleetGet<MCPServer>(`${ns(namespace)}/mcpservers/${seg(name)}/approval`, "POST", { generation, contentHash }),
  revokeServer: (namespace: string, name: string, reason: string, block: boolean) =>
    fleetGet<MCPServer>(`${ns(namespace)}/mcpservers/${seg(name)}/revocation`, "POST", { reason, block }),
};
