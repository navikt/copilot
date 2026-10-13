import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: vi.fn(), isAdmin: vi.fn() }));
vi.mock("@/lib/innsyn-load", async (orig) => ({
  ...(await orig<typeof import("@/lib/innsyn-load")>()),
  loadInnsyn: vi.fn(),
}));

import { getUserToken, isAdmin } from "@/lib/auth";
import { loadInnsyn } from "@/lib/innsyn-load";
import { GET } from "./route";

describe("GET /api/admin-innsyn", () => {
  beforeEach(() => {
    vi.mocked(loadInnsyn).mockReset();
    vi.mocked(getUserToken).mockResolvedValue("token");
  });

  it("answers 404 to non-admins without fetching", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    const res = await GET(new Request("http://x/api/admin-innsyn?year=2026"));
    expect(res.status).toBe(404);
    expect(loadInnsyn).not.toHaveBeenCalled();
  });

  it("answers 401 without a token", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(getUserToken).mockResolvedValue(null);
    expect((await GET(new Request("http://x/api/admin-innsyn?year=2026"))).status).toBe(401);
  });

  it("serves the JSON to admins", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(loadInnsyn).mockResolvedValue({ schema: "innsyn/v1" } as never);
    const res = await GET(new Request("http://x/api/admin-innsyn?year=2026"));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Disposition")).toContain("innsynssvar-copilot-2026.json");
    expect(await res.json()).toEqual({ schema: "innsyn/v1" });
  });
});
