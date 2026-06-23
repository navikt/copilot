import { describe, it, expect } from "vitest";
import { generateSetupScript, type OS } from "./interactive-setup-wizard";

describe("generateSetupScript", () => {
  it("returns editor instructions when workflow is editor", () => {
    const result = generateSetupScript("mac", "editor", "kotlin-backend");
    expect(result.title).toBe("Klar for koding i editoren!");
    expect(result.code).toBeNull();
    expect(result.steps.length).toBeGreaterThan(0);
    expect(result.steps[0]).toContain("VS Code eller IntelliJ");
  });

  describe("macOS", () => {
    const os: OS = "mac";

    it("generates correct CLI script", () => {
      const result = generateSetupScript(os, "cli", "kotlin-backend");
      expect(result.code).toContain("brew install navikt/tap/nav-pilot navikt/tap/cplt rtk");
      expect(result.code).toContain("npm install -g @github/copilot");
      expect(result.code).toContain("nav-pilot install kotlin-backend");
      expect(result.code).not.toContain("opencode");
    });

    it("generates correct OpenCode script", () => {
      const result = generateSetupScript(os, "opencode", "kotlin-backend");
      expect(result.code).toContain("brew install navikt/tap/nav-pilot navikt/tap/cplt rtk");
      expect(result.code).toContain("nav-pilot config set client opencode");
      expect(result.code).toContain("nav-pilot --client opencode");
      expect(result.code).toContain("nav-pilot install kotlin-backend");
    });
  });

  describe("Linux", () => {
    const os: OS = "linux";

    it("generates correct CLI script with curl", () => {
      const result = generateSetupScript(os, "cli", "kotlin-backend");
      expect(result.code).toContain("npm install -g @github/copilot");
      expect(result.code).toContain(
        "curl -sL https://github.com/navikt/copilot/releases/latest/download/nav-pilot_linux_amd64 -o nav-pilot"
      );
      expect(result.code).toContain(
        "curl -sL https://github.com/rtk-ai/rtk/releases/latest/download/rtk-linux-amd64 -o rtk"
      );
      expect(result.code).toContain("chmod +x nav-pilot cplt rtk");
      expect(result.code).toContain("nav-pilot install kotlin-backend");
    });
  });

  describe("Windows", () => {
    const os: OS = "windows";

    it("generates winget command and no nav-pilot execution", () => {
      const result = generateSetupScript(os, "cli", "kotlin-backend");
      expect(result.code).toContain("winget install GitHub.Copilot");
      expect(result.code).toContain("best i WSL");
      expect(result.code).not.toContain("nav-pilot install");
      expect(result.code).not.toContain("nav-pilot config");
    });
  });
});
