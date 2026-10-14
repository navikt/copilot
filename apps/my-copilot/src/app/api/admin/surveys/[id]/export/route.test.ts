import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: vi.fn(), isAdmin: vi.fn() }));
vi.mock("@/lib/survey", () => ({ fetchSurveyExport: vi.fn() }));

import { isAdmin, getUserToken } from "@/lib/auth";
import { fetchSurveyExport } from "@/lib/survey";
import { GET } from "./route";

const call = () =>
  GET(new Request("http://x/api/admin/surveys/q4-2026/export"), { params: Promise.resolve({ id: "q4-2026" }) });

describe("GET /api/admin/surveys/[id]/export", () => {
  beforeEach(() => {
    vi.mocked(fetchSurveyExport).mockReset();
    vi.mocked(getUserToken).mockResolvedValue("token");
  });

  it("answers 404 to non-admins without fetching", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    expect((await call()).status).toBe(404);
    expect(fetchSurveyExport).not.toHaveBeenCalled();
  });

  it("passes the CSV and its headers through", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(fetchSurveyExport).mockResolvedValue(
      new Response("﻿overall\n3\n", {
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition": 'attachment; filename="q4-2026-svar.csv"',
          "Set-Cookie": "x=y",
        },
      })
    );
    const res = await call();
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("text/csv; charset=utf-8");
    expect(res.headers.get("Content-Disposition")).toBe('attachment; filename="q4-2026-svar.csv"');
    expect(res.headers.get("Cache-Control")).toBe("no-store");
    expect(res.headers.get("Set-Cookie")).toBeNull();
    expect(new Uint8Array(await res.arrayBuffer()).slice(0, 3)).toEqual(new Uint8Array([0xef, 0xbb, 0xbf]));
  });

  it("keeps 410 with its message", async () => {
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(fetchSurveyExport).mockResolvedValue(new Response("deleted", { status: 410 }));
    const res = await call();
    expect(res.status).toBe(410);
    expect(await res.text()).toBe("deleted");
  });
});
