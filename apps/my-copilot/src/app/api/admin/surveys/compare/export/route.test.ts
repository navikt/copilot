import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: vi.fn(), isAdmin: vi.fn() }));
vi.mock("@/lib/survey", () => ({ fetchSurveyCompareExport: vi.fn() }));

import { isAdmin, getUserToken } from "@/lib/auth";
import { fetchSurveyCompareExport } from "@/lib/survey";
import { GET } from "./route";

const call = (ids = "q2-2026,q4-2026") => GET(new Request(`http://x/api/admin/surveys/compare/export?ids=${ids}`));

describe("GET /api/admin/surveys/compare/export", () => {
  beforeEach(() => {
    vi.mocked(fetchSurveyCompareExport).mockReset();
    vi.mocked(getUserToken).mockResolvedValue("token");
  });

  it("answers 404 to non-admins without fetching", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    expect((await call()).status).toBe(404);
    expect(fetchSurveyCompareExport).not.toHaveBeenCalled();
  });

  it("passes the ids and the CSV through, without cookies", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(fetchSurveyCompareExport).mockResolvedValue(
      new Response("kind\n", {
        headers: { "Content-Type": "text/csv; charset=utf-8", "Content-Disposition": "attachment", "Set-Cookie": "x=y" },
      })
    );
    const res = await call();
    expect(fetchSurveyCompareExport).toHaveBeenCalledWith("token", ["q2-2026", "q4-2026"]);
    expect(res.status).toBe(200);
    expect(res.headers.get("Set-Cookie")).toBeNull();
    expect(res.headers.get("Cache-Control")).toBe("no-store");
    expect(await res.text()).toBe("kind\n");
  });

  it("answers 400 to bad ids without fetching, 502 when copilot-survey fails", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    expect((await call("q2-2026")).status).toBe(400);
    expect((await call("Q2,x/../y")).status).toBe(400);
    expect(fetchSurveyCompareExport).not.toHaveBeenCalled();
    vi.mocked(fetchSurveyCompareExport).mockResolvedValue(null);
    expect((await call()).status).toBe(502);
  });
});
