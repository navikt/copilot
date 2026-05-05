import { describe, it, expect, vi, beforeEach } from "vitest";
import { isPrivatePath, middleware } from "./middleware";

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

function createMockRequest(pathname: string, options: { auth?: boolean; search?: string } = {}): NextRequest {
  return {
    headers: {
      get: (name: string) => {
        if (name === "Authorization" && options.auth) return "Bearer token";
        return null;
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
    ["/kalkulator", true],
    ["/kalkulator/foo", true],
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

describe("middleware", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("public paths → always pass through", () => {
    it("passes through public paths without auth", () => {
      middleware(createMockRequest("/praksis"));
      expect(NextResponse.next).toHaveBeenCalled();
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("passes through /api/contributors without auth", () => {
      middleware(createMockRequest("/api/contributors"));
      expect(NextResponse.next).toHaveBeenCalled();
    });

    it("passes through root path", () => {
      middleware(createMockRequest("/"));
      expect(NextResponse.next).toHaveBeenCalled();
    });
  });

  describe("private pages + authenticated → pass through", () => {
    it("passes through /statistikk with auth", () => {
      middleware(createMockRequest("/statistikk", { auth: true }));
      expect(NextResponse.next).toHaveBeenCalled();
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("passes through /kostnad with auth", () => {
      middleware(createMockRequest("/kostnad", { auth: true }));
      expect(NextResponse.next).toHaveBeenCalled();
    });
  });

  describe("private pages + unauthenticated → login redirect", () => {
    it("redirects /statistikk to /oauth2/login", () => {
      middleware(createMockRequest("/statistikk"));
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.pathname).toBe("/oauth2/login");
      expect(url.searchParams.get("redirect")).toBe("/statistikk");
    });

    it("redirects /adopsjon to /oauth2/login", () => {
      middleware(createMockRequest("/adopsjon"));
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.searchParams.get("redirect")).toBe("/adopsjon");
    });

    it("preserves query string in redirect", () => {
      middleware(createMockRequest("/statistikk", { search: "?tab=models" }));
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.searchParams.get("redirect")).toBe("/statistikk?tab=models");
    });

    it("redirects /kalkulator to /oauth2/login", () => {
      middleware(createMockRequest("/kalkulator"));
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
    });
  });

  describe("private API + unauthenticated → 401 JSON", () => {
    it("returns 401 for /api/copilot without auth", () => {
      middleware(createMockRequest("/api/copilot"));
      expect(NextResponse.json).toHaveBeenCalledWith({ error: "Unauthorized" }, { status: 401 });
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("returns 401 for /api/adoption without auth", () => {
      middleware(createMockRequest("/api/adoption"));
      expect(NextResponse.json).toHaveBeenCalledWith({ error: "Unauthorized" }, { status: 401 });
    });

    it("returns 401 for /statistikk/json without auth", () => {
      middleware(createMockRequest("/statistikk/json"));
      expect(NextResponse.json).toHaveBeenCalledWith({ error: "Unauthorized" }, { status: 401 });
      expect(NextResponse.redirect).not.toHaveBeenCalled();
    });

    it("passes through /api/copilot with auth", () => {
      middleware(createMockRequest("/api/copilot", { auth: true }));
      expect(NextResponse.next).toHaveBeenCalled();
      expect(NextResponse.json).not.toHaveBeenCalled();
    });
  });
});
