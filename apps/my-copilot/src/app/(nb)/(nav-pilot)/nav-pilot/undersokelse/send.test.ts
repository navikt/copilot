import { afterEach, describe, expect, it, vi } from "vitest";
import { sendSurvey } from "./send";

const respond = (r: Partial<Response> | Error) =>
  vi.stubGlobal(
    "fetch",
    vi.fn(() => (r instanceof Error ? Promise.reject(r) : Promise.resolve(r)))
  );
const json = (body: unknown, status = 200) =>
  ({
    status,
    type: "basic",
    headers: new Headers({ "content-type": "application/json" }),
    json: async () => body,
  }) as never;

describe("sendSurvey", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("posts the answers as the body, to the route handler", async () => {
    respond(json({ status: "recorded" }));
    expect(await sendSurvey("s 1", { a: 1 })).toEqual({ status: "recorded" });
    expect(fetch).toHaveBeenCalledWith(
      "/api/survey/s%201/responses",
      expect.objectContaining({ method: "POST", body: '{"a":1}', redirect: "manual" })
    );
  });

  it("calls an expired login expired: 401 or a redirect to log in", async () => {
    respond(json({ status: "error" }, 401));
    expect(await sendSurvey("s", {})).toEqual({ status: "expired" });
    respond({ status: 0, type: "opaqueredirect", headers: new Headers() });
    expect(await sendSurvey("s", {})).toEqual({ status: "expired" });
  });

  it("calls an HTML page or a network failure an error, so the form offers a retry", async () => {
    respond({ status: 502, type: "basic", headers: new Headers({ "content-type": "text/html" }) });
    expect(await sendSurvey("s", {})).toEqual({ status: "error" });
    respond(new TypeError("Failed to fetch"));
    expect(await sendSurvey("s", {})).toEqual({ status: "error" });
  });
});
