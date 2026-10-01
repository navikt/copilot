import assert from "node:assert/strict";
import { test } from "node:test";
import { GET } from "../../workspace/app/api/saker/route.ts";

test("invalid filter is a client error without internal details", async () => {
  const response = await GET(new Request("http://localhost/api/saker?status=ukjent"));
  assert.equal(response.status, 400);
  const body = await response.json();
  assert.equal(typeof body.error, "string");
  assert.ok(body.error.length > 0);
  assert.ok(!body.error.includes("Ukjent status:"));
});

test("empty filter is invalid", async () => {
  const response = await GET(new Request("http://localhost/api/saker?status="));
  assert.equal(response.status, 400);
});
