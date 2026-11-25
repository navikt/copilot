import { NextResponse } from "next/server";
import { getUser, getUserToken } from "@/lib/auth";
import { requireFleet } from "@/lib/fleet-control";
import { isName, isSameOrigin } from "@/lib/fleet-names";
import { DiscoveryError, discoverTools } from "@/lib/fleet-tools";

/**
 * Tool discovery for the registration wizard: tools/list through fleet-gateway
 * as the logged-in person. Answers { tools } or { error, kind }; the wizard
 * shows the nais.yaml snippet for kind "access".
 */
const fail = (error: string, kind: string, status: number) => NextResponse.json({ error, kind }, { status });

const MESSAGES: Record<string, [string, number]> = {
  unavailable: ["Henting av verktøy er ikke slått på her. Skriv inn verktøyene selv.", 503],
  auth: ["Vi fikk ikke logget deg inn mot Fleet. Last inn siden på nytt og prøv igjen.", 401],
  access: [
    "Fleet fikk ikke kontakt med MCP-serveren. Sjekk at appen kjører, og at den slipper inn fleet-gateway i accessPolicy.inbound.",
    502,
  ],
  noroute: [
    "Fleet har ingen rute til MCP-serveren ennå, så verktøyene kan ikke hentes før den er godkjent. Skriv inn verktøyene selv.",
    404,
  ],
  toolarge: ["Svaret fra MCP-serveren var for stort. Skriv inn verktøyene selv.", 502],
  invalid: [
    "Svaret fra MCP-serveren kunne ikke brukes. Verktøynavn kan bare ha bokstaver, tall, _, . og -. Skriv inn verktøyene selv.",
    502,
  ],
};

export async function POST(request: Request) {
  requireFleet();
  const token = await getUserToken();
  if (!(await getUser(false)) || !token) return fail(MESSAGES.auth[0], "auth", 401);
  if (!isSameOrigin(request.headers))
    return fail("Vi kunne ikke bekrefte at skjemaet kom fra denne siden. Last inn siden på nytt.", "origin", 403);
  const b = (await request.json().catch(() => null)) as { namespace?: unknown; name?: unknown } | null;
  const ns = typeof b?.namespace === "string" ? b.namespace : "";
  const name = typeof b?.name === "string" ? b.name : "";
  if (!isName(ns) || !isName(name))
    return fail("Team og navn må være små bokstaver, tall og bindestrek, høyst 63 tegn.", "invalid", 400);
  try {
    return NextResponse.json({ tools: await discoverTools(token, ns, name) });
  } catch (err) {
    const kind = err instanceof DiscoveryError ? err.kind : "access";
    const [msg, status] = MESSAGES[kind];
    return fail(msg, kind, status);
  }
}
