import { vi } from "vitest";

// Mock next/headers before importing auth
vi.mock("next/headers", () => ({
  headers: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(() => {
    throw new Error("NEXT_REDIRECT");
  }),
}));

const fetchMock = vi.fn<typeof globalThis.fetch>();
vi.stubGlobal("fetch", fetchMock);

import { getUser, isAuthenticated } from "./auth";
import { headers } from "next/headers";
import { redirect } from "next/navigation";

describe("getUser", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
    fetchMock.mockReset();
  });

  it("returns mock user in development mode", async () => {
    vi.stubEnv("NODE_ENV", "development");

    const user = await getUser();
    expect(user).not.toBeNull();
    expect(user!.firstName).toBe("Hans Kristian");
    expect(user!.email).toContain("@nav.no");
  });

  it("throws when NAIS_TOKEN_INTROSPECTION_ENDPOINT is missing", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");

    vi.mocked(headers).mockResolvedValue({
      get: () => "Bearer some-token",
    } as unknown as Awaited<ReturnType<typeof headers>>);

    await expect(getUser(false)).rejects.toThrow("NAIS_TOKEN_INTROSPECTION_ENDPOINT");
  });

  it("returns null when no Authorization header and shouldRedirect is false", async () => {
    vi.stubEnv("NODE_ENV", "production");

    vi.mocked(headers).mockResolvedValue({
      get: () => null,
    } as unknown as Awaited<ReturnType<typeof headers>>);

    const user = await getUser(false);
    expect(user).toBeNull();
  });

  it("redirects when no Authorization header and shouldRedirect is true", async () => {
    vi.stubEnv("NODE_ENV", "production");

    vi.mocked(headers).mockResolvedValue({
      get: () => null,
    } as unknown as Awaited<ReturnType<typeof headers>>);

    await expect(getUser(true)).rejects.toThrow("NEXT_REDIRECT");
    expect(redirect).toHaveBeenCalledWith("/oauth2/login");
  });

  it("returns null on inactive token when shouldRedirect is false", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");

    vi.mocked(headers).mockResolvedValue({
      get: () => "Bearer invalid-token",
    } as unknown as Awaited<ReturnType<typeof headers>>);

    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ active: false, error: "token is expired" }))
    );

    const user = await getUser(false);
    expect(user).toBeNull();
    expect(fetchMock).toHaveBeenCalledWith("http://texas/introspect", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ identity_provider: "entra_id", token: "invalid-token" }),
    });
  });

  it("returns null when introspection request fails", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");

    vi.mocked(headers).mockResolvedValue({
      get: () => "Bearer some-token",
    } as unknown as Awaited<ReturnType<typeof headers>>);

    fetchMock.mockRejectedValueOnce(new Error("Connection refused"));

    const user = await getUser(false);
    expect(user).toBeNull();
  });

  it("parses user from valid introspection response", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");

    vi.mocked(headers).mockResolvedValue({
      get: () => "Bearer valid-token",
    } as unknown as Awaited<ReturnType<typeof headers>>);

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          active: true,
          name: "Flaatten, Hans Kristian",
          preferred_username: "Hans.Kristian.Flaatten@Nav.No",
          groups: ["admin", "copilot-users"],
        })
      )
    );

    const user = await getUser(false);
    expect(user).not.toBeNull();
    expect(user!.firstName).toBe("Hans Kristian");
    expect(user!.lastName).toBe("Flaatten");
    expect(user!.email).toBe("hans.kristian.flaatten@nav.no");
    expect(user!.groups).toEqual(["admin", "copilot-users"]);
  });

  it("handles missing name in introspection response", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");

    vi.mocked(headers).mockResolvedValue({
      get: () => "Bearer valid-token",
    } as unknown as Awaited<ReturnType<typeof headers>>);

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          active: true,
          preferred_username: "user@nav.no",
          groups: [],
        })
      )
    );

    const user = await getUser(false);
    expect(user).not.toBeNull();
    expect(user!.firstName).toBe("");
    expect(user!.lastName).toBe("");
  });
});

describe("isAuthenticated", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
    fetchMock.mockReset();
  });

  it("returns true in development mode", async () => {
    vi.stubEnv("NODE_ENV", "development");
    expect(await isAuthenticated()).toBe(true);
  });

  it("returns false when no auth header", async () => {
    vi.stubEnv("NODE_ENV", "production");

    vi.mocked(headers).mockResolvedValue({
      get: () => null,
    } as unknown as Awaited<ReturnType<typeof headers>>);

    expect(await isAuthenticated()).toBe(false);
  });
});
