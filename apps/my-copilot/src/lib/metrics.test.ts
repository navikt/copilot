import { describe, expect, it } from "vitest";
import { recordRequestError, register } from "./metrics";

describe("recordRequestError", () => {
  it("counts errors per route and route type", async () => {
    recordRequestError("/api/survey/[id]/responses", "route");
    recordRequestError("/api/survey/[id]/responses", "route");
    recordRequestError("/admin/undersokelser/[id]/fritekst", "action");
    const out = await register.metrics();
    expect(out).toContain('my_copilot_request_errors_total{route="/api/survey/[id]/responses",route_type="route"} 2');
    expect(out).toContain(
      'my_copilot_request_errors_total{route="/admin/undersokelser/[id]/fritekst",route_type="action"} 1'
    );
  });
});
