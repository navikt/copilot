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

const pick = (statement: string, step: string) =>
  within(screen.getByRole("group", { name: statement })).getByRole("radio", { name: step });

describe("SurveyForm preview", () => {
  it("checks the answers but never sends them, and only listed emails may preview", async () => {
    sendSurvey.mockReset();
    render(<SurveyForm survey={survey} preview />);
    fireEvent.click(pick("Den er rask.", "Enig"));
    fireEvent.click(pick("Den er trygg.", "Enig"));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText(/ikke sendt eller lagret/)).toBeInTheDocument();
    expect(sendSurvey).not.toHaveBeenCalled();

    expect(canPreviewSurveys("Eier@nav.no", "a@nav.no, eier@nav.no")).toBe(true);
    expect(canPreviewSurveys("andre@nav.no", "eier@nav.no")).toBe(false);
    expect(canPreviewSurveys("", "")).toBe(false);
  });
});

describe("SurveyForm matrix", () => {
  it("is one button row per statement, nothing preselected, sent as one answer per item", async () => {
    sendSurvey.mockReset().mockResolvedValue({ status: "recorded" });
    render(<SurveyForm survey={survey} />);
    expect(screen.getByRole("group", { name: "Hvor enig er du?" })).toBeInTheDocument();
    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(6); // one row, same on every width
    radios.forEach((r) => expect(r).not.toBeChecked());

    fireEvent.click(pick("Den er rask.", "Enig"));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    const item = await screen.findByRole("link", { name: "Hvor enig er du?: Svar på alle påstandene." });
    expect(item).toHaveAttribute("href", "#q-grid");
    expect(document.getElementById("q-grid")).toContainElement(screen.getByRole("group", { name: "Den er rask." }));
    expect(sendSurvey).not.toHaveBeenCalled();

    fireEvent.click(pick("Den er trygg.", "Uenig"));
    fireEvent.click(screen.getByRole("button", { name: "Send svar" }));
    expect(await screen.findByText("Takk! Svaret ditt er sendt.")).toBeInTheDocument();
    expect(sendSurvey).toHaveBeenCalledWith("s", { fast: 3, safe: 1 });
  });

  it("groups each row's radios under one name, so arrow keys move within the row", () => {
    render(<SurveyForm survey={survey} />);
    // jsdom has no native arrow-key radio handling; the browser does it for radios sharing a name.
    const row = within(screen.getByRole("group", { name: "Den er rask." })).getAllByRole("radio");
    expect(row[0].closest("div")?.getAttribute("style")).toContain("repeat(3, minmax(0, 1fr))"); // one column per step
    expect(new Set(row.map((r) => r.getAttribute("name")))).toEqual(new Set(["q-fast"]));
    expect(row.map((r) => r.getAttribute("value"))).toEqual(["1", "2", "3"]);
    row.forEach((r) => expect(r).not.toBeChecked());
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

describe("SurveyForm multi limit", () => {
  it("disables the unpicked boxes once max_choices are picked, and says why", () => {
    const multi: Survey = {
      ...survey,
      questions: [{ id: "m", type: "multi", text: "Hva?", required: true, options: ["a", "b", "c"], max_choices: 2 }],
    };
    render(<SurveyForm survey={multi} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "a" }));
    expect(screen.getByRole("checkbox", { name: "c" })).toBeEnabled();
    fireEvent.click(screen.getByRole("checkbox", { name: "b" }));
    expect(screen.getByRole("checkbox", { name: "c" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "a" })).toBeEnabled();
    expect(screen.getByText(/Du har valgt 2/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "a" }));
    expect(screen.getByRole("checkbox", { name: "c" })).toBeEnabled();
  });

  it("keeps the exclusive option open at the limit, and everything open once it is picked", () => {
    const multi: Survey = {
      ...survey,
      questions: [
        { id: "m", type: "multi", text: "Hva?", options: ["a", "b", "ingen"], max_choices: 1, exclusive: "ingen" },
      ],
    };
    render(<SurveyForm survey={multi} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "a" }));
    expect(screen.getByRole("checkbox", { name: "b" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "ingen" })).toBeEnabled();
    fireEvent.click(screen.getByRole("checkbox", { name: "ingen" }));
    expect(screen.getByRole("checkbox", { name: "a" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "b" })).toBeEnabled();
    fireEvent.click(screen.getByRole("checkbox", { name: "b" }));
    expect(screen.getByRole("checkbox", { name: "ingen" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "a" })).toBeDisabled();
  });
});
