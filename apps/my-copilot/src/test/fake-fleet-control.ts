// A fake fleet-control for tests: serves the golden answers fleet-control's
// TestRealPostgresContract writes (apps/fleet-control/testdata/contract, #3087).
import { readFileSync } from "node:fs";
import path from "node:path";
import { vi } from "vitest";

const DIR = path.resolve(__dirname, "../../../fleet-control/testdata/contract");
const NS = "/apis/fleet.nav.no/v1alpha1/namespaces";

export const golden = (file: string): unknown => JSON.parse(readFileSync(path.join(DIR, file), "utf8"));

/** "METHOD path" to [status, golden file]. Anything else is 404. */
export const FLEET_ROUTES: Record<string, [number, string]> = {
  [`GET ${NS}/team-a/mcpservers`]: [200, "mcpserver-list.json"],
  [`GET ${NS}/team-a/mcpservers/svc`]: [200, "mcpserver-pending.json"],
  [`POST ${NS}/team-a/mcpservers`]: [409, "status-already-exists.json"],
  [`POST ${NS}/team-a/mcpservers/svc/approval`]: [200, "mcpserver-published.json"],
  [`GET ${NS}/team-c/agentprofiles`]: [200, "agentprofile-list.json"],
  [`GET ${NS}/team-c/agentprofiles/assistant`]: [200, "agentprofile-ready.json"],
};

export function fakeFleetControl(routes = FLEET_ROUTES) {
  return vi.fn(async (url: string | URL, init?: RequestInit) => {
    const hit = routes[`${init?.method ?? "GET"} ${new URL(url).pathname}`];
    if (!hit) return new Response("{}", { status: 404 });
    return new Response(readFileSync(path.join(DIR, hit[1])), {
      status: hit[0],
      headers: { "Content-Type": "application/json" },
    });
  });
}
