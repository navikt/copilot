import { fireEvent, render, screen, within } from "@testing-library/react";
import TeamGrossUsage from "./team-gross-usage";
import TeamControls from "./team-controls";
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
  it("unifies month, search and optional columns without changing cost defaults", () => {
    render(
      <TeamControls month="2026-09">
        <TeamGrossUsage
          data={{
            ...gross,
            usage: { "1": { models: ["model-a", "model-b"], feature: "copilot_cli", language: "kotlin" } },
          }}
          net={net}
          myTeams={[]}
          previous={null}
        />
      </TeamControls>
    );
    const table = screen.getByRole("table", { name: "Andre team" });
    expect(screen.getByRole("combobox", { name: "Måned" })).toHaveValue("2026-09");
    expect(within(table).getAllByRole("columnheader")).toHaveLength(5);
    fireEvent.click(screen.getByText("Velg kolonner"));
    for (const label of ["Modeller", "Funksjon", "Språk"])
      fireEvent.click(screen.getByRole("checkbox", { name: label }));
    expect(within(table).getAllByRole("columnheader")).toHaveLength(8);
    expect(within(table).getByText("model-a, model-b")).toBeInTheDocument();
    expect(within(table).getByText("Copilot CLI")).toBeInTheDocument();
    expect(within(table).getByText("kotlin")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox", { name: "Søk etter team" }), { target: { value: "alpha" } });
    expect(within(table).queryByText("beta")).not.toBeInTheDocument();
    expect(within(table).getByText("alpha")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Modeller" }));
    expect(within(table).queryByText("model-a, model-b")).not.toBeInTheDocument();
  });
  it("keeps the distinct bill separate from overlapping team rows and puts mine first", () => {
    render(<TeamGrossUsage data={gross} net={net} myTeams={["beta"]} previous={null} />);
    expect(screen.getByText(/teambeløpene kan derfor ikke summeres/i)).toBeInTheDocument();
    expect(screen.getByText(/per medlem er gjennomsnittet/i)).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Mine team" })).getByText("beta")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Andre team" })).getByText("alpha")).toBeInTheDocument();
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
    expect(screen.getAllByText("—")).toHaveLength(2);
  });

  it("keeps browsing available when identity resolution fails", () => {
    render(<TeamGrossUsage data={gross} net={null} myTeams={null} previous={null} />);
    expect(screen.getByText(/kunne ikke finne dine team/i)).toBeInTheDocument();
    expect(screen.getByText(/beløpene er før fradrag/i)).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Andre team" })).toBeInTheDocument();
  });

  it("uses net suppression totals rather than the gross user set", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={{ ...net, small_teams: 3, small_teams_users: 7 }}
        myTeams={null}
        previous={null}
      />
    );
    expect(screen.getByText(/3 team med færre enn fem medlemmer med forbruk/)).toBeInTheDocument();
  });

  it("sorts members, consumption and change without changing the alphabetical default", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={net}
        myTeams={null}
        previous={{
          ...net,
          month: "2026-08",
          teams: [
            { team_id: "1", team_slug: "alpha", users: 5, net_usd: 60 },
            { team_id: "2", team_slug: "beta", users: 6, net_usd: 20 },
          ],
        }}
      />
    );
    const table = screen.getByRole("table", { name: "Andre team" });
    const names = () =>
      within(table)
        .getAllByRole("row")
        .slice(1)
        .map((row) => row.querySelector("td")?.textContent);
    expect(names()).toEqual(["alpha", "beta"]);
    fireEvent.click(within(table).getByRole("button", { name: /medlemmer/i }));
    fireEvent.click(within(table).getByRole("button", { name: /medlemmer/i }));
    expect(names()).toEqual(["beta", "alpha"]);
    fireEvent.click(within(table).getByRole("button", { name: /^forbruk/i }));
    expect(names()).toEqual(["beta", "alpha"]);
    fireEvent.click(within(table).getByRole("button", { name: /per medlem/i }));
    expect(names()).toEqual(["beta", "alpha"]);
    fireEvent.click(within(table).getByRole("button", { name: /endring/i }));
    expect(names()).toEqual(["alpha", "beta"]);
  });

  it("compares visible adjacent net months on the same basis", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={net}
        myTeams={null}
        previous={{ ...net, month: "2026-08", teams: [{ team_id: "1", team_slug: "alpha", users: 5, net_usd: 60 }] }}
      />
    );
    expect(screen.getByText(/\+20,00/)).toBeInTheDocument();
    expect(screen.getByText(/\+20,00/)).toHaveClass("text-[var(--ax-text-danger)]");
  });

  it("shows a per-contributor average and highlights material decreases", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={net}
        myTeams={null}
        previous={{
          ...net,
          month: "2026-08",
          teams: [
            { team_id: "1", team_slug: "alpha", users: 5, net_usd: 100 },
            { team_id: "2", team_slug: "beta", users: 6, net_usd: 75 },
          ],
        }}
      />
    );
    expect(screen.getByText(/16,00/)).toBeInTheDocument();
    expect(screen.getByText(/[−-]20,00/)).toHaveClass("text-[var(--ax-text-success)]");
    expect(screen.getByText(/[−-]5,00/)).not.toHaveAttribute("class");
  });
});
