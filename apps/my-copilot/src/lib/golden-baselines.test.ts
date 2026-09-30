import path from "node:path";
import { describe, expect, it } from "vitest";
import { passRate, runsBySuite, sourceUrl } from "./golden-baselines";
import { loadGoldenSummary } from "./golden-summary-file";

const FIXTURE = path.join(__dirname, "__fixtures__", "golden-summary.json");

describe("golden-baselines", () => {
  it("leser fixturen og grupperer per suite", () => {
    const summary = loadGoldenSummary(FIXTURE)!;
    expect(runsBySuite(summary.runs).map(([suite, runs]) => [suite, runs.length])).toEqual([
      ["planning", 3],
      ["review", 2],
    ]);
  });

  it("gir null, ikke tall, når filen mangler, er tom eller ikke er JSON", () => {
    expect(loadGoldenSummary(path.join(__dirname, "finnes-ikke.json"))).toBeNull();
    expect(loadGoldenSummary(__filename)).toBeNull();
  });

  it("regner bestått som andel av n × krav, og godtar boolsk passed", () => {
    const [a, , , d, e] = loadGoldenSummary(FIXTURE)!.runs;
    expect(passRate(a)).toBe(9 / 10);
    expect(passRate(d)).toBe(1);
    expect(passRate(e)).toBe(2 / 5);
  });

  it("lenker repo-stier til GitHub og lar fulle URL-er stå", () => {
    expect(sourceUrl("docs/golden-baselines/x.txt")).toBe(
      "https://github.com/navikt/copilot/blob/main/docs/golden-baselines/x.txt"
    );
    expect(sourceUrl("https://example.org/x")).toBe("https://example.org/x");
  });
});
