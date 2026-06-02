import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({
  getUser: vi.fn(),
  getUserToken: vi.fn(),
}));

// Keep the real BackendApiError class so `instanceof` checks in the route work,
// but stub out the network-calling backendRequest.
vi.mock("@/lib/backend-api", async (importActual) => {
  const actual = await importActual<typeof import("@/lib/backend-api")>();
  return { ...actual, backendRequest: vi.fn() };
});

vi.mock("@/lib/logger", () => ({
  getLoggerWithTraceContext: () => ({ info: vi.fn(), warn: vi.fn(), error: vi.fn() }),
  getTraceId: () => "test-trace-id",
}));

vi.mock("@opentelemetry/api", () => ({
  context: { active: () => ({}) },
}));

import { getUser, getUserToken } from "@/lib/auth";
import { BackendApiError, backendRequest } from "@/lib/backend-api";
import type { User } from "@/lib/auth";
import { GET, POST } from "./route";

const mockedGetUser = vi.mocked(getUser);
const mockedGetUserToken = vi.mocked(getUserToken);
const mockedBackendRequest = vi.mocked(backendRequest);

function makeUser(overrides: Partial<User> = {}): User {
  return {
    firstName: "Test",
    lastName: "User",
    email: "test.user@nav.no",
    groups: ["allowed-group"],
    ...overrides,
  };
}

function postRequest(body: unknown): Request {
  return new Request("http://localhost/api/copilot", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("GET /api/copilot", () => {
  it("returns 401 when the user is not authenticated", async () => {
    mockedGetUser.mockResolvedValue(null);
    mockedGetUserToken.mockResolvedValue(null);

    const response = await GET();

    expect(response.status).toBe(401);
    expect(mockedBackendRequest).not.toHaveBeenCalled();
  });

  it("returns 500 when the authenticated user has no email", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ email: "" }));
    mockedGetUserToken.mockResolvedValue("token");

    const response = await GET();

    expect(response.status).toBe(500);
    expect(mockedBackendRequest).not.toHaveBeenCalled();
  });

  it("reports eligibility=true for a member of an allowed group", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");
    // SAML lookup: GitHub account not linked.
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: null });

    const response = await GET();
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.githubAccountLinked).toBe(false);
    expect(data.icanhazcopilot).toBe(true);
  });

  it("reports eligibility=false for a user in no allowed group", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: [] }));
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: "octocat" });
    // Seat lookup returns an active seat.
    mockedBackendRequest.mockResolvedValueOnce({ assignee: "octocat" });

    const response = await GET();
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.githubAccountLinked).toBe(true);
    expect(data.icanhazcopilot).toBe(false);
    expect(data.subscription).toEqual({ assignee: "octocat" });
    expect(data.githubUsername).toBe("octocat");
  });

  it("treats a 404 from the seat endpoint as 'no subscription' rather than an error", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: "octocat" });
    mockedBackendRequest.mockRejectedValueOnce(new BackendApiError(404));

    const response = await GET();
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.githubAccountLinked).toBe(true);
    expect(data.subscription).toBeNull();
    expect(data.icanhazcopilot).toBe(true);
  });

  it("returns 500 when a non-404 backend error occurs", async () => {
    mockedGetUser.mockResolvedValue(makeUser());
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: "octocat" });
    mockedBackendRequest.mockRejectedValueOnce(new BackendApiError(500));

    const response = await GET();

    expect(response.status).toBe(500);
  });
});

describe("POST /api/copilot self-service authorization", () => {
  it("returns 401 when the user is not authenticated", async () => {
    mockedGetUser.mockResolvedValue(null);
    mockedGetUserToken.mockResolvedValue(null);

    const response = await POST(postRequest({ action: "activate" }));

    expect(response.status).toBe(401);
    expect(mockedBackendRequest).not.toHaveBeenCalled();
  });

  it("returns 500 when the authenticated user has no email", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ email: "" }));
    mockedGetUserToken.mockResolvedValue("token");

    const response = await POST(postRequest({ action: "activate" }));

    expect(response.status).toBe(500);
    expect(mockedBackendRequest).not.toHaveBeenCalled();
  });

  it("forbids self-service for a user that is not in any allowed group (403)", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: [] }));
    mockedGetUserToken.mockResolvedValue("token");

    const response = await POST(postRequest({ action: "activate" }));
    const data = await response.json();

    expect(response.status).toBe(403);
    expect(data.error).toMatch(/not a member of any groups/i);
    // No license is created and the backend is never touched for an unauthorized user.
    expect(mockedBackendRequest).not.toHaveBeenCalled();
  });

  it("returns 400 when an allowed user omits the action", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");

    const response = await POST(postRequest({}));

    expect(response.status).toBe(400);
    expect(mockedBackendRequest).not.toHaveBeenCalled();
  });

  it("allows an allowed-group member to activate a license (201)", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: "octocat" });
    mockedBackendRequest.mockResolvedValueOnce({ seats_created: 1 });

    const response = await POST(postRequest({ action: "activate" }));
    const data = await response.json();

    expect(response.status).toBe(201);
    expect(data.seats_created).toBe(1);
    expect(mockedBackendRequest).toHaveBeenCalledWith(
      "/api/v1/copilot/seats",
      "token",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ username: "octocat" }) })
    );
  });

  it("allows an allowed-group member to deactivate a license (200)", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: "octocat" });
    mockedBackendRequest.mockResolvedValueOnce({ seats_cancelled: 1 });

    const response = await POST(postRequest({ action: "deactivate" }));
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.seats_cancelled).toBe(1);
    expect(mockedBackendRequest).toHaveBeenCalledWith(
      "/api/v1/copilot/seats/octocat",
      "token",
      expect.objectContaining({ method: "DELETE" })
    );
  });

  it("returns 400 when the allowed user has no linked GitHub account", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: null });

    const response = await POST(postRequest({ action: "activate" }));

    expect(response.status).toBe(400);
    // Only the SAML lookup ran; no seat was created.
    expect(mockedBackendRequest).toHaveBeenCalledTimes(1);
  });

  it("returns 400 for an unknown action", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: "octocat" });

    const response = await POST(postRequest({ action: "delete-everything" }));
    const data = await response.json();

    expect(response.status).toBe(400);
    expect(data.error).toMatch(/unknown action/i);
  });

  it("returns 500 when the backend fails during activation", async () => {
    mockedGetUser.mockResolvedValue(makeUser({ groups: ["allowed-group"] }));
    mockedGetUserToken.mockResolvedValue("token");
    mockedBackendRequest.mockResolvedValueOnce({ identity: "test.user@nav.no", username: "octocat" });
    mockedBackendRequest.mockRejectedValueOnce(new BackendApiError(502));

    const response = await POST(postRequest({ action: "activate" }));

    expect(response.status).toBe(500);
  });
});
