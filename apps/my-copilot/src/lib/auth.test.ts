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

function mockHeaders(authHeader: string | null) {
  vi.mocked(headers).mockResolvedValue({
    get: (name: string) => (name === "Authorization" ? authHeader : null),
  } as unknown as Awaited<ReturnType<typeof headers>>);
}

describe("getUser", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
    fetchMock.mockReset();
  });

  describe("development mode", () => {
    it("returns mock user when Texas endpoint is not configured", async () => {
      vi.stubEnv("NODE_ENV", "development");

      const user = await getUser();
      expect(user).not.toBeNull();
      expect(user!.firstName).toBe("Hans Kristian");
      expect(user!.email).toContain("@nav.no");
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("uses real introspection when Texas endpoint IS configured", async () => {
      vi.stubEnv("NODE_ENV", "development");
      vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://localhost:6969/introspect");
      mockHeaders("Bearer dev-token");

      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            active: true,
            name: "Nordmann, Ola",
            preferred_username: "ola.nordmann@nav.no",
            groups: [],
          })
        )
      );

      const user = await getUser(false);
      expect(user!.email).toBe("ola.nordmann@nav.no");
      expect(fetchMock).toHaveBeenCalled();
    });
  });

  describe("production mode", () => {
    beforeEach(() => {
      vi.stubEnv("NODE_ENV", "production");
      vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");
    });

    it("throws when NAIS_TOKEN_INTROSPECTION_ENDPOINT is missing", async () => {
      vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");
      mockHeaders("Bearer some-token");

      await expect(getUser(false)).rejects.toThrow("NAIS_TOKEN_INTROSPECTION_ENDPOINT");
    });

    it("returns null when no Authorization header (shouldRedirect=false)", async () => {
      mockHeaders(null);

      const user = await getUser(false);
      expect(user).toBeNull();
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("redirects to /oauth2/login when no Authorization header (shouldRedirect=true)", async () => {
      mockHeaders(null);

      await expect(getUser(true)).rejects.toThrow("NEXT_REDIRECT");
      expect(redirect).toHaveBeenCalledWith("/oauth2/login");
    });

    it("redirects on invalid token when shouldRedirect is true", async () => {
      mockHeaders("Bearer expired-token");
      fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ active: false, error: "token is expired" })));

      await expect(getUser(true)).rejects.toThrow("NEXT_REDIRECT");
      expect(redirect).toHaveBeenCalledWith("/oauth2/login");
    });

    it("returns null on inactive token (shouldRedirect=false)", async () => {
      mockHeaders("Bearer invalid-token");
      fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ active: false, error: "token is expired" })));

      const user = await getUser(false);
      expect(user).toBeNull();
    });

    it("sends correct introspection request", async () => {
      mockHeaders("Bearer my-jwt-token");
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ active: true, preferred_username: "a@nav.no", groups: [] }))
      );

      await getUser(false);
      expect(fetchMock).toHaveBeenCalledWith("http://texas/introspect", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identity_provider: "entra_id", token: "my-jwt-token" }),
      });
    });

    it("strips Bearer prefix from token", async () => {
      mockHeaders("Bearer abc123");
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ active: true, preferred_username: "a@nav.no", groups: [] }))
      );

      await getUser(false);
      const body = JSON.parse(fetchMock.mock.calls[0][1]!.body as string);
      expect(body.token).toBe("abc123");
    });

    it("returns null when introspection request fails (network error)", async () => {
      mockHeaders("Bearer some-token");
      fetchMock.mockRejectedValueOnce(new Error("Connection refused"));

      const user = await getUser(false);
      expect(user).toBeNull();
    });

    it("returns null when introspection returns non-200 status", async () => {
      mockHeaders("Bearer some-token");
      fetchMock.mockResolvedValueOnce(new Response("Internal Server Error", { status: 500 }));

      const user = await getUser(false);
      expect(user).toBeNull();
    });

    it("parses user correctly from valid introspection response", async () => {
      mockHeaders("Bearer valid-token");
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
      expect(user).toEqual({
        firstName: "Hans Kristian",
        lastName: "Flaatten",
        email: "hans.kristian.flaatten@nav.no",
        groups: ["admin", "copilot-users"],
      });
    });

    it("lowercases email from preferred_username", async () => {
      mockHeaders("Bearer valid-token");
      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            active: true,
            preferred_username: "OLA.NORDMANN@NAV.NO",
            groups: [],
          })
        )
      );

      const user = await getUser(false);
      expect(user!.email).toBe("ola.nordmann@nav.no");
    });

    it("handles missing name in response", async () => {
      mockHeaders("Bearer valid-token");
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ active: true, preferred_username: "user@nav.no", groups: [] }))
      );

      const user = await getUser(false);
      expect(user!.firstName).toBe("");
      expect(user!.lastName).toBe("");
    });

    it("handles missing groups in response", async () => {
      mockHeaders("Bearer valid-token");
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ active: true, name: "Test, User", preferred_username: "user@nav.no" }))
      );

      const user = await getUser(false);
      expect(user!.groups).toEqual([]);
    });

    it("handles missing preferred_username in response", async () => {
      mockHeaders("Bearer valid-token");
      fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ active: true, name: "Test, User", groups: [] })));

      const user = await getUser(false);
      expect(user!.email).toBe("");
    });
  });
});

describe("isAuthenticated", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
    fetchMock.mockReset();
  });

  it("returns true in development mode without Texas", async () => {
    vi.stubEnv("NODE_ENV", "development");
    expect(await isAuthenticated()).toBe(true);
  });

  it("returns false when no auth header in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    mockHeaders(null);

    expect(await isAuthenticated()).toBe(false);
  });

  it("returns true when token is valid in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");
    mockHeaders("Bearer valid");
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ active: true, preferred_username: "u@nav.no", groups: [] }))
    );

    expect(await isAuthenticated()).toBe(true);
  });

  it("returns false when token is invalid in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");
    mockHeaders("Bearer bad");
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ active: false, error: "expired" })));

    expect(await isAuthenticated()).toBe(false);
  });
});
