import { fireEvent, render, screen, within } from "@testing-library/react";
import PriserPage from "./page";

vi.mock("next/navigation", () => ({
  usePathname: () => "/priser",
}));

describe("kampanjemerket på prissiden", () => {
  it("lar cella hete modellnavnet og den norske merketeksten", () => {
    render(<PriserPage />);

    const badges = screen.getAllByText(/^Kampanjepris t\.o\.m\. /);
    expect(badges.map((b) => b.textContent)).toEqual([
      "Kampanjepris t.o.m. 31. desember 2026",
      "Kampanjepris t.o.m. 31. desember 2026",
      "Kampanjepris t.o.m. 31. desember 2026",
    ]);

    expect(badges[0].closest("td")).toHaveAccessibleName(
      "Gemini 3.6 Flash (Default) Kampanjepris t.o.m. 31. desember 2026"
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
  function pristabell() {
    return screen.getByRole("table");
  }

  function radFor(navn: string) {
    return within(pristabell()).getByText(navn).closest("tr")!;
  }

  it("viser OpenAI-radenes cache write, som fotnoten var eneste eksponering av", () => {
    render(<PriserPage />);

    // Kolonnen var gated på `provider === "Anthropic"`, så disse tallene falt ut
    // av sida selv om de lå i generert data. Sol-fotnoten oppga dem på hover.
    expect(within(radFor("GPT-5.6 Sol (Default, ≤ 272K)")).getByText("$5.00")).toBeInTheDocument();
    expect(within(radFor("GPT-5.6 Sol (Long context, 272K)")).getByText("$10.00")).toBeInTheDocument();
  });

  it("gir rader uten cache write en tankestrek, ikke en tom celle", () => {
    render(<PriserPage />);

    expect(within(radFor("GPT-5.4 (Default, ≤ 272K)")).getByText("—")).toBeInTheDocument();
  });

  it("viser leverandører og priser i én tabell med cache write-kolonne", () => {
    render(<PriserPage />);

    const table = pristabell();
    expect(screen.getAllByRole("table")).toHaveLength(1);
    expect(within(table).getByRole("columnheader", { name: "Leverandør" })).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Cache write" })).toBeInTheDocument();

    const geminiRow = radFor("Gemini 3.6 Flash (Default)");
    expect(within(geminiRow).getByText("Google")).toBeInTheDocument();
  });

  it("filtrerer rader på valgte verdier uten å skjule celler", () => {
    render(<PriserPage />);

    const table = pristabell();
    fireEvent.click(screen.getByText("Kategori: Alle"));
    const versatileToggle = screen.getByRole("checkbox", { name: "Versatile" });
    expect(within(table).getByRole("columnheader", { name: "Kategori" })).toBeInTheDocument();
    expect(radFor("Claude Sonnet 4")).toBeInTheDocument();
    expect(radFor("GPT-5.6 Sol (Default, ≤ 272K)")).toBeInTheDocument();

    fireEvent.click(versatileToggle);
    expect(within(screen.getByRole("table")).getByRole("columnheader", { name: "Kategori" })).toBeInTheDocument();
    expect(within(pristabell()).queryByText("Claude Sonnet 4")).toBeNull();
    expect(radFor("GPT-5.6 Sol (Default, ≤ 272K)")).toBeInTheDocument();

    fireEvent.click(versatileToggle);
    expect(radFor("Claude Sonnet 4")).toBeInTheDocument();
    expect(within(radFor("Claude Sonnet 4")).getByText("Versatile")).toBeInTheDocument();
  });

  it("filtrerer prisverdier med min- og maksgrenser", () => {
    render(<PriserPage />);

    const inputMinimum = screen.getByRole("spinbutton", { name: "Input min" });
    expect(screen.getByRole("spinbutton", { name: "Cache write min" })).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: "$0.13" })).toBeNull();
    fireEvent.change(inputMinimum, { target: { value: "5" } });

    expect(radFor("GPT-5.5 (Default, ≤ 272K)")).toBeInTheDocument();
    expect(within(pristabell()).queryByText("Claude Sonnet 4")).toBeNull();
    expect(screen.getByRole("columnheader", { name: "Input" })).toBeInTheDocument();
  });

  it("filtrerer cache write med et prisintervall og tar da med bare rader med pris", () => {
    render(<PriserPage />);

    fireEvent.change(screen.getByRole("spinbutton", { name: "Cache write min" }), { target: { value: "0.2" } });

    expect(radFor("GPT-5.6 Luna (Default, ≤ 200K)")).toBeInTheDocument();
    expect(within(pristabell()).queryByText("GPT-6 Luna (Default, ≤ 272K)")).toBeNull();
    expect(within(pristabell()).queryByText("GPT-5.5 (Default, ≤ 272K)")).toBeNull();
  });

  it("sorts cache write prices with missing prices last in either direction", () => {
    render(<PriserPage />);

    const cacheWriteSort = screen.getByRole("button", { name: /Sorter etter Cache write/ });
    fireEvent.click(cacheWriteSort);
    expect(within(pristabell()).getAllByRole("row").at(-1)).toHaveTextContent("Kimi K3");

    fireEvent.click(cacheWriteSort);
    expect(within(pristabell()).getAllByRole("row").at(-1)).toHaveTextContent("Kimi K3");
  });
});

