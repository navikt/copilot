import { describe, it, expect, vi, beforeEach } from "vitest";
import { isPrivatePath, middleware } from "./middleware";

vi.mock("next/server", () => {
  return {
    NextResponse: {
      redirect: vi.fn((url: URL) => ({ type: "redirect", url: url.toString() })),
      next: vi.fn(() => ({ type: "next" })),
    },
  };
});

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

function createMockRequest(pathname: string, host: string): NextRequest {
  return {
    headers: { get: (name: string) => (name === "host" ? host : null) },
    nextUrl: { pathname, search: "" },
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

  describe("public domain → redirects private paths", () => {
    it("redirects /statistikk to internal host", () => {
      middleware(createMockRequest("/statistikk", "ki-utvikling.nav.no"));
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
      const url = vi.mocked(NextResponse.redirect).mock.calls[0][0] as URL;
      expect(url.toString()).toBe("https://min-copilot.ansatt.nav.no/statistikk");
    });

    it("redirects /api/copilot to internal host", () => {
      middleware(createMockRequest("/api/copilot", "ki-utvikling.nav.no"));
      expect(NextResponse.redirect).toHaveBeenCalledTimes(1);
    });

    it("passes through public paths", () => {
      middleware(createMockRequest("/praksis", "ki-utvikling.nav.no"));
      expect(NextResponse.redirect).not.toHaveBeenCalled();
      expect(NextResponse.next).toHaveBeenCalled();
    });

    it("passes through /api/contributors (public API)", () => {
      middleware(createMockRequest("/api/contributors", "ki-utvikling.nav.no"));
      expect(NextResponse.redirect).not.toHaveBeenCalled();
      expect(NextResponse.next).toHaveBeenCalled();
    });
  });

  describe("internal domain → passes through all paths", () => {
    it("does not redirect private paths on internal domain", () => {
      middleware(createMockRequest("/statistikk", "min-copilot.ansatt.nav.no"));
      expect(NextResponse.redirect).not.toHaveBeenCalled();
      expect(NextResponse.next).toHaveBeenCalled();
    });
  });
});
