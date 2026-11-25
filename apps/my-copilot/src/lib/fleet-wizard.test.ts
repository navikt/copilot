import { emptyState, firstInvalidStep, toSpec, validateStep, type WizardState } from "./fleet-wizard";

const dev = { requireRos: false };
const prod = { requireRos: true };

const filled = (over: Partial<WizardState> = {}): WizardState => ({
  ...emptyState("team-a"),
  name: "kodeverk",
  app: "kodeverk-mcp",
  url: "http://kodeverk-mcp.team-a/mcp",
  tools: [
    { name: "hent_kode", description: "Henter én kode.", allowed: true, risk: "" },
    { name: "slett_kode", allowed: false, risk: "" },
  ],
  defaultRisk: "read",
  dataCategory: "Intern",
  ...over,
});

describe("validateStep", () => {
  it("step 1: team and a DNS-label name", () => {
    expect(validateStep(1, emptyState("team-a"), dev)).toEqual({ name: "Skriv et navn." });
    expect(validateStep(1, { ...emptyState(""), name: "Kodeverk" }, dev)).toMatchObject({
      team: expect.any(String),
      name: expect.stringMatching(/små bokstaver/),
    });
    expect(validateStep(1, filled(), dev)).toEqual({});
  });

  it("step 2: an app", () => {
    expect(validateStep(2, filled({ app: "" }), dev).app).toMatch(/Velg appen/);
    expect(validateStep(2, filled({ app: "A/b" }), dev).app).toMatch(/små bokstaver/);
  });

  it.each(["", "https://x.team-a/mcp", "http://x/mcp?a=1", "http://x/mcp#y", "ikke en url"])(
    "step 3: refuses the address %j",
    (url) => {
      expect(validateStep(3, filled({ url }), dev).url).toBeTruthy();
    }
  );

  it("step 4: at least one allowed tool, gateway charset, no duplicates, a default level", () => {
    expect(validateStep(4, filled({ tools: [{ name: "a", allowed: false, risk: "" }] }), dev).tools).toMatch(
      /minst ett/
    );
    expect(validateStep(4, filled({ tools: [{ name: "a/b", allowed: true, risk: "" }] }), dev).tools).toMatch(
      /bokstaver/
    );
    expect(
      validateStep(
        4,
        filled({
          tools: [
            { name: "a", allowed: true, risk: "" },
            { name: "a", allowed: true, risk: "" },
          ],
        }),
        dev
      ).tools
    ).toMatch(/én gang/);
    expect(validateStep(4, filled({ defaultRisk: "" }), dev).defaultRisk).toBeTruthy();
    expect(validateStep(4, filled(), dev)).toEqual({});
  });

  it("step 4: a write tool needs a ROS in prod only, and a missing level counts as write", () => {
    const writes = filled({ defaultRisk: "write" });
    expect(validateStep(4, writes, dev)).toEqual({});
    expect(validateStep(4, writes, prod).rosRef).toMatch(/ROS/);
    expect(validateStep(4, { ...writes, rosRef: "https://ros/1" }, prod)).toEqual({});
    // An exception to write under a read default still needs a ROS.
    const exception = filled({ tools: [{ name: "a", allowed: true, risk: "write" }] });
    expect(validateStep(4, exception, prod).rosRef).toBeTruthy();
    // Only allowed tools count, and propose needs no ROS.
    expect(validateStep(4, filled({ defaultRisk: "propose" }), prod)).toEqual({});
    expect(validateStep(4, filled({ rosRef: " x" }), dev).rosRef).toBeTruthy();
  });

  it("step 5: a data class, none preselected", () => {
    expect(emptyState("team-a").dataCategory).toBe("");
    expect(validateStep(5, filled({ dataCategory: "" }), dev).dataCategory).toBeTruthy();
    expect(validateStep(5, filled({ dataCategory: "StrengtFortrolig" }), dev).dataCategory).toBeTruthy();
    expect(validateStep(5, filled({ readme: "æ".repeat(32 * 1024 + 1) }), dev).readme).toBeTruthy();
    expect(validateStep(5, filled(), dev)).toEqual({});
  });

  it("firstInvalidStep finds the earliest step left", () => {
    expect(firstInvalidStep(filled(), dev)).toBe(0);
    expect(firstInvalidStep(filled({ app: "", dataCategory: "" }), dev)).toBe(2);
  });
});

describe("toSpec", () => {
  it("sends allowed tools only, and toolRisk only for exceptions", () => {
    const s = filled({
      tools: [
        { name: "hent", description: "Henter", allowed: true, risk: "" },
        { name: "lagre", allowed: true, risk: "write" },
        { name: "les", allowed: true, risk: "read" },
        { name: "slett", allowed: false, risk: "write" },
      ],
      readme: "# Hei",
    });
    expect(toSpec(s)).toEqual({
      dataCategory: "Intern",
      upstream: { app: "kodeverk-mcp", url: "http://kodeverk-mcp.team-a/mcp" },
      allowedTools: ["hent", "lagre", "les"],
      tools: [{ name: "hent", description: "Henter" }],
      defaultRisk: "read",
      toolRisk: { lagre: "write" },
      readme: "# Hei",
    });
  });
});
