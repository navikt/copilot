import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(async () => ({ name: "a" })), getUserToken: vi.fn(async () => "t") }));
vi.mock("@/lib/backend-api", async (orig) => ({
  ...(await orig<typeof import("@/lib/backend-api")>()),
  backendRequest: vi.fn(),
}));

import { BackendApiError, backendRequest } from "@/lib/backend-api";
import { POST } from "./route";

afterEach(() => vi.restoreAllMocks());

const req = (body: unknown, key: string | null = "k1") =>
  new Request("http://x/api/budget/raises", {
    method: "POST",
    headers: key ? { "Idempotency-Key": key } : {},
    body: JSON.stringify(body),
  });

describe("POST /api/budget/raises", () => {
  it("forwards only the known fields and the idempotency key, never an amount", async () => {
    vi.mocked(backendRequest).mockResolvedValue({ budgetUsd: 300 });
    const res = await POST(
      req({ category: "koding", expectedValue: "spare_tid", amount: 5000, githubLogin: "kari", freeText: " " })
    );
    expect(res.status).toBe(201);
    const [path, , init] = vi.mocked(backendRequest).mock.calls[0];
    expect(path).toBe("/api/v1/copilot/budget/raises");
    expect(init?.headers).toEqual({ "Idempotency-Key": "k1" });
    expect(JSON.parse(init?.body as string)).toEqual({ category: "koding", expectedValue: "spare_tid" });
  });

  it("refuses a request without an idempotency key", async () => {
    expect((await POST(req({}, null))).status).toBe(400);
    expect(backendRequest).not.toHaveBeenCalled();
  });

  it.each([
    [409, "cap_reached"],
    [409, "not_open"],
    [409, "own_budget"],
    [503, "budget_writes_unavailable"],
  ])("passes backend %i %s through", async (status, code) => {
    vi.mocked(backendRequest).mockRejectedValue(new BackendApiError(status, code));
    const res = await POST(req({ category: "koding", expectedValue: "spare_tid" }));
    expect(res.status).toBe(status);
    expect(await res.json()).toEqual({ error: code });
  });
});
