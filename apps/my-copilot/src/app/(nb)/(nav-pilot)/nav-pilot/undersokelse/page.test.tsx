import { render, screen } from "@testing-library/react";
import type { Survey } from "@/lib/survey";
import SurveyPage from "./page";

const survey: Survey = { id: "q4-2026", title: "Q4", ends: "2026-12-01", questions: [] };
const mocks = vi.hoisted(() => ({ status: vi.fn(), preview: vi.fn(), admin: false }));

vi.mock("next/navigation", () => ({ usePathname: () => "/nav-pilot/undersokelse" }));
vi.mock("@/lib/auth", () => ({
  getUser: async () => ({}),
  getUserToken: async () => "token",
  isAdmin: () => mocks.admin,
}));
vi.mock("@/lib/survey", async (original) => ({
  ...(await original<typeof import("@/lib/survey")>()),
  getActiveSurveys: async () => ({ status: "ok", surveys: [survey] }),
  getSurveyPreview: mocks.preview,
  getSurveyStatus: mocks.status,
}));
vi.mock("./survey-form", () => ({
  SurveyForm: ({ preview }: { preview?: boolean }) => <form aria-label={preview ? "forhåndsvisning" : "skjema"} />,
}));

async function show(params: { forhandsvis?: string } = {}) {
  render(await SurveyPage({ searchParams: Promise.resolve(params) }));
}

describe("undersøkelsessiden", () => {
  beforeEach(() => {
    mocks.status.mockReset();
    mocks.preview.mockReset().mockResolvedValue(survey);
    mocks.admin = false;
  });

  it("takker i stedet for skjemaet når brukeren har svart", async () => {
    mocks.status.mockResolvedValue(true);
    await show();
    expect(screen.getByText("Du har svart på undersøkelsen. Takk!")).toBeInTheDocument();
    expect(screen.queryByRole("form")).toBeNull();
  });

  it.each([false, undefined])("viser skjemaet når status er %s", async (answered) => {
    mocks.status.mockResolvedValue(answered);
    await show();
    expect(screen.getByRole("form", { name: "skjema" })).toBeInTheDocument();
    expect(screen.queryByText(/Du har svart/)).toBeNull();
  });

  it("forhåndsvisning for admin ser bort fra status", async () => {
    mocks.admin = true;
    mocks.status.mockResolvedValue(true);
    await show({ forhandsvis: "q4-2026" });
    expect(screen.getByRole("form", { name: "forhåndsvisning" })).toBeInTheDocument();
    expect(mocks.status).not.toHaveBeenCalled();
  });

  it("henter aldri forhåndsvisning for andre enn admin", async () => {
    await show({ forhandsvis: "q4-2026" });
    expect(mocks.preview).not.toHaveBeenCalled();
  });

  it("henter forhåndsvisningen for admin", async () => {
    mocks.admin = true;
    await show({ forhandsvis: "q4-2026" });
    expect(mocks.preview).toHaveBeenCalledWith("q4-2026");
  });
});
