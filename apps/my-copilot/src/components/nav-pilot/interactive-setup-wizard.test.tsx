import { describe, it, expect } from "vitest";
import { generateSetupScript, type OS } from "./interactive-setup-wizard";

describe("generateSetupScript", () => {
  it("returns editor instructions when workflow is editor", () => {
    const result = generateSetupScript("mac", "editor");
    expect(result.title).toBe("Klar for koding i editoren!");
    expect(result.code).toBeNull();
    expect(result.steps.length).toBeGreaterThan(0);
    expect(result.steps[0]).toContain("VS Code eller IntelliJ");
  });

  describe("macOS", () => {
    const os: OS = "mac";

    it("generates correct CLI script", () => {
      const result = generateSetupScript(os, "cli");
      expect(result.code).toContain("brew install navikt/tap/nav-pilot navikt/tap/cplt");
      expect(result.code).toContain("curl -fsSL https://gh.io/copilot-install | bash");
      expect(result.code).not.toContain("npm install");
      expect(result.code).toContain("nav-pilot install nav-pilot");
      expect(result.code).toContain('export PATH="$HOME/.local/bin:$PATH"');
      expect(result.code).not.toContain("which -a");
      expect(result.code).not.toContain("opencode");
    });

    it("generates correct OpenCode script", () => {
      const result = generateSetupScript(os, "opencode");
      expect(result.code).toContain("brew install navikt/tap/nav-pilot navikt/tap/cplt");
      expect(result.code).toContain("curl -fsSL https://opencode.ai/install | bash");
      expect(result.code).toContain("nav-pilot config set client opencode");
      expect(result.code).toContain("nav-pilot --client opencode");
      expect(result.code).toContain("nav-pilot install nav-pilot");
      expect(result.code).toContain('export PATH="$HOME/.local/bin:$PATH"');
      expect(result.code).not.toContain("@github/copilot");
      expect(result.code).not.toContain("npm install");
    });
  });

  describe("Linux", () => {
    const os: OS = "linux";

    it("generates correct CLI script with curl", () => {
      const result = generateSetupScript(os, "cli");
      expect(result.code).toContain("curl -fsSL https://gh.io/copilot-install | bash");
      expect(result.code).not.toContain("npm install");
      expect(result.code).toContain(
        "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash"
      );
      expect(result.code).toContain("nav-pilot install nav-pilot");
      expect(result.code).toContain('export PATH="$HOME/.local/bin:$PATH"');
      expect(result.code).not.toContain("which -a");
    });

    it("generates correct OpenCode script with curl", () => {
      const result = generateSetupScript(os, "opencode");
      expect(result.code).toContain("curl -fsSL https://opencode.ai/install | bash");
      expect(result.code).not.toContain("npm install -g opencode");
      expect(result.code).toContain(
        "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash"
      );
      expect(result.code).toContain("nav-pilot config set client opencode");
      expect(result.code).toContain("nav-pilot install nav-pilot");
      expect(result.code).toContain('export PATH="$HOME/.local/bin:$PATH"');
    });
  });

  describe("Windows", () => {
    const os: OS = "windows";

    it("generates WSL instructions for CLI", () => {
      const result = generateSetupScript(os, "cli");
      expect(result.code).toContain("WSL2-terminalen");
      expect(result.code).toContain("curl -fsSL https://gh.io/copilot-install | bash");
      expect(result.code).not.toContain("npm install");
      expect(result.code).toContain("which -a copilot cplt nav-pilot");
      expect(result.code).toContain(
        "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash"
      );
      expect(result.code).toContain("nav-pilot install nav-pilot");
      expect(result.code).toContain('export PATH="$HOME/.local/bin:$PATH"');
    });

    it("generates WSL instructions for OpenCode", () => {
      const result = generateSetupScript(os, "opencode");
      expect(result.code).toContain("WSL2-terminalen");
      expect(result.code).toContain("curl -fsSL https://opencode.ai/install | bash");
      expect(result.code).toContain("nav-pilot config set client opencode");
      expect(result.code).toContain("nav-pilot install nav-pilot");
      expect(result.code).toContain('export PATH="$HOME/.local/bin:$PATH"');
      expect(result.code).toContain("which -a opencode cplt nav-pilot");
      expect(result.code).not.toContain("npm install");
    });
  });
});
