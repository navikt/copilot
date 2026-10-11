import { render, screen } from "@testing-library/react";
import type { Survey } from "@/lib/survey";
import { SurveyNotice } from "./survey-notice";

const survey: Survey = { id: "q4", title: "Utviklerundersøkelsen", ends: "2099-12-31", questions: [] };

describe("SurveyNotice", () => {
  it("links to the survey while one is open", () => {
    render(<SurveyNotice active={{ status: "ok", surveys: [survey] }} />);
    expect(screen.getByText(/Utviklerundersøkelsen er åpen/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Svar her" })).toHaveAttribute("href", "/nav-pilot/undersokelse");
  });

  it("shows nothing when no survey is open or the service fails", () => {
    const { container, rerender } = render(<SurveyNotice active={{ status: "ok", surveys: [] }} />);
    expect(container).toBeEmptyDOMElement();
    rerender(<SurveyNotice active={{ status: "error" }} />);
    expect(container).toBeEmptyDOMElement();
  });
});
