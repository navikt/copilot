import { fireEvent, render, screen, within } from "@testing-library/react";
import TeamGrossUsage from "./team-gross-usage";
import TeamControls from "./team-controls";
import type { TeamGrossOverview, TeamNetOverview } from "@/lib/types";

const row = (
  team_id: string,
  team_slug: string,
  users: number,
  amount_usd: number,
  change_usd: number | null = null
) => ({
  team_id,
  team_slug,
  users,
  amount_usd,
  per_user_usd: Math.round((amount_usd / users) * 100) / 100,
  change_usd,
  highlight: false,
});

const gross: TeamGrossOverview = {
  month: "2026-09",
  teams: [row("1", "alpha", 5, 100), row("2", "beta", 6, 90)],
  small_teams: 2,
  last_usage_day: "2026-09-30",
  comparison: "incomplete",
};

const net: TeamNetOverview = {
  month: "2026-09",
  teams: [row("1", "alpha", 5, 80), row("2", "beta", 6, 70)],
  small_teams: 2,
  loaded_at: "2026-10-02 10:00:00+00",
  comparison: "ok",
};

describe("Team insight", () => {
  it("unifies month, search and optional columns without changing cost defaults", () => {
    render(
      <TeamControls month="2026-09">
        <TeamGrossUsage
          data={{
            ...gross,
            usage: {
              "1": {
                providers: ["OpenAI", "Anthropic"],
                categories: ["Versatile", "Lightweight"],
                feature: "copilot_cli",
                language: "kotlin",
              },
              "2": { providers: ["Unclassified"], categories: ["Unclassified"], feature: "", language: "" },
            },
          }}
          net={net}
          myTeams={[]}
        />
      </TeamControls>
    );
    const table = screen.getByRole("table", { name: "Andre team" });
    expect(screen.getByRole("combobox", { name: "Måned" })).toHaveValue("2026-09");
    expect(within(table).getAllByRole("columnheader")).toHaveLength(5);
    for (const label of ["Leverandører", "Modelltyper", "Funksjon", "Språk"])
      expect(within(table).queryByRole("columnheader", { name: label })).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Velg kolonner"));
    expect(screen.getByRole("button", { name: "Velg kolonner" })).toHaveAttribute("aria-expanded", "true");
    for (const label of ["Leverandører", "Modelltyper", "Funksjon", "Språk"])
      fireEvent.click(screen.getByRole("checkbox", { name: label }));
    expect(within(table).getAllByRole("columnheader")).toHaveLength(9);
    const rows = within(table).getAllByRole("row").slice(1);
    expect(
      Array.from(within(rows[0]).getAllByRole("cell")[5].querySelectorAll("span"), (span) => span.textContent)
    ).toEqual(["OpenAI", "Anthropic"]);
    expect(within(rows[0]).getAllByRole("cell")[6]).toHaveTextContent(/^VersatileLightweight$/);
    expect(within(rows[1]).getAllByRole("cell")[5]).toHaveTextContent("Uklassifisert");
    expect(within(rows[1]).getAllByRole("cell")[6]).toHaveTextContent("Uklassifisert");
    for (const row of rows)
      for (const index of [5, 6])
        expect(within(row).getAllByRole("cell")[index].querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    expect(within(table).getByText("Copilot CLI")).toBeInTheDocument();
    expect(within(table).getByText("kotlin")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox", { name: "Søk etter team" }), { target: { value: "alpha" } });
    expect(within(table).queryByText("beta")).not.toBeInTheDocument();
    expect(within(table).getByText("alpha")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Leverandører" }));
    expect(within(table).queryByText("OpenAI")).not.toBeInTheDocument();
    expect(within(table).getByText("Versatile")).toBeInTheDocument();
    const categoryCheckbox = screen.getByRole("checkbox", { name: "Modelltyper" });
    categoryCheckbox.focus();
    fireEvent.click(categoryCheckbox);
    expect(categoryCheckbox).toHaveFocus();
    expect(within(table).queryByText("Versatile")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Leverandører" }));
    expect(within(table).getByText("OpenAI")).toBeInTheDocument();
    expect(within(table).queryByText("Versatile")).not.toBeInTheDocument();
    const checkbox = screen.getByRole("checkbox", { name: "Språk" });
    checkbox.focus();
    fireEvent.keyDown(checkbox, { key: "Escape" });
    expect(screen.getByRole("button", { name: "Velg kolonner" })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("button", { name: "Velg kolonner" })).toHaveFocus();
  });
  it("distinguishes no model interactions from unclassified activity", () => {
    render(
      <TeamControls month="2026-09">
        <TeamGrossUsage
          data={{ ...gross, usage: { "1": { providers: [], categories: [], feature: "", language: "" } } }}
          net={null}
          myTeams={[]}
        />
      </TeamControls>
    );
    fireEvent.click(screen.getByRole("button", { name: "Velg kolonner" }));
    for (const label of ["Leverandører", "Modelltyper"]) fireEvent.click(screen.getByRole("checkbox", { name: label }));
    const row = screen.getByText("alpha").closest("tr")!;
    for (const cell of within(row).getAllByRole("cell").slice(5)) {
      expect(cell).toHaveTextContent("Ingen modellinteraksjoner");
      expect(cell.querySelector("svg")).toBeNull();
    }
    expect(within(row).queryByText("Uklassifisert")).not.toBeInTheDocument();
  });
  it("reports missing backend summaries", () => {
    render(
      <TeamControls month="2026-09">
        <TeamGrossUsage data={gross} net={net} myTeams={[]} />
      </TeamControls>
    );
    fireEvent.click(screen.getByRole("button", { name: "Velg kolonner" }));
    for (const label of ["Leverandører", "Modelltyper"]) fireEvent.click(screen.getByRole("checkbox", { name: label }));
    expect(screen.getByText(/bruksmønster er ikke tilgjengelig fra datatjenesten/i)).toBeInTheDocument();
  });
  it("handles the older model schema without inferring summaries or losing features and languages", () => {
    render(
      <TeamControls month="2026-09">
        <TeamGrossUsage
          data={{
            ...gross,
            usage: JSON.parse('{"1":{"models":["gpt-4o"],"feature":"copilot_cli","language":"kotlin"}}'),
          }}
          net={net}
          myTeams={[]}
        />
      </TeamControls>
    );
    fireEvent.click(screen.getByRole("button", { name: "Velg kolonner" }));
    for (const label of ["Leverandører", "Modelltyper", "Funksjon", "Språk"])
      fireEvent.click(screen.getByRole("checkbox", { name: label }));
    expect(screen.getByText(/leverandør- og modelltypeoversikt mangler/i)).toBeInTheDocument();
    expect(screen.queryByText("gpt-4o")).not.toBeInTheDocument();
    expect(screen.queryByText("OpenAI")).not.toBeInTheDocument();
    expect(screen.getByText("Copilot CLI")).toBeInTheDocument();
    expect(screen.getByText("kotlin")).toBeInTheDocument();
    const rows = within(screen.getByRole("table", { name: "Andre team" }))
      .getAllByRole("row")
      .slice(1);
    for (const row of rows)
      for (const index of [5, 6])
        expect(within(row).getAllByRole("cell")[index]).toHaveTextContent("Ikke tilgjengelig");
  });
  it("shows provider and category summaries in both own and other team rows", () => {
    render(
      <TeamControls month="2026-09">
        <TeamGrossUsage
          data={{
            ...gross,
            usage: {
              "1": { providers: ["Google"], categories: ["Powerful"], feature: "", language: "" },
              "2": { providers: ["Unclassified"], categories: ["Unclassified"], feature: "", language: "" },
            },
          }}
          net={null}
          myTeams={["beta"]}
        />
      </TeamControls>
    );
    fireEvent.click(screen.getByRole("button", { name: "Velg kolonner" }));
    for (const label of ["Leverandører", "Modelltyper"]) fireEvent.click(screen.getByRole("checkbox", { name: label }));
    expect(within(screen.getByRole("table", { name: "Mine team" })).getAllByText("Uklassifisert")).toHaveLength(2);
    const others = within(screen.getByRole("table", { name: "Andre team" }));
    expect(others.getByText("Google", { selector: "span" })).toBeInTheDocument();
    expect(others.getByText("Powerful")).toBeInTheDocument();
    expect(screen.queryByText(/oversikt mangler/i)).not.toBeInTheDocument();
    expect(screen.getByText(/antall brukerinteraksjoner uten kostnadsvekting/)).toBeInTheDocument();
    expect(screen.getByText(/funksjon og språk vises bare med minst fem bidragsytere/i)).toBeInTheDocument();
  });
  it("keeps the distinct bill separate from overlapping team rows and puts mine first", () => {
    render(<TeamGrossUsage data={gross} net={net} myTeams={["beta"]} />);
    expect(screen.getByText(/teambeløpene kan derfor ikke summeres/i)).toBeInTheDocument();
    expect(screen.getByText(/per medlem er gjennomsnittet/i)).toBeInTheDocument();
    expect(screen.getByText(/innsamlet 2026-10-02 10:00:00\+00/)).toBeInTheDocument();
    expect(screen.getByText(/senere fakturakorreksjoner er ikke med/i)).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Mine team" })).getByText("beta")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Andre team" })).getByText("alpha")).toBeInTheDocument();
  });

  it("shows a dash when the API sends no change", () => {
    render(<TeamGrossUsage data={gross} net={net} myTeams={[]} />);
    expect(screen.getAllByText("—")).toHaveLength(2);
  });

  it("keeps browsing available when identity resolution fails", () => {
    render(<TeamGrossUsage data={gross} net={null} myTeams={null} />);
    expect(screen.getByText(/kunne ikke finne dine team/i)).toBeInTheDocument();
    expect(screen.getByText(/beløpene er før fradrag/i)).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Andre team" })).toBeInTheDocument();
  });

  it("uses net suppression totals rather than the gross user set", () => {
    render(<TeamGrossUsage data={gross} net={{ ...net, small_teams: 3 }} myTeams={null} />);
    expect(screen.getByText(/3 team med færre enn fem medlemmer med forbruk/)).toBeInTheDocument();
  });

  it("sorts members, consumption and change without changing the alphabetical default", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={{ ...net, teams: [row("1", "alpha", 5, 80, 20), row("2", "beta", 6, 70, 50)] }}
        myTeams={null}
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

  it("colours an increase the API marks as material", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={{ ...net, teams: [{ ...row("1", "alpha", 5, 80, 20), highlight: true }, row("2", "beta", 6, 70)] }}
        myTeams={null}
      />
    );
    expect(screen.getByText(/\+20,00/)).toBeInTheDocument();
    expect(screen.getByText(/\+20,00/)).toHaveClass("text-[var(--ax-text-danger)]");
  });

  it("shows a per-contributor average and highlights material decreases", () => {
    render(
      <TeamGrossUsage
        data={gross}
        net={{
          ...net,
          teams: [{ ...row("1", "alpha", 5, 80, -20), highlight: true }, row("2", "beta", 6, 70, -5)],
        }}
        myTeams={null}
      />
    );
    expect(screen.getByText(/16,00/)).toBeInTheDocument();
    expect(screen.getByText(/[−-]20,00/)).toHaveClass("text-[var(--ax-text-success)]");
    expect(screen.getByText(/[−-]5,00/)).not.toHaveAttribute("class");
  });
});
