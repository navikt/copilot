import { NextResponse } from "next/server";
import { getUser, getUserToken } from "@/lib/auth";
import { FleetControlError, fleetApi, requireFleet, type MCPServerSpec } from "@/lib/fleet-control";
import { isName, isSameOrigin } from "@/lib/fleet-names";

/**
 * Writes for /fleet: register, edit, approve and revoke an MCP server, and «gi tilgang» to an agent profile. A route
 * handler, not a server action: an action id dies with each deploy (#1804).
 * fleet-control checks team membership and four eyes again with the OBO token.
 */
type Body = Record<string, unknown>;

const str = (b: Body, k: string) => (typeof b[k] === "string" ? (b[k] as string).trim() : "");
const fail = (error: string, status: number, detail?: string, fields?: Record<string, string>) =>
  NextResponse.json({ error, detail, fields }, { status });

const NO_ACCESS = (team: string) =>
  `Du har ikke tilgang. Bare medlemmer av ${team} kan gjøre dette. Er du på teamet, be en eier legge deg til i Nais Console.`;

// fleet-control's 403 on approval has no distinct reason, so the exact message decides; anything else is NO_ACCESS.
const SELF_APPROVAL = "the submitter cannot approve their own generation";
const NO_PERSON = "the pending generation has no person as submitter; a person must submit it again";
const NO_OID = "the approver has no object id";
const SELF_GRANT = "the submitter cannot grant access to their own profile";

/**
 * A 422 cause field as a message by the form field it belongs to; "" is the form as a whole.
 * Unknown fields return null so the caller keeps the generic message and the English detail.
 */
function fieldMessage(field: string, detail: string, team: string): [string, string] | null {
  switch (field) {
    case "metadata.name":
      return ["name", "Navnet kan bare ha små bokstaver, tall og bindestrek, høyst 63 tegn."];
    case "spec.upstream.app":
      return ["app", "Appnavnet kan bare ha små bokstaver, tall og bindestrek."];
    case "spec.allowedTools":
    case "spec.tools":
      return ["tools", "Sjekk verktøyene. Skriv minst ett verktøynavn, og hvert navn bare én gang."];
    case "spec.readme":
      return ["readme", "Beskrivelsen kan være høyst 64 KiB."];
    case "spec.dataCategory":
      return detail.includes("StrengtFortrolig")
        ? [
            "dataCategory",
            "Strengt fortrolig krever egen risikovurdering og godkjenning først. Velg en annen datakategori.",
          ]
        : ["dataCategory", "Velg en datakategori fra listen."];
    case "spec.registry.name":
      return [
        "",
        `Navnet i MCP-registeret tilhører ikke ${team}. Bruk no.nav.${team}/<navn> eller io.github.navikt/${team}, eller be plattformteamet registrere navnet.`,
      ];
    case "spec":
      // The upstream URL check reports the whole spec; its message names route.upstream_url.
      if (detail.startsWith("route.upstream_url"))
        return [
          "url",
          `Den interne adressen må gå til appen i ${team}, for eksempel http://min-app.${team}/mcp, uten ? eller #.`,
        ];
      return null;
    default:
      return null;
  }
}

/** Maps fleet-control's Status (code, reason, message, causes) to a Norwegian message with the next step. */
function failure(err: unknown, team: string, name: string, generation: number, what = "MCP-serveren") {
  const fce = err instanceof FleetControlError ? err : null;
  const status = fce?.status ?? 0;
  const detail = fce?.detail ?? "";
  if (status === 401) return fail("Du er logget ut. Last inn siden på nytt og logg inn.", 401);
  if (status === 403) {
    if (detail === SELF_APPROVAL)
      return fail(`Du sendte inn versjon ${generation} selv. En annen på ${team} må godkjenne den.`, 403);
    if (detail === NO_PERSON)
      return fail(`Ingen person sendte inn versjon ${generation}. Noen på ${team} må sende den inn på nytt.`, 403);
    if (detail === SELF_GRANT) return fail(`Du sendte inn agentprofilen selv. En annen på ${team} må gi tilgang.`, 403);
    if (detail === NO_OID)
      return fail("Fleet kjenner ikke igjen innloggingen din. Logg ut, logg inn og prøv igjen.", 403);
    return fail(NO_ACCESS(team), 403);
  }
  if (status === 404) return fail(`Fant ikke ${what}, eller du har ikke tilgang.`, 404);
  if (status === 409) {
    if (fce?.reason === "AlreadyExists")
      return fail(
        `Det finnes allerede en MCP-server som heter ${name} i ${team}. Velg et annet navn, eller åpne den som finnes.`,
        409
      );
    if (fce?.fields.includes("spec.registry.name"))
      return fail(
        "En annen MCP-server bruker allerede dette navnet i MCP-registeret. Send inn en ny versjon med et annet navn.",
        409
      );
    return fail("Noen andre har endret dette. Last inn siden på nytt.", 409);
  }
  if (status === 400 || status === 422) {
    const mapped = (fce?.fields ?? []).map((f) => fieldMessage(f, detail, team));
    if (mapped.length > 0 && mapped.every((m): m is [string, string] => m !== null)) {
      const fields = Object.fromEntries(mapped.filter(([k]) => k !== ""));
      const whole = mapped.find(([k]) => k === "");
      return fail(whole?.[1] ?? "Rett feltene som er merket, og send inn på nytt.", 422, undefined, fields);
    }
    return fail("Fleet godtok ikke endringen. Sjekk feltene og prøv igjen.", 422, fce?.detail);
  }
  return fail("Vi får ikke kontakt med Fleet akkurat nå. Prøv igjen om litt.", 502);
}

