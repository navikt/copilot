import path from "node:path";
import { describe, expect, it } from "vitest";
import { knownCredits, modelName, passRate, runsBySuite, sourceUrl } from "./golden-baselines";
import { loadGoldenSummary } from "./golden-summary-file";

const FIXTURE = path.join(__dirname, "__fixtures__", "golden-summary.json");

describe("golden-baselines", () => {
  it("leser fixturen og grupperer per suite", () => {
    const summary = loadGoldenSummary(FIXTURE)!;
    expect(runsBySuite(summary.runs).map(([suite, runs]) => [suite, runs.length])).toEqual([
      ["planning", 4],
      ["review", 3],
      ["norsk", 1],
    ]);
  });

  it("gir null, ikke tall, når filen mangler, er tom eller ikke er JSON", () => {
    expect(loadGoldenSummary(path.join(__dirname, "finnes-ikke.json"))).toBeNull();
    expect(loadGoldenSummary(__filename)).toBeNull();
  });

  const run = (letter: string) =>
    loadGoldenSummary(FIXTURE)!.runs.find((r) => r.source.endsWith(`fixture-${letter}.txt`))!;

  it("regner bestått som andel av n × krav", () => {
    expect(passRate(run("a"))).toBe(9 / 10);
    expect(passRate(run("d"))).toBe(1);
    expect(passRate(run("e"))).toBe(0);
  });

  it("viser ikke credits når forbruket mangler, er ufullstendig eller modellen er ubekreftet", () => {
    expect(knownCredits(run("a"))).toBe(120.5);
    expect(knownCredits(run("b"))).toBe(70);
    expect(knownCredits(run("c"))).toBeNull(); // usage_complete: false
    expect(knownCredits(run("f"))).toBeNull(); // credits: null
    expect(knownCredits(run("h"))).toBeNull(); // model_verified: false, credits present
  });

  it("slår opp visningsnavn i priskatalogen og faller tilbake til id-en", () => {
    expect(modelName("gpt-6-sol")).toBe("GPT-6 Sol");
    expect(modelName("claude-opus-5.5")).toBe("Claude Opus 5.5");
    expect(modelName("gpt-5.3-codex")).toBe("GPT-5.3-Codex");
    expect(modelName("fixture-unknown-model")).toBe("fixture-unknown-model");
  });

  it("lenker repo-stier til GitHub og lar fulle URL-er stå", () => {
    expect(sourceUrl("docs/golden-baselines/x.txt")).toBe(
      "https://github.com/navikt/copilot/blob/main/docs/golden-baselines/x.txt"
    );
    expect(sourceUrl("https://example.org/x")).toBe("https://example.org/x");
  });
});
