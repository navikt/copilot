import { render, screen } from "@testing-library/react";
import FleetFaqPage from "./page";

vi.mock("next/navigation", () => ({ usePathname: () => "/mcp/tilby-server/vanlige-sporsmal" }));

describe("Fleet: vanlige spørsmål", () => {
  it("renders the summary first and the enable command", () => {
    render(<FleetFaqPage />);
    expect(screen.getAllByRole("heading", { level: 2 })[0]).toHaveTextContent("Kort fortalt");
    expect(screen.getAllByText("nav-pilot mcp enable <navn>").length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: "ki-utvikling.ekstern.dev.nav.no/fleet" })).toHaveAttribute(
      "href",
      "https://ki-utvikling.ekstern.dev.nav.no/fleet"
    );
  });
});
