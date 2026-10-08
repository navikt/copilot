import { afterEach, describe, expect, it, vi } from "vitest";
import { buildReports, buildTable } from "./local-models-manifest";
import { FALLBACK_REPORTS, FALLBACK_TABLE, getLocalModels, getLocalReports } from "./local-models";

const entry = (over: Record<string, unknown> = {}) => ({
  key: "m",
  name: "M",
  model: "org/M",
  role: "r",
  default: true,
  weights_gb: 25,
  min_ram_gb: 48,
  params: { MLX_OPENCODE_CONTEXT: "65536", MLX_OPENCODE_OUTPUT: "16384", MLX_NAV_PILOT_TEMPERATURE: "0.6" },
  capabilities: {
    bar: { confidence: 0.9, x_caught: 0.9, x_silent: 0.95, min_runs: 5, min_tasks: 2 },
    classes: { debug: { delegate: "cloud", local: "cloud", local_k: 0, local_n: 3 } },
  },
  ...over,
});

const block = { classes: { debug: { delegate: "cloud", local: "cloud", local_k: 1, local_n: 4 } } };
const caps = (blocks: Record<string, unknown> = {}) => ({ manifest_blocks: { "org/M": block, ...blocks } });

describe("buildTable", () => {
  it("projects the numbers the page shows, run counts included", () => {
    const [m] = buildTable({ models: [entry({ min_nav_pilot: "2026.09.24-110317-3596754" })] }, caps()).models;
    expect(m.id).toBe("m");
    expect(m.context).toBe(65536);
    expect(m.output).toBe(16384);
    expect(m.temperature).toBe(0.6);
    expect(m.top_p).toBeNull();
    expect(m.min_nav_pilot).toBe("2026.09.24-110317-3596754");
    expect(m.classes).toEqual({
      debug: { delegate: "cloud", local: "cloud", delegate_k: 0, delegate_n: 0, local_k: 0, local_n: 3 },
    });
    expect(m.bar).toMatchObject({ min_runs: 5, min_tasks: 2, confidence: 0.9 });
  });

  it("lists measured models the manifest dropped as rejected, with their successor", () => {
    const { rejected } = buildTable(
      { models: [entry()], replaced: { "org/Old": "m" } },
      caps({ "org/Old": block, "org/Other": {} })
    );
    expect(rejected.map((r) => [r.model, r.replaced_by])).toEqual([
      ["org/Old", "m"],
      ["org/Other", null],
    ]);
    expect(rejected[0].classes.debug).toMatchObject({ local_k: 1, local_n: 4 });
    expect(rejected[1].bar).toBeNull();
  });

  it.each([
    ["no models list", {}],
    ["an empty list", { models: [] }],
    ["no default", { models: [entry({ default: false })] }],
    ["two defaults", { models: [entry(), entry({ key: "n" })] }],
    ["no context/output", { models: [entry({ params: {} })] }],
    ["a non-numeric param", { models: [entry({ params: { MLX_OPENCODE_CONTEXT: "x", MLX_OPENCODE_OUTPUT: "1" } })] }],
    ["a non-string param", { models: [entry({ params: { MLX_OPENCODE_CONTEXT: null, MLX_OPENCODE_OUTPUT: "1" } })] }],
    ["a string min_ram_gb", { models: [entry({ min_ram_gb: "48" })] }],
    [
      "a bar probability above 1",
      { models: [entry({ capabilities: { bar: { ...entry().capabilities.bar, confidence: 2 } } })] },
    ],
    [
      "a confidence the chart cannot compute",
      { models: [entry({ capabilities: { bar: { ...entry().capabilities.bar, confidence: 0.8 } } })] },
    ],
    [
      "a fractional min_runs",
      { models: [entry({ capabilities: { bar: { ...entry().capabilities.bar, min_runs: 1.5 } } })] },
    ],
    ["no weights_gb", { models: [entry({ weights_gb: undefined })] }],
    ["a null min_nav_pilot", { models: [entry({ min_nav_pilot: null })] }],
    ["an unreadable min_nav_pilot", { models: [entry({ min_nav_pilot: "soon" })] }],
    ["a missing role", { models: [entry({ role: "" })] }],
    ["a negative run count", { models: [entry({ capabilities: { classes: { debug: { local_n: -1 } } } })] }],
  ])("refuses %s", (_, manifest) => {
    expect(() => buildTable(manifest, caps())).toThrow();
  });

  it("refuses capabilities without manifest_blocks", () => {
    expect(() => buildTable({ models: [entry()] }, {})).toThrow();
  });
});

