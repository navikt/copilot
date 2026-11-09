import { render, screen } from "@testing-library/react";
import { vi } from "vitest";
import { getAdminMonthlyOverview, getCohortRetention, getCopilotPRsMonthly } from "@/lib/cached-bigquery";
import MonthlyOwnerDraftPage from "./page";

vi.mock("next/navigation", () => ({ usePathname: () => "/admin/kostnad/manedsutkast" }));
vi.mock("@/lib/auth", () => ({ requireAdmin: vi.fn(), getUserToken: vi.fn().mockResolvedValue("test-token") }));
vi.mock("@/lib/cached-bigquery", () => ({
  getAdminMonthlyOverview: vi.fn(),
  getCohortRetention: vi.fn(),
  getCopilotPRsMonthly: vi.fn(),
}));

describe("monthly owner preview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-11-05T00:00:00Z"));
    vi.mocked(getAdminMonthlyOverview).mockResolvedValue({
      year: 2026,
      coverage: {
        membership_from: "",
        last_gross_day: "",
        gross_stale: false,
        last_net_month: "2026-10",
        seats_snapshot: "",
        seats_from: "",
      },
      months: [
        {
          month: "2026-10",
          basis: "net",
          enterprise_net_usd: 100,
          assigned_net_usd: 80,
          residual_net_usd: 20,
          residual_pct: null,
          no_team_net_usd: null,
          no_usage_net_usd: null,
          gross_usd: null,
          estimated_net_usd: null,
          seats: null,
          licence_usd_estimate: null,
          total_usd: null,
          loaded_at: "2026-11-04T12:00:00Z",
        },
      ],
    });
    vi.mocked(getCohortRetention).mockResolvedValue({ cohorts: [], error: null });
    vi.mocked(getCopilotPRsMonthly).mockResolvedValue({ months: [], error: null });
  });
  afterEach(() => vi.useRealTimers());

  it("renders a formatted collection date and keeps source failures distinct from empty data", async () => {
    vi.mocked(getCohortRetention).mockResolvedValue({ cohorts: [], error: "unavailable" });
    render(await MonthlyOwnerDraftPage());
    expect(screen.getByText(/Innsamlet: 4. november 2026/)).toBeInTheDocument();
    expect(screen.getByText(/Kunne ikke hente kohortretensjon/)).toBeInTheDocument();
    expect(screen.getByText(/PR-tall er ikke tilgjengelige/)).toBeInTheDocument();
    expect(screen.getByText(/forrige kalendermåned i UTC/)).toBeInTheDocument();
  });

  it("passes PR fetch failures to the draft independently of cohort data", async () => {
    vi.mocked(getCopilotPRsMonthly).mockResolvedValue({ months: [], error: "unavailable" });
    render(await MonthlyOwnerDraftPage());
    expect(screen.getByText(/Kunne ikke hente PR-tall/)).toBeInTheDocument();
    expect(screen.getByText(/Kohortretensjon er ikke tilgjengelig/)).toBeInTheDocument();
  });

  it("does not fetch usage while the immediately previous month is pending", async () => {
    const admin = await getAdminMonthlyOverview(2026, "test-token");
    vi.mocked(getAdminMonthlyOverview).mockResolvedValue({ ...admin, months: [] });
    render(await MonthlyOwnerDraftPage());
    expect(screen.getByRole("heading", { name: "Venter på innsamling" })).toBeInTheDocument();
    expect(getCohortRetention).not.toHaveBeenCalled();
    expect(getCopilotPRsMonthly).not.toHaveBeenCalled();
  });

  it.each([false, true])(
    "keeps January ready when December comparison is missing or fails, failure=%s",
    async (failed) => {
      const admin = await getAdminMonthlyOverview(2026, "test-token");
      vi.mocked(getAdminMonthlyOverview).mockReset();
      vi.setSystemTime(new Date("2027-02-05T00:00:00Z"));
      vi.mocked(getAdminMonthlyOverview).mockResolvedValueOnce({
        ...admin,
        year: 2027,
        coverage: { ...admin.coverage, last_net_month: "2027-01" },
        months: [{ ...admin.months[0], month: "2027-01", loaded_at: "2027-02-04T12:00:00Z" }],
      });
      if (failed) vi.mocked(getAdminMonthlyOverview).mockRejectedValueOnce(new Error("unavailable"));
      else vi.mocked(getAdminMonthlyOverview).mockResolvedValueOnce({ ...admin, months: [] });
      vi.mocked(getCohortRetention).mockResolvedValue({
        cohorts: [{ cohort_month: "2026-12", cohort_size: 10, m1: 50, m3: null, m6: null }],
        error: null,
      });
      vi.mocked(getCopilotPRsMonthly).mockResolvedValue({
        months: [{ month: "2027-01", created_by_copilot: 3, reviewed_by_copilot: 7, days: 31 }],
        error: null,
      });

      render(await MonthlyOwnerDraftPage());
      expect(getAdminMonthlyOverview).toHaveBeenNthCalledWith(1, 2027, "test-token");
      expect(getAdminMonthlyOverview).toHaveBeenNthCalledWith(2, 2026, "test-token");
      expect(getAdminMonthlyOverview).toHaveBeenCalledTimes(2);
      expect(screen.getByRole("heading", { name: "Utkast til gjennomgang" })).toBeInTheDocument();
      expect(screen.getByText(/10 brukere.*50 %.*2027-01/)).toBeInTheDocument();
      expect(screen.getByText(/3 PR-er.*7.*31 dager/)).toBeInTheDocument();
      expect(screen.getByText(/nettokostnad.*100 USD/)).toBeInTheDocument();
      expect(screen.getByText(/80 USD.*20 USD/)).toBeInTheDocument();
      if (failed) {
        expect(screen.getByText("Kunne ikke hente forrige måned. Prøv igjen før gjennomgang.")).toBeInTheDocument();
        expect(screen.queryByText(/Kostnadsendring er ikke tilgjengelig/)).not.toBeInTheDocument();
      } else {
        expect(screen.getByText(/Kostnadsendring er ikke tilgjengelig/)).toBeInTheDocument();
        expect(screen.queryByText(/Kunne ikke hente forrige måned/)).not.toBeInTheDocument();
      }
    }
  );

  it("uses the 2026 overview in January without fetching an older comparison year", async () => {
    const admin = await getAdminMonthlyOverview(2026, "test-token");
    vi.mocked(getAdminMonthlyOverview).mockReset();
    vi.setSystemTime(new Date("2027-01-05T00:00:00Z"));
    vi.mocked(getAdminMonthlyOverview).mockResolvedValue({
      ...admin,
      coverage: { ...admin.coverage, last_net_month: "2026-12" },
      months: [
        { ...admin.months[0], month: "2026-11" },
        { ...admin.months[0], month: "2026-12" },
      ],
    });
    render(await MonthlyOwnerDraftPage());
    expect(getAdminMonthlyOverview).toHaveBeenCalledExactlyOnceWith(2026, "test-token");
    expect(screen.getByRole("heading", { name: "Månedsutkast for Copilot, 2026-12" })).toBeInTheDocument();
    expect(screen.getByText(/endret med 0 USD siden 2026-11/)).toBeInTheDocument();
  });
});
