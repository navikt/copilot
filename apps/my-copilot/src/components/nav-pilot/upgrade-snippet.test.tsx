import { fireEvent, render, screen } from "@testing-library/react";
import { UPGRADE_COMMANDS } from "@/lib/install-commands";
import { UpgradeSnippet } from "./upgrade-snippet";

function setPlatform(platform: string) {
  Object.defineProperty(navigator, "userAgentData", { configurable: true, get: () => ({ platform }) });
}

afterEach(() => {
  localStorage.clear();
  Reflect.deleteProperty(navigator, "userAgentData");
});

describe("UpgradeSnippet", () => {
  it("opens on apt for Linux and switches to the script commands", () => {
    setPlatform("Linux");
    render(<UpgradeSnippet />);
    fireEvent.click(screen.getByRole("button", { name: /Oppgrader først/ }));
    expect(screen.getByRole("radio", { name: "apt" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByText(/sudo apt update && sudo apt upgrade nav-pilot cplt/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "Skript" }));
    expect(screen.getByText(/cplt update/)).toBeInTheDocument();
    expect(UPGRADE_COMMANDS.script).toContain("nav-pilot upgrade");
  });

  it("opens on Homebrew for macOS", () => {
    setPlatform("macOS");
    render(<UpgradeSnippet />);
    fireEvent.click(screen.getByRole("button", { name: /Oppgrader først/ }));
    expect(screen.getByText(/brew upgrade navikt\/tap\/nav-pilot navikt\/tap\/cplt/)).toBeInTheDocument();
  });
});