describe("getLocalModels", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  const stubFetch = (impl: (url: RequestInfo | URL) => Promise<Response>) => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const fetchMock = vi.fn<typeof fetch>(impl);
    vi.stubGlobal("fetch", fetchMock);
    return fetchMock;
  };

  it("returns the manifest when the fetch succeeds", async () => {
    const fetchMock = stubFetch(async (url) =>
      Response.json(
        String(url).endsWith("capabilities.json") ? caps({ "org/Gone": block }) : { models: [entry({ key: "live" })] }
      )
    );
    const table = await getLocalModels();
    expect(table.models.map((m) => m.id)).toEqual(["live"]);
    expect(table.rejected.map((r) => r.model)).toEqual(["org/Gone"]);
    expect(table.fetchedAt).toEqual(expect.any(String));
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ next: { revalidate: 3600 }, signal: expect.any(AbortSignal) });
    expect(console.error).not.toHaveBeenCalled();
  });

  it.each([
    ["a fetch error", () => Promise.reject(new Error("network down"))],
    ["a non-200 answer", async () => new Response("nope", { status: 503 })],
    ["invalid JSON", async () => new Response("<html>")],
    ["a manifest that fails validation", async () => Response.json({ models: [entry({ default: false })] })],
    [
      "a capabilities fetch error",
      async (url: RequestInfo | URL) =>
        String(url).endsWith("capabilities.json")
          ? new Response("nope", { status: 503 })
          : Response.json({ models: [entry()] }),
    ],
  ])("falls back to the checked-in copy on %s", async (_, impl) => {
    stubFetch(impl);
    expect(await getLocalModels()).toEqual({ ...FALLBACK_TABLE, fetchedAt: null });
    expect(console.error).toHaveBeenCalledOnce();
  });

  it("ships a fallback copy with exactly one default model", () => {
    expect(FALLBACK_TABLE.models.filter((m) => m.default)).toHaveLength(1);
  });
});

const report = (over: Record<string, unknown> = {}) => ({
  id: "2026-10-01-x/report",
  title: "X",
  date: "2026-10-01",
  class: "C",
  summary: "s",
  path: "reports/2026-10-01-x/report.md",
  url: "https://github.com/navikt/mlx-workspace/blob/main/reports/2026-10-01-x/report.md",
  justifies: [],
  ...over,
});
const index = (reports: unknown[], over: Record<string, unknown> = {}) => ({
  schema_version: 1,
  reports,
  nav_pilot: [],
  unmeasured: [{ item: "**Kev** on [sets](x.md)", status: "`waiting`", when: "-", where: "-", decides: "-" }],
  ...over,
});

describe("buildReports", () => {
  it("keeps the fields the page shows, newest first, and strips Markdown from unmeasured items", () => {
    const r = buildReports(
      index([report({ date: "2026-09-01" }), report({ id: "b", verdict: "pass", headline: { k: 3, n: 4 } })])
    );
    expect(r.reports.map((x) => [x.id, x.verdict, x.headline])).toEqual([
      ["b", "pass", { k: 3, n: 4 }],
      ["2026-10-01-x/report", null, null],
    ]);
    expect(r.unmeasured).toEqual([{ item: "Kev on sets", status: "waiting" }]);
  });

  it.each([
    ["another schema", index([report()], { schema_version: 2 })],
    ["no reports", index([])],
    ["a link outside the repo", index([report({ url: "https://evil.example/x" })])],
    ["a javascript: link", index([report({ url: "javascript:alert(1)" })])],
    ["an unknown verdict", index([report({ verdict: "great" })])],
    ["a headline with k above n", index([report({ headline: { k: 5, n: 4 } })])],
    ["a headline with n = 0", index([report({ headline: { k: 0, n: 0 } })])],
    ["a bad date", index([report({ date: "1. oktober" })])],
    ["a date that is not on the calendar", index([report({ date: "2026-02-30" })])],
    ["a date of 2026-99-99", index([report({ date: "2026-99-99" })])],
    [
      "a link that climbs out of the repo",
      index([report({ url: "https://github.com/navikt/mlx-workspace/../../other/repo" })]),
    ],
    ["an unmeasured row with no fields", index([report()], { unmeasured: [{}] })],
  ])("refuses %s", (_, raw) => {
    expect(() => buildReports(raw)).toThrow();
  });

  it("ships a fallback index that validates", () => {
    expect(FALLBACK_REPORTS.reports.length).toBeGreaterThan(0);
    expect(FALLBACK_TABLE).not.toHaveProperty("reports");
  });
});

describe("getLocalReports", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("returns the live index", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json(index([report({ id: "live" })])))
    );
    expect((await getLocalReports()).reports.map((r) => r.id)).toEqual(["live"]);
  });

  it("falls back to the checked-in index on a 404", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("nope", { status: 404 }))
    );
    expect(await getLocalReports()).toBe(FALLBACK_REPORTS);
  });
});
