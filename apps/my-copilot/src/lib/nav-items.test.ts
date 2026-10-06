import { describe, expect, it } from "vitest";
import { activeTop, inSection, sectionGroup } from "./nav-items";

describe("activeTop", () => {
  it.each([
    ["/nav-pilot", "/nav-pilot"],
    ["/nav-pilot/guider/tilpasse", "/nav-pilot"],
    ["/nav-pilot/docs", "/nav-pilot"],
    ["/cplt", "/nav-pilot"],
    ["/nav-pilot/lokal", "/kom-i-gang"],
    ["/nav-pilot/lokal/egen-server", "/kom-i-gang"],
    ["/nav-pilot/agentpakker", "/verktoy"],
    ["/verktoy", "/verktoy"],
    ["/retningslinjer", "/praksis"],
    ["/praksis/guide/wrap-metoden", "/praksis"],
    ["/kostnad", "/innsikt"],
    ["/innsikt/team", "/innsikt"],
    ["/priser", "/innsikt"],
    ["/modeller", "/innsikt"],
    ["/nav-pilotx", undefined],
    ["/", undefined],
  ])("%s → %s", (path, top) => expect(activeTop(path)).toBe(top));
});

it("finds the section-menu group of a page", () => {
  expect(inSection("/nav-pilot")).toBe(true);
  expect(inSection("/praksis")).toBe(false);
  expect(inSection("/cplt")).toBe(false);
  expect(sectionGroup("/verktoy")?.label).toBe("Tilpasning");
  expect(sectionGroup("/kom-i-gang")?.label).toBe("Kom i gang");
  expect(sectionGroup("/nav-pilot/lokal/decide")?.label).toBe("Kom i gang");
  expect(sectionGroup("/nav-pilot/guider")?.label).toBe("Guider");
  expect(sectionGroup("/nav-pilot/forklaring/sandkassen")?.label).toBe("Forklaring");
  expect(sectionGroup("/nav-pilot")).toBeUndefined();
  expect(sectionGroup("/cplt")).toBeUndefined();
});
