import { paretoFrontier, spreadLabels } from "./pass-rate-chart";

describe("spreadLabels", () => {
  it("skyver etiketter fra hverandre uten å bytte rekkefølge", () => {
    expect(spreadLabels([50, 10, 52, 100], 13)).toEqual([50, 10, 63, 100]);
  });
});

describe("paretoFrontier", () => {
  it("beholder bare punkter ingen andre slår på både pris og bestått", () => {
    const p = (credits: number, passPercent: number) => ({ credits, passPercent });
    const points = [p(154, 100), p(1.6, 40), p(20, 40), p(10, 80), p(10, 60), p(200, 90)];
    expect(paretoFrontier(points)).toEqual([p(1.6, 40), p(10, 80), p(154, 100)]);
  });
});
