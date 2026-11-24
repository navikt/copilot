import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn(async () => new Headers({ Authorization: "Bearer user-token" })) }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn((to: string) => {
    throw new Error(`NEXT_REDIRECT ${to}`);
  }),
}));

import { FleetControlError, fleetApi, fleetLoad, fleetProblem } from "./fleet-control";

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });

describe("fleet-control client", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("is 404 without FLEET_CONTROL_URL", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "");
    await expect(fleetApi.whoami()).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("exchanges the user token for fleet-control's audience and sends it", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "http://fleet-control");
    vi.stubEnv("NAIS_CLUSTER_NAME", "dev-gcp");
    vi.stubEnv("NAIS_TOKEN_EXCHANGE_ENDPOINT", "http://texas/exchange");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");
    const fetch = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(async (url) =>
      url === "http://texas/exchange" ? json({ access_token: "obo" }) : json({ name: "A123456", teams: ["t"] })
    );
    vi.stubGlobal("fetch", fetch);

    await expect(fleetApi.whoami()).resolves.toEqual({ name: "A123456", teams: ["t"] });
    const exchange = JSON.parse(String(fetch.mock.calls[0]![1]!.body));
    expect(exchange).toMatchObject({
      target: "api://dev-gcp.copilot.fleet-control/.default",
      user_token: "user-token",
    });
    expect(fetch.mock.calls[1][0]).toBe("http://fleet-control/apis/fleet.nav.no/v1alpha1/whoami");
    expect((fetch.mock.calls[1]![1]!.headers as Record<string, string>).Authorization).toBe("Bearer obo");
  });

  it("escapes path segments", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    const fetch = vi.fn<(url: string) => Promise<Response>>(async () => json({ items: [] }));
    vi.stubGlobal("fetch", fetch);
    await fleetApi.history("mcpservers", "a/b", "c?d");
    expect(fetch.mock.calls[0]![0]).toBe(
      "http://fc/apis/fleet.nav.no/v1alpha1/namespaces/a%2Fb/mcpservers/c%3Fd/history"
    );
  });

  it("maps errors: 401 logs in, 404 is not found, 403 forbidden, the rest unavailable", async () => {
    expect(() => fleetProblem(new FleetControlError(401))).toThrow("NEXT_REDIRECT /oauth2/login");
    expect(() => fleetProblem(new FleetControlError(404))).toThrow("NEXT_NOT_FOUND");
    expect(fleetProblem(new FleetControlError(403))).toBe("forbidden");
    expect(fleetProblem(new FleetControlError(409))).toBe("conflict");
    expect(fleetProblem(new FleetControlError(503))).toBe("unavailable");
    expect(fleetProblem(new Error("timed out"))).toBe("unavailable");
  });

  it("turns a failed response into a problem", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => json({}, 503))
    );
    await expect(fleetLoad(fleetApi.whoami)).resolves.toEqual({ problem: "unavailable" });
  });

  it("reads reason from a metav1.Status body and maps Conflict", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    const status = { apiVersion: "v1", kind: "Status", status: "Failure", code: 409, reason: "Conflict", message: "m" };
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => json(status, 409))
    );
    const err = await fleetApi.whoami().catch((e: unknown) => e);
    expect(err).toMatchObject({ status: 409, reason: "Conflict" });
    expect(fleetProblem(err)).toBe("conflict");
  });

  it("never lets a Status reason lift a 403", () => {
    expect(fleetProblem(new FleetControlError(403, "Conflict"))).toBe("forbidden");
  });
});
