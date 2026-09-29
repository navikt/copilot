import { describe, expect, it } from "vitest";
import { isNavAllowedModel, NAV_PILOT_MODEL_CHOICES, navPilotPurposesFor, normalizeModelName } from "./model-policy";

describe("Navs modellpolicy", () => {
  it("matcher prisradene uten prisnivå-suffiks", () => {
    expect(normalizeModelName("GPT-5.4 (Default, ≤ 272K)")).toBe("GPT-5.4");
    expect(normalizeModelName("GPT-5.4 (Long context, 272K)")).toBe("GPT-5.4");
    expect(normalizeModelName("Claude Opus 4.8 (fast mode) (preview)")).toBe("Claude Opus 4.8 (fast mode)");
  });

  it("skiller modeller som er aktivert av Nav fra resten av GitHubs prisliste", () => {
    expect(isNavAllowedModel("GPT-5.4 (Default, ≤ 272K)")).toBe(false);
    expect(isNavAllowedModel("Claude Fable 5.1")).toBe(false);
    expect(isNavAllowedModel("GPT-6 Sol (Default, ≤ 272K)")).toBe(true);
    expect(isNavAllowedModel("Claude Sonnet 5")).toBe(true);
  });
});

describe("nav-pilots modellvalg", () => {
  it("har ett tydelig bruksområde per primærvalg", () => {
    expect(NAV_PILOT_MODEL_CHOICES.map((choice) => choice.purpose)).toEqual([
      "Daglig agentisk koding",
      "Research og faste maler",
      "Høyrisikoplanlegging og kodegjennomgang",
      "Aksel, tilgjengelighet og norsk tekst",
      "Rask Aksel-scaffolding",
    ]);
  });

  it("viser både primær- og fallback-bruk på prisraden", () => {
    expect(navPilotPurposesFor("GPT-6 Sol (Default, ≤ 272K)")).toEqual(["Daglig agentisk koding"]);
    expect(navPilotPurposesFor("GPT-5.3-Codex (Default)")).toEqual([
      "Daglig agentisk koding",
      "Research og faste maler",
      "Høyrisikoplanlegging og kodegjennomgang",
    ]);
  });
});
