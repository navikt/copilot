import { NextResponse } from "next/server";
import { FleetControlError, fleetApi, requireFleet } from "@/lib/fleet-control";
import { isName, isSameOrigin } from "@/lib/fleet-names";

/**
 * Writes for /fleet: register, approve and revoke an MCP server. A route
 * handler, not a server action: an action id dies with each deploy (#1804).
 * fleet-control checks team membership and four eyes again with the OBO token.
 */
type Body = Record<string, unknown>;

const str = (b: Body, k: string) => (typeof b[k] === "string" ? (b[k] as string).trim() : "");
const fail = (error: string, status: number) => NextResponse.json({ error }, { status });

/** Only the status code decides; 400/422 show fleet-control's own message. */
function failure(err: unknown) {
  const fce = err instanceof FleetControlError ? err : null;
  const status = fce?.status ?? 0;
  if (status === 401) return fail("Du er logget ut. Last inn siden på nytt og logg inn.", 401);
  if (status === 403 || status === 404) return fail("Du har ikke tilgang. Bare eierteamet kan gjøre dette.", 403);
  if (status === 409) return fail("Noen andre har endret dette. Last inn siden på nytt.", 409);
  if (status === 400 || status === 422)
    return fail(
      fce?.detail ? `fleet-control godtok ikke endringen: ${fce.detail}` : "fleet-control godtok ikke endringen.",
      422
    );
  return fail("Vi får ikke kontakt med fleet-control akkurat nå. Prøv igjen om litt.", 502);
}

export async function POST(request: Request) {
  requireFleet();
  if (!isSameOrigin(request.headers)) return fail("Forespørselen kom ikke fra denne siden.", 403);
  const b = (await request.json().catch(() => null)) as Body | null;
  if (!b || typeof b !== "object") return fail("Ugyldig forespørsel.", 400);
  const [ns, name] = [str(b, "namespace"), str(b, "name")];
  if (!isName(ns) || !isName(name))
    return fail("Team og navn må være små bokstaver, tall og bindestrek, høyst 63 tegn.", 400);
  const href = `/fleet/mcp-servere/${ns}/${name}`;
  try {
    switch (b.action) {
      case "register": {
        const app = str(b, "app");
        if (!isName(app)) return fail("App må være små bokstaver, tall og bindestrek, høyst 63 tegn.", 400);
        const allowedTools = str(b, "tools")
          .split(/[\s,]+/)
          .filter(Boolean);
        await fleetApi.createServer(ns, name, {
          dataCategory: str(b, "dataCategory"),
          upstream: { app, url: str(b, "url") },
          allowedTools,
        });
        break;
      }
      case "approve": {
        const generation = Number(b.generation);
        if (!Number.isInteger(generation)) return fail("Ugyldig forespørsel.", 400);
        await fleetApi.approveServer(ns, name, generation, str(b, "contentHash"));
        break;
      }
      case "revoke": {
        const reason = str(b, "reason");
        if (!reason || reason.length > 500) return fail("Skriv en begrunnelse på høyst 500 tegn.", 400);
        await fleetApi.revokeServer(ns, name, reason, b.block === true);
        break;
      }
      default:
        return fail("Ugyldig forespørsel.", 400);
    }
  } catch (err) {
    return failure(err);
  }
  return NextResponse.json({ href });
}
