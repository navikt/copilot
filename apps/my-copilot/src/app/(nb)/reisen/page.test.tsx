import { render, screen, within } from "@testing-library/react";
import { PHASES } from "./milestones";
import ReisenPage from "./page";

vi.mock("next/navigation", () => ({ usePathname: () => "/reisen" }));

describe("reisesiden", () => {
  it("viser hver fase i tidslinjen med kildene sine", () => {
    render(<ReisenPage />);
    const steps = within(screen.getByRole("list", { name: "Tidslinje" })).getAllByRole("heading", { level: 3 });
    expect(steps).toHaveLength(PHASES.length);
    for (const m of PHASES) {
      expect(m.sources.length).toBeGreaterThan(0);
      for (const s of m.sources) {
        if (!("url" in s)) continue;
        expect(s.url).toMatch(/^https:\/\//);
        expect(screen.getByRole("link", { name: s.label })).toHaveAttribute("href", s.url);
      }
    }
  });

  it("holder tidslinjen i kronologisk rekkefølge", () => {
    // A date without a day («2026-10») sorts after every day in that month.
    const dates = PHASES.map((m) => (m.date.length === 4 ? m.date : m.date.padEnd(10, "-99")));
    expect(dates).toEqual([...dates].sort());
    for (const m of PHASES) if (m.end) expect(m.end >= m.date).toBe(true);
    for (const m of PHASES) {
      expect(m.status).not.toBe("");
      expect(m.milestones.length).toBeGreaterThan(0);
      const inPhase = m.milestones.map((ms) => ms.date);
      expect(inPhase).toEqual([...inPhase].sort());
    }
  });

  it("viser perioden for hver fase", () => {
    render(<ReisenPage />);
    expect(screen.getByText("mai–oktober 2026")).toBeInTheDocument();
    expect(screen.getByText("januar–november 2025")).toBeInTheDocument();
    expect(screen.getByText("mars–november 2024")).toBeInTheDocument();
  });

  it("viser tallene med lenke til kilden", () => {
    render(<ReisenPage />);
    expect(screen.getByRole("heading", { name: "Tall vi kan vise fram" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "skills i katalogen" })).toHaveAttribute("href", "/verktoy");
  });

  it("lenker navnet til GitHub-profilen", () => {
    render(<ReisenPage />);
    expect(screen.getByRole("link", { name: "Hans Kristian Flaatten" })).toHaveAttribute(
      "href",
      "https://github.com/Starefossen"
    );
  });
});