describe("Nav-status og nav-pilots modellvalg", () => {
  function pristabell() {
    return screen.getByRole("table");
  }

  function radFor(navn: string) {
    return within(pristabell()).getByText(navn).closest("tr")!;
  }

  it("forklarer hvorfor GitHubs globale prisliste er større enn Navs modellutvalg", () => {
    render(<PriserPage />);

    expect(screen.getByRole("heading", { name: "Hvorfor tabellen viser flere modeller" })).toBeInTheDocument();
    expect(within(pristabell()).getByRole("columnheader", { name: /Nav-status/ })).toBeInTheDocument();
    expect(screen.getByText("Nav-status: Alle")).toBeInTheDocument();
    expect(screen.getByText("nav-pilot: Alle")).toBeInTheDocument();
  });

  it("merker og filtrerer modeller etter Navs modellpolicy", () => {
    render(<PriserPage />);

    expect(within(radFor("GPT-5.4 (Default, ≤ 272K)")).getByText("Ikke aktivert i Nav")).toBeInTheDocument();
    expect(within(radFor("GPT-6 Sol (Default, ≤ 272K)")).getByText("Aktivert i Nav")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Nav-status: Alle"));
    fireEvent.click(screen.getByRole("checkbox", { name: "Ikke aktivert i Nav" }));

    expect(radFor("GPT-6 Sol (Default, ≤ 272K)")).toBeInTheDocument();
    expect(within(pristabell()).queryByText("GPT-5.4 (Default, ≤ 272K)")).toBeNull();
  });

  it("viser hvilket formål nav-pilot foretrekker modellen til", () => {
    render(<PriserPage />);

    expect(within(radFor("GPT-6 Sol (Default, ≤ 272K)")).getByText("Daglig agentisk koding")).toBeInTheDocument();
    const reviewChoice = screen
      .getByRole("heading", {
        name: "Høyrisikoplanlegging og kodegjennomgang",
      })
      .closest("li")!;
    expect(within(reviewChoice).getByText("@nav-pilot-opus og @code-review")).toBeInTheDocument();
    expect(within(reviewChoice).getByText("Claude Opus 5.5")).toBeInTheDocument();
    expect(within(reviewChoice).getByText("Claude Opus 5")).toBeInTheDocument();
    expect(within(reviewChoice).getAllByText("Aktivert i Nav")).toHaveLength(2);
    expect(within(reviewChoice).getByText("Ikke aktivert i Nav")).toBeInTheDocument();
  });

  it("filtrerer modeller som brukes til flere nav-pilot-formål", () => {
    render(<PriserPage />);

    fireEvent.click(screen.getByText("nav-pilot: Alle"));
    for (const purpose of [
      "Research og faste maler",
      "Høyrisikoplanlegging og kodegjennomgang",
      "Aksel, tilgjengelighet og norsk tekst",
      "Rask Aksel-scaffolding",
      "Ikke brukt",
    ]) {
      fireEvent.click(screen.getByRole("checkbox", { name: purpose }));
    }

    expect(radFor("GPT-6 Sol (Default, ≤ 272K)")).toBeInTheDocument();
    expect(radFor("GPT-5.3-Codex (Default)")).toBeInTheDocument();
    expect(within(pristabell()).queryByText("Gemini 3.8 Flash (Default)")).toBeNull();
  });
});
