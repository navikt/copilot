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
      expect(m.source.url).toMatch(/^https:\/\/github\.com\/navikt\//);
      expect(screen.getByRole("link", { name: m.source.label })).toHaveAttribute("href", m.source.url);
    }
  });

  it("holder tidslinjen i kronologisk rekkefølge", () => {
    const dates = MILESTONES.map((m) => m.date);
    expect(dates).toEqual([...dates].sort());
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
