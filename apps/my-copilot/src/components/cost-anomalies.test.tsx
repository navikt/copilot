import { render, screen } from "@testing-library/react";
import { vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUserToken: vi.fn() }));
vi.mock("@/lib/backend-api", () => ({ backendRequest: vi.fn() }));

import { AnomalyList, anomalyText, type Anomaly } from "./cost-anomalies";

describe("AnomalyList", () => {
  it("puts data gaps first and links each finding", () => {
    const anomalies: Anomaly[] = [
      { rule: "A3", date: "2026-09-23", value: 801, threshold: 10, model: "Claude Opus 5.5" },
      { rule: "A1", date: "2026-08-29", value: 460, threshold: 179 },
      { rule: "A7", date: "2026-09", value: 0, threshold: 5 },
    ];
    render(<AnomalyList anomalies={anomalies} />);
    const items = screen.getAllByRole("listitem");
    expect(items[0]).toHaveTextContent("Fakturert forbruk for 2026-09 er ikke lest inn");
    expect(items[1]).toHaveTextContent("Medianen for samme type dag");
    expect(items[2]).toHaveTextContent("Ny modell: Claude Opus 5.5");
    expect(screen.getAllByRole("link")).toHaveLength(3);
  });

  it("says so when there is nothing or the fetch failed", () => {
    const { rerender } = render(<AnomalyList anomalies={[]} />);
    expect(screen.getByText("Ingen avvik de siste 30 dagene.")).toBeInTheDocument();
    rerender(<AnomalyList anomalies={null} />);
    expect(screen.getByText("Vi klarte ikke å hente avvikene nå.")).toBeInTheDocument();
  });

  it("tells stale data from a single missing day", () => {
    expect(anomalyText({ rule: "A7", date: "2026-10-01", value: 0, threshold: 2 }).text).toMatch(/^Ingen nye/);
    expect(anomalyText({ rule: "A7", date: "", value: 0, threshold: 2 }).text).toBe("Det finnes ingen forbruksdata.");
    expect(anomalyText({ rule: "A7", date: "2026-10-01", value: 0, threshold: 0 }).text).toMatch(
      /^Forbruksdata mangler/
    );
  });
});
