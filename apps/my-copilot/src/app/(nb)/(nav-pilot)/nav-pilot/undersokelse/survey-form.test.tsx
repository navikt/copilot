import { fireEvent, render, screen, within } from "@testing-library/react";
import { canPreviewSurveys, type Survey } from "@/lib/survey";
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

describe("SurveyForm preview", () => {
  it("checks the answers but never sends them, and only listed emails may preview", async () => {
    sendSurvey.mockReset();
    render(<SurveyForm survey={survey} preview />);
    fireEvent.click(screen.getByRole("radio", { name: "Den er rask. Enig" }));
    fireEvent.click(screen.getByRole("radio", { name: "Den er trygg. Enig" }));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText(/ikke sendt eller lagret/)).toBeInTheDocument();
    expect(sendSurvey).not.toHaveBeenCalled();

    expect(canPreviewSurveys("Eier@nav.no", "a@nav.no, eier@nav.no")).toBe(true);
    expect(canPreviewSurveys("andre@nav.no", "eier@nav.no")).toBe(false);
    expect(canPreviewSurveys("", "")).toBe(false);
  });
});

describe("SurveyForm matrix", () => {
  it("is a table of radios named by statement and step, sent as one answer per item", async () => {
    sendSurvey.mockResolvedValue({ status: "recorded" });
    render(<SurveyForm survey={survey} />);
    expect(screen.getByRole("group", { name: "Hvor enig er du?" })).toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(12); // table, and the stacked groups shown below md

    fireEvent.click(screen.getByRole("radio", { name: "Den er rask. Enig" }));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText("Hvor enig er du?: Svar på alle påstandene.")).toBeInTheDocument();
    expect(sendSurvey).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("radio", { name: "Den er trygg. Uenig" }));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText("Takk! Svaret ditt er sendt.")).toBeInTheDocument();
    expect(sendSurvey).toHaveBeenCalledWith("s", { fast: 3, safe: 1 });
  });

  it("stacks each statement as its own radio group below md, with the same answers and error focus", async () => {
    sendSurvey.mockReset().mockResolvedValue({ status: "recorded" });
    render(<SurveyForm survey={survey} />);
    const fast = screen.getByRole("radiogroup", { name: "Den er rask." });
    fireEvent.click(within(fast).getByRole("radio", { name: "Enig" }));
    expect(screen.getByRole("radio", { name: "Den er rask. Enig" })).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    const item = await screen.findByRole("link", { name: "Hvor enig er du?: Svar på alle påstandene." });
    expect(item).toHaveAttribute("href", "#q-grid");
    expect(document.getElementById("q-grid")).toContainElement(fast);

    fireEvent.click(
      within(screen.getByRole("radiogroup", { name: "Den er trygg." })).getByRole("radio", { name: "Uenig" })
    );
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText("Takk! Svaret ditt er sendt.")).toBeInTheDocument();
    expect(sendSurvey).toHaveBeenCalledWith("s", { fast: 3, safe: 1 });
  });

  it("skips a required grid whole when an earlier answer says so", async () => {
    sendSurvey.mockReset().mockResolvedValue({ status: "recorded" });
    const skipping: Survey = {
      ...survey,
      questions: [
        { id: "verktoy", type: "multi", text: "Hvilke verktøy?", required: true, options: ["Copilot", "Ingen"] },
        { ...survey.questions[0], skip_if: { question: "verktoy", answer: "Ingen" } },
      ],
    };
    render(<SurveyForm survey={skipping} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Ingen" }));
    expect(screen.queryByRole("group", { name: "Hvor enig er du?" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText("Takk! Svaret ditt er sendt.")).toBeInTheDocument();
    expect(sendSurvey).toHaveBeenCalledWith("s", { verktoy: ["Ingen"] });
  });
});
