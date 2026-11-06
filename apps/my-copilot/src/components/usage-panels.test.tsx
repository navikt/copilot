vi.mock("react-chartjs-2", () => ({ Bar: () => null }));

import { render, screen } from "@testing-library/react";
import { ClientPanels, EditorPanels, editorLabel, looksLikeVersion } from "./usage-panels";
import { topTakeaway } from "@/lib/ranked";

describe("ClientPanels", () => {
  it("ranks by requests, keeps sessions as its own panel and keeps raw names", () => {
    render(
      <ClientPanels
        clients={[
          { client: "opencode", requests: 10, sessions: 50 },
          { client: "vscode", requests: 90, sessions: 5 },
          { client: "newtool", requests: 1, sessions: 1 },
        ]}
      />
    );
    expect(screen.getByText("VS Code har flest forespørsler: 90 av 101, 89 %.")).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual(["Forespørsler", "Økter"]);
    expect(screen.getAllByText("newtool").length).toBeGreaterThan(0);
  });
});

describe("EditorPanels", () => {
  const editors = [
    {
      editor: "vscode",
      first_day: "2026-10-23",
      last_day: "2026-11-05",
      days: 14,
      generations: 0,
      acceptances: null,
      interactions: 120,
    },
    {
      editor: "1.138.0",
      first_day: "2026-11-01",
      last_day: "2026-11-05",
      days: 5,
      generations: 3,
      acceptances: 2,
      interactions: 4,
    },
  ];
  it("shows three panels, flags a version-like name, and keeps unknown apart from zero", () => {
    render(<EditorPanels editors={editors} />);
    expect(screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual([
      "Genereringer",
      "Aksepteringer",
      "Interaksjoner",
    ]);
    expect(screen.getAllByText("1.138.0 (uvanlig navn)").length).toBeGreaterThan(0);
    expect(screen.getByText(/1 av 2 mangler tall/)).toBeInTheDocument();
    expect(screen.getByText(/14 dager med data/)).toBeInTheDocument();
    expect(screen.getAllByText("Ukjent").length).toBeGreaterThan(0);
    expect(screen.getByText("1.138.0")).toBeInTheDocument(); // raw name in the full table
  });
  it("flags only version-like names", () => {
    expect(looksLikeVersion("1.138.0")).toBe(true);
    expect(looksLikeVersion("vscode")).toBe(false);
    expect(editorLabel("neovim")).toBe("neovim");
  });
});

describe("topTakeaway", () => {
  it("leaves unknown out of the total and handles empty data", () => {
    expect(
      topTakeaway(
        [
          { label: "a", value: 3 },
          { label: "b", value: null },
        ],
        "kall"
      )
    ).toBe("a har flest kall: 3 av 3, 100\u00a0%.");
    expect(topTakeaway([], "kall")).toBe("Ingen kall i perioden.");
  });
});
