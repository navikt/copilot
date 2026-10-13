import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// The figures behind these labels are fixed by the data (billed vs preliminary, one-day
// review average). Guard the wording so it keeps matching what is computed.
const src = (path: string) => readFileSync(join(__dirname, "../../../..", path), "utf8");

describe("innsikt/bruk labels", () => {
  it("shows actual cost only, with the approved terms, and keeps the old anchor", () => {
    const page = src("app/(nb)/innsikt/bruk/page.tsx");
    expect(page).not.toMatch(/[Pp]rognose,|forecast/);
    expect(page).toContain('label: "Fakturert"');
    expect(page).toContain('label: "Foreløpig forbruk"');
    expect(page).toContain('id="kostnad"');
    expect(page).toContain('id="kostnad-over-tid"');
  });

  it("states the review time as a one-day average, not a median", () => {
    const page = src("app/(nb)/innsikt/bruk/page.tsx");
    expect(page).toContain('label="Snitt tid til første review"');
    expect(page).toContain("siste dag med data");
    expect(page).not.toContain("Median tid fra en pull request opprettes til den får første review");
  });

  it("shows the agent share change in percentage points and names the all-time window", () => {
    const page = src("app/(nb)/innsikt/bruk/page.tsx");
    expect(page).toContain("formatPp(agentShare(latest) - agentShare(prev))");
    expect(page).toContain('label="PR-er laget av Copilot, hele perioden"');
  });
});
