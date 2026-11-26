import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn(async () => new Headers()) }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
}));
const auth = { user: { name: "A123456" } as unknown, token: "t" as string | null };
vi.mock("@/lib/auth", () => ({ getUser: vi.fn(async () => auth.user), getUserToken: vi.fn(async () => auth.token) }));
const discover = vi.fn();
vi.mock("@/lib/fleet-tools", async (orig) => ({
  ...(await orig<typeof import("@/lib/fleet-tools")>()),
  discoverTools: (...a: unknown[]) => discover(...a),
}));

import { DiscoveryError } from "@/lib/fleet-tools";
import { POST } from "./route";

const SAME = { origin: "https://my.nav.no", host: "my.nav.no" };
const post = (body: unknown, headers: Record<string, string> = SAME) =>
  POST({ headers: new Headers(headers), json: async () => body } as unknown as Request);

describe("POST /api/fleet/tools", () => {
  beforeEach(() => {
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    discover.mockReset();
    auth.user = { name: "A123456" };
    auth.token = "t";
  });

  it("passes the person's own token, builds the address from team and app, and answers the tools", async () => {
    discover.mockResolvedValueOnce([{ name: "a", suggestedRisk: "read", destructive: false }]);
    const res = await post({ namespace: "team-a", app: "docs", url: "http://evil.team-b/mcp" });
    expect(res.status).toBe(200);
    // The client's url is ignored.
    expect(discover).toHaveBeenCalledWith("t", "team-a", "http://docs.team-a/mcp");
    await expect(res.json()).resolves.toEqual({ tools: [{ name: "a", suggestedRisk: "read", destructive: false }] });
  });

  it("keeps the path, never the host, from the client", async () => {
    discover.mockResolvedValue([]);
    await post({ namespace: "team-a", app: "docs", path: "/mcp/gw" });
    expect(discover).toHaveBeenLastCalledWith("t", "team-a", "http://docs.team-a/mcp/gw");
    await post({ namespace: "team-a", app: "docs", path: "/" });
    expect(discover).toHaveBeenLastCalledWith("t", "team-a", "http://docs.team-a/");
  });

  it.each(["mcp", "/../x", "//evil.no/mcp", "/a?b", "/a#b", "/møte", "/" + "a".repeat(256), "/a@b"])(
    "refuses the path %s before any call",
    async (path) => {
      expect((await post({ namespace: "team-a", app: "docs", path })).status).toBe(400);
      expect(discover).not.toHaveBeenCalled();
    }
  );

  it("refuses without a login, from another origin, or without team and address, before any call", async () => {
    auth.token = null;
    expect((await post({ namespace: "team-a", app: "docs" })).status).toBe(401);
    auth.token = "t";
    expect(
      (await post({ namespace: "team-a", app: "docs" }, { origin: "https://evil", host: "my.nav.no" })).status
    ).toBe(403);
    expect((await post({ namespace: "../x", app: "docs" })).status).toBe(400);
    expect((await post({ namespace: "team-a", app: " " })).status).toBe(400);
    expect((await post({ namespace: "team-a", app: "x.team-b" })).status).toBe(400);
    expect((await post({ namespace: "team-a", url: "http://docs.team-a/mcp" })).status).toBe(400);
    expect(discover).not.toHaveBeenCalled();
  });

  it.each([
    ["access", 502, /fikk ikke hentet verktøyene fra appen/],
    ["auth", 401, /^Logg inn på nytt og prøv igjen\.$/],
    ["member", 403, /ikke medlem av teamet/],
    ["target", 422, /hører ikke til teamet/],
    ["noroute", 502, /teamets namespace/],
    ["toolarge", 502, /beskrivelsene/],
    ["invalid", 502, /som en MCP-server/],
    ["unavailable", 503, /Prøv igjen/],
    ["off", 503, /ikke slått på/],
  ] as const)("maps %s to %i with a next step", async (kind, status, msg) => {
    discover.mockRejectedValueOnce(new DiscoveryError(kind));
    const res = await post({ namespace: "team-a", app: "docs" });
    expect(res.status).toBe(status);
    await expect(res.json()).resolves.toMatchObject({ kind, error: expect.stringMatching(msg) });
  });
});
