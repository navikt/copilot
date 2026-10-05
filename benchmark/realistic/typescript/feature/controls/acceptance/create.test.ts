import assert from "node:assert/strict";
import { test } from "node:test";
import * as route from "../../workspace/app/api/saker/route.ts";

test("creates and lists case across route and store", async () => {
  const response = await route.POST(new Request("http://localhost/api/saker", {
    method: "POST",
    body: JSON.stringify({ tittel: "Ny sak", prioritet: "hoy" }),
  }));
  assert.equal(response.status, 201);
  const created = await response.json();
  assert.equal(created.tittel, "Ny sak");
  assert.equal(created.prioritet, "hoy");
  assert.ok(created.id > 101);
  assert.ok((await (await route.GET()).json()).some((sak: { id: number }) => sak.id === created.id));
  const next = await route.POST(new Request("http://localhost/api/saker", {
    method: "POST", body: JSON.stringify({ tittel: "Andre sak" }),
  }));
  assert.equal(next.status, 201);
  const second = await next.json();
  assert.equal(second.prioritet, "normal");
  assert.notEqual(second.id, created.id);
  const listed = await (await route.GET()).json();
  assert.ok(listed.some((sak: { id: number }) => sak.id === created.id));
  assert.ok(listed.some((sak: { id: number }) => sak.id === second.id));
});

test("invalid inputs do not create a case", async () => {
  const before = (await (await route.GET()).json()).length;
  for (const body of [{}, { tittel: "   " }, { tittel: "Sak", prioritet: "ukjent" }]) {
    const response = await route.POST(new Request("http://localhost/api/saker", {
      method: "POST", body: JSON.stringify(body),
    }));
    assert.equal(response.status, 400);
  }
  assert.equal((await (await route.GET()).json()).length, before);
});
