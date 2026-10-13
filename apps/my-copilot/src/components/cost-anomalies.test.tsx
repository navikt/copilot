import { render, screen } from "@testing-library/react";
import { vi } from "vitest";

vi.mock("@/lib/backend-api", () => ({ backendRequest: vi.fn() }));
vi.mock("@/lib/cached-bigquery", () => ({}));

import { MonthInsightList } from "./cost-anomalies";

describe("MonthInsightList", () => {
  it("shows each statement with its link, lever and details in a fold", () => {
    render(
      <MonthInsightList
        insights={[
          {
            id: "M1",
            text: "Kredittene gikk tom.",
            details: ["Prognose."],
            href: "#prognose",
            action: "Senk grensen.",
          },
          { id: "M6", text: "Personer uten team.", href: "#per-maned", action: "Finn eiere." },
        ]}
      />
    );
    expect(screen.getAllByRole("listitem")[0]).toHaveTextContent("Kredittene gikk tom.");
    expect(screen.getAllByRole("link", { name: "Se grafen" }).map((l) => l.getAttribute("href"))).toEqual([
      "#prognose",
      "#per-maned",
    ]);
    expect(screen.getAllByText("Hva kan vi gjøre?")).toHaveLength(2);
    expect(screen.getAllByText("Detaljer")).toHaveLength(1);
  });

  it("says so when there is nothing to report", () => {
    render(<MonthInsightList insights={[]} />);
    expect(screen.getByText("Ingenting å melde denne måneden.")).toBeInTheDocument();
  });
});
