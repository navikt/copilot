import { vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), isAdmin: vi.fn() }));
vi.mock("@/lib/survey", () => ({
  getActiveSurveys: vi.fn(async () => ({ status: "ok", surveys: [] })),
  getSurveyPreview: vi.fn(async () => undefined),
  SURVEY_SCHEDULE: { starts: "2026-10-01", ends: "2026-10-31" },
  surveyWindow: () => "during",
}));

import { getUser, isAdmin } from "@/lib/auth";
import { getSurveyPreview } from "@/lib/survey";
import SurveyPage from "./page";

const render = () => SurveyPage({ searchParams: Promise.resolve({ forhandsvis: "q4-2026" }) });

describe("SurveyPage preview", () => {
  beforeEach(() => {
    vi.mocked(getUser).mockResolvedValue({ firstName: "A", lastName: "B", email: "a@nav.no", groups: [] });
    vi.mocked(getSurveyPreview).mockClear();
  });

  it("never loads a preview for a non-admin", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    await render();
    expect(getSurveyPreview).not.toHaveBeenCalled();
  });

  it("loads the preview for an admin", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    await render();
    expect(getSurveyPreview).toHaveBeenCalledWith("q4-2026");
  });
});
