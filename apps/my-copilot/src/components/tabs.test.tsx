import { afterEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import Tabs from "./tabs";

const testTabs = [
  { id: "usage", label: "Bruk", content: <div>Usage content</div> },
  { id: "adoption", label: "Adopsjon", content: <div>Adoption content</div> },
  { id: "cost", label: "Kostnad", content: <div>Cost content</div> },
];

describe("Tabs", () => {
  afterEach(() => {
    window.history.replaceState(null, "", "/");
  });

  it("renders all tab buttons", () => {
    render(<Tabs tabs={testTabs} />);

    expect(screen.getByRole("tab", { name: "Bruk" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Adopsjon" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Kostnad" })).toBeInTheDocument();
  });

  it("shows first tab content by default", () => {
    render(<Tabs tabs={testTabs} />);

    expect(screen.getByText("Usage content")).toBeInTheDocument();
    expect(screen.queryByText("Adoption content")).not.toBeInTheDocument();
  });

  it("shows specified default tab", () => {
    render(<Tabs tabs={testTabs} defaultTab="adoption" />);

    expect(screen.getByText("Adoption content")).toBeInTheDocument();
    expect(screen.queryByText("Usage content")).not.toBeInTheDocument();
  });

  it("activates tab from hash on initial load", () => {
    window.history.replaceState(null, "", "#adoption");

    render(<Tabs tabs={testTabs} enableHashNavigation />);

    expect(screen.getByRole("tab", { name: "Adopsjon" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Adoption content")).toBeInTheDocument();
  });

  it("switches tab on click", () => {
    render(<Tabs tabs={testTabs} />);

    fireEvent.click(screen.getByRole("tab", { name: "Kostnad" }));
    expect(screen.getByText("Cost content")).toBeInTheDocument();
  });

  it("marks active tab with aria-selected", () => {
    render(<Tabs tabs={testTabs} />);

    expect(screen.getByRole("tab", { name: "Bruk" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "Adopsjon" })).toHaveAttribute("aria-selected", "false");
  });

  it("has tabpanel with correct role", () => {
    render(<Tabs tabs={testTabs} />);

    expect(screen.getByRole("tabpanel")).toBeInTheDocument();
  });

  it("has tablist navigation", () => {
    render(<Tabs tabs={testTabs} />);

    expect(screen.getByRole("tablist")).toBeInTheDocument();
  });
});
