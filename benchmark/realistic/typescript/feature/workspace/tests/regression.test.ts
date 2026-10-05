import assert from "node:assert/strict";
import { test } from "node:test";
import { GET } from "../app/api/saker/route.ts";

test("existing cases remain visible", async () => {
  const response = await GET();
  assert.equal(response.status, 200);
  assert.ok((await response.json()).some((sak: { id: number }) => sak.id === 101));
});
