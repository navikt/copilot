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

  it("gir hvert bilde alternativ tekst og bildetekst", () => {
    render(<ReisenPage />);
    const figures = PHASES.flatMap((m) => m.figures ?? []);
    expect(figures.length).toBeGreaterThan(0);
    for (const f of figures) {
      expect(f.alt.trim()).not.toBe("");
      expect(f.caption.trim()).not.toBe("");
      expect(screen.getByRole("img", { name: f.alt })).toHaveAttribute("src", f.src);
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

  it("gir hvert tall en kildelenke, og dato der tallet er et øyeblikksbilde", () => {
    render(<ReisenPage />);
    const grid = screen.getByRole("heading", { name: "Tall vi kan vise fram" }).parentElement!.querySelector("dl")!;
    const items = Array.from(grid.children) as HTMLElement[];
    expect(items.length).toBeGreaterThanOrEqual(10);
    for (const item of items) {
      const href = within(item).getByRole("link").getAttribute("href")!;
      expect(href).toMatch(/^(https:\/\/|\/)/);
      // The catalogue and news counts are computed per request; every other number is a dated snapshot.
      if (!["/verktoy", "/nyheter"].includes(href)) expect(item.querySelector("time[datetime]")).not.toBeNull();
    }
    for (const name of [
      "daglige brukere i juni 2026",
      "vekst i brukere per måned i juni 2026",
      "høyere kostnad etter AI Credits",
    ]) {
      expect(screen.getByRole("link", { name })).toHaveAttribute(
        "href",
        "https://www.kode24.no/artikkel/nav-ma-betale-tre-til-fire-ganger-mer-for-sine-600-copilot-brukere/264699"
      );
    }
    expect(screen.getByRole("link", { name: "mergede pull requests i navikt/copilot" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "KI-modeller målt" })).toHaveAttribute("href", "/modeller");
    expect(screen.getByRole("link", { name: "benchmark-kjøringer" })).toHaveAttribute("href", "/modeller");
  });

  it("lenker navnet til GitHub-profilen", () => {
    render(<ReisenPage />);
    expect(screen.getByRole("link", { name: "Hans Kristian Flaatten" })).toHaveAttribute(
      "href",
      "https://github.com/Starefossen"
    );
  });
});
