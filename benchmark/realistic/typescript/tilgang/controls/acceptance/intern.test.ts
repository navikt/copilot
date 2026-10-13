import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { bearer, startIssuer } from "../issuer.mjs";
import { GET as liste } from "../../workspace/app/api/intern/saker/route.ts";
import { GET as enkelt } from "../../workspace/app/api/intern/saker/[id]/route.ts";
import { finnInternSak } from "../../workspace/lib/saker.ts";
import { POST as notat } from "../../workspace/app/api/intern/saker/[id]/notater/route.ts";

let issuer: Awaited<ReturnType<typeof startIssuer>>;
before(async () => {
  issuer = await startIssuer();
  Object.assign(process.env, issuer.env);
});
after(() => issuer.close());

const url = "http://localhost/api/intern/saker";
const params = { params: Promise.resolve({ id: "101" }) };

test("interne saker krever gyldig token", async () => {
  assert.equal((await liste(new Request(url))).status, 401);
  assert.equal((await liste(new Request(url, bearer(await issuer.forged())))).status, 401);
  assert.equal((await liste(new Request(url, bearer(await issuer.otherAudience())))).status, 401);
  assert.equal((await liste(new Request(url, bearer(await issuer.expired())))).status, 401);
  const ok = await liste(new Request(url, bearer(await issuer.valid())));
  assert.equal(ok.status, 200);
  assert.equal((await ok.json())[0].internNotat, "Venter på arbeidsgiver");
});

test("enkeltsak krever gyldig token", async () => {
  assert.equal((await enkelt(new Request(`${url}/101`), params)).status, 401);
  assert.equal((await enkelt(new Request(`${url}/101`, bearer(await issuer.forged())), params)).status, 401);
  const ok = await enkelt(new Request(`${url}/101`, bearer(await issuer.valid())), params);
  assert.equal(ok.status, 200);
  assert.equal((await ok.json()).id, 101);
});

test("nye notater krever gyldig token", async () => {
  const sak = { params: Promise.resolve({ id: "102" }) };
  const post = (token?: string) =>
    new Request(`${url}/102/notater`, {
      method: "POST",
      headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
      body: JSON.stringify({ tekst: "Ringt bruker" }),
    });
  const notatFør = finnInternSak(102)?.internNotat;
  assert.equal((await notat(post(), sak)).status, 401);
  assert.equal((await notat(post(await issuer.forged()), sak)).status, 401);
  assert.equal((await notat(post(await issuer.expired()), sak)).status, 401);
  assert.equal(finnInternSak(102)?.internNotat, notatFør, "rejected requests must not change the case");
  const ok = await notat(post(await issuer.valid()), sak);
  assert.equal(ok.status, 201);
  assert.match((await ok.json()).internNotat, /Ringt bruker/);
});
