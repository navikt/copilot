import { costEvents, eventAnnotations, eventEffects, eventMonths, eventSummary } from "./cost-events";
import fixture from "./month-insights.fixture.json";
import type { MonthInsightData } from "./month-insights";

const real = fixture as unknown as MonthInsightData;
const plain = (s: string) => s.replace(/[\u00a0\u202f]/g, " ");

describe("costEvents", () => {
  const events = costEvents({
    limit_changes: [
      { date: "2026-06-01", usd: 200 },
      { date: "2026-07-01", usd: 400 },
      { date: "2026-09-22", usd: 800 },
    ],
    allowance_run_out_date: "2026-10-09",
    allowance_used_up: true,
  });

  it("marks the switch, later limit changes and the run-out day", () => {
    expect(events.map((e) => [e.date, e.text])).toEqual([
      ["2026-06-01", "KI-kreditter"],
      ["2026-07-01", "Grense 400"],
      ["2026-09-22", "Grense 800"],
      ["2026-10-09", "Kreditter brukt opp"],
    ]);
    expect(events[1]).toMatchObject({ fromUsd: 200, toUsd: 400 });
    expect(events[2]).toMatchObject({ fromUsd: 400, toUsd: 800 });
    expect(events[3].label).toContain("ble brukt opp");
  });

  it("only marks the switch without a forecast", () => {
    expect(costEvents(null).map((e) => e.text)).toEqual(["KI-kreditter"]);
  });

  it("groups events per month and skips months the chart does not show", () => {
    const markers = eventAnnotations(events, ["2026-06", "2026-07", "2026-08", "2026-09"]);
    expect(markers.map((m) => [m.month, m.text])).toEqual([
      ["2026-06", "KI-kreditter"],
      ["2026-07", "Grense 400"],
      ["2026-09", "Grense 800"],
    ]);
    expect(eventSummary(markers)).toMatch(/^ Markert i grafen: 1\. juni 2026: overgang til KI-kreditter; /);
  });

  it("matches days too", () => {
    expect(eventAnnotations(events, ["2026-10-08", "2026-10-09"]).map((m) => m.text)).toEqual(["Kreditter brukt opp"]);
  });

  it("computes before/after effect for 22 September 2026 limit change (#1777)", () => {
    expect(eventMonths(events)).toEqual(["2026-06", "2026-07", "2026-09", "2026-10"]);
    const band = (first: number, last: number, users: number) => ({ first, last, label: "", users });
    const budgetDays = [
      { day: "2026-09-21", users: 685, snapshots: 0, bands: [band(0, 4, 657), band(5, 6, 28)] },
      { day: "2026-09-30", users: 685, snapshots: 0, bands: [band(0, 4, 678), band(5, 6, 7)] },
    ];
    const effects = eventEffects({
      events,
      daily: real.prevDaily,
      active: real.active,
      budgetDays,
      forecast: real.forecast,
    });
    expect(effects).toHaveLength(1);
    const sep22 = effects[0];
    expect(sep22.event.date).toBe("2026-09-22");
    expect(plain(sep22.summary)).toBe(
      "Etter at grensen gikk fra 400 til 800 USD 22. sep.: kostnad per person +22 %, andel nær grensen fra 4,1 % til 1,0 %."
    );
    expect(sep22.costPerUser?.beforeUsd).toBeCloseTo(20.94, 2);
    expect(sep22.costPerUser?.afterUsd).toBeCloseTo(25.46, 2);
    expect(sep22.runOut).toEqual({
      beforeMonth: "2026-09",
      beforeDate: "2026-09-08",
      afterMonth: "2026-10",
      afterDate: "2026-10-07",
    });
  });

  it("computes weekday-adjusted model mix shift when daily rows include model names", () => {
    const daily = real.prevDaily.flatMap((r) => {
      const after = r.day >= "2026-09-22";
      return [
        { ...r, model: "Claude Sonnet 5", gross_amount: r.gross_amount * (after ? 0.4 : 0.7) },
        { ...r, model: "GPT-6 Sol", gross_amount: r.gross_amount * (after ? 0.6 : 0.3) },
      ];
    });
    const [sep22] = eventEffects({
      events,
      daily,
      active: real.active,
      forecast: real.forecast,
    });
    expect(sep22.modelMix.map((m) => [m.family, Math.round(m.beforePct), Math.round(m.afterPct)])).toEqual([
      ["claude_sonnet", 70, 40],
      ["gpt", 30, 60],
    ]);
  });
});
