import { render, screen, within } from "@testing-library/react";
import { UsageQualityTable } from "./usage-quality";
import type { UsageQualityDay } from "@/lib/usage-quality";

vi.mock("@/lib/backend-api", () => ({ backendRequest: vi.fn() }));

const day = (overrides: Partial<UsageQualityDay>): UsageQualityDay => ({
  day: "2026-11-05",
  scope: "enterprise",
  scope_id: "nav",
  ...overrides,
});

describe("UsageQualityTable", () => {
  it("keeps missing and null unknown while displaying real zero", () => {
    render(
      <UsageQualityTable
        days={[
          day({ day: "2026-11-01" }),
          day({ day: "2026-11-02", missing_client_telemetry_user_count: null }),
          day({ day: "2026-11-03", missing_client_telemetry_user_count: 0, below_usage_metrics_minimum_user_count: 6 }),
        ]}
      />
    );
    const old = screen.getByRole("rowheader", { name: "2026-11-01" }).closest("tr")!;
    expect(within(old).getAllByText("Ukjent")).toHaveLength(2);
    const zero = screen.getByRole("rowheader", { name: "2026-11-03" }).closest("tr")!;
    expect(within(zero).getByText("0")).toBeInTheDocument();
    expect(screen.getByText(/Av 3 dagsrapporter/)).toHaveTextContent("1 telemetritall og 1 versjonstall");
  });

  it("shows separate daily counts, activity and overlap warnings, not a combined total", () => {
    render(
      <UsageQualityTable
        days={[day({ missing_client_telemetry_user_count: 72, below_usage_metrics_minimum_user_count: 63 })]}
      />
    );
    expect(screen.getByText(/Siste rapportdag/)).toHaveTextContent(
      "Antall brukere uten klienttelemetri: 72. Antall under minstekravene til IDE eller Copilot-utvidelse: 63."
    );
    expect(screen.getByText(/Siste rapportdag/)).toHaveTextContent("ikke på tvers av dager");
    expect(screen.getByText(/teller fortsatt som aktive/)).toHaveTextContent("Flagget sier ikke hvorfor");
    expect(screen.queryByText("135")).not.toBeInTheDocument();
  });

  it("labels organization coverage without presenting it as enterprise-wide", () => {
    render(<UsageQualityTable days={[day({ scope: "organization", scope_id: "navikt" })]} />);
    expect(screen.getByText("organization: navikt")).toBeInTheDocument();
    expect(screen.getByText(/Organisasjonstall dekker ikke hele enterprise/)).toBeInTheDocument();
  });

  it("does not turn absent reports into zero users", () => {
    render(<UsageQualityTable days={[]} />);
    expect(screen.getByText("Ingen dagsrapporter om datakvalitet ennå.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
