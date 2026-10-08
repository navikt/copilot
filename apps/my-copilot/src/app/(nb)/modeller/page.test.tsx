import path from "node:path";
import { render, screen, within } from "@testing-library/react";
import { getAllCustomizations } from "@/lib/customizations";
import { NAV_PILOT_DEFAULT_MODEL, NAV_PILOT_MODEL_CHOICES } from "@/lib/model-policy";
import ModellerPage from "./page";

vi.mock("next/navigation", () => ({ usePathname: () => "/modeller" }));
// Chart.js needs a canvas; the table under each chart carries the same numbers.
vi.mock("./pass-rate-chart", () => ({
  PassRateChart: ({ title, points }: { title: string; points: { model: string; effort: string }[] }) => (
    <figure aria-label={title} data-points={points.map((p) => `${p.model}/${p.effort}`).join(",")} />
  ),
}));

const summaryFile = vi.hoisted(() => ({ path: "" }));
vi.mock("@/lib/golden-summary-file", async (original) => {
  const actual = await original<typeof import("@/lib/golden-summary-file")>();
  return { ...actual, loadGoldenSummary: () => actual.loadGoldenSummary(summaryFile.path) };
});

const FIXTURE = path.join(__dirname, "..", "..", "..", "lib", "__fixtures__", "golden-summary.json");

