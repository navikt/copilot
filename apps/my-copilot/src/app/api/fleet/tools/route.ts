import { NextResponse } from "next/server";
import { getUser, getUserToken } from "@/lib/auth";
import { requireFleet } from "@/lib/fleet-control";
import { isName, isSameOrigin } from "@/lib/fleet-names";
import { DiscoveryError, type DiscoveryFailure, discoverTools, otherFleet } from "@/lib/fleet-tools";

/**
 * Tool discovery for the registration wizard and the edit form: fleet-gateway's
 * probe as the logged-in person. Answers { tools } or { error, kind }. The
 * address is built here from team and app, never taken from the client. The
 * client adds the checklist and nais.yaml snippet for "access".
 */
/**
 * Path from the registered address. No query, fragment, "..", "//" or non-ASCII:
 * fleet-gateway's ValidateUpstreamURL refuses query and fragment, the rest keeps it a plain path.
 */
const PATH = /^\/[A-Za-z0-9._~\/-]{0,255}$/;
const isPath = (p: string) => PATH.test(p) && !p.includes("..") && !p.includes("//");

const fail = (error: string, kind: string, status: number, extra?: object) =>
  NextResponse.json({ error, kind, ...extra }, { status });

// Short and actionable; the client always offers typing the tools in by hand.
const cluster = () => process.env.NAIS_CLUSTER_NAME || "dette clusteret";
const MESSAGES: Record<Exclude<DiscoveryFailure, "notincluster">, [string, number]> = {
  off: ["Henting av verktøy er ikke slått på her.", 503],
  unavailable: ["Fleet kunne ikke hente verktøyene akkurat nå. Prøv igjen om litt.", 503],
  auth: ["Logg inn på nytt og prøv igjen.", 401],
  // The gateway gives every failure at the app the same answer, so the client lists all causes.
  access: ["Fleet fikk ikke hentet verktøyene fra appen.", 502],
  member: ["Du er ikke medlem av teamet. Be noen på teamet hente verktøyene, eller skriv dem inn selv.", 403],
  target: ["Adressen hører ikke til teamet. Velg en app som kjører i teamets namespace.", 422],
  noroute: ["Fant ikke MCP-serveren på adressen. Sjekk appnavnet og at appen kjører i teamets namespace.", 502],
  toolarge: ["Svaret fra MCP-serveren var for stort. Ta bort verktøy eller kort ned beskrivelsene.", 502],
  invalid: ["MCP-serveren svarte ikke som en MCP-server.", 502],
};

export async function POST(request: Request) {
  requireFleet();
  const token = await getUserToken();
  if (!(await getUser(false)) || !token) return fail(MESSAGES.auth[0], "auth", 401);
  if (!isSameOrigin(request.headers))
    return fail("Vi kunne ikke bekrefte at skjemaet kom fra denne siden. Last inn siden på nytt.", "origin", 403);
  const b = (await request.json().catch(() => null)) as { namespace?: unknown; app?: unknown; path?: unknown } | null;
  const ns = typeof b?.namespace === "string" ? b.namespace : "";
  const app = typeof b?.app === "string" ? b.app.trim() : "";
  const path = (typeof b?.path === "string" && b.path.trim()) || "/mcp";
  if (!isName(ns) || !isName(app)) return fail("Velg team og app først.", "invalid", 400);
  if (!isPath(path)) return fail("Stien i adressen er ugyldig. Bruk for eksempel /mcp.", "invalid", 400);
  try {
    // Host from team and app only; fleet-gateway resolves it to <app>.<team>.svc.cluster.local.
    return NextResponse.json({ tools: await discoverTools(token, ns, `http://${app}.${ns}${path}`) });
  } catch (err) {
    const kind = err instanceof DiscoveryError ? err.kind : "unavailable";
    if (kind === "notincluster")
      return fail(`Fant ikke appen i ${cluster()}. Fleet på denne siden når bare apper i ${cluster()}.`, kind, 502, {
        other: otherFleet(),
      });
    const [msg, status] = MESSAGES[kind];
    return fail(msg, kind, status);
  }
}
