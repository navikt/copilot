import { fireEvent, render, screen } from "@testing-library/react";
import { InstallPicker } from "./install-picker";

const props = {
  lang: "nb" as const,
  mac: "brew install x",
  linux: "curl install.sh | bash",
  apt: "apt block",
  windowsNote: "Bruk WSL2.",
};

function setPlatform(platform: string) {
  Object.defineProperty(navigator, "userAgentData", { configurable: true, get: () => ({ platform }) });
}

afterEach(() => {
  localStorage.clear();
  Reflect.deleteProperty(navigator, "userAgentData");
});

describe("InstallPicker", () => {
  it("preselects the detected OS and shows its command and the apt disclosure", () => {
    setPlatform("Linux");
    render(<InstallPicker {...props} />);
    expect(screen.getByRole("radio", { name: "Linux" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByText(props.linux)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bruk apt-arkivet (Debian/Ubuntu)" })).toBeInTheDocument();
  });

  it("speaks English with lang en", () => {
    setPlatform("Linux");
    render(<InstallPicker {...props} lang="en" />);
    expect(screen.getByRole("button", { name: "Use the apt archive (Debian/Ubuntu)" })).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Copy the command" }).length).toBeGreaterThan(0);
  });

  it("uses the stored choice over detection", () => {
    setPlatform("Linux");
    localStorage.setItem("install-os", "mac");
    render(<InstallPicker {...props} />);
    expect(screen.getByText(props.mac)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Bruk apt-arkivet (Debian/Ubuntu)" })).not.toBeInTheDocument();
  });

  it("switches on a manual choice and remembers it", () => {
    setPlatform("macOS");
    render(<InstallPicker {...props} />);
    fireEvent.click(screen.getByRole("radio", { name: "Windows" }));
    expect(screen.getByText(props.windowsNote)).toBeInTheDocument();
    expect(screen.getByText(props.linux)).toBeInTheDocument();
    expect(localStorage.getItem("install-os")).toBe("windows");
  });
});
