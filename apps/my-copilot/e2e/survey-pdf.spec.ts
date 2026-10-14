import { spawn, type ChildProcess } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, test } from "@playwright/test";

// «Lagre som PDF» on the survey results (#1826): the pages in print media, and a PDF of each.
// The PDFs land in PDF_OUT when set (for a person to look at), else in the test output.

const PORT = 3997;
const BASE = `http://127.0.0.1:${PORT}`;
const ADMINS = "e2e-admins";

const opts = (counts: Record<string, number>) =>
  Object.entries(counts).map(([value, count]) => ({ value, count, suppressed: false }));
const scale = (c: number[], version = 1) => {
  const n = c.reduce((a, b) => a + b, 0);
  return {
    version,
    n,
    options: opts(Object.fromEntries(c.map((x, i) => [String(i + 1), x]))),
    mean: c.reduce((a, x, i) => a + x * (i + 1), 0) / n,
    agree: (c[3] + c[4]) / n,
  };
};
const choice = (counts: Record<string, number>) => {
  const n = Object.values(counts).reduce((a, b) => a + b, 0);
  return { version: 1, n, options: opts(counts) };
};
const wave = (id: string, title: string, starts: string, ends: string, responses: number) => ({
  id,
  title,
  starts,
  ends,
  status: "stengt",
  responses,
  suppressed: false,
  series: "utviklerundersokelsen",
  channels: { web: responses },
});

const STATEMENTS = [
  ["tilfredshet", "Alt i alt er jeg fornøyd med KI-kodeverktøyene vi har tilgang til i Nav."],
  ["kognitiv-belastning", "KI-kodeverktøy reduserer mental innsats på repetitive oppgaver."],
  ["oppgavegjennomforing", "KI-kodeverktøy hjelper meg å komme videre når jeg står fast."],
];

// Question ids match apps/copilot-survey/surveys/q4-2026.json, so the page draws the Likert bars.
const RESULTS = {
  ...wave("q4-2026", "Utviklerundersøkelsen høsten 2026", "2026-10-13", "2026-10-27", 142),
  status: "åpen",
  questions: [
    {
      id: "erfaring",
      type: "choice",
      text: "Hvor mange års erfaring har du som teknolog?",
      versions: [choice({ "0–2": 12, "3–5": 30, "6–10": 45, "11+": 55 })],
    },
    {
      id: "pastander",
      type: "matrix",
      text: "Hvor enig er du i disse påstandene?",
      items: STATEMENTS.map(([id, text], i) => ({ id, text, versions: [scale([4 + i, 10, 20, 50 - i * 5, 30])] })),
    },
  ],
};

const COMPARE = {
  surveys: [
    wave("q2-2026", "Utviklerundersøkelsen våren 2026", "2026-03-24", "2026-04-30", 165),
    wave("q4-2026", "Utviklerundersøkelsen høsten 2026", "2026-10-13", "2026-10-27", 142),
  ],
  questions: [
    {
      id: "verktoy",
      type: "multi",
      text: "Hvilke KI-kodemiljøer bruker du i dag?",
      break: false,
      waves: [
        { excluded: 0, versions: [choice({ "VS Code": 99, IntelliJ: 50, "Copilot CLI": 16 })] },
        { excluded: 0, versions: [choice({ "VS Code": 80, IntelliJ: 40, "Copilot CLI": 22 })] },
      ],
    },
    {
      id: "pastander",
      type: "matrix",
      text: "Hvor enig er du i disse påstandene?",
      min: 1,
      max: 5,
      labels: ["Helt uenig", "Uenig", "Nøytral", "Enig", "Helt enig"],
      items: STATEMENTS.map(([id, text], i) => ({
        id,
        text,
        break: i === 2,
        waves: [
          { excluded: 14, versions: [scale([10, 20, 40, 60 - i * 10, 35])] },
          { excluded: 0, versions: [scale([4, 10, 20, 50, 30 + i * 5], i === 2 ? 2 : 1)] },
        ],
      })),
    },
  ],
};

let mock: Server;
let app: ChildProcess | undefined;

