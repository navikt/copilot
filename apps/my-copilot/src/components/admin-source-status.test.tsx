import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AdminSourceStatus } from "./admin-source-status";
import type { AdminMonth, AdminMonthlyOverview } from "@/lib/types";

const month: AdminMonth = {
  month: "2026-10",
  basis: "net",
  loaded_at: "2026-11-03 14:00:00+00",
  enterprise_net_usd: 0,
  assigned_net_usd: 0,
  residual_net_usd: 0,
  residual_pct: null,
  no_team_net_usd: null,
  no_usage_net_usd: null,
  gross_usd: null,
  estimated_net_usd: null,
  seats: null,
  licence_usd_estimate: null,
  total_usd: null,
};
const data: AdminMonthlyOverview = {
  year: 2026,
  coverage: {
    last_gross_day: "2026-11-02",
    gross_stale: false,
    last_net_month: "2026-10",
    seats_snapshot: "",
    seats_from: "",
    membership_from: "",
  },
  months: [month],
};

describe("AdminSourceStatus", () => {
  it("keeps missing metadata unknown, without claiming success or failure", () => {
    render(<AdminSourceStatus />);
    expect(screen.getByText("Siste dag med foreløpig forbruk: ukjent.")).toBeInTheDocument();
    expect(screen.getByText(/Innlesingsdato for månedsdata: ukjent/)).toBeInTheDocument();
    expect(screen.queryByText(/forsinket|fullført|ikke kontrollert|oppdatert/i)).not.toBeInTheDocument();
  });

  it.each(["net", "premium"] as const)("shows observed dates for %s, including a zero bill", (basis) => {
    render(<AdminSourceStatus data={{ ...data, months: [{ ...month, basis }] }} />);
    expect(screen.getByText("Siste dag med foreløpig forbruk: 2. november 2026.")).toBeInTheDocument();
    expect(screen.getByText(/Månedsdata for 2026-10 er innlest 3. november 2026/)).toBeInTheDocument();
    expect(screen.getByText(/Status for pågående innsamling er ukjent/)).toBeInTheDocument();
    expect(screen.queryByText(/fullført|verifisert|ikke kontrollert|oppdatert/i)).not.toBeInTheDocument();
  });

  it("preserves the stale warning from explicit coverage metadata", () => {
    render(<AdminSourceStatus data={{ ...data, coverage: { ...data.coverage, gross_stale: true } }} />);
    expect(screen.getByText(/Foreløpig forbruk er sist hentet for 2. november 2026/)).toBeInTheDocument();
    expect(screen.getByText(/Tallene for denne måneden kan være for lave/)).toBeInTheDocument();
  });

  it.each([
    { ...data, months: [{ ...month, loaded_at: "" }] },
    { ...data, months: [{ ...month, basis: "gross" as const }] },
    { ...data, year: 2027, coverage: { ...data.coverage, last_net_month: "" }, months: [] },
  ])("does not infer an import date or delay from absent monthly metadata", (missing) => {
    render(<AdminSourceStatus data={missing} />);
    expect(screen.getByText(/Innlesingsdato for månedsdata: ukjent/)).toBeInTheDocument();
    expect(screen.queryByText(/er innlest|forsinket|fullført|ikke kontrollert/i)).not.toBeInTheDocument();
  });
});
