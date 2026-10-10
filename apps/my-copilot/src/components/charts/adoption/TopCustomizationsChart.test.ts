import { describe, expect, it } from "vitest";
import { originDatasets } from "./TopCustomizationsChart";

describe("originDatasets", () => {
  it("puts each file in its origin's series and leaves the other null", () => {
    const items = [
      { category: "agents", file_name: "nais.agent.md", repo_count: 9, active_repo_count: 4 },
      { category: "agents", file_name: "mine.agent.md", repo_count: 3, active_repo_count: 1 },
    ];
    const [official, own] = originDatasets(items, new Set(["nais.agent.md"]));
    expect(official.data).toEqual([9, null]);
    expect(own.data).toEqual([null, 3]);
    expect([official.label, own.label]).toEqual(["Fra navikt/copilot", "Egne"]);
  });
});
