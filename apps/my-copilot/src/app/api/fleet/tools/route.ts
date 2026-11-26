import { NextResponse } from "next/server";
import { getUser, getUserToken } from "@/lib/auth";
import { requireFleet } from "@/lib/fleet-control";
import { isName, isSameOrigin } from "@/lib/fleet-names";
import { DiscoveryError, type DiscoveryFailure, discoverTools } from "@/lib/fleet-tools";

/**
 * Tool discovery for the registration wizard and the edit form: fleet-gateway's
 * probe as the logged-in person. Answers { tools } or { error, kind }. The
 * client adds the nais.yaml snippet for "access" and links for "invalid".
 */
const fail = (error: string, kind: string, status: number) => NextResponse.json({ error, kind }, { status });

// Short and actionable; the client always offers typing the tools in by hand.
const MESSAGES: Record<DiscoveryFailure, [string, number]> = {
  off: ["Henting av verktøy er ikke slått på her.", 503],
  unavailable: ["Fleet kunne ikke hente verktøyene akkurat nå. Prøv igjen om litt.", 503],
  auth: ["MCP-serveren godtok ikke tokenet. Appen må godta OBO-tokener fra fleet-gateway.", 502],
  access: ["fleet-gateway slipper ikke inn til appen. Legg fleet-gateway til i accessPolicy.inbound.", 502],
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
  const b = (await request.json().catch(() => null)) as { namespace?: unknown; url?: unknown } | null;
  const ns = typeof b?.namespace === "string" ? b.namespace : "";
  const url = typeof b?.url === "string" ? b.url.trim() : "";
  if (!isName(ns) || !url) return fail("Velg team og fyll inn den interne adressen først.", "invalid", 400);
  try {
    return NextResponse.json({ tools: await discoverTools(token, ns, url) });
  } catch (err) {
    const kind = err instanceof DiscoveryError ? err.kind : "unavailable";
    const [msg, status] = MESSAGES[kind];
    return fail(msg, kind, status);
  }
}
