import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/auth", () => ({ getUserToken: vi.fn() }));
vi.mock("@/lib/survey", () => ({ submitAnswers: vi.fn() }));

import { getUserToken } from "@/lib/auth";
import { submitAnswers } from "@/lib/survey";
import { POST } from "./route";

const post = (body: string) =>
  POST(new Request("http://x/api/survey/s%201/responses", { method: "POST", body }), {
    params: Promise.resolve({ id: "s 1" }),
  });

describe("POST /api/survey/[id]/responses", () => {
  beforeEach(() => {
    vi.mocked(submitAnswers).mockReset().mockResolvedValue({ status: "recorded" });
    vi.mocked(getUserToken).mockResolvedValue("token");
  });

  it("forwards the posted answers unchanged, so the payload to copilot-survey stays the same", async () => {
    const answers = { fast: 3, verktoy: ["Ingen"], "verktoy.other": "x", fritekst: "æøå «»" };
    const res = await post(JSON.stringify(answers));
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ status: "recorded" });
    expect(submitAnswers).toHaveBeenCalledWith("token", "s 1", answers);
    expect(JSON.stringify(vi.mocked(submitAnswers).mock.calls[0][2])).toBe(JSON.stringify(answers));
  });

  it("passes the result through", async () => {
    vi.mocked(submitAnswers).mockResolvedValue({ status: "duplicate" });
    expect(await (await post("{}")).json()).toEqual({ status: "duplicate" });
  });

  it("answers 401 without a token, and sends nothing", async () => {
    vi.mocked(getUserToken).mockResolvedValue(null);
    expect((await post("{}")).status).toBe(401);
    expect(submitAnswers).not.toHaveBeenCalled();
  });

  it("refuses a body that is not an answers object", async () => {
    for (const body of ["nope", "[]", "null"]) expect((await post(body)).status).toBe(400);
    expect(submitAnswers).not.toHaveBeenCalled();
  });

  it("answers error when the send throws", async () => {
    vi.mocked(submitAnswers).mockRejectedValue(new Error("boom"));
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(await (await post("{}")).json()).toEqual({ status: "error" });
  });
});
