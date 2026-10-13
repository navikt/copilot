import { render, screen } from "@testing-library/react";
import { CpuIcon } from "@navikt/aksel-icons";
import TeamUsageValue from "./team-usage-value";
import { ModelCategoryIcon, ModelProviderIcon } from "./model-icons";

describe("Team usage summaries", () => {
  it.each(["OpenAI", "Anthropic", "Google", "GitHub", "Microsoft", "Moonshot AI"])(
    "shows the provider mark for %s",
    (value) => {
      const { container } = render(<TeamUsageValue kind="provider" value={value} />);
      const expected = render(<ModelProviderIcon provider={value} />).container.querySelector("svg");
      expect(container.querySelector("svg")?.innerHTML).toBe(expected?.innerHTML);
      expect(screen.getByText(value, { selector: "span" })).toBeInTheDocument();
    }
  );

  it.each(["Lightweight", "Versatile", "Powerful"])("shows the category icon for %s", (value) => {
    const { container } = render(<TeamUsageValue kind="category" value={value} />);
    const expected = render(<ModelCategoryIcon category={value} />).container.querySelector("svg");
    expect(container.querySelector("svg")?.innerHTML).toBe(expected?.innerHTML);
    expect(screen.getByText(value)).toBeInTheDocument();
  });

  it.each(["provider", "category"] as const)("uses a neutral icon for unclassified and unknown %s values", (kind) => {
    const { container } = render(
      <>
        <TeamUsageValue kind={kind} value="Unclassified" />
        <TeamUsageValue kind={kind} value="Future value" />
        <TeamUsageValue kind={kind} value="constructor" />
      </>
    );
    expect(screen.getByText("Uklassifisert")).toBeInTheDocument();
    expect(screen.getByText("Future value")).toBeInTheDocument();
    expect(screen.getByText("constructor")).toBeInTheDocument();
    const expected = render(<CpuIcon />).container.querySelector("svg");
    for (const icon of container.querySelectorAll("svg")) {
      expect(icon.innerHTML).toBe(expected?.innerHTML);
      expect(icon).toHaveAttribute("aria-hidden", "true");
    }
  });
});
