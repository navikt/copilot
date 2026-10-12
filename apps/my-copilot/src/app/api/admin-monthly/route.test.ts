import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: vi.fn(), isAdmin: vi.fn() }));
vi.mock("@/lib/cached-bigquery", () => ({ getAdminMonthlyOverview: vi.fn() }));

import { getUserToken, isAdmin } from "@/lib/auth";
import { getAdminMonthlyOverview } from "@/lib/cached-bigquery";
import { GET } from "./route";

describe("GET /api/admin-monthly", () => {
  beforeEach(() => {
    vi.mocked(getUserToken).mockResolvedValue("token");
    vi.mocked(getAdminMonthlyOverview).mockResolvedValue({
      year: 2026,
      months: [],
      coverage: { membership_from: "", last_gross_day: "", gross_stale: false, last_net_month: "", seats_snapshot: "" },
    });
  });

  it("answers 404 to non-admins without fetching", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    const res = await GET(new Request("http://x/api/admin-monthly?year=2026"));
    expect(res.status).toBe(404);
    expect(getAdminMonthlyOverview).not.toHaveBeenCalled();
  });

  it("serves the CSV to admins", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    const res = await GET(new Request("http://x/api/admin-monthly?year=2026"));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Disposition")).toContain("copilot-nav-2026.csv");
    expect(await res.text()).toContain("# Copilot-kostnad i USD for hele Nav, 2026");
  });
});
