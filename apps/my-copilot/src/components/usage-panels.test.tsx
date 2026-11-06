vi.mock("react-chartjs-2", () => ({ Bar: () => null }));

import { render, screen } from "@testing-library/react";
import { ClientPanels, EditorPanels, clientLabel, editorLabel, looksLikeVersion } from "./usage-panels";
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
    expect(screen.getAllByText("1.138.0").length).toBeGreaterThan(0); // raw name in the full table
  });
  it("flags only version-like names", () => {
    expect(looksLikeVersion("1.138.0")).toBe(true);
    expect(looksLikeVersion("vscode")).toBe(false);
    expect(editorLabel("neovim")).toBe("neovim");
  });
});

describe("topTakeaway", () => {
  const nb = (s: string) => s.replace(/\u00a0/g, " ");
  it("finds the leader regardless of order and names ties", () => {
    expect(
      nb(
        topTakeaway(
          [
            { label: "a", value: 1 },
            { label: "b", value: 9 },
          ],
          "kall"
        )
      )
    ).toBe("b har flest kall: 9 av 10, 90 %.");
    expect(
      topTakeaway(
        [
          { label: "a", value: 5 },
          { label: "b", value: 5 },
        ],
        "kall"
      )
    ).toContain("a og b har flest");
  });
  it("does not call unknown zero", () => {
    expect(topTakeaway([{ label: "a", value: null }], "kall")).toBe("Tall for kall er ikke tilgjengelig.");
    expect(topTakeaway([], "kall")).toBe("Tall for kall er ikke tilgjengelig.");
    expect(
      topTakeaway(
        [
          { label: "a", value: 0 },
          { label: "b", value: 0 },
        ],
        "kall"
      )
    ).toBe("Alle observerte rader har 0 kall.");
  });
  it("restricts zero claims to the known rows when some are unknown", () => {
    const text = topTakeaway(
      [
        { label: "a", value: 0 },
        { label: "b", value: null },
      ],
      "kall"
    );
    expect(text).toBe("Radene med kjente tall har 0 kall. 1 av 2 rader mangler tall.");
    expect(text).not.toContain("Alle observerte");
  });
  it("qualifies partial and top-N totals", () => {
    expect(
      nb(
        topTakeaway(
          [
            { label: "a", value: 3 },
            { label: "b", value: null },
          ],
          "kall"
        )
      )
    ).toBe("a har flest kall blant radene med kjente tall: 3 av 3 i radene, 100 %.");
    expect(
      topTakeaway(
        [
          { label: "a", value: 3 },
          { label: "b", value: 1 },
        ],
        "kall",
        true
      )
    ).toBe("a har flest kall blant de viste radene: 3.");
  });
});

describe("EditorPanels leaders", () => {
  it("computes each metric's leader independently of the shared order", () => {
    const e = (editor: string, generations: number, acceptances: number, interactions: number) => ({
      editor,
      first_day: "2026-10-01",
      last_day: "2026-10-28",
      days: 28,
      generations,
      acceptances,
      interactions,
    });
    render(<EditorPanels editors={[e("alfa", 10, 1, 100), e("beta", 5, 50, 20)]} />);
    expect(screen.getByText(/alfa har flest genereringer/)).toBeInTheDocument();
    expect(screen.getByText(/beta har flest aksepteringer/)).toBeInTheDocument();
    expect(screen.getByText(/alfa har flest interaksjoner/)).toBeInTheDocument();
  });
  it("labels vscode like the client panels and keeps the raw name", () => {
    expect(editorLabel("vscode")).toBe("VS Code");
    expect(clientLabel("vscode")).toBe("VS Code");
    expect(clientLabel("jetbrains")).toBe("JetBrains");
    expect(clientLabel("newtool")).toBe("newtool");
    render(
      <EditorPanels
        editors={[
          {
            editor: "vscode",
            first_day: "2026-10-01",
            last_day: "2026-10-28",
            days: 28,
            generations: 1,
            acceptances: 1,
            interactions: 1,
          },
        ]}
      />
    );
    expect(screen.getAllByText("vscode").length).toBeGreaterThan(0);
  });
});
