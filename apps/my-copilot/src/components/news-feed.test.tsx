import { computeGridSpans } from "./news-feed";

describe("computeGridSpans (bento layout)", () => {
  it("packs each 3-col row to full width without gaps", () => {
    const spans = computeGridSpans(6, 3);
    // Sum every consecutive run that fills a row equals 3.
    let row = 0;
    for (const span of spans) {
      row += span;
      expect(row).toBeLessThanOrEqual(3);
      if (row === 3) row = 0;
    }
    expect(row).toBe(0);
  });

  it("alternates the wide cell side per row for a varied grid", () => {
    // [2,1] then [1,2] then [2,1] ...
    expect(computeGridSpans(6, 3)).toEqual([2, 1, 1, 2, 2, 1]);
  });

  it("is independent of item type / content mix", () => {
    // Same count always yields the same pattern regardless of feed mix.
    expect(computeGridSpans(5, 3)).toEqual(computeGridSpans(5, 3));
    expect(computeGridSpans(5, 3)).toEqual([2, 1, 1, 2, 3]);
  });

  it("never produces a long run of identical spans on desktop", () => {
    const spans = computeGridSpans(12, 3);
    let run = 1;
    for (let i = 1; i < spans.length; i++) {
      run = spans[i] === spans[i - 1] ? run + 1 : 1;
      expect(run).toBeLessThanOrEqual(2);
    }
  });

  it("degrades to a simple uniform grid in the compact 2-col variant", () => {
    // Wide span is clamped so it always leaves room for a neighbour.
    expect(computeGridSpans(4, 2)).toEqual([1, 1, 1, 1]);
  });

  it("handles an empty rest list", () => {
    expect(computeGridSpans(0, 3)).toEqual([]);
  });
});
