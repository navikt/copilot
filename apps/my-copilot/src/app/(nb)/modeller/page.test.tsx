import path from "node:path";
import { render, screen, within } from "@testing-library/react";
import { getAllCustomizations } from "@/lib/customizations";
import { NAV_PILOT_DEFAULT_MODEL, NAV_PILOT_MODEL_CHOICES } from "@/lib/model-policy";
import ModellerPage from "./page";

vi.mock("next/navigation", () => ({ usePathname: () => "/modeller" }));
// Chart.js needs a canvas; the table under each chart carries the same numbers.
vi.mock("./pass-rate-chart", () => ({
  PassRateChart: ({ title }: { title: string }) => <figure aria-label={title} />,
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
    expect(screen.getByRole("figure", { name: "Planlegging: andel bestått mot median credits" })).toBeInTheDocument();
    const row = screen.getByRole("link", { name: "Testdata A" }).closest("tr")!;
    expect(within(row).getByText("90 %")).toBeInTheDocument();
    expect(within(row).getByText("0.0.0-fixture")).toBeInTheDocument();
    expect(within(row).getByText("2000-01-01")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Testdata A" })).toHaveAttribute(
      "href",
      "https://github.com/navikt/copilot/blob/main/docs/golden-baselines/fixture-a.txt"
    );
    expect(screen.getByText(/ikke til å rangere modellene generelt/)).toBeInTheDocument();
  });

  it("lister agentene fra frontmatter-pinnene og lenker til Slack-kanalen", () => {
    summaryFile.path = FIXTURE;
    render(<ModellerPage />);
    const opus = screen.getByRole("cell", { name: "Høyrisikoplanlegging og kodegjennomgang" }).closest("tr")!;
    expect(within(opus).getByText("@code-review, @nav-pilot-opus")).toBeInTheDocument();
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
