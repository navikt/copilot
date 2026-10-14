import { spawn, type ChildProcess } from "node:child_process";
import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, test } from "@playwright/test";

// The survey form against real production builds (hack/e2e-build.sh), with a
// mock that plays both Texas (token introspection) and copilot-survey. #1804:
// a page loaded before a deploy could not send, because its server action id
// died with the old build. These tests hold that the form survives a deploy
// and an expired login.

const SURVEY = {
  id: "e2e",
  title: "E2E-undersøkelse",
  ends: "2099-01-01",
  questions: [
    { id: "q1", type: "choice", text: "Hvor ofte?", required: true, options: ["Daglig", "Sjelden"] },
    { id: "q2", type: "text", text: "Noe mer?" },
  ],
};
const ANSWERS = { q1: "Daglig", q2: "hei" };
const PORT = 3998;
const URL = `http://127.0.0.1:${PORT}/nav-pilot/undersokelse`;

let mock: Server;
let mockUrl = "";
let loggedIn = true;
let received: unknown[] = [];
let app: ChildProcess | undefined;

async function body(req: NodeJS.ReadableStream) {
  let s = "";
  for await (const c of req) s += c;
  return s ? JSON.parse(s) : {};
}

test.beforeAll(async () => {
  mock = createServer(async (req, res) => {
    const json = (status: number, data: unknown) => {
      res.writeHead(status, { "Content-Type": "application/json" });
      res.end(JSON.stringify(data));
    };
    if (req.url === "/introspect") {
      await body(req);
      return json(
        200,
        loggedIn
          ? { active: true, name: "E2E, Test", preferred_username: "e2e@nav.no", groups: [] }
          : { active: false, error: "token is expired" }
      );
    }
    if (req.url === "/api/v1/surveys/active") return json(200, { surveys: [SURVEY] });
    if (req.url === `/api/v1/surveys/${SURVEY.id}/responses` && req.method === "POST") {
      received.push(await body(req));
      return json(201, {});
    }
    json(404, {});
  });
  await new Promise<void>((r) => mock.listen(0, "127.0.0.1", r));
  mockUrl = `http://127.0.0.1:${(mock.address() as AddressInfo).port}`;
});

test.afterAll(() => mock.close());
test.afterEach(async () => stop());
test.beforeEach(() => {
  loggedIn = true;
  received = [];
});
test.use({ extraHTTPHeaders: { Authorization: "Bearer e2e" } });

/** Starts build a or b on PORT and waits until it answers. */
async function start(build: "a" | "b") {
  app = spawn("node", [`.e2e/${build}/server.js`], {
    env: {
      ...process.env,
      NODE_ENV: "production",
      PORT: String(PORT),
      HOSTNAME: "127.0.0.1",
      COPILOT_SURVEY_URL: mockUrl,
      COPILOT_API_URL: mockUrl,
      NAIS_TOKEN_INTROSPECTION_ENDPOINT: `${mockUrl}/introspect`,
    },
    stdio: "inherit",
  });
  for (let i = 0; i < 100; i++) {
    try {
      await fetch(`http://127.0.0.1:${PORT}/`);
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 200));
    }
  }
  throw new Error(`build ${build} did not start`);
}

async function stop() {
  const p = app;
  app = undefined;
  if (!p || p.exitCode !== null) return;
  await new Promise((r) => {
    p.once("exit", r);
    p.kill();
  });
}

test("deploy skew: a page from the old build sends to the new one", async ({ page }) => {
  await start("a");
  await page.goto(URL);
  await page.getByLabel("Daglig").check();
  await page.getByLabel("Noe mer?").fill("hei");

  await stop();
  await start("b");

  await page.getByRole("button", { name: "Send svar" }).click();
  await expect(page.getByText("Takk! Svaret ditt er sendt.")).toBeVisible();
  expect(received).toEqual([
    { answers: ANSWERS, context: { version: "web", os: "other", client: "web", local_models: false } },
  ]);
});

test("expired login: says so, and the draft survives a reload", async ({ page }) => {
  await start("a");
  await page.goto(URL);
  await page.getByLabel("Daglig").check();
  await page.getByLabel("Noe mer?").fill("hei");

  loggedIn = false;
  await page.getByRole("button", { name: "Send svar" }).click();
  await expect(page.getByText(/Innloggingen har gått ut/)).toBeVisible();
  expect(received).toEqual([]);

  loggedIn = true;
  await page.reload();
  await expect(page.getByLabel("Daglig")).toBeChecked();
  await expect(page.getByLabel("Noe mer?")).toHaveValue("hei");
  await page.getByRole("button", { name: "Send svar" }).click();
  await expect(page.getByText("Takk! Svaret ditt er sendt.")).toBeVisible();
  expect(received).toHaveLength(1);
});
