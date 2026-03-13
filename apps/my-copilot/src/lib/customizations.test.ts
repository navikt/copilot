import { getOfficialFileNames } from "./customizations";

describe("getOfficialFileNames", () => {
  it("returns a non-empty set", () => {
    const names = getOfficialFileNames();
    expect(names.size).toBeGreaterThan(0);
  });

  it("includes agent file basenames", () => {
    const names = getOfficialFileNames();
    // Agent files use basename like "nais.agent.md"
    expect(names.has("nais.agent.md")).toBe(true);
  });

  it("includes skill directory names, not SKILL.md", () => {
    const names = getOfficialFileNames();
    // Skills should be stored by directory name, not "SKILL.md"
    expect(names.has("SKILL.md")).toBe(false);
    expect(names.has("observability-setup")).toBe(true);
    expect(names.has("aksel-spacing")).toBe(true);
  });

  it("includes instruction file basenames", () => {
    const names = getOfficialFileNames();
    expect(names.has("nextjs-aksel.instructions.md")).toBe(true);
  });
});
