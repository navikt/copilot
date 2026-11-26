import reasons from "../../../fleet-control/internal/api/reasons.json";
import { REASON_TEXT } from "./fleet-approval";

describe("rule text", () => {
  it("lists every fleet-control reason, and only those", () => {
    expect(Object.keys(REASON_TEXT).sort()).toEqual([...reasons].sort());
  });
});
