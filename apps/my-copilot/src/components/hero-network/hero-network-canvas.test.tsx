import { render } from "@testing-library/react";
import { HeroNetworkCanvas } from "./hero-network-canvas";

describe("HeroNetworkCanvas", () => {
  it("renders no canvas when WebGL is unavailable", () => {
    // jsdom has no WebGL: getContext returns null.
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
    const { container } = render(<HeroNetworkCanvas clusters={[{ name: "Agenter", count: 3 }]} />);
    expect(container.querySelector("canvas")).toBeNull();
    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });
});
