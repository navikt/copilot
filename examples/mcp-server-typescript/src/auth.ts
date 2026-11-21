// Nav-specific auth, ported from apps/mcp-kodeverk/auth.go. Same semantics, fail closed.
import { timingSafeEqual } from "node:crypto";

export const MAX_BEARER_LEN = 16 * 1024;
export const MAX_TEXAS_RESP_BYTES = 1 << 20;

export class Unauthorized extends Error {} // 401: inactive or expired token
export class Forbidden extends Error {} // 403: claims rejected
export class NotConfigured extends Error {} // 503: gateway client ID or Texas missing

export type Fetch = typeof fetch;

export interface AuthConfig {
  introspectionEndpoint: string;
  exchangeEndpoint: string;
  gatewayClientId: string;
  fetch: Fetch;
}

/** fleet-gateway's Entra client ID in this cluster, from AZURE_APP_PRE_AUTHORIZED_APPS. Empty string = not configured. */
export function extractGatewayClientId(raw: string | undefined, cluster: string | undefined): string {
  if (!raw?.trim() || !cluster?.trim()) return "";
  const apps = JSON.parse(raw) as { name?: string; clientId?: string }[];
  const hits = apps.filter((a) => a.name === `${cluster.trim()}:copilot:fleet-gateway` && a.clientId);
  return hits.length === 1 ? hits[0]!.clientId! : "";
}

export function extractBearer(header: string | undefined): string | undefined {
  if (!header || header.length <= 7 || header.length > MAX_BEARER_LEN) return undefined;
  if (header.slice(0, 7).toLowerCase() !== "bearer ") return undefined;
  const tok = header.slice(7).trim();
  return tok && !/\s/.test(tok) ? tok : undefined;
}

// shortcut: buffers the whole body before checking. Texas is a localhost sidecar; stream it if that ever changes.
async function readLimited(res: Response, limit: number): Promise<string> {
  const text = await res.text();
  if (text.length > limit) throw new Error(`response body exceeded ${limit} bytes`);
  return text;
}

async function texas(cfg: AuthConfig, url: string, body: object): Promise<unknown> {
  const res = await cfg.fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`texas returned status ${res.status}`);
  return JSON.parse(await readLimited(res, MAX_TEXAS_RESP_BYTES));
}

function safeEqual(a: string, b: string): boolean {
  const x = Buffer.from(a);
  const y = Buffer.from(b);
  return x.length === y.length && timingSafeEqual(x, y);
}

/** Texas introspection. Returns NAVident, or throws Unauthorized/Forbidden/NotConfigured. */
export async function validateToken(cfg: AuthConfig, token: string): Promise<string> {
  if (!cfg.introspectionEndpoint || !cfg.gatewayClientId) throw new NotConfigured("introspection is not configured");
  const c = (await texas(cfg, cfg.introspectionEndpoint, { identity_provider: "entra_id", token })) as {
    active?: boolean; idtyp?: string; roles?: string[]; NAVident?: string; azp?: string;
  };
  if (!c.active) throw new Unauthorized("invalid or expired bearer token");
  // App tokens (client credentials) are never a user: reject both signals.
  if (c.idtyp?.toLowerCase() === "app" || c.roles?.includes("access_as_application")) throw new Forbidden("token claims rejected");
  const navIdent = c.NAVident?.trim();
  if (!navIdent) throw new Forbidden("token claims rejected");
  if (!safeEqual(c.azp ?? "", cfg.gatewayClientId)) throw new Forbidden("token claims rejected");
  return navIdent;
}

/** Texas OBO: exchange the user's token for one scoped to the downstream API. Never pass the inbound token on. */
export async function exchangeToken(cfg: AuthConfig, userToken: string, target: string): Promise<string> {
  if (!cfg.exchangeEndpoint || !target) throw new Error("downstream token exchange is not configured");
  const out = (await texas(cfg, cfg.exchangeEndpoint, { identity_provider: "entra_id", target, user_token: userToken })) as {
    access_token?: string;
  };
  if (!out.access_token?.trim()) throw new Error("invalid token exchange response");
  return out.access_token;
}
