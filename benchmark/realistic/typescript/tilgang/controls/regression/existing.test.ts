import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { bearer, startIssuer } from "../issuer.mjs";
import { GET as isalive } from "../../workspace/app/api/isalive/route.ts";
import { GET as isready } from "../../workspace/app/api/isready/route.ts";
import { GET as saker } from "../../workspace/app/api/saker/route.ts";

let issuer: Awaited<ReturnType<typeof startIssuer>>;
before(async () => {
  issuer = await startIssuer();
  Object.assign(process.env, issuer.env);
});
after(() => issuer.close());

test("saker gir svar til saksbehandler uten interne notater", async () => {
  const response = await saker(new Request("http://localhost/api/saker", bearer(await issuer.valid())));
  assert.equal(response.status, 200);
  const body = await response.json();
  assert.equal(body.length, 2);
  assert.equal(body[0].internNotat, undefined);
  assert.equal((await saker(new Request("http://localhost/api/saker", bearer(await issuer.forged())))).status, 401);
});

test("NAIS-probene svarer uten token", async () => {
  assert.equal((await isalive()).status, 200);
  assert.equal((await isready()).status, 200);
});
