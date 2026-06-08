import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, waitFor } from "@testing-library/react";
import { HashAnchorScroll } from "./hash-anchor-scroll";

vi.mock("next/navigation", () => ({
  usePathname: () => "/statistikk",
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
});
