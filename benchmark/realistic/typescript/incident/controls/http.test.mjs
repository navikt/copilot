import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { after, before, test } from "node:test";

const url = "http://127.0.0.1:32117/api/saker";
let server;

before(async () => {
  server = spawn(process.execPath, ["/deps/node_modules/next/dist/bin/next", "start",
    "--hostname", "127.0.0.1", "--port", "32117"], { cwd: "/task/workspace", stdio: "ignore" });
  for (let attempt = 0; attempt < 60; attempt++) {
    if (server.exitCode !== null) throw new Error("Next.js server exited before readiness");
    try {
      if ((await fetch(url)).ok) return;
    } catch {
      // Wait for the server to bind.
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("Next.js server did not become ready");
});

after(() => {
  if (server?.exitCode === null) server.kill("SIGTERM");
});

test("the reported HTTP 500 is fixed without breaking valid requests", async () => {
  const invalid = await fetch(`${url}?status=ukjent`);
  assert.equal(invalid.status, 400);
  const valid = await fetch(`${url}?status=apen`);
  assert.equal(valid.status, 200);
  assert.deepEqual(await valid.json(), [{ id: 101, status: "apen" }]);
});
