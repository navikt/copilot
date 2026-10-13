import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(async () => ({ name: "a" })), getUserToken: vi.fn(async () => "t") }));
vi.mock("@/lib/backend-api", async (orig) => ({
  ...(await orig<typeof import("@/lib/backend-api")>()),
  backendRequest: vi.fn(),
}));

import { BackendApiError, backendRequest } from "@/lib/backend-api";
import { GET } from "./route";

afterEach(() => vi.restoreAllMocks());

describe("GET /api/budget", () => {
  it.each([
    [403, "no_github_account"],
    [404, "no_budget"],
    [503, "unavailable"],
  ])("maps backend %i to %s without console.error", async (status, error) => {
    vi.mocked(backendRequest).mockRejectedValue(new BackendApiError(status));
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const res = await GET();
    expect(res.status).toBe(status);
    expect(await res.json()).toEqual({ error });
    expect(err).not.toHaveBeenCalled();
  });

  it("logs and returns 500 for other failures", async () => {
    vi.mocked(backendRequest).mockRejectedValue(new BackendApiError(502));
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    expect((await GET()).status).toBe(500);
    expect(err).toHaveBeenCalled();
  });
});