test.beforeAll(async () => {
  mock = createServer((req, res) => {
    const json = (data: unknown) => {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify(data));
    };
    if (req.url === "/introspect")
      return json({ active: true, name: "E2E, Test", preferred_username: "e2e@nav.no", groups: [ADMINS] });
    if (req.url === "/api/v1/surveys/active") return json({ surveys: [] });
    if (req.url === "/api/v1/admin/surveys/q4-2026/results") return json(RESULTS);
    if (req.url?.startsWith("/api/v1/admin/surveys/compare")) return json(COMPARE);
    if (req.url === "/defs/q4-2026.json")
      return json(JSON.parse(readFileSync("../copilot-survey/surveys/q4-2026.json", "utf8")));
    res.writeHead(404).end();
  });
  await new Promise<void>((r) => mock.listen(0, "127.0.0.1", r));
  const url = `http://127.0.0.1:${(mock.address() as AddressInfo).port}`;
  app = spawn("node", [".e2e/a/server.js"], {
    env: {
      ...process.env,
      NODE_ENV: "production",
      PORT: String(PORT),
      HOSTNAME: "127.0.0.1",
      COPILOT_SURVEY_URL: url,
      COPILOT_API_URL: url,
      SURVEY_DEFINITIONS_URL: `${url}/defs`,
      COPILOT_ADMIN_GROUP_ID: ADMINS,
      NAIS_TOKEN_INTROSPECTION_ENDPOINT: `${url}/introspect`,
    },
    stdio: "inherit",
  });
  for (let i = 0; i < 100; i++) {
    try {
      await fetch(`${BASE}/`);
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 200));
    }
  }
  throw new Error("app did not start");
});

test.afterAll(async () => {
  mock.close();
  const p = app;
  if (p && p.exitCode === null) await new Promise((r) => (p.once("exit", r), p.kill()));
});

test.use({ extraHTTPHeaders: { Authorization: "Bearer e2e" } });

for (const [name, path, title] of [
  ["resultater", "/admin/undersokelser/q4-2026", "Utviklerundersøkelsen høsten 2026"],
  ["sammenligning", "/admin/undersokelser/sammenlign?ids=q2-2026,q4-2026", "Sammenligning: Våren 2026 og Høsten 2026"],
] as const) {
  test(`${name}: «Lagre som PDF» prints without site chrome, with the numbers open`, async ({ page }) => {
    await page.goto(BASE + path);
    // CSS locators: getByRole skips hidden elements, so a hidden check on it would always pass.
    const button = page.locator("button", { hasText: "Lagre som PDF" });
    await expect(button).toBeVisible();
    await expect(page.getByTestId("print-header")).toBeHidden();
    const tables = page.locator("details table");
    expect(await tables.count()).toBeGreaterThan(0);
    await expect(tables.first()).toBeHidden();

    await page.emulateMedia({ media: "print" });
    await expect(button).toBeHidden();
    await expect(page.locator("header").first()).toBeHidden();
    await expect(page.locator("footer").first()).toBeHidden();
    await expect(page.locator('main a[href="/admin/undersokelser"]')).toBeHidden();
    await expect(page.locator('a[href$="/fritekst"]')).toHaveCount(name === "resultater" ? 1 : 0);
    await expect(page.locator('a[href$="/fritekst"]')).toBeHidden();
    await expect(page.getByTestId("print-header")).toContainText(title);
    await expect(page.getByTestId("print-header")).toContainText("Eksportert");
    await expect(page.locator("details summary").first()).toBeHidden();
    for (const t of await tables.all()) await expect(t).toBeVisible();

    const pdf = await page.pdf({ format: "A4", preferCSSPageSize: true, printBackground: true });
    expect(pdf.subarray(0, 5).toString()).toBe("%PDF-");
    await test.info().attach(`${name}.pdf`, { body: pdf, contentType: "application/pdf" });
    if (process.env.PDF_OUT) {
      mkdirSync(process.env.PDF_OUT, { recursive: true });
      writeFileSync(`${process.env.PDF_OUT}/${name}.pdf`, pdf);
    }
  });
}
