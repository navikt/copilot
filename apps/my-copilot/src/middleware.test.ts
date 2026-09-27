import { describe, it, expect, vi, beforeEach } from "vitest";
import { isPrivatePath, proxy } from "./proxy";

vi.mock("next/server", () => {
  return {
    NextResponse: {
      redirect: vi.fn((url: URL) => ({ type: "redirect", url: url.toString() })),
      next: vi.fn(() => ({ type: "next" })),
      json: vi.fn((body: unknown, init: { status: number }) => ({ type: "json", body, status: init.status })),
    },
  };
});

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

function createMockRequest(pathname: string, options: { auth?: boolean | string; search?: string } = {}): NextRequest {
  const authHeader = options.auth === true ? "Bearer token" : options.auth || null;
  return {
    headers: {
      get: (name: string) => {
        if (name === "Authorization") return authHeader;
      },
    },
    nextUrl: { pathname, search: options.search ?? "" },
    url: `https://ki-utvikling.nav.no${pathname}${options.search ?? ""}`,
  } as unknown as NextRequest;
}

describe("isPrivatePath", () => {
  it.each([
    ["/statistikk", true],
    ["/statistikk/json", true],
    ["/adopsjon", true],
    ["/adopsjon/debug", true],
    ["/kostnad", true],
    ["/abonnement", true],
    ["/api/copilot", true],
    ["/api/copilot/seats", true],
    ["/api/adoption", true],
    ["/api/adoption/debug", true],
  ])("%s → private (%s)", (path, expected) => {
    expect(isPrivatePath(path)).toBe(expected);
  });

  it.each([
    ["/", false],
    ["/nyheter", false],
    ["/nyheter/some-article", false],
    ["/praksis", false],
    ["/praksis/sections/foo", false],
    ["/verktoy", false],
    ["/ordliste", false],
    ["/ordbok", false],
    ["/nav-pilot", false],
    ["/nav-pilot/docs", false],
    ["/install", false],
    ["/install/agent", false],
    ["/api/contributors", false],
  ])("%s → public (%s)", (path, expected) => {
    expect(isPrivatePath(path)).toBe(expected);
  });

  it("does not match partial path names (prefix collision)", () => {
    expect(isPrivatePath("/statistikkfoo")).toBe(false);
    expect(isPrivatePath("/adopsjon-test")).toBe(false);
    expect(isPrivatePath("/kostnadsfri")).toBe(false);
  });
});

describe("proxy", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas/introspect");
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ active: true }) });
  });

  it("skips auth checks in development mode without Texas", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");
    await proxy(createMockRequest("/statistikk"));
    expect(NextResponse.next).toHaveBeenCalled();
    expect(NextResponse.redirect).not.toHaveBeenCalled();
  });

  it("enforces auth in development mode when Texas is configured", async () => {
    vi.stubEnv("NODE_ENV", "development");
    await proxy(createMockRequest("/statistikk"));
    expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
  });

  describe("public paths → always pass through", () => {
    it("passes through public paths without auth", async () => {
      await proxy(createMockRequest("/praksis"));
      expect(NextResponse.next).toHaveBeenCalled();
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("passes through /api/contributors without auth", async () => {
      await proxy(createMockRequest("/api/contributors"));
      expect(NextResponse.next).toHaveBeenCalled();
    });

    it("passes through root path", async () => {
      await proxy(createMockRequest("/"));
      expect(NextResponse.next).toHaveBeenCalled();
    });
  });

  describe("private pages + authenticated → pass through", () => {
    it("passes through /statistikk with auth", async () => {
      await proxy(createMockRequest("/statistikk", { auth: true }));
      expect(NextResponse.next).toHaveBeenCalled();
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("passes through /kostnad with auth", async () => {
      await proxy(createMockRequest("/kostnad", { auth: true }));
      expect(NextResponse.next).toHaveBeenCalled();
    });
  });

  describe("private pages + unauthenticated → login redirect", () => {
    it("redirects /statistikk to /oauth2/login", async () => {
      await proxy(createMockRequest("/statistikk"));
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.pathname).toBe("/oauth2/login");
      expect(url.searchParams.get("redirect")).toBe("/statistikk");
    });

    it("redirects /adopsjon to /oauth2/login", async () => {
      await proxy(createMockRequest("/adopsjon"));
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.searchParams.get("redirect")).toBe("/adopsjon");
    });

    it("preserves query string in redirect", async () => {
      await proxy(createMockRequest("/statistikk", { search: "?tab=models" }));
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.searchParams.get("redirect")).toBe("/statistikk?tab=models");
    });
  });

  describe("private API + unauthenticated → 401 JSON", () => {
    it("returns 401 for /api/copilot without auth", async () => {
      await proxy(createMockRequest("/api/copilot"));
      expect(NextResponse.json).toHaveBeenCalledWith({ error: "Unauthorized" }, { status: 401 });
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("returns 401 for /api/adoption without auth", async () => {
      await proxy(createMockRequest("/api/adoption"));
      expect(NextResponse.json).toHaveBeenCalledWith({ error: "Unauthorized" }, { status: 401 });
    });

    it("returns 401 for /statistikk/json without auth", async () => {
      await proxy(createMockRequest("/statistikk/json"));
      expect(NextResponse.json).toHaveBeenCalledWith({ error: "Unauthorized" }, { status: 401 });
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("passes through /api/copilot with auth", async () => {
      await proxy(createMockRequest("/api/copilot", { auth: true }));
      expect(NextResponse.next).toHaveBeenCalled();
      expect(NextResponse.json).not.toHaveBeenCalled();
    });
  });

  describe("client-supplied Authorization header is validated, not trusted (#1070)", () => {
    it("redirects a private page when Texas says the token is inactive", async () => {
      fetchMock.mockResolvedValue({ ok: true, json: async () => ({ active: false, error: "token is not valid" }) });
      await proxy(createMockRequest("/abonnement", { auth: "Bearer x" }));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://texas/introspect",
        expect.objectContaining({ body: JSON.stringify({ identity_provider: "entra_id", token: "x" }) })
      );
      expect(NextResponse.next).not.toHaveBeenCalled();
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.pathname).toBe("/oauth2/login");
    });

    it("returns 401 on a private API route when Texas says the token is inactive", async () => {
      fetchMock.mockResolvedValue({ ok: true, json: async () => ({ active: false }) });
      await proxy(createMockRequest("/api/copilot", { auth: "Bearer x" }));
      expect(NextResponse.next).not.toHaveBeenCalled();
      expect(NextResponse.json).toHaveBeenCalledWith({ error: "Unauthorized" }, { status: 401 });
    });

    it("fails closed when Texas is unreachable", async () => {
      fetchMock.mockRejectedValue(new Error("connect ECONNREFUSED"));
      await proxy(createMockRequest("/kostnad", { auth: "Bearer x" }));
      expect(NextResponse.next).not.toHaveBeenCalled();
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
    });

    it("rejects a header that is not a bearer token without calling Texas", async () => {
      await proxy(createMockRequest("/statistikk", { auth: "Basic x" }));
      expect(fetchMock).not.toHaveBeenCalled();
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
    });
  });
});
