import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUserToken: vi.fn() }));
vi.mock("@/lib/cached-bigquery", () => ({ getTeamYearOverview: vi.fn() }));

import { getUserToken } from "@/lib/auth";
import { getTeamYearOverview } from "@/lib/cached-bigquery";
import type { TeamYearOverview } from "@/lib/types";
import { GET } from "./route";

const data: TeamYearOverview = {
  team_id: "123",
  team_slug: "team-a",
  year: 2026,
  coverage: {
    membership_from: "2026-05-06",
    gross_from: "2026-06-15",
    last_usage_day: "2026-10-09",
    net_months: ["2026-08"],
  },
  months: [
    {
      month: "2026-04",
      basis: "none",
      hidden: false,
      users: null,
      net_usd: null,
      gross_usd: null,
      no_usage_net_usd: null,
    },
    {
      month: "2026-07",
      basis: "gross",
      hidden: true,
      users: null,
      net_usd: null,
      gross_usd: null,
      no_usage_net_usd: null,
    },
    { month: "2026-08", basis: "net", hidden: false, users: 6, net_usd: 80.1, gross_usd: 100, no_usage_net_usd: 0 },
  ],
};

const get = (query: string) => GET(new Request(`http://localhost/api/team-year?${query}`));

describe("GET /api/team-year", () => {
  beforeEach(() => {
    vi.mocked(getUserToken).mockResolvedValue("token");
    vi.mocked(getTeamYearOverview).mockResolvedValue(data);
  });

  it("requires a login", async () => {
    vi.mocked(getUserToken).mockResolvedValue(null as unknown as string);
    expect((await get("team=123&year=2026")).status).toBe(401);
  });

  it("rejects a team that is not an id", async () => {
    expect((await get("team=team-a&year=2026")).status).toBe(400);
    expect((await get("team=123&year=x")).status).toBe(400);
    expect(getTeamYearOverview).not.toHaveBeenCalledWith("team-a", expect.anything(), expect.anything());
  });

  it("falls back to the team id in the filename when the slug is unsafe", async () => {
    vi.mocked(getTeamYearOverview).mockResolvedValue({ ...data, team_slug: 'a"b' });
    expect((await get("team=123&year=2026")).headers.get("Content-Disposition")).toContain(
      'filename="copilot-123-2026.csv"'
    );
  });

  it("returns the months as CSV with caveats as comments", async () => {
    const res = await get("team=123&year=2026");
    expect(res.headers.get("Content-Type")).toContain("text/csv");
    expect(res.headers.get("Content-Disposition")).toContain("copilot-team-a-2026.csv");
    const lines = (await res.text()).trim().split("\n");
    const header = lines.findIndex((line) => !line.startsWith("#"));
    expect(lines.slice(0, header).some((line) => line.includes("kan ikke legges sammen"))).toBe(true);
    expect(lines.slice(header)).toEqual([
      "måned,grunnlag,medlemmer_med_forbruk,netto_usd,brutto_usd,netto_uten_bruk_usd",
      "2026-04,Ingen data,,,,",
      "2026-07,Brutto (skjult),,,,",
      "2026-08,Netto,6,80.10,100.00,0.00",
    ]);
    expect(getTeamYearOverview).toHaveBeenCalledWith("123", 2026, "token");
  });
});
