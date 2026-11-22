import { fireEvent, render, screen } from "@testing-library/react";
import { SiteHeader, type HeaderLabels } from "./site-nav";

vi.mock("next/navigation", () => ({ usePathname: () => "/" }));
vi.mock("@/components/navigation/site-search", () => ({ SiteSearch: () => null }));
vi.mock("@/components/budget-usage", () => ({
  BudgetUsage: () => null,
  budgetHigh: () => false,
  budgetText: (t: string) => t,
  useBudgetPct: () => null,
}));

const labels = {
  lang: "nb",
  userMenu: "Brukermeny",
  subscription: "Abonnement",
  subscriptionHref: "/abonnement",
  showSection: "Vis menyen for {name}",
  menu: "Meny",
} as HeaderLabels;

describe("SiteHeader user menu", () => {
  it("shows Admin to admins", () => {
    render(<SiteHeader labels={labels} userName="Ada Admin" admin />);
    fireEvent.click(screen.getByRole("button", { name: /Ada Admin, Brukermeny/ }));
    expect(screen.getByRole("link", { name: "Admin" })).toHaveAttribute("href", "/admin");
  });

  it("hides Admin from everyone else", () => {
    render(<SiteHeader labels={labels} userName="Ola Nordmann" />);
    fireEvent.click(screen.getByRole("button", { name: /Ola Nordmann, Brukermeny/ }));
    expect(screen.getByRole("link", { name: "Abonnement" })).toBeInTheDocument();
    expect(screen.queryByText("Admin")).not.toBeInTheDocument();
  });
});

describe("SiteHeader mobile menu", () => {
  it("names the section each show-menu button opens", () => {
    render(<SiteHeader labels={labels} />);
    fireEvent.click(screen.getByRole("button", { name: "Meny" }));
    expect(screen.getByRole("button", { name: "Vis menyen for nav-pilot" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Vis menyen for MCP" })).toBeInTheDocument();
  });
});
