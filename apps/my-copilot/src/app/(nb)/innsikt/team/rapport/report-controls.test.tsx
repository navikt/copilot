import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({
  getUser: vi.fn(async () => ({ email: "test@nav.no", groups: [] })),
  getUserToken: vi.fn(async () => "dev-token"),
  requireAdmin: vi.fn(async () => undefined),
}));

vi.mock("@/lib/cached-bigquery", () => ({
  getTeamYearOverview: vi.fn(async () => ({
    team_id: "1",
    team_slug: "teamkuhr",
    year: 2026,
    coverage: {
      membership_from: "2026-05-06",
      gross_from: "2026-06-15",
      last_usage_day: "2026-10-11",
      net_months: ["2026-09"],
      seats_snapshot: "2026-10-11",
      team_members_from: "2026-10-11",
    },
    months: [
      {
        month: "2026-09",
        basis: "net",
        users: 3,
        net_usd: 10.5,
        gross_usd: null,
        no_usage_net_usd: 0,
        seat_months: 3,
        licence_usd_estimate: 57,
        total_usd: 67.5,
        total_basis: "net",
      },
      {
        month: "2026-10",
        basis: "gross",
        users: 3,
        net_usd: null,
        gross_usd: 20,
        no_usage_net_usd: null,
        seat_months: 3,
        licence_usd_estimate: 57,
        total_usd: 77,
        total_basis: "gross",
      },
    ],
  })),
}));

vi.mock("@/lib/rapport-load", () => ({
  parseRapportYear: (y?: string) => Number(y ?? 2026),
  loadRapport: vi.fn(async () => ({
    schema: "copilot-rapport",
    version: 1,
    title: "Copilot i Nav: bruk og kostnad 2026",
    year: 2026,
    generated: "2026-10-13",
    period: { from: "2026-01-01", to: "2026-10-12" },
    currency: "USD",
    order: ["kostnad", "forbruk", "brukere", "modeller", "bruksomrade", "metode", "ikke_dekket"],
    sections: {
      kostnad: {
        title: "Kostnad",
        answers: ["1"],
        summary: "Sammendrag",
        facts: {},
        tables: [
          {
            id: "fakturert",
            caption: "Fakturert per måned",
            rows: [
              { month: "2026-09", billed_usd: 2000, preliminary: false },
              { month: "2026-10", billed_usd: 500, preliminary: true },
            ],
          },
        ],
        notes: [],
      },
      forbruk: { title: "Forbruk", answers: [], summary: "Forbruk", facts: {}, tables: [], notes: [] },
      brukere: { title: "Brukere", answers: [], summary: "Brukere", facts: {}, tables: [], notes: [] },
      modeller: {
        title: "Modeller",
        answers: [],
        summary: "Modeller",
        facts: {},
        tables: [
          {
            id: "familier",
            caption: "Modellfamilier",
            rows: [{ family: "Claude Opus", share_pct: 60, models: ["claude-opus-4.6"] }],
          },
        ],
        notes: [],
      },
      bruksomrade: { title: "Bruksområde", answers: [], summary: "Bruksområde", facts: {}, tables: [], notes: [] },
      metode: { title: "Slik har vi talt", answers: [], summary: "Metode", facts: {}, tables: [], notes: [] },
      ikke_dekket: { title: "Ikke dekket", answers: [], summary: "Ikke dekket", facts: {}, tables: [], notes: [] },
    },
  })),
}));

import { DocumentTitle } from "@/components/report-controls";
import TeamReportPage from "./page";
import RapportPage from "@/app/(nb)/admin/kostnad/rapport/page";

describe("DocumentTitle", () => {
  it("sets the title used as the PDF file name and restores it on unmount", () => {
    document.title = "Kostnadsrapport for team";
    const { unmount } = render(<DocumentTitle title="Copilot-kostnad teamkuhr 2026 (til og med 2026-10-11)" />);
    expect(document.title).toBe("Copilot-kostnad teamkuhr 2026 (til og med 2026-10-11)");
    unmount();
    expect(document.title).toBe("Kostnadsrapport for team");
  });
});

describe("Print reports (#1826)", () => {
  it("renders PrintHeader and print-safe monthly bars (striped for preliminary) on /innsikt/team/rapport", async () => {
    render(await TeamReportPage({ searchParams: Promise.resolve({ team: "1", year: "2026" }) }));
    expect(screen.getByTestId("print-header")).toHaveTextContent("Copilot-kostnad for teamkuhr, 2026");
    expect(screen.getByTestId("month-bar-2026-09")).not.toHaveClass("print-stripe");
    expect(screen.getByTestId("month-bar-2026-10")).toHaveClass("print-stripe");
  });

  it("renders PrintHeader and print-safe inline bars on /admin/kostnad/rapport", async () => {
    render(await RapportPage({ searchParams: Promise.resolve({ year: "2026" }) }));
    expect(screen.getByTestId("print-header")).toHaveTextContent("Copilot i Nav: bruk og kostnad 2026");
    expect(screen.getByTestId("bar-billed_usd-0")).not.toHaveClass("print-stripe");
    expect(screen.getByTestId("bar-billed_usd-1")).toHaveClass("print-stripe");
    expect(screen.getByTestId("bar-share_pct-0")).toHaveStyle({ width: "60%" });
  });
});
