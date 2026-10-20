import { render, screen } from "@testing-library/react";
import ErrorState, { SectionError } from "./error-state";

describe("SectionError", () => {
  it("renders the message as an alert without a heading or main", () => {
    render(<SectionError message="Kunne ikke hente prognosen." />);

    expect(screen.getByText("Kunne ikke hente prognosen.")).toBeInTheDocument();
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
    expect(screen.queryByRole("main")).not.toBeInTheDocument();
  });
});

describe("ErrorState", () => {
  it("renders default title and message", () => {
    render(<ErrorState message="Noe gikk galt" />);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Copilot Statistikk");
    expect(screen.getByText("Noe gikk galt")).toBeInTheDocument();
  });

  it("renders custom title", () => {
    render(<ErrorState title="Bruksstatistikk" message="Ingen data" />);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Bruksstatistikk");
  });

  it("applies error styling for messages starting with Feil", () => {
    render(<ErrorState message="Feil: kunne ikke laste data" />);

    const message = screen.getByText("Feil: kunne ikke laste data");
    expect(message).toHaveClass("text-red-500");
  });

  it("does not apply error styling for normal messages", () => {
    render(<ErrorState message="Ingen data tilgjengelig" />);

    const message = screen.getByText("Ingen data tilgjengelig");
    expect(message).not.toHaveClass("text-red-500");
  });
});
