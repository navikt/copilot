import { render, screen } from "@testing-library/react";
import EditorUsageTable from "./editor-usage-table";

describe("EditorUsageTable", () => {
  it("renders observed dates, missing counts and a valid zero", () => {
    render(
      <EditorUsageTable
        editors={[
          {
            editor: "vscode",
            first_day: "2026-10-23",
            last_day: "2026-11-05",
            days: 14,
            generations: 0,
            acceptances: null,
            interactions: 120,
          },
        ]}
      />
    );
    for (const text of ["vscode", "2026-10-23", "2026-11-05", "14", "0", "Ukjent", "120"]) {
      expect(screen.getByText(text)).toBeInTheDocument();
    }
    expect(screen.queryByText(/brukere/i)).not.toBeInTheDocument();
  });
});
