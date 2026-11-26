/**
 * Tool discovery for /fleet: fleet-gateway's probe (POST /probe/v1/tools,
 * #2985) runs tools/list against the team's own app with the person's own OBO
 * token, never a service token. The gateway checks team membership and that
 * the address is in the team's namespace.
 * The answer only pre-fills the form. Annotations are hints from the MCP
 * server and never decide access; fleet-control validates what is submitted.
 */
import { exchangeTokenFor, fetchWithTimeout, isLocalDev } from "@/lib/backend-api";
import { isName } from "@/lib/fleet-names";

export type Risk = "read" | "propose" | "write";
export type DiscoveredTool = { name: string; description?: string; suggestedRisk: Risk; destructive: boolean };

/**
 * Why discovery failed: the probe's error kinds, "off" where the probe is not
 * enabled, and from the status: "member" (403, not on the team) and "target"
 * (422, address outside the team).
 */
export type DiscoveryFailure =
  "off" | "unavailable" | "auth" | "access" | "member" | "target" | "noroute" | "toolarge" | "invalid";
const KINDS = new Set<string>(["unavailable", "auth", "access", "noroute", "toolarge", "invalid"]);

export class DiscoveryError extends Error {
  constructor(public readonly kind: DiscoveryFailure) {
    super(`tool discovery failed: ${kind}`);
    this.name = "DiscoveryError";
  }
}

/** fleet-control's toolNameRegex (seed.go), the charset the gateway routes on. */
export const TOOL_NAME = /^[A-Za-z0-9_.-]{1,128}$/;
export const MAX_TOOLS = 256;
/** Cap per response body, the probe's own MaxResponseBytes. */
export const MAX_BODY_BYTES = 512 << 10;
const MAX_DESCRIPTION_BYTES = 1024;
const MAX_URL_LENGTH = 2048;

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

export async function discoverTools(userToken: string, team: string, upstreamUrl: string): Promise<DiscoveredTool[]> {
  const base = process.env.FLEET_GATEWAY_URL;
  if (!base) throw new DiscoveryError("off");
  if (!isName(team) || !upstreamUrl || upstreamUrl.length > MAX_URL_LENGTH || !URL.canParse(upstreamUrl))
    throw new DiscoveryError("invalid");
  // Local dev talks to a local gateway without a token, like fleetGet does with fleet-control.
  const token = isLocalDev()
    ? ""
    : await exchangeTokenFor(userToken, "fleet-gateway").catch(() => {
        throw new DiscoveryError("auth");
      });
  let res: Response;
  try {
    res = await fetchWithTimeout(
      `${base}/probe/v1/tools`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json", ...(token && { Authorization: `Bearer ${token}` }) },
        cache: "no-store",
        redirect: "error",
        body: JSON.stringify({ team, upstream_url: upstreamUrl }),
      },
      15000,
      "fleet-gateway timed out"
    );
  } catch {
    throw new DiscoveryError("unavailable");
  }
  // 404: the probe route is not registered in this environment.
  if (res.status === 404) throw new DiscoveryError("off");
  if (res.status === 429) throw new DiscoveryError("unavailable");
  // Caller errors keep their status: the body says only "access" or "invalid".
  if (res.status === 403) throw new DiscoveryError("member");
  if (res.status === 422) throw new DiscoveryError("target");
  const body = await readCapped(res);
  let out: { tools?: unknown; error?: unknown };
  try {
    out = JSON.parse(body);
  } catch {
    throw new DiscoveryError(res.ok ? "invalid" : "unavailable");
  }
  if (!res.ok)
    throw new DiscoveryError(
      typeof out?.error === "string" && KINDS.has(out.error) ? (out.error as DiscoveryFailure) : "unavailable"
    );
  return parseTools(out);
}
