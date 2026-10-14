import { describe, expect, it, vi } from "vitest";

vi.mock("next/server", () => ({ connection: async () => {} }));

import { GET } from "./route";

describe("GET /metrics", () => {
  it("serves metrics to a direct scrape", async () => {
    const res = await GET(new Request("http://10.0.0.1:3000/metrics"));
    expect(res.status).toBe(200);
    expect(await res.text()).toContain("my_copilot_request_errors_total");
  });

  it("hides metrics from requests that came through the ingress", async () => {
    const res = await GET(
      new Request("https://ki-utvikling.nav.no/metrics", { headers: { authorization: "Bearer token-from-wonderwall" } })
    );
    expect(res.status).toBe(404);
  });
});
