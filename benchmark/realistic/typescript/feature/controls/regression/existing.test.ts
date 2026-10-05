import assert from "node:assert/strict";
import { test } from "node:test";
import { GET } from "../../workspace/app/api/saker/route.ts";

test("existing cases are still listed", async () => {
  const response = await GET();
  assert.equal(response.status, 200);
  assert.ok((await response.json()).some((sak: { id: number; tittel: string }) =>
    sak.id === 101 && sak.tittel === "Eksisterende sak"));
});
