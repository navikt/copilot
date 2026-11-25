import { describe, expect, it } from "vitest";
import { fleetGrafanaUrl } from "./fleet-grafana";

describe("fleetGrafanaUrl", () => {
  it("encodes the route and picks prod for a prod cluster", () => {
    expect(fleetGrafanaUrl("/mcp/team/navn", "prod-gcp")).toBe(
      "https://grafana.nav.cloud.nais.io/d/copilot-fleet-insights?orgId=1&var-route=%2Fmcp%2Fteam%2Fnavn&var-env=prod"
    );
  });

  it("picks dev for a dev cluster and for local dev", () => {
    expect(fleetGrafanaUrl("/mcp/a/b", "dev-gcp")).toMatch(/&var-env=dev$/);
    expect(fleetGrafanaUrl("/mcp/a/b", "")).toMatch(/&var-env=dev$/);
  });
});
