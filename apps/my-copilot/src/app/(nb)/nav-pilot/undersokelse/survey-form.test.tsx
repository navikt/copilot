import { fireEvent, render, screen } from "@testing-library/react";
import type { Survey } from "@/lib/survey";
import { SurveyForm } from "./survey-form";

const sendSurvey = vi.fn();
vi.mock("./actions", () => ({ sendSurvey: (...args: unknown[]) => sendSurvey(...args) }));

const survey: Survey = {
  id: "s",
  title: "S",
  ends: "2099-12-31",
  questions: [
    {
      id: "grid",
      type: "matrix",
      text: "Hvor enig er du?",
      required: true,
      min: 1,
      max: 3,
      labels: ["Uenig", "Nøytral", "Enig"],
      items: [
        { id: "fast", text: "Den er rask." },
        { id: "safe", text: "Den er trygg." },
      ],
    },
  ],
};

describe("SurveyForm matrix", () => {
  it("is a table of radios named by statement and step, sent as one answer per item", async () => {
    sendSurvey.mockResolvedValue({ status: "recorded" });
    render(<SurveyForm survey={survey} />);
    expect(screen.getByRole("group", { name: "Hvor enig er du?" })).toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(6);

    fireEvent.click(screen.getByRole("radio", { name: "Den er rask. Enig" }));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText("Hvor enig er du?: Svar på alle påstandene.")).toBeInTheDocument();
    expect(sendSurvey).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("radio", { name: "Den er trygg. Uenig" }));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText("Takk! Svaret ditt er sendt.")).toBeInTheDocument();
    expect(sendSurvey).toHaveBeenCalledWith("s", { fast: 3, safe: 1 });
  });
});
