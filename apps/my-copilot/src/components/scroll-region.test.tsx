import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ScrollRegion } from "./scroll-region";

describe("ScrollRegion", () => {
  it("is a focusable, named region", () => {
    render(<ScrollRegion label="Tabell: test">innhold</ScrollRegion>);
    const region = screen.getByRole("region", { name: "Tabell: test" });
    expect(region).toHaveAttribute("tabindex", "0");
    expect(region).toHaveClass("overflow-x-auto");
  });
});
