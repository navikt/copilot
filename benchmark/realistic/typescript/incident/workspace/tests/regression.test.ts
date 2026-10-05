import assert from "node:assert/strict";
import { test } from "node:test";
import { GET } from "../app/api/saker/route.ts";

test("valid filter still returns matching cases", async () => {
  const response = await GET(new Request("http://localhost/api/saker?status=apen"));
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), [{ id: 101, status: "apen" }]);
});
