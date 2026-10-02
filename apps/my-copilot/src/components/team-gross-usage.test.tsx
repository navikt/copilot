import { render, screen, within } from "@testing-library/react";
import TeamGrossUsage from "./team-gross-usage";
import type { TeamGrossOverview, TeamNetOverview } from "@/lib/types";

const gross: TeamGrossOverview = {
  month: "2026-09",
  teams: [
    { team_id: "1", team_slug: "alpha", users: 5, gross_usd: 100 },
    { team_id: "2", team_slug: "beta", users: 6, gross_usd: 90 },
  ],
  small_teams: 2,
  small_teams_users: 4,
  small_teams_gross_usd: 40,
  distinct_gross_usd: 120,
  unassigned_gross_usd: 10,
  last_usage_day: "2026-09-30",
  days_with_usage: 30,
};

const net: TeamNetOverview = {
  month: "2026-09",
  teams: [
    { team_id: "1", team_slug: "alpha", users: 5, net_usd: 80 },
    { team_id: "2", team_slug: "beta", users: 6, net_usd: 70 },
  ],
  small_teams: 2,
  small_teams_users: 4,
  small_teams_net_usd: 30,
  known_net_usd: 90,
  unassigned_net_usd: 10,
  no_usage_net_usd: 0,
  enterprise_net_usd: 92,
  residual_net_usd: 2,
  loaded_at: "2026-10-02 10:00:00",
  estimated_timing: true,
  sku: "Copilot AI Credits + Copilot Cloud Agent",
};

describe("Team insight", () => {
  it("keeps the distinct bill separate from overlapping team rows and puts mine first", () => {
    render(<TeamGrossUsage data={gross} net={net} myTeams={["beta"]} previous={null} />);
    expect(screen.getByText(/teambeløp og prosenter kan derfor ikke summeres/i)).toBeInTheDocument();
    expect(screen.getByText(/92,00/)).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Mine team" })).getByText("beta")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Andre team" })).getByText("alpha")).toBeInTheDocument();
    expect(screen.getByRole("form")).toHaveAttribute("action", "/innsikt/team");
  });

  it("does not compare net to gross or reveal a suppressed previous month", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={net}
        myTeams={[]}
        previous={{ ...gross, teams: [{ team_id: "1", team_slug: "alpha", users: 4, gross_usd: 75 }] }}
      />
    );
    expect(screen.getAllByText("Ikke tilgjengelig")).toHaveLength(2);
  });

  it("keeps browsing available when identity resolution fails", () => {
    render(<TeamGrossUsage data={gross} net={null} myTeams={null} previous={null} />);
    expect(screen.getByText(/kunne ikke finne dine team/i)).toBeInTheDocument();
    expect(screen.getByText(/dette er ikke Navs fakturerte nettokostnad/i)).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Andre team" })).toBeInTheDocument();
  });
});