/** The spec from the register and edit forms, or the message for the field that is wrong. */
function specFrom(b: Body): MCPServerSpec | [string, string] | string {
  const app = str(b, "app");
  if (!isName(app)) return "App må være små bokstaver, tall og bindestrek, høyst 63 tegn.";
  const dataCategory = str(b, "dataCategory");
  if (!dataCategory) return ["dataCategory", "Velg hva slags data MCP-serveren gir ut."];
  const allowedTools = str(b, "tools")
    .split(/[\s,]+/)
    .filter(Boolean);
  // Not trimmed: leading whitespace is Markdown (indented code).
  const readme = typeof b.readme === "string" && b.readme.trim() ? b.readme : "";
  if (new TextEncoder().encode(readme).length > 64 << 10) return "Beskrivelsen kan være høyst 64 KiB.";
  return { dataCategory, upstream: { app, url: str(b, "url") }, allowedTools, ...(readme && { readme }) };
}

const reasonOf = (b: Body) => {
  const reason = str(b, "reason");
  return reason && reason.length <= 500 ? reason : null;
};

export async function POST(request: Request) {
  requireFleet();
  // Same check as the other protected API routes: an introspected user, not only a token.
  if (!(await getUser(false)) || !(await getUserToken()))
    return fail("Du er logget ut. Last inn siden på nytt og logg inn.", 401);
  if (!isSameOrigin(request.headers))
    return fail("Vi kunne ikke bekrefte at skjemaet kom fra denne siden. Last inn siden på nytt.", 403);
  const b = (await request.json().catch(() => null)) as Body | null;
  if (!b || typeof b !== "object") return fail("Noe er galt med skjemaet. Last inn siden på nytt.", 400);
  const [ns, name] = [str(b, "namespace"), str(b, "name")];
  if (!isName(ns) || !isName(name))
    return fail("Team og navn må være små bokstaver, tall og bindestrek, høyst 63 tegn.", 400);
  const href = `/fleet/mcp-servere/${ns}/${name}`;
  try {
    switch (b.action) {
      case "register":
      case "edit": {
        const spec = specFrom(b);
        if (typeof spec === "string") return fail(spec, 400);
        if (Array.isArray(spec)) return fail(spec[1], 400, undefined, { [spec[0]]: spec[1] });
        if (b.action === "register") await fleetApi.createServer(ns, name, spec);
        else {
          const rv = str(b, "resourceVersion");
          if (!rv) return fail("Noe er galt med skjemaet. Last inn siden på nytt.", 400);
          await fleetApi.updateServer(ns, name, rv, spec);
        }
        break;
      }
      case "approve": {
        const generation = Number(b.generation);
        if (!Number.isInteger(generation)) return fail("Noe er galt med skjemaet. Last inn siden på nytt.", 400);
        await fleetApi.approveServer(ns, name, generation, str(b, "contentHash"));
        break;
      }
      case "revoke": {
        const reason = reasonOf(b);
        if (!reason) return fail("Skriv en begrunnelse på høyst 500 tegn.", 400);
        await fleetApi.revokeServer(ns, name, reason, b.block === true);
        break;
      }
      case "grant": {
        // namespace and name are the agent profile; the MCP server is serverNamespace/serverName.
        const [sns, sname] = [str(b, "serverNamespace"), str(b, "serverName")];
        const generation = Number(b.generation);
        if (!isName(sns) || !isName(sname) || !Number.isInteger(generation))
          return fail("Noe er galt med skjemaet. Last inn siden på nytt.", 400);
        try {
          await fleetApi.grantAccess(
            ns,
            name,
            generation,
            { group: "fleet.nav.no", kind: "MCPServer", namespace: sns, name: sname },
            str(b, "contentHash")
          );
        } catch (err) {
          return failure(err, sns, sname, generation, "agentprofilen");
        }
        return NextResponse.json({ href: `/fleet/agenter/${ns}/${name}` });
      }
      default:
        return fail("Noe er galt med skjemaet. Last inn siden på nytt.", 400);
    }
  } catch (err) {
    return failure(err, ns, name, Number(b.generation));
  }
  return NextResponse.json({ href });
}