describe("modellsiden", () => {
  it("viser en ærlig tom tilstand uten målinger", () => {
    summaryFile.path = path.join(__dirname, "finnes-ikke.json");
    render(<ModellerPage />);
    expect(screen.getByText(/Ingen målinger publisert ennå/)).toBeInTheDocument();
    expect(screen.queryByRole("figure")).toBeNull();
  });

  it("viser målingene per suite med n, dato, CLI-versjon og kilde", () => {
    summaryFile.path = FIXTURE;
    render(<ModellerPage />);
    const row = screen.getByRole("link", { name: "fixture-a.txt" }).closest("tr")!;
    expect(within(row).getByText("GPT-6 Sol")).toBeInTheDocument();
    expect(within(row).getByText("90 %")).toBeInTheDocument();
    expect(within(row).getByText("120,5")).toBeInTheDocument();
    expect(within(row).getByText("0.0.0-fixture")).toBeInTheDocument();
    expect(within(row).getByText("1. januar 2000")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "fixture-a.txt" })).toHaveAttribute(
      "href",
      "https://github.com/navikt/copilot/blob/main/docs/golden-baselines/fixture-a.txt"
    );
    expect(screen.getByText(/Få kjøringer holder til å finne tydelige feil/)).toBeInTheDocument();
  });

  it("viser ufullstendig forbruk som –, holder det utenfor diagrammet og merker benchmarker med én kjøring", () => {
    summaryFile.path = FIXTURE;
    render(<ModellerPage />);
    const rowFor = (file: string) => screen.getByRole("link", { name: file }).closest("tr")!;

    // usage_complete: false and credits: null are both unknown, not a number.
    expect(within(rowFor("fixture-c.txt")).getByText("–")).toBeInTheDocument();
    expect(within(rowFor("fixture-f.txt")).getByText("–")).toBeInTheDocument();
    expect(screen.getByRole("figure", { name: "Planlegging: andel bestått mot median credits" })).toHaveAttribute(
      "data-points",
      "GPT-6 Sol/high,GPT-6 Sol/medium"
    );
    expect(screen.getAllByText(/Strek \(–\) betyr at forbruket ikke ble registrert for alle kall/)).toHaveLength(2);

    expect(within(rowFor("fixture-e.txt")).getByText("GPT-6 Luna (benchmark)")).toBeInTheDocument();
    expect(within(rowFor("fixture-f.txt")).getByText("fixture-unknown-model")).toBeInTheDocument();
    // ran_at is shown only when it differs from the requested effort.
    expect(within(rowFor("fixture-b.txt")).getByText("medium (kjørte på high)")).toBeInTheDocument();
    expect(within(rowFor("fixture-c.txt")).getByText("high")).toBeInTheDocument();
  });

  it("merker ubekreftet modell, holder den utenfor diagrammet og viser subagentenes modeller", () => {
    summaryFile.path = FIXTURE;
    render(<ModellerPage />);
    const rowFor = (file: string) => screen.getByRole("link", { name: file }).closest("tr")!;

    // fixture-h has credits; only model_verified: false keeps them off the table and the chart.
    const unverified = rowFor("fixture-h.txt");
    expect(within(unverified).getByText("GPT-5.6 Sol (ikke bekreftet)")).toBeInTheDocument();
    expect(within(unverified).getByText("–")).toBeInTheDocument();
    expect(
      screen.getByRole("figure", { name: "Planlegging: andel bestått mot median credits" }).getAttribute("data-points")
    ).not.toContain("GPT-5.6 Sol");
    expect(screen.getAllByText(/vi vet ikke sikkert hvilken modell som svarte/)).toHaveLength(1);

    expect(
      within(rowFor("fixture-a.txt")).getByText("Subagenter brukte også: GPT-6 Luna, fixture-other-model")
    ).toBeInTheDocument();
  });

  it("holder benchmarker med én kjøring utenfor diagrammene og lister dem etter de ekte kjøringene", () => {
    summaryFile.path = FIXTURE;
    render(<ModellerPage />);

    // The smoke run on GPT-6 Luna has known credits, so only the smoke flag keeps it off the chart.
    expect(screen.getByRole("figure", { name: "Kodegjennomgang: andel bestått mot median credits" })).toHaveAttribute(
      "data-points",
      "Claude Opus 5.5/low"
    );
    const reviewTable = screen.getByRole("link", { name: "fixture-d.txt" }).closest("table")!;
    expect(
      within(reviewTable)
        .getAllByRole("link")
        .map((link) => link.textContent)
    ).toEqual(["fixture-d.txt", "fixture-f.txt", "fixture-e.txt"]);

    // A suite with only smoke runs gets its table, no chart.
    expect(screen.getByRole("heading", { name: "Norsk tekst" })).toBeInTheDocument();
    expect(screen.getByText("Claude Sonnet 5.5 (benchmark)")).toBeInTheDocument();
    expect(screen.queryByRole("figure", { name: /^Norsk tekst/ })).toBeNull();

    // Every suite that shows a smoke run explains what a smoke run is.
    expect(screen.getAllByText(/En benchmark med én kjøring sjekker at testoppsettet virker/)).toHaveLength(2);
  });

  it("viser research som egen suite med norske datoer", () => {
    summaryFile.path = FIXTURE;
    render(<ModellerPage />);
    expect(screen.getByRole("heading", { name: "Research" })).toBeInTheDocument();
    expect(screen.getByRole("figure", { name: "Research: andel bestått mot median credits" })).toHaveAttribute(
      "data-points",
      "GPT-6 Luna/medium"
    );
    const row = screen.getByRole("link", { name: "fixture-i.txt" }).closest("tr")!;
    expect(within(row).getByText("2. januar 2000")).toBeInTheDocument();
    expect(screen.getByText(/Sist oppdatert 1\. januar 2000/)).toBeInTheDocument();
  });

  it("lister agentene fra frontmatter-pinnene og lenker til Slack-kanalen", () => {
    summaryFile.path = FIXTURE;
    render(<ModellerPage />);
    const opus = screen.getByRole("cell", { name: "Høyrisikoplanlegging og kodegjennomgang" }).closest("tr")!;
    expect(within(opus).getByText("@code-review, @nav-pilot-opus, @security-champion")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "#github-copilot på Slack" })).toHaveAttribute(
      "href",
      "https://nav-it.slack.com/archives/C055TNXBM17"
    );
  });
});

it("har et modellvalg for hver modell en agent eller prompt er pinnet til", () => {
  const primaries = NAV_PILOT_MODEL_CHOICES.map((choice) => choice.primary);
  const pins = getAllCustomizations()
    .filter((item) => item.type === "agent" || item.type === "prompt")
    .flatMap((item) => item.model ?? []);
  expect(pins.length).toBeGreaterThan(0);
  expect([...new Set(pins)].filter((pin) => !primaries.includes(pin))).toEqual([]);
  expect(primaries).toContain(NAV_PILOT_DEFAULT_MODEL);
});
