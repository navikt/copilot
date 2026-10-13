import assert from "node:assert/strict";
import { test } from "node:test";
import { GET as isalive } from "../app/api/isalive/route.ts";
import { GET as saker } from "../app/api/saker/route.ts";

test("isalive svarer uten token", async () => {
  assert.equal((await isalive()).status, 200);
});

test("saker krever token", async () => {
  assert.equal((await saker(new Request("http://localhost/api/saker"))).status, 401);
});
