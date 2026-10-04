import { render, screen } from "@testing-library/react";
import { CpuIcon } from "@navikt/aksel-icons";
import TeamUsageValue from "./team-usage-value";
import { ModelProviderIcon } from "./model-icons";

describe("Team model logos", () => {
  it.each([
    ["claude-haiku-4.5", "Anthropic"],
    ["gemini-2.5-pro", "Google"],
    ["gpt-4o", "OpenAI"],
    ["o3-mini", "OpenAI"],
    ["GPT-6 Sol", "OpenAI"],
    ["mai-code-1.1-flash", "Microsoft"],
    ["kimi-k2.7-code", "Moonshot AI"],
  ] as const)("shows %s with its provider mark and original name", (value, provider) => {
    const { container } = render(<TeamUsageValue kind="model" value={value} />);
    const actual = container.querySelector("svg");
    const expected = render(<ModelProviderIcon provider={provider} />).container.querySelector("svg");
    expect(actual?.innerHTML).toBe(expected?.innerHTML);
    expect(actual).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByText(value)).toBeInTheDocument();
  });

  it("keeps unknown model names without assigning them a provider", () => {
    const { container } = render(<TeamUsageValue kind="model" value="custom-model" />);
    expect(screen.getByText("custom-model")).toBeInTheDocument();
    const expected = render(<CpuIcon />).container.querySelector("svg");
    expect(container.querySelector("svg")?.innerHTML).toBe(expected?.innerHTML);
  });
});
