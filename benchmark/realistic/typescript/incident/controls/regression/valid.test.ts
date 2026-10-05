import assert from "node:assert/strict";
import { test } from "node:test";
import { GET } from "../../workspace/app/api/saker/route.ts";

test("valid filters and default preserve existing behavior", async () => {
  const open = await GET(new Request("http://localhost/api/saker?status=apen"));
  assert.equal(open.status, 200);
  assert.deepEqual(await open.json(), [{ id: 101, status: "apen" }]);
  const all = await GET(new Request("http://localhost/api/saker"));
  assert.equal(all.status, 200);
  assert.deepEqual(await all.json(), [
    { id: 101, status: "apen" },
    { id: 102, status: "lukket" },
  ]);
});
