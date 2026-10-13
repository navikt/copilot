import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: vi.fn(), isAdmin: vi.fn() }));
vi.mock("@/lib/rapport-load", async (orig) => ({
  ...(await orig<typeof import("@/lib/rapport-load")>()),
  loadRapport: vi.fn(),
}));

import { getUserToken, isAdmin } from "@/lib/auth";
import { loadRapport } from "@/lib/rapport-load";
import { GET } from "./route";

describe("GET /api/admin-rapport", () => {
  beforeEach(() => {
    vi.mocked(loadRapport).mockReset();
    vi.mocked(getUserToken).mockResolvedValue("token");
  });

  it("answers 404 to non-admins without fetching", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    const res = await GET(new Request("http://x/api/admin-rapport?year=2026"));
    expect(res.status).toBe(404);
    expect(loadRapport).not.toHaveBeenCalled();
  });

  it("answers 401 without a token", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(getUserToken).mockResolvedValue(null);
    expect((await GET(new Request("http://x/api/admin-rapport?year=2026"))).status).toBe(401);
  });

  it("serves the JSON to admins", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(loadRapport).mockResolvedValue({ schema: "copilot-rapport" } as never);
    const res = await GET(new Request("http://x/api/admin-rapport?year=2026"));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Disposition")).toContain("copilot-rapport-2026.json");
    expect(await res.json()).toEqual({ schema: "copilot-rapport" });
  });
});
