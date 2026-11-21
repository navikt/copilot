import { render, screen, fireEvent } from "@testing-library/react";
import { CommandBlock, formatCommandForDisplay } from "./shared";

const COMMAND = `code --add-mcp '{"name":"io.github.navikt/kodeverk","type":"http","url":"https://fleet-gateway.intern.nav.no/mcp/copilot/kodeverk"}'`;

describe("CommandBlock", () => {
  it("shows the JSON over several lines but copies the original one-line command", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });

    const { container } = render(<CommandBlock command={COMMAND} />);
    expect(container.querySelector("pre")?.textContent).toBe(formatCommandForDisplay(COMMAND));
    expect(container.querySelector("pre")?.textContent).toContain('\n  "name"');

    fireEvent.click(screen.getByRole("button"));
    await vi.waitFor(() => expect(writeText).toHaveBeenCalledWith(COMMAND));
  });

  it("leaves commands without a JSON argument unchanged", () => {
    expect(formatCommandForDisplay("nav-pilot mcp enable io.github.navikt/kodeverk")).toBe(
      "nav-pilot mcp enable io.github.navikt/kodeverk"
    );
  });
});
