import { render, screen, within } from "@testing-library/react";
import { MILESTONES } from "./milestones";
import ReisenPage from "./page";

vi.mock("next/navigation", () => ({ usePathname: () => "/reisen" }));

describe("reisesiden", () => {
  it("viser hvert steg i tidslinjen med dato og kildelenke", () => {
    render(<ReisenPage />);
    const steps = within(screen.getByRole("list", { name: "Tidslinje" })).getAllByRole("listitem");
    expect(steps).toHaveLength(MILESTONES.length);
    for (const m of MILESTONES) {
      expect(m.sources.length).toBeGreaterThan(0);
      for (const s of m.sources) {
        expect(s.url).toMatch(/^https:\/\/github\.com\/navikt\//);
        expect(screen.getByRole("link", { name: s.label })).toHaveAttribute("href", s.url);
      }
    }
  });

  it("holder tidslinjen i kronologisk rekkefølge", () => {
    const dates = MILESTONES.map((m) => m.date);
    expect(dates).toEqual([...dates].sort());
    for (const m of MILESTONES) if (m.end) expect(m.end >= m.date).toBe(true);
    expect(MILESTONES.some((m) => m.major)).toBe(true);
  });

  it("viser perioden for steg som spenner over flere måneder", () => {
    render(<ReisenPage />);
    expect(screen.getByText("juli–oktober 2026")).toBeInTheDocument();
    expect(screen.getByText("desember 2025")).toBeInTheDocument();
    expect(screen.getByText("10. januar 2025")).toBeInTheDocument();
  });

  it("viser tallene med lenke til kilden og godkjenningssaken", () => {
    render(<ReisenPage />);
    expect(screen.getByRole("heading", { name: "Tall vi kan vise fram" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /flettede pull requests/ })).toHaveAttribute(
      "href",
      expect.stringContaining("is%3Amerged")
    );
    expect(screen.getByRole("link", { name: "skills i katalogen" })).toHaveAttribute("href", "/verktoy");
    expect(screen.getByRole("link", { name: "#1511" })).toBeInTheDocument();
  });
});
