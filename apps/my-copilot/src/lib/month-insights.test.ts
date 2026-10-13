import { costFactors, monthInsights, runOutDate, type MonthInsightData } from "./month-insights";
import fixture from "./month-insights.fixture.json";

// Aggregates from copilot-prod on 12 October 2026 (read-only bq): billed months August and September,
// October to the 11th. Every USD amount is scaled by 0.4 and seats are removed, so the repo shows no
// real spend; shares, ratios, dates and user counts are as measured. The GitHub budget is not in
// BigQuery; the budget case sets one.
const real = fixture as unknown as MonthInsightData;
// Plain spaces in place of the non-breaking ones from Intl, so the expected text reads normally.
const plain = (s: string) => s.replace(/[\u00a0\u202f]/g, " ");
const byId = (data: MonthInsightData) =>
  Object.fromEntries(
    monthInsights(data).map((i) => [i.id, { ...i, text: plain(i.text), details: i.details?.map(plain) }])
  );

describe("monthInsights on October 2026", () => {
  const got = byId({ ...real, anomalies: [] });

  it("M1: run-out date against September and the forecast", () => {
    expect(runOutDate(real.prevDaily)).toBe("2026-09-08");
    expect(got.M1.text).toBe(
      "Kredittene i lisensene gikk tom 7. oktober, 1 dag tidligere enn i september; forventet faktura er 23 023 USD."
    );
    expect(got.M1.href).toBe("#prognose");
  });

  it("M2 stays hidden: nobody is at 90 % on 11 October (bq: max 69 % of an 800 USD limit)", () => {
    expect(got.M2).toBeUndefined();
  });

  it("M3: September against August, decomposed", () => {
    expect(got.M3.text).toBe(
      "Det fakturerte beløpet endret seg +100 % fra august til september: fakturerte personer +8 %, forbruk per person til listepris +36 %, andel over kredittene i lisensene +37 %."
    );
    const [aug, sep] = real.trend;
    const f = costFactors(aug, sep, 637, 685);
    expect(f.users * f.perUser * f.rest).toBeCloseTo(sep.total_net_amount / aug.total_net_amount, 10);
  });

  it("M4: GPT gained share and costs more per user than the median", () => {
    expect(got.M4.text).toBe(
      "GPT fikk +17,4 prosentpoeng større andel av det fakturerte beløpet i september enn i august, og koster mer per person enn medianen."
    );
    expect(got.M4.details).toEqual(["GPT: 17 USD per person i september, median for modellfamiliene 10 USD."]);
    expect(got.M3.details!.join(" ")).toContain("modellvalget");
  });

  it("M5 stays quiet; M6 shows the rise in cost with no team", () => {
    expect(got.M5).toBeUndefined();
    expect(got.M6.text).toBe(
      "Personer uten team sto for 11 % av det fakturerte beløpet i september, mot 7,5 % i august."
    );
  });
});

describe("monthInsights thresholds", () => {
  it("M1 gives the budget share, flags over budget and folds in the daily spikes", () => {
    const models = real.forecast!.models.map((m) => ({ ...m, budget_pct: 115, over_budget: true }));
    const { M1 } = byId({
      ...real,
      forecast: { ...real.forecast!, budget_status: "set", budget_usd: 50000, models },
      anomalies: [{ rule: "A1", date: "2026-10-09", value: 2400, threshold: 1100 }],
    });
    expect(M1.text).toContain("forventet faktura er 115 % av budsjettet");
    expect(M1.details).toContain("Forventet faktura er over budsjettet i GitHub.");
    expect(M1.details!.some((d) => d.includes("Uvanlig høyt forbruk"))).toBe(true);
    expect(M1.action).toMatch(/personlige grensen/);
  });

  it("M4 fires for a family that gained more than 10 points and costs more per user than the median", () => {
    const breakdown = [
      ...["2026-08", "2026-09"].flatMap((m) => [
        { year_month: m, model: "GPT-5.4 mini", gross_amount: 0, net_amount: 1000, pct_of_monthly_net: 0 },
        { year_month: m, model: "Claude Sonnet 5", gross_amount: 0, net_amount: 1000, pct_of_monthly_net: 0 },
      ]),
      { year_month: "2026-08", model: "Claude Opus 5", gross_amount: 0, net_amount: 500, pct_of_monthly_net: 0 },
      { year_month: "2026-09", model: "Claude Opus 5", gross_amount: 0, net_amount: 3000, pct_of_monthly_net: 0 },
      { year_month: "2026-09", model: "Gemini 3", gross_amount: 0, net_amount: 3000, pct_of_monthly_net: 0 },
    ];
    const familyUsers = [
      { month: "2026-09", family: "gpt_mini", users: 100 },
      { month: "2026-09", family: "claude_sonnet", users: 100 },
      { month: "2026-09", family: "claude_opus", users: 50 },
      { month: "2026-09", family: "gemini", users: 5 },
    ] as MonthInsightData["familyUsers"];
    const { M4 } = byId({ ...real, breakdown, familyUsers });
    // Gemini is new in September: never an alert on its own.
    expect(M4.text).toMatch(/^Claude Opus fikk \+\d/);
    expect(M4.details).toHaveLength(1);
  });

  it("M5 and M6 appear past their thresholds and not before", () => {
    const high = real.high!.map((h) =>
      h.month === "2026-09"
        ? { ...h, tiers: h.tiers.map((t) => (t.top_pct === 5 ? { ...t, net_share_pct: 36 } : t)) }
        : h
    );
    expect(byId({ ...real, high }).M5.text).toContain("+5,4 prosentpoeng");
    const months = real.months.map((m) => (m.month === "2026-09" ? { ...m, no_team_net_usd: 1200 } : m));
    expect(byId({ ...real, months }).M6).toBeUndefined();
  });

  it("M7 keeps the data-gap text and comes first", () => {
    const list = monthInsights({ ...real, anomalies: [{ rule: "A7", date: "2026-09", value: 0, threshold: 5 }] });
    expect(list[0].id).toBe("M7");
    expect(list[0].text).toContain("Fakturert forbruk for 2026-09 er ikke lest inn");
  });

  it("M2 shows a rise of more than one point at 90 %", () => {
    const band = (first: number, last: number, users: number) => ({ first, last, label: "", users });
    const budgetDays = [
      { day: "2026-09-11", users: 600, snapshots: 0, bands: [band(0, 4, 590), band(5, 6, 10)] },
      {
        day: "2026-10-11",
        users: 600,
        snapshots: 0,
        bands: [band(0, 2, 560), band(3, 3, 10), band(4, 4, 10), band(5, 6, 20)],
      },
    ];
    const { M2 } = byId({ ...real, budgetDays });
    expect(M2.text).toBe(
      "20 personer (3,3 %) har brukt minst 90 % av grensen 11. oktober, mot 10 (1,7 %) 11. september; ≈40 når grensen før månedsslutt (grovt anslag)."
    );
  });

  it("M3 stays quiet under 15 %", () => {
    const trend = real.trend.map((t) => (t.year_month.startsWith("2026-09") ? { ...t, total_net_amount: 14000 } : t));
    expect(byId({ ...real, trend }).M3).toBeUndefined();
  });
});
