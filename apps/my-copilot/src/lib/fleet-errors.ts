import { NextResponse } from "next/server";
import { FleetControlError } from "@/lib/fleet-control";

export const fail = (error: string, status: number, detail?: string, fields?: Record<string, string>) =>
  NextResponse.json({ error, detail, fields }, { status });

const NO_ACCESS = (team: string) =>
  `Du har ikke tilgang. Bare medlemmer av ${team} kan gjøre dette. Er du på teamet, be en eier legge deg til i Nais Console.`;

// The 403 reason decides (apps/fleet-control/internal/api/reasons.json); anything else is NO_ACCESS.
// shortcut: the message match is the fallback for fleet-control without reasons; drop it one release after #3086.
const NOT_APPROVER = "only the platform team approves MCP server versions";

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
    case "spec.defaultRisk":
    case "spec.toolRisk":
      return ["defaultRisk", "Velg Leser, Foreslår eller Utfører for verktøyene."];
    case "spec.rosRef":
      return [
        "rosRef",
        "Verktøy som utfører krever en ROS i prod. Lim inn lenken, eller sett verktøyene til «Leser» eller «Foreslår».",
      ];
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
export function failure(err: unknown, team: string, name: string, generation: number, what = "MCP-serveren") {
  const fce = err instanceof FleetControlError ? err : null;
  const status = fce?.status ?? 0;
  const detail = fce?.detail ?? "";
  if (status === 401) return fail("Du er logget ut. Last inn siden på nytt og logg inn.", 401);
  if (status === 403) {
    const reason = fce?.reason;
    const legacy = !reason || reason === "Forbidden";
    if (reason === "NotPlatformTeam" || (legacy && detail === NOT_APPROVER))
      return fail(`Team copilot godkjenner MCP-servere. Versjon ${generation} venter på dem.`, 403);
    if (reason === "SubmitterNotPerson" || (legacy && detail === NO_PERSON))
      return fail(`Ingen person sendte inn versjon ${generation}. Noen på ${team} må sende den inn på nytt.`, 403);
    if (reason === "SelfApproval" || (legacy && detail === SELF_GRANT))
      return fail(`Du sendte inn agentprofilen selv. En annen på ${team} må gi tilgang.`, 403);
    if (reason === "ApproverNoOid" || (legacy && detail === NO_OID))
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
