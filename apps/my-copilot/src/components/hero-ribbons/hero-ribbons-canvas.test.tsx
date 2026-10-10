import { render } from "@testing-library/react";
import { HeroRibbonsCanvas } from "./hero-ribbons-canvas";

describe("HeroRibbonsCanvas", () => {
  afterEach(() => vi.restoreAllMocks());

  it("renders no canvas without WebGL 2", () => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
    const { container } = render(<HeroRibbonsCanvas />);
    expect(container.querySelector("canvas")).toBeNull();
    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });
});
