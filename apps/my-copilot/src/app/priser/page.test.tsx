import { render, screen, within } from "@testing-library/react";
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

describe("cache write-kolonnen", () => {
  function radFor(navn: string) {
    return screen.getByText(navn).closest("tr")!;
  }

  it("viser OpenAI-radenes cache write, som fotnoten var eneste eksponering av", () => {
    render(<PriserPage />);

    // Kolonnen var gated på `provider === "Anthropic"`, så disse tallene falt ut
    // av sida selv om de lå i generert data. Sol-fotnoten oppga dem på hover.
    expect(within(radFor("GPT-5.6 Sol (Default, ≤ 272K)")).getByText("$2.50")).toBeInTheDocument();
    expect(within(radFor("GPT-5.6 Sol (Long context, 272K)")).getByText("$5.00")).toBeInTheDocument();
  });

  it("gir rader uten cache write en tankestrek, ikke en tom celle", () => {
    render(<PriserPage />);

    expect(within(radFor("GPT-5.4 (Default, ≤ 272K)")).getByText("—")).toBeInTheDocument();
  });

  it("lar tabeller uten cache write slippe kolonnen helt", () => {
    render(<PriserPage />);

    const google = radFor("Gemini 3.6 Flash (Default)").closest("table")!;
    expect(within(google).queryByRole("columnheader", { name: "Cache write" })).toBeNull();

    const openai = radFor("GPT-5.6 Sol (Default, ≤ 272K)").closest("table")!;
    expect(within(openai).getByRole("columnheader", { name: "Cache write" })).toBeInTheDocument();
  });
});
