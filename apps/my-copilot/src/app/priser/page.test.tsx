import { render, screen } from "@testing-library/react";
import PriserPage from "./page";

vi.mock("next/navigation", () => ({
  usePathname: () => "/priser",
}));

describe("kampanjemerket på prissiden", () => {
  it("lar cella hete modellnavnet og den norske merketeksten", () => {
    render(<PriserPage />);

    const badges = screen.getAllByText(/^Kampanjepris t\.o\.m\. /);
    expect(badges.map((b) => b.textContent)).toEqual([
      "Kampanjepris t.o.m. 3. september 2026",
      "Kampanjepris t.o.m. 3. september 2026",
      "Kampanjepris t.o.m. 31. desember 2026",
      "Kampanjepris t.o.m. 31. desember 2026",
    ]);

    expect(badges[0].closest("td")).toHaveAccessibleName(
      "GPT-5.6 Sol (Default, ≤ 272K) Kampanjepris t.o.m. 3. september 2026"
    );
  });

  it("henger ikke GitHubs engelske fotnote på merket", () => {
    render(<PriserPage />);

    // Fotnoten er GitHubs råtekst, og hvert tall i den står allerede i radens
    // egne priskolonner. Å feste den på merket — som `title`, `aria-label`
    // eller `aria-describedby` — gir bare et engelsk avsnitt oppå den norske
    // merketeksten. `aria-label` ville dessuten erstattet den.
    for (const badge of screen.getAllByText(/^Kampanjepris t\.o\.m\. /)) {
      expect(badge).not.toHaveAttribute("title");
      expect(badge).not.toHaveAttribute("aria-label");
      expect(badge).not.toHaveAttribute("aria-describedby");
      expect(badge).toHaveAccessibleDescription("");
    }

    expect(document.body.textContent).not.toContain("promotional pricing");
  });
});
