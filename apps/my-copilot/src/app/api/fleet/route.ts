import { NextResponse } from "next/server";
import { getUser, getUserToken } from "@/lib/auth";
import { fleetApi, requireFleet, type MCPServerSpec } from "@/lib/fleet-control";
import { MAX_TOOLS, RISKS, TOOL_NAME } from "@/lib/fleet-wizard";
import { isName, isSameOrigin } from "@/lib/fleet-names";
import { fail, failure } from "@/lib/fleet-errors";

/**
 * Writes for /fleet: register, edit, approve and revoke an MCP server, and «gi tilgang» to an agent profile. A route
 * handler, not a server action: an action id dies with each deploy (#1804).
 * fleet-control checks team membership and four eyes again with the OBO token.
 */
type Body = Record<string, unknown>;

const str = (b: Body, k: string) => (typeof b[k] === "string" ? (b[k] as string).trim() : "");

/** The wizard's spec, or a message when its shape is wrong. Names, levels and sizes only: fleet-control validates the rest. */
function parseRegisterSpec(v: unknown): MCPServerSpec | string {
  const bad = "Noe er galt med skjemaet. Last inn siden på nytt.";
  if (!v || typeof v !== "object") return bad;
  const s = v as Record<string, unknown>;
  const up = s.upstream as Record<string, unknown> | undefined;
  if (!up || typeof up.app !== "string" || !isName(up.app))
    return "App må være små bokstaver, tall og bindestrek, høyst 63 tegn.";
  if (typeof up.url !== "string" || typeof s.dataCategory !== "string" || !s.dataCategory) return bad;
  const tools = s.allowedTools;
  if (
    !Array.isArray(tools) ||
    tools.length === 0 ||
    tools.length > MAX_TOOLS ||
    !tools.every((t) => typeof t === "string" && TOOL_NAME.test(t)) ||
    new Set(tools).size !== tools.length
  )
    return "Verktøynavn kan bare ha bokstaver, tall, _, . og -. Velg minst ett, og hvert navn bare én gang.";
  const isRisk = (r: unknown) => typeof r === "string" && (RISKS as string[]).includes(r);
  if (s.defaultRisk !== undefined && !isRisk(s.defaultRisk)) return bad;
  const toolRisk = s.toolRisk ?? {};
  if (typeof toolRisk !== "object" || !Object.entries(toolRisk).every(([k, r]) => tools.includes(k) && isRisk(r)))
    return bad;
  const docs = s.tools ?? [];
  if (!Array.isArray(docs) || !docs.every((d) => d && typeof d.name === "string" && tools.includes(d.name))) return bad;
  if (s.rosRef !== undefined && typeof s.rosRef !== "string") return bad;
  const readme = typeof s.readme === "string" ? s.readme : "";
  if (new TextEncoder().encode(readme).length > 64 << 10) return "Beskrivelsen kan være høyst 64 KiB.";
  return {
    dataCategory: s.dataCategory,
    upstream: { app: up.app, url: up.url.trim() },
    allowedTools: tools as string[],
    ...(docs.length && {
      tools: (docs as { name: string; description?: unknown }[]).map((d) => ({
        name: d.name,
        ...(typeof d.description === "string" && { description: d.description }),
      })),
    }),
    ...(isRisk(s.defaultRisk) && { defaultRisk: s.defaultRisk as string }),
    ...(Object.keys(toolRisk).length && { toolRisk: toolRisk as Record<string, string> }),
    ...(s.rosRef && { rosRef: s.rosRef as string }),
    ...(readme && { readme }),
  };
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
      case "register": {
        // The wizard sends the spec it built (toSpec in fleet-wizard.ts); check its shape before fleet-control does.
        const spec = parseRegisterSpec(b.spec);
        if (typeof spec === "string") return fail(spec, 400);
        await fleetApi.createServer(ns, name, spec);
        break;
      }
      case "edit": {
        const spec = specFrom(b);
        if (typeof spec === "string") return fail(spec, 400);
        if (Array.isArray(spec)) return fail(spec[1], 400, undefined, { [spec[0]]: spec[1] });
        const rv = str(b, "resourceVersion");
        if (!rv) return fail("Noe er galt med skjemaet. Last inn siden på nytt.", 400);
        await fleetApi.updateServer(ns, name, rv, spec);
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
  // ?sendt=1 shows the «sendt til godkjenning» receipt on the MCP server page.
  return NextResponse.json({ href: b.action === "register" || b.action === "edit" ? `${href}?sendt=1` : href });
}
