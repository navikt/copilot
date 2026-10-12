import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { after, before, test } from "node:test";
import { bearer, startIssuer } from "./issuer.mjs";

const base = "http://127.0.0.1:32118";
let server;
let issuer;

before(async () => {
  issuer = await startIssuer();
  server = spawn(process.execPath, ["/deps/node_modules/next/dist/bin/next", "start",
    "--hostname", "127.0.0.1", "--port", "32118"],
  { cwd: "/task/workspace", stdio: "ignore", env: { ...process.env, ...issuer.env } });
  for (let attempt = 0; attempt < 100; attempt++) {
    if (server.exitCode !== null) throw new Error("Next.js server exited before readiness");
    try {
      await fetch(`${base}/api/isready`);
      return;
    } catch {
      // Wait for the server to bind.
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("Next.js server did not become ready");
});

after(() => {
  if (server?.exitCode === null) server.kill("SIGTERM");
  issuer?.close();
});

test("the reported route rejects missing and forged tokens and accepts valid ones", async () => {
  assert.equal((await fetch(`${base}/api/intern/saker`)).status, 401);
  assert.equal((await fetch(`${base}/api/intern/saker`, bearer(await issuer.forged()))).status, 401);
  assert.equal((await fetch(`${base}/api/intern/saker/101`, bearer(await issuer.forged()))).status, 401);
  assert.equal((await fetch(`${base}/api/intern/saker`, bearer(await issuer.expired()))).status, 401);
  const forgedNote = bearer(await issuer.forged());
  forgedNote.method = "POST";
  forgedNote.headers["content-type"] = "application/json";
  forgedNote.body = JSON.stringify({ tekst: "Forfalsket" });
  assert.equal((await fetch(`${base}/api/intern/saker/102/notater`, forgedNote)).status, 401);
  assert.equal((await fetch(`${base}/api/intern/saker`, bearer(await issuer.valid()))).status, 200);
});

test("NAIS probes stay open and existing routes still work", async () => {
  assert.equal((await fetch(`${base}/api/isalive`)).status, 200);
  assert.equal((await fetch(`${base}/api/isready`)).status, 200);
  assert.equal((await fetch(`${base}/api/saker`, bearer(await issuer.valid()))).status, 200);
});
