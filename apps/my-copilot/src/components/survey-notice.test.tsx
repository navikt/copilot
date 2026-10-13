import { act, render, screen } from "@testing-library/react";
import { SurveyNotice } from "./survey-notice";

describe("SurveyNotice", () => {
  it("links to the survey while one is open", async () => {
    await act(async () => {
      render(<SurveyNotice titles={Promise.resolve(["Utviklerundersøkelsen"])} />);
    });
    expect(await screen.findByRole("link", { name: "Svar i nettleseren" })).toHaveAttribute(
      "href",
      "/nav-pilot/undersokelse"
    );
    expect(screen.getByText(/Utviklerundersøkelsen er åpen/)).toBeInTheDocument();
    expect(screen.getByRole("complementary", { name: "Brukerundersøkelse" })).toBeInTheDocument();
    expect(screen.getByText("nav-pilot survey")).toHaveClass("whitespace-nowrap");
  });

  it("shows nothing when no survey is open", async () => {
    let container!: HTMLElement;
    await act(async () => {
      ({ container } = render(<SurveyNotice titles={Promise.resolve([])} />));
    });
    expect(container).toBeEmptyDOMElement();
  });
});
