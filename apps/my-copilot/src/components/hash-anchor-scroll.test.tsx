import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, waitFor } from "@testing-library/react";
import { HashAnchorScroll } from "./hash-anchor-scroll";

const nav = vi.hoisted(() => ({ pathname: "/statistikk", replace: vi.fn() }));
vi.mock("next/navigation", () => ({
  usePathname: () => nav.pathname,
  useRouter: () => ({ replace: nav.replace }),
}));
vi.mock("@/lib/legacy-anchors", () => ({
  LEGACY_ANCHORS: {
    "/nav-pilot/docs#gammelt-anker": "/nav-pilot/lokal#nytt-anker",
    "/nav-pilot/lokal#gammelt-anker": "/nav-pilot/lokal#nytt-anker",
  },
}));

describe("HashAnchorScroll", () => {
  const originalHash = window.location.hash;
  const originalScrollIntoView = Element.prototype.scrollIntoView;
  const scrollIntoView = vi.fn();

  beforeEach(() => {
    window.location.hash = "";
    Element.prototype.scrollIntoView = scrollIntoView;
  });

  afterEach(() => {
    window.location.hash = originalHash;
    document.body.innerHTML = "";
    Element.prototype.scrollIntoView = originalScrollIntoView;
    scrollIntoView.mockReset();
    nav.pathname = "/statistikk";
    nav.replace.mockReset();
  });

  it("scrolls when the anchor appears after initial render", async () => {
    window.location.hash = "#m%C3%A5ned-hittil-modeller-og-kostnad";

    render(<HashAnchorScroll />);

    expect(scrollIntoView).not.toHaveBeenCalled();

    await act(async () => {
      const target = document.createElement("div");
      target.id = "måned-hittil-modeller-og-kostnad";
      document.body.appendChild(target);
    });

    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalled();
    });
  });

  it("sends a moved anchor to its new place", () => {
    nav.pathname = "/nav-pilot/docs";
    window.location.hash = "#gammelt-anker";
    render(<HashAnchorScroll />);
    expect(nav.replace).toHaveBeenCalledWith("/nav-pilot/lokal#nytt-anker");
  });

  it("sends an anchor moved within the page to its new place", () => {
    nav.pathname = "/nav-pilot/lokal";
    window.location.hash = "#gammelt-anker";
    render(<HashAnchorScroll />);
    expect(nav.replace).toHaveBeenCalledWith("/nav-pilot/lokal#nytt-anker");
  });

  it("leaves the same anchor alone on another page", async () => {
    nav.pathname = "/statistikk";
    window.location.hash = "#gammelt-anker";
    render(<HashAnchorScroll />);
    await new Promise((r) => setTimeout(r, 100));
    expect(nav.replace).not.toHaveBeenCalled();
  });

  it("scrolls instead of redirecting when the anchor exists", async () => {
    nav.pathname = "/nav-pilot/docs";
    window.location.hash = "#gammelt-anker";
    const target = document.createElement("div");
    target.id = "gammelt-anker";
    document.body.appendChild(target);
    render(<HashAnchorScroll />);
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
    expect(nav.replace).not.toHaveBeenCalled();
  });
});
