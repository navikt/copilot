import { describe, it, expect } from "vitest";
import { buildCommands } from "./command-builder";

describe("buildCommands", () => {
  it("builds a copilot terminal launch with the collection model", () => {
    const cmd = buildCommands({
      client: "copilot",
      surface: "terminal",
      collection: "kotlin-backend",
    });
    expect(cmd.launch).toBe("nav-pilot --client copilot --model github-copilot/claude-sonnet-4.5");
    expect(cmd.clientLabel).toBe("GitHub Copilot CLI");
  });

  it("uses opus for the fullstack collection", () => {
    const cmd = buildCommands({
      client: "opencode",
      surface: "terminal",
      collection: "fullstack",
    });
    expect(cmd.launch).toContain("--client opencode");
    expect(cmd.launch).toContain("github-copilot/claude-opus-4.6");
  });

  it("returns an @nav-pilot mention for the editor surface regardless of client", () => {
    const cmd = buildCommands({
      client: "copilot",
      surface: "editor",
      collection: "nextjs-frontend",
    });
    expect(cmd.launch.startsWith("@nav-pilot")).toBe(true);
  });

  it("drops flags for the interactive client", () => {
    const cmd = buildCommands({
      client: "interactive",
      surface: "terminal",
      collection: "kotlin-backend",
    });
    expect(cmd.launch).toBe("nav-pilot");
  });
});
