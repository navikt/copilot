import { costEvents, eventAnnotations, eventSummary } from "./cost-events";

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
});
