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
    seats_snapshot: "2026-10-11",
    team_members_from: "2026-10-11",
  },
  months: [
    {
      month: "2026-04",
      basis: "none",
      users: null,
      net_usd: null,
      gross_usd: null,
      no_usage_net_usd: null,
      seat_months: null,
      licence_usd_estimate: null,
      total_usd: null,
      total_basis: "none",
    },
    {
      month: "2026-05",
      basis: "premium",
      users: 7,
      net_usd: 50.5,
      gross_usd: null,
      no_usage_net_usd: 1,
      residual_pct: 7.13,
      seat_months: 7,
      licence_usd_estimate: 133,
      total_usd: 184.5,
      total_basis: "premium",
    },
    {
      month: "2026-07",
      basis: "gross",
      users: 4,
      net_usd: null,
      gross_usd: 60,
      no_usage_net_usd: null,
      seat_months: 9,
      licence_usd_estimate: 171,
      total_usd: 231,
      total_basis: "gross",
    },
    {
      month: "2026-08",
      basis: "net",
      users: 6,
      net_usd: 80.1,
      gross_usd: 100,
      no_usage_net_usd: 2,
      residual_pct: 2.4,
      seat_months: 8,
      licence_usd_estimate: 152,
      total_usd: 234.1,
      total_basis: "net",
    },
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

  it("says licences are not loaded before the first seat snapshot", async () => {
    vi.mocked(getTeamYearOverview).mockResolvedValue({ ...data, coverage: { ...data.coverage, seats_snapshot: "" } });
    const text = await (await get("team=123&year=2026")).text();
    expect(text).toContain("Lisensene er ikke lest inn ennå");
    expect(text).not.toContain("Lisenser er ikke med");
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
    expect(lines.slice(0, header).some((line) => line.includes("Totalraden er summen"))).toBe(true);
    expect(lines.slice(header)).toEqual([
      "måned,grunnlag,medlemmer_med_forbruk,fakturert_usd,foreløpig_forbruk_usd,fakturert_uten_bruk_usd,medlemmer_med_lisens,lisenskostnad_usd_anslag,totalt_inkl_lisenser_usd",
      "2026-04,Ingen data,,,,,,,",
      "2026-05,Premiumforespørsler,7,50.50,,1.00,7,133.00,184.50",
      "2026-07,Foreløpig forbruk,4,,60.00,,9,171.00,231.00",
      "2026-08,Fakturert,6,80.10,100.00,2.00,8,152.00,234.10",
      "Totalt mai–august,,,130.60,,3.00,,285.00,418.60",
    ]);
    expect(lines.some((line) => line.includes("Lisenskostnad er et anslag"))).toBe(true);
    expect(lines.some((line) => /skjult|færre enn fem/i.test(line))).toBe(false);
    expect(lines.filter((line) => line.includes("knyttes til en person"))).toEqual([
      "# Ikke fordelt: andelen av Navs fakturerte beløp som ikke kunne knyttes til en person, og som derfor ikke er med i tallene for noe team. mai 7\u00a0%, august 2\u00a0%.",
    ]);
    expect(lines.some((line) => line.includes("brukte Copilot i måneden"))).toBe(false);
    expect(getTeamYearOverview).toHaveBeenCalledWith("123", 2026, "token");
  });
});
