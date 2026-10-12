import { render } from "@testing-library/react";
import { DocumentTitle } from "./report-controls";

describe("DocumentTitle", () => {
  it("sets the title used as the PDF file name and restores it on unmount", () => {
    document.title = "Kostnadsrapport for team";
    const { unmount } = render(<DocumentTitle title="Copilot-kostnad teamkuhr 2026 (til og med 2026-10-11)" />);
    expect(document.title).toBe("Copilot-kostnad teamkuhr 2026 (til og med 2026-10-11)");
    unmount();
    expect(document.title).toBe("Kostnadsrapport for team");
  });
});
