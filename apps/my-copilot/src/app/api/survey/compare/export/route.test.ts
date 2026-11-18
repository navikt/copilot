import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: vi.fn() }));
vi.mock("@/lib/survey", () => ({ fetchPublishedSeriesExport: vi.fn() }));

import { getUser, getUserToken } from "@/lib/auth";
import { fetchPublishedSeriesExport } from "@/lib/survey";
import { GET } from "./route";

const call = (series = "utviklerundersokelsen") =>
  GET(new Request(`http://x/api/survey/compare/export?series=${series}`));

describe("GET /api/survey/compare/export", () => {
  beforeEach(() => {
    vi.mocked(fetchPublishedSeriesExport).mockReset();
    vi.mocked(getUser).mockResolvedValue({ navIdent: "Z999999" } as never);
    vi.mocked(getUserToken).mockResolvedValue("token");
  });

  it("answers 401 without a user, without fetching", async () => {
    vi.mocked(getUser).mockResolvedValue(null);
    expect((await call()).status).toBe(401);
    expect(fetchPublishedSeriesExport).not.toHaveBeenCalled();
  });

  it("passes the series and the CSV through, without cookies", async () => {
    vi.mocked(fetchPublishedSeriesExport).mockResolvedValue(
      new Response("kind\n", {
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition": "attachment",
          "Set-Cookie": "x=y",
        },
      })
    );
    const res = await call();
    expect(fetchPublishedSeriesExport).toHaveBeenCalledWith("token", "utviklerundersokelsen");
    expect(res.status).toBe(200);
    expect(res.headers.get("Set-Cookie")).toBeNull();
    expect(res.headers.get("Cache-Control")).toBe("no-store");
    expect(await res.text()).toBe("kind\n");
  });

  it("answers 400 to a bad series, 404 when unpublished, 502 when copilot-survey fails", async () => {
    expect((await call("../x")).status).toBe(400);
    expect((await call("")).status).toBe(400);
    expect(fetchPublishedSeriesExport).not.toHaveBeenCalled();
    vi.mocked(fetchPublishedSeriesExport).mockResolvedValue(new Response("", { status: 404 }));
    expect((await call()).status).toBe(404);
    vi.mocked(fetchPublishedSeriesExport).mockResolvedValue(null);
    expect((await call()).status).toBe(502);
  });
});
