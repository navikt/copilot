import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// The figures behind these labels are fixed by the data (model consumption, one-day
// review average). Guard the wording so it keeps matching what is computed.
const src = (path: string) => readFileSync(join(__dirname, "../../../..", path), "utf8");

describe("innsikt/bruk labels", () => {
  it("does not turn historical model consumption into invoice totals", () => {
    const page = src("app/(nb)/innsikt/bruk/page.tsx");
    expect(page).not.toMatch(/[Pp]rognose,|forecast/);
    expect(page).not.toContain('label: "Fakturert"');
    expect(page).not.toContain("t.year_month < current");
    expect(page).toContain('label: "Foreløpig forbruk, netto"');
    expect(page).toContain("data: months.map((t) => t.total_net_amount)");
    expect(page).toContain("heller ikke for avsluttede måneder");
    expect(page).toContain("Lisensabonnementer og CodeQuality er ikke med");
    expect(page).not.toContain("getAdminMonthlyOverview");
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
