import { render } from "@testing-library/react";
import { LazyCanvas } from "./lazy-canvas";

describe("LazyCanvas", () => {
  afterEach(() => vi.restoreAllMocks());

  it("never loads three when only WebGL 1 is available", () => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(((id: string) =>
      id === "webgl" ? ({} as WebGLRenderingContext) : null) as typeof HTMLCanvasElement.prototype.getContext);
    const load = vi.fn();
    const { container } = render(<LazyCanvas load={load} />);
    expect(load).not.toHaveBeenCalled();
    expect(container).toBeEmptyDOMElement();
  });
});
