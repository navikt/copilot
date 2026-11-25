/**
 * Tool discovery for the /fleet registration wizard: an MCP `tools/list` call
 * through fleet-gateway with the person's own OBO token, never a service token.
 * The answer only pre-fills the form. Annotations are hints from the MCP
 * server and never decide access; fleet-control validates what is submitted.
 */
import { exchangeTokenFor, fetchWithTimeout, isLocalDev } from "@/lib/backend-api";
import { isName } from "@/lib/fleet-names";

export type Risk = "read" | "propose" | "write";
export type DiscoveredTool = { name: string; description?: string; suggestedRisk: Risk; destructive: boolean };

/** Why discovery failed. "access" is also the accessPolicy check: 403 or no connection. */
export type DiscoveryFailure = "unavailable" | "auth" | "access" | "noroute" | "toolarge" | "invalid";

export class DiscoveryError extends Error {
  constructor(public readonly kind: DiscoveryFailure) {
    super(`tool discovery failed: ${kind}`);
    this.name = "DiscoveryError";
  }
}

/** fleet-control's toolNameRegex (seed.go), the charset the gateway routes on. */
export const TOOL_NAME = /^[A-Za-z0-9_.-]{1,128}$/;
export const MAX_TOOLS = 256;
/** Cap per response body. A tools/list with 256 tools and full schemas fits well under it. */
export const MAX_BODY_BYTES = 512 << 10;
const MAX_DESCRIPTION_BYTES = 1024;
const PROTOCOL_VERSION = "2025-06-18";

/** Reads at most MAX_BODY_BYTES; more is DiscoveryError("toolarge"). */
async function readCapped(res: Response): Promise<string> {
  if (Number(res.headers.get("content-length") ?? 0) > MAX_BODY_BYTES) throw new DiscoveryError("toolarge");
  if (!res.body) return "";
  const reader = res.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > MAX_BODY_BYTES) {
      await reader.cancel();
      throw new DiscoveryError("toolarge");
    }
    chunks.push(value);
  }
  return new TextDecoder().decode(Buffer.concat(chunks));
}

/** A JSON-RPC answer from a JSON body or the first SSE `data:` event that parses. */
function rpcResult(body: string, contentType: string): unknown {
  const candidates = contentType.includes("text/event-stream")
    ? body
        .split(/\r?\n/)
        .filter((l) => l.startsWith("data:"))
        .map((l) => l.slice(5).trim())
    : [body];
  for (const c of candidates) {
    try {
      const msg = JSON.parse(c) as { result?: unknown; error?: unknown };
      if (msg && typeof msg === "object" && "result" in msg) return msg.result;
    } catch {
      // try the next event
    }
  }
  throw new DiscoveryError("invalid");
}

const clip = (s: string) => {
  const b = new TextEncoder().encode(s);
  return b.length <= MAX_DESCRIPTION_BYTES
    ? s
    : new TextDecoder().decode(b.slice(0, MAX_DESCRIPTION_BYTES - 3)).replace(/�$/, "") + "…";
};

/**
 * Turns a tools/list result into form rows. A name outside TOOL_NAME, a
 * duplicate or more than MAX_TOOLS fails the whole answer: a partial list
 * would hide tools from the team.
 */
export function parseTools(result: unknown): DiscoveredTool[] {
  const list = (result as { tools?: unknown })?.tools;
  if (!Array.isArray(list) || list.length > MAX_TOOLS) throw new DiscoveryError("invalid");
  const seen = new Set<string>();
  return list.map((t) => {
    const name = (t as { name?: unknown })?.name;
    if (typeof name !== "string" || !TOOL_NAME.test(name) || seen.has(name)) throw new DiscoveryError("invalid");
    seen.add(name);
    const desc = (t as { description?: unknown }).description;
    const ann = (t as { annotations?: Record<string, unknown> }).annotations ?? {};
    // Hints only: a server can claim anything. Read-only pre-fills read; everything else write.
    const readOnly = ann.readOnlyHint === true;
    return {
      name,
      ...(typeof desc === "string" && desc.trim() && { description: clip(desc.trim()) }),
      suggestedRisk: readOnly ? "read" : "write",
      destructive: !readOnly && ann.destructiveHint !== false,
    };
  });
}

/** The gateway route an MCP server gets, /mcp/{team}/{name}. */
export const gatewayPath = (team: string, name: string) => `/mcp/${team}/${name}`;

export async function discoverTools(userToken: string, team: string, name: string): Promise<DiscoveredTool[]> {
  const base = process.env.FLEET_GATEWAY_URL;
  if (!base) throw new DiscoveryError("unavailable");
  if (!isName(team) || !isName(name)) throw new DiscoveryError("invalid");
  // Local dev talks to a local gateway without a token, like fleetGet does with fleet-control.
  const token = isLocalDev()
    ? ""
    : await exchangeTokenFor(userToken, "fleet-gateway").catch(() => {
        throw new DiscoveryError("auth");
      });
  const url = `${base}${gatewayPath(team, name)}`;
  let session: string | null = null;
  const call = async (id: number | undefined, method: string, params: object) => {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
      "MCP-Protocol-Version": PROTOCOL_VERSION,
    };
    if (token) headers.Authorization = `Bearer ${token}`;
    if (session) headers["Mcp-Session-Id"] = session;
    let res: Response;
    try {
      res = await fetchWithTimeout(
        url,
        {
          method: "POST",
          headers,
          cache: "no-store",
          redirect: "error",
          body: JSON.stringify({ jsonrpc: "2.0", ...(id !== undefined && { id }), method, params }),
        },
        10000,
        "fleet-gateway timed out"
      );
    } catch {
      throw new DiscoveryError("access");
    }
    if (res.status === 401) throw new DiscoveryError("auth");
    if (res.status === 404) throw new DiscoveryError("noroute");
    if (res.status === 403 || res.status >= 500) throw new DiscoveryError("access");
    if (!res.ok) throw new DiscoveryError("invalid");
    session = res.headers.get("mcp-session-id") ?? session;
    const body = await readCapped(res);
    return id === undefined ? undefined : rpcResult(body, res.headers.get("content-type") ?? "");
  };
  await call(1, "initialize", {
    protocolVersion: PROTOCOL_VERSION,
    capabilities: {},
    clientInfo: { name: "ki-utvikling-fleet", version: "1" },
  });
  await call(undefined, "notifications/initialized", {});
  return parseTools(await call(2, "tools/list", {}));
}
