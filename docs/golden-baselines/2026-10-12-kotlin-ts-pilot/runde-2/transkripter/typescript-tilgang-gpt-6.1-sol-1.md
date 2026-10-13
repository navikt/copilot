# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `a0b7c323-6822-493f-b3db-2016e0854de9`  
> - **Started:** 10/12/2026, 11:12:54 PM  
> - **Duration:** 4m 9s  
> - **Exported:** 10/12/2026, 11:17:03 PM  

---

<sub>2s</sub>

### Warning

1 MCP server was blocked by policy: 'github-copilot-browser'

---

<sub>3s</sub>

### User

En penetrasjonstest viser at `/api/intern/saker` svarer uten gyldig innlogging.
Finn årsaken og rett feilen. Bruk kun dette repoet.

---

<sub>10s</sub>

### task (Completed)

### `skill`

**security-review**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "security-review"
}
```

</details>

```
Skill "security-review" loaded successfully. Follow the instructions in the skill context.
```


### `bash` — Failed

**Check scoped changes and applicable repository instructions**

$ pwd; git --no-pager status --short -- .; find . -name AGENTS.md -not -path './node_modules/*' -not -path './.git/*'; for p in /AGENTS.md /Users/AGENTS.md ~/AGENTS.md ~/nav-benchmark-attempts/AGENTS.md <attempts>/AGENTS.md <attempts>/typescript-tilgang-gpt-6.1-sol-1/AGENTS.md; do if [ -f "$p" ]; then printf '\n--- %s ---\n' "$p"; cat "$p"; fi; done; git --no-pager diff -- .; git --no-pager diff --staged -- .

```
Permission denied because no interactive user response was available. Retry in an interactive session so the user can approve it, or try an alternative that does not require this permission.
```


### `bash`

**Inspect current workspace git changes**

$ git --no-pager status --short -- .; git --no-pager diff -- .; git --no-pager diff --staged -- .

```
fatal: unable to get current working directory: Operation not permitted
fatal: unable to get current working directory: Operation not permitted
fatal: unable to get current working directory: Operation not permitted
<shellId: 1 completed with exit code 128>
```


### `glob`

****/AGENTS.md**

"**/AGENTS.md"

```
No files matched the pattern.
```


### `view`

**view the file at <workspace>/proxy.ts.**

<workspace>/proxy.ts

<details>
<summary>13 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { NextResponse, type NextRequest } from "next/server";

// Rask sjekk før route handlerne: forespørsler uten token avvises med en gang.
// Selve valideringen av tokenet gjør hver route handler med requireSaksbehandler().
export function proxy(request: NextRequest): NextResponse {
  if (!getToken(request)) {
    return NextResponse.json({ error: "Mangler token" }, { status: 401 });
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/api/saker/:path*"],
};
```

</details>


### `glob`

**{app,lib,tests,docs}/**/***

"{app,lib,tests,docs}/**/*"

<details>
<summary>10 matches</summary>

```
<workspace>/lib/auth.ts
<workspace>/lib/saker.ts
<workspace>/docs/drift.md
<workspace>/tests/probes.test.ts
<workspace>/app/api/isready/route.ts
<workspace>/app/api/isalive/route.ts
<workspace>/app/api/intern/saker/[id]/route.ts
<workspace>/app/api/intern/saker/[id]/notater/route.ts
<workspace>/app/api/intern/saker/route.ts
<workspace>/app/api/saker/route.ts
```

</details>


### `view`

**view the file at <workspace>/lib/auth.ts.**

<workspace>/lib/auth.ts

<details>
<summary>16 lines</summary>

```
import { getToken, validateToken } from "@navikt/oasis";

/**
 * Validerer Azure-tokenet til saksbehandleren. Returnerer et 401-svar som route
 * handleren skal sende videre, eller null når tokenet er gyldig.
 */
export async function requireSaksbehandler(request: Request): Promise<Response | null> {
  const token = getToken(request);
  if (!token) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const validation = await validateToken(token);
  if (!validation.ok) {
    return Response.json({ error: "Ugyldig token" }, { status: 401 });
  }
  return null;
}
```

</details>


### `view`

**view the file at <workspace>/lib/saker.ts.**

<workspace>/lib/saker.ts

<details>
<summary>20 lines</summary>

```
export type Sak = { id: number; tittel: string; status: "apen" | "lukket" };
export type InternSak = Sak & { internNotat: string };

const saker: InternSak[] = [
  { id: 101, tittel: "Søknad om dagpenger", status: "apen", internNotat: "Venter på arbeidsgiver" },
  { id: 102, tittel: "Klage på vedtak", status: "lukket", internNotat: "Omgjort etter klage" },
];

export function listSaker(): Sak[] {
  return saker.map(({ internNotat: _, ...sak }) => sak);
}

export function listInterneSaker(): InternSak[] {
  return [...saker];
}

export function finnInternSak(id: number): InternSak | undefined {
  return saker.find((sak) => sak.id === id);
}

export function leggTilNotat(id: number, tekst: string): InternSak | undefined {
  const sak = finnInternSak(id);
  if (sak) sak.internNotat = `${sak.internNotat}\n${tekst}`;
  return sak;
}
```

</details>


### `view`

**view the file at <workspace>/app/api/intern/saker/route.ts.**

<workspace>/app/api/intern/saker/route.ts

<details>
<summary>8 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { listInterneSaker } from "../../../../lib/saker.ts";

export async function GET(request: Request): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  return Response.json(listInterneSaker());
}
```

</details>


### `view`

**view the file at <workspace>/app/api/intern/saker/[id]/route.ts.**

<workspace>/app/api/intern/saker/[id]/route.ts

<details>
<summary>15 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { finnInternSak } from "../../../../../lib/saker.ts";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const sak = finnInternSak(Number((await params).id));
  if (!sak) {
    return Response.json({ error: "Fant ikke saken" }, { status: 404 });
  }
  return Response.json(sak);
}
```

</details>


### `view`

**view the file at <workspace>/app/api/intern/saker/[id]/notater/route.ts.**

<workspace>/app/api/intern/saker/[id]/notater/route.ts

<details>
<summary>19 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { leggTilNotat } from "../../../../../../lib/saker.ts";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const { tekst } = (await request.json()) as { tekst?: unknown };
  if (typeof tekst !== "string" || tekst.trim() === "") {
    return Response.json({ error: "Notatet mangler tekst" }, { status: 400 });
  }
  const sak = leggTilNotat(Number((await params).id), tekst);
  if (!sak) {
    return Response.json({ error: "Fant ikke saken" }, { status: 404 });
  }
  return Response.json(sak, { status: 201 });
}
```

</details>


### `view`

**view the file at <workspace>/app/api/saker/route.ts.**

<workspace>/app/api/saker/route.ts

<details>
<summary>7 lines</summary>

```
import { requireSaksbehandler } from "../../../lib/auth.ts";
import { listSaker } from "../../../lib/saker.ts";

export async function GET(request: Request): Promise<Response> {
  const unauthorized = await requireSaksbehandler(request);
  if (unauthorized) return unauthorized;
  return Response.json(listSaker());
}
```

</details>


### `view`

**view the file at <workspace>/tests/probes.test.ts.**

<workspace>/tests/probes.test.ts

<details>
<summary>10 lines</summary>

```
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
```

</details>


### `view`

**view the file at <workspace>/docs/drift.md.**

<workspace>/docs/drift.md

<details>
<summary>9 lines</summary>

```
# Drift

Appen kjører på NAIS med Azure AD-innlogging via Wonderwall. Wonderwall legger
saksbehandlerens token i `Authorization`-headeren.

- NAIS kaller `/api/isalive` og `/api/isready` uten token. De må alltid svare 200,
  ellers starter podene på nytt i en løkke.
- Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres
  med `requireSaksbehandler()` i `lib/auth.ts`.
- `AZURE_OPENID_CONFIG_JWKS_URI`, `AZURE_OPENID_CONFIG_ISSUER` og
  `AZURE_APP_CLIENT_ID` settes av NAIS.
```

</details>


### `view`

**view the file at <workspace>/package.json.**

<workspace>/package.json

<details>
<summary>24 lines</summary>

```
{
  "name": "saksoversikt",
  "private": true,
  "type": "module",
  "engines": { "node": "24" },
  "scripts": {
    "build": "next build --webpack",
    "start": "next start",
    "test": "node --test tests/*.test.ts",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@navikt/oasis": "4.2.2",
    "next": "16.3.8",
    "react": "19.3.0",
    "react-dom": "19.3.0"
  },
  "devDependencies": {
    "@types/node": "26.6.2",
    "@types/react": "19.3.0",
    "jose": "6.2.12",
    "typescript": "6.0.3"
  }
}
```

</details>


### `rg`

**getToken|validateToken**

"getToken|validateToken" (node_modules/@navikt/oasis)

<details>
<summary>20 matches</summary>

```
[grep content: 15 matches across 3 file(s) under <workspace>/node_modules/@navikt/oasis/dist; compact summary]
index.mjs (9 match(es)):
  332: function getToken(val) {
  334: else if (val instanceof Headers) return getTokenFromHeaders(val);
  335: else if (val instanceof Request) return getTokenFromHeaders(val.headers);
  341: function getTokenFromHeaders(headers) {
  420: function getTokenXEnvs() {
  499: const validateTokenxToken = (token) => validateJwt({
  501: ...getTokenXEnvs()
  509: const validateToken = async (token) => {
  519: export { TokenResult, ValidationResult, expiresIn, getToken, parseAzureUse ... [+105 chars] ... alidateAzureToken, validateIdportenToken, validateToken, validateTokenxToken };
index.mjs.map (1 match(es)):
  ... .now() / 1000 > this.expirationTime;\n }\n}\n\n// Implementation of a Sieve Cache\n// https://cachemon.github.io/SIEVE-website/blog/2023/12/17/sieve-is-simpler-than-lru/\nc ...
index.d.mts (5 match(es)):
  68: declare function getToken(val: string): string;
  69: declare function getToken(val: Request | IncomingMessageLike | Headers): string | null;
  120: declare const validateTokenxToken: (token: string) => Promise<ValidationResult>;
  127: declare const validateToken: (token: string) => Promise<ValidationResult>;
  152: export { type ParseResult, TokenResult, ValidationResult, expiresIn, getTo ... [+123 chars] ... alidateAzureToken, validateIdportenToken, validateToken, validateTokenxToken };
[extensions: .mjs=1, .map=1, .mts=1]
```

</details>


### `view`

**view the file at <workspace>.**

<workspace>

<details>
<summary>10 lines</summary>

```
.github
app
docs
lib
node_modules
package-lock.json
package.json
proxy.ts
tests
tsconfig.json
```

</details>


### `bash`

**Verify invalid bearer token passes internal list guard**

$ cd <workspace> && node --input-type=module -e 'import {getToken} from "@navikt/oasis"; const r = new Request("http://localhost/api/intern/saker", {headers:{Authorization:"Bearer invalid"}}); console.log("getToken with invalid token:",getToken(r)); const {GET}=await import("./app/api/intern/saker/route.ts"); const response=await GET(r); console.log("internal list status:",response.status); console.log("internal list body:",await response.text());'

```
getToken with invalid token: invalid
internal list status: 200
internal list body: [{"id":101,"tittel":"Søknad om dagpenger","status":"apen","internNotat":"Venter på arbeidsgiver"},{"id":102,"tittel":"Klage på vedtak","status":"lukket","internNotat":"Omgjort etter klage"}]
<shellId: 2 completed with exit code 0>
```


### `view`

**view the file at <workspace>/node_modules/@navikt/oasis/dist/index.mjs.**

<workspace>/node_modules/@navikt/oasis/dist/index.mjs (lines 330-350)

<details>
<summary>21 lines</summary>

```
//#endregion
//#region src/token-utils/get-token.ts
function getToken(val) {
	if (typeof val === "string") return stripBearer(val);
	else if (val instanceof Headers) return getTokenFromHeaders(val);
	else if (val instanceof Request) return getTokenFromHeaders(val.headers);
	else {
		const authHeader = val.headers.authorization;
		return authHeader ? stripBearer(authHeader) : null;
	}
}
function getTokenFromHeaders(headers) {
	const authHeader = headers.get("authorization");
	return authHeader ? stripBearer(authHeader) : null;
}
//#endregion
//#region src/token-utils/parse-token.ts
/**
* Parses Idporten token, returning PID if present. This does not support parsing
* tokenx tokens.
*
```

</details>


### `bash`

**Check workspace status and run existing auth probe tests**

$ cd <workspace> && git --no-pager status --short -- .; cd <workspace> && npm test

<details>
<summary>14 lines</summary>

```
fatal: unable to get current working directory: Operation not permitted
npm notice run test
npm notice run node --test tests/*.test.ts
✔ isalive svarer uten token (0.988916ms)
✔ saker krever token (0.553584ms)
ℹ tests 2
ℹ suites 0
ℹ pass 2
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 113.906625
<shellId: 3 completed with exit code 0>
```

</details>


### Copilot

## Security Findings

### Alert 1
**File:** `<workspace>/app/api/intern/saker/route.ts:5–8`  
**Category:** AuthenticationFailure  
**Severity: HIGH | Confidence: 10/10**

**Problem:** Internal handlers check only whether `getToken(request)` returns a string. That function extracts the Authorization header; it does not validate the token. An attacker can supply `Authorization: Bearer invalid` to read internal cases and notes, or append notes, without valid authentication.

The proxy does not compensate: `<workspace>/proxy.ts:14` matches only `/api/saker/:path*`, excluding `/api/intern/saker`. Its guard also checks token presence rather than validity.

**Exact affected surfaces:**
- `GET /api/intern/saker` — guard and response at `<workspace>/app/api/intern/saker/route.ts:5–8`.
- `GET /api/intern/saker/[id]` — guard at `<workspace>/app/api/intern/saker/[id]/route.ts:8–10`.
- `POST /api/intern/saker/[id]/notater` — guard at `<workspace>/app/api/intern/saker/[id]/notater/route.ts:8–10`.

**Evidence:** Directly invoked the internal list handler with `Bearer invalid`: it returned **200** with both cases, including `internNotat`. The installed Oasis implementation confirms that `getToken` only extracts the header. Requests with no token are rejected; arbitrary nonempty tokens bypass authentication.

**Minimal recommended fix:** In all three internal handlers, replace the `getToken` check with:
```ts
const unauthorized = await requireSaksbehandler(request);
if (unauthorized) return unauthorized;
```
Use the existing helper at `<workspace>/lib/auth.ts:7–17`, which calls `validateToken`. Perform this before reading or modifying cases. Extend the proxy matcher to include `/api/intern/saker/:path*` for consistency, but matcher changes alone do not fix the bypass.

**Existing test coverage:** `<workspace>/tests/probes.test.ts:6–12` contains two passing tests: unauthenticated liveness and rejection of a missing token on `/api/saker`. Neither covers internal endpoints or invalid tokens. Add missing-token and invalid-token rejection tests for all three affected handlers, including verification that rejected POST requests cannot modify notes.

No files modified. Git inspection was blocked by filesystem permissions; the finding is verified against the current files and runtime behavior.


---

<sub>1m 6s</sub>

### `skill`

**nav-auth**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "nav-auth"
}
```

</details>

```
Skill "nav-auth" loaded successfully. Follow the instructions in the skill context.
```


---

<sub>1m 6s</sub>

### `skill`

**klarsprak**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "klarsprak"
}
```

</details>

```
Skill "klarsprak" loaded successfully. Follow the instructions in the skill context.
```


---

<sub>1m 6s</sub>

### `bash`

**Inspect repo files and local modifications**

$ pwd && ls -a && git --no-pager status --short -- .

<details>
<summary>15 lines</summary>

```
<workspace>
.
..
.github
app
docs
lib
node_modules
package-lock.json
package.json
proxy.ts
tests
tsconfig.json
fatal: unable to get current working directory: Operation not permitted
<shellId: 4 completed with exit code 128>
```

</details>


---

<sub>1m 6s</sub>

### `glob`

**.github/instructions/*.md**

".github/instructions/*.md"

<details>
<summary>16 matches</summary>

```
./.github/instructions/norwegian-text.instructions.md
./.github/instructions/testing.instructions.md
./.github/instructions/testing-kotlin.instructions.md
./.github/instructions/security-owasp.instructions.md
./.github/instructions/kotlin.instructions.md
./.github/instructions/security-core.instructions.md
./.github/instructions/testing-typescript.instructions.md
./.github/instructions/golang.instructions.md
./.github/instructions/github-actions.instructions.md
./.github/instructions/nextjs-aksel.instructions.md
./.github/instructions/database.instructions.md
./.github/instructions/output-style.instructions.md
./.github/instructions/accessibility.instructions.md
./.github/instructions/docker.instructions.md
./.github/instructions/performance.instructions.md
./.github/instructions/deliberate-ai-use.instructions.md
```

</details>


---

<sub>1m 17s</sub>

### Info

Session limits: 18.05/30 AI credits used.

---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/lib/auth.ts.**

<workspace>/lib/auth.ts

<details>
<summary>16 lines</summary>

```
import { getToken, validateToken } from "@navikt/oasis";

/**
 * Validerer Azure-tokenet til saksbehandleren. Returnerer et 401-svar som route
 * handleren skal sende videre, eller null når tokenet er gyldig.
 */
export async function requireSaksbehandler(request: Request): Promise<Response | null> {
  const token = getToken(request);
  if (!token) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const validation = await validateToken(token);
  if (!validation.ok) {
    return Response.json({ error: "Ugyldig token" }, { status: 401 });
  }
  return null;
}
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/app/api/intern/saker/route.ts.**

<workspace>/app/api/intern/saker/route.ts

<details>
<summary>8 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { listInterneSaker } from "../../../../lib/saker.ts";

export async function GET(request: Request): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  return Response.json(listInterneSaker());
}
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/app/api/intern/saker/[id]/route.ts.**

<workspace>/app/api/intern/saker/[id]/route.ts

<details>
<summary>15 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { finnInternSak } from "../../../../../lib/saker.ts";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const sak = finnInternSak(Number((await params).id));
  if (!sak) {
    return Response.json({ error: "Fant ikke saken" }, { status: 404 });
  }
  return Response.json(sak);
}
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/app/api/intern/saker/[id]/notater/route.ts.**

<workspace>/app/api/intern/saker/[id]/notater/route.ts

<details>
<summary>19 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { leggTilNotat } from "../../../../../../lib/saker.ts";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const { tekst } = (await request.json()) as { tekst?: unknown };
  if (typeof tekst !== "string" || tekst.trim() === "") {
    return Response.json({ error: "Notatet mangler tekst" }, { status: 400 });
  }
  const sak = leggTilNotat(Number((await params).id), tekst);
  if (!sak) {
    return Response.json({ error: "Fant ikke saken" }, { status: 404 });
  }
  return Response.json(sak, { status: 201 });
}
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/proxy.ts.**

<workspace>/proxy.ts

<details>
<summary>13 lines</summary>

```
import { getToken } from "@navikt/oasis";
import { NextResponse, type NextRequest } from "next/server";

// Rask sjekk før route handlerne: forespørsler uten token avvises med en gang.
// Selve valideringen av tokenet gjør hver route handler med requireSaksbehandler().
export function proxy(request: NextRequest): NextResponse {
  if (!getToken(request)) {
    return NextResponse.json({ error: "Mangler token" }, { status: 401 });
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/api/saker/:path*"],
};
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/tests/probes.test.ts.**

<workspace>/tests/probes.test.ts

<details>
<summary>10 lines</summary>

```
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
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/package.json.**

<workspace>/package.json

<details>
<summary>24 lines</summary>

```
{
  "name": "saksoversikt",
  "private": true,
  "type": "module",
  "engines": { "node": "24" },
  "scripts": {
    "build": "next build --webpack",
    "start": "next start",
    "test": "node --test tests/*.test.ts",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@navikt/oasis": "4.2.2",
    "next": "16.3.8",
    "react": "19.3.0",
    "react-dom": "19.3.0"
  },
  "devDependencies": {
    "@types/node": "26.6.2",
    "@types/react": "19.3.0",
    "jose": "6.2.12",
    "typescript": "6.0.3"
  }
}
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/.github/instructions/security-owasp.instructions.md.**

<workspace>/.github/instructions/security-owasp.instructions.md

<details>
<summary>9 lines</summary>

```
---
description: "Hvor OWASP Top 10:2025-mønstrene for Kotlin, Go, Java og TypeScript ligger: tilgangskontroll, feilkonfigurasjon, forsyningskjede, kryptografi, injeksjon, logging og feilhåndtering."
applyTo: "**/*.{kt,go,java,ts,tsx}"
---

# Security Essentials

The rules that always apply (logging, secrets, queries, ownership, `azp`, TLS) are in `security-core.instructions.md`, which is loaded in every session. If it is not installed next to this file, tell the developer to install it: https://github.com/navikt/copilot/blob/main/instructions/security-core.instructions.md

For detailed OWASP Top 10:2025 code-level patterns (Kotlin, Go, Java, Node.js), invoke the `$security-owasp` skill.
For scanning workflows (trivy, zizmor, govulncheck), use the `$security-review` skill.
For architecture-level threat modeling, use `@security-champion`.
```

</details>


---

<sub>1m 18s</sub>

### `view`

**view the file at <workspace>/.github/instructions/testing.instructions.md.**

<workspace>/.github/instructions/testing.instructions.md

<details>
<summary>85 lines</summary>

````
---
name: Testing Standards
description: "Felles testprinsipper for Nav. Språkspesifikke eksempler finnes i egne instruksjoner for Kotlin og TypeScript."
applyTo: "**/*.test.{ts,tsx,kt,kts}"
---

# Testing Standards

Common testing principles for Nav. Language-specific examples can be found in separate instructions for [Kotlin](testing-kotlin.instructions.md) and [TypeScript](testing-typescript.instructions.md).

## Test Coverage

### Coverage Requirements

- **Utilities in `lib/`**: 80%+ coverage required
- **Business logic**: 70%+ coverage required
- **API routes**: Test happy path + error cases
- **Repositories**: Test CRUD operations
- **Event handlers**: Test event processing + publishing

## Test Naming

```kotlin
// ✅ Good - describes behavior
`should create user when valid data provided`
`should throw exception when email is invalid`
`should publish event after successful processing`

// ❌ Bad - not descriptive
`test1`
`createUserTest`
`testValidation`
```

## Test Strategy

Choose test type based on what you're verifying:

| What to test | Test type | Tools |
|---|---|---|
| Pure functions, utils | Unit test | Kotest / Vitest |
| Controller + validation | Slice test | `@WebMvcTest` + MockkBean |
| Repository + SQL | Slice test | `@DataJpaTest` + Testcontainers |
| Full API flow | Integration test | `@SpringBootTest` + Testcontainers |
| User workflows | E2E test | Playwright |
| Accessibility | E2E test | Playwright + axe-core |

### When to use what

- **Unit**: Business logic, data transformations, formatting
- **Slice** (`@WebMvcTest`, `@DataJpaTest`): Faster than full integration, tests one layer
- **Integration** (`@SpringBootTest`): Auth flow, multi-layer, real DB
- **E2E** (Playwright): Critical user journeys, form submission, navigation

## Playwright E2E Tests

```typescript
import { test, expect } from "@playwright/test";

test.describe("Oversikt", () => {
  test("should display vedtak list", async ({ page }) => {
    await page.goto("/oversikt");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByRole("table")).toBeVisible();
  });

  test("should filter by status", async ({ page }) => {
    await page.goto("/oversikt");
    await page.getByRole("combobox", { name: /status/i }).selectOption("aktiv");
    await expect(page.getByRole("row")).toHaveCount(await page.getByRole("row").count());
  });
});
```

### Accessibility in E2E

```typescript
import AxeBuilder from "@axe-core/playwright";

test("should have no a11y violations", async ({ page }) => {
  await page.goto("/oversikt");
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});
```

## Boundaries

### ✅ Always

- Write tests for new code before committing
- Test both success and error cases
- Use descriptive test names
- Clean up test data after each test
- Run full test suite before pushing

### ⚠️ Ask First

- Changing test framework or structure
- Adding complex test fixtures
- Modifying shared test utilities
- Disabling or skipping tests

### 🚫 Never

- Commit failing tests
- Skip tests without good reason
- Test implementation details
- Share mutable state between tests
- Commit without running tests
````

</details>


---

<sub>1m 19s</sub>

### `view`

**view the file at <workspace>/.github/instructions/testing-typescript.instructions.md.**

<workspace>/.github/instructions/testing-typescript.instructions.md

<details>
<summary>75 lines</summary>

````
---
name: TypeScript Testing
description: "TypeScript-spesifikke testmønstre for Nav: Vitest, mocking, async og React-komponenttesting."
applyTo: "**/*.test.{ts,tsx}"
---

# TypeScript Testing (Vitest)

TypeScript-specific test patterns for Nav: Vitest, mocking, async, and React component testing.

## Test Structure

```typescript
import { formatNumber } from "./format";

describe("formatNumber", () => {
  it("should format numbers with Norwegian locale", () => {
    expect(formatNumber(151354)).toBe("151 354");
  });

  it("should handle decimal numbers", () => {
    expect(formatNumber(1234.56)).toBe("1 234,56");
  });

  it("should handle negative numbers", () => {
    expect(formatNumber(-1000)).toBe("-1 000");
  });
});
```

## Testing Async Functions

```typescript
describe("fetchData", () => {
  it("should fetch data successfully", async () => {
    const result = await fetchData("test-id");

    expect(result).toBeDefined();
    expect(result.id).toBe("test-id");
  });

  it("should handle errors", async () => {
    await expect(fetchData("invalid")).rejects.toThrow("Not found");
  });
});
```

## Mocking

```typescript
import { vi } from "vitest";

// Mock external module
vi.mock("./cached-bigquery", () => ({
  getCachedBigQueryUsage: vi.fn(),
}));

import { getCachedBigQueryUsage } from "./cached-bigquery";

describe("API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return usage data", async () => {
    vi.mocked(getCachedBigQueryUsage).mockResolvedValue({
      usage: [{ date: "2025-01-01", total_active_users: 100 }],
      error: null,
    });

    const response = await GET();
    const data = await response.json();

    expect(data.usage).toHaveLength(1);
  });
});
```

## Testing React Components

```typescript
import { render, screen } from "@testing-library/react";
import { MetricCard } from "./metric-card";

describe("MetricCard", () => {
  it("should render title and value", () => {
    render(<MetricCard title="Total Users" value={100} icon={UserIcon} />);

    expect(screen.getByText("Total Users")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
  });
});
```

## Run Tests

```bash
pnpm test
pnpm test --coverage
```
````

</details>


---

<sub>1m 19s</sub>

### `glob`

**docs/***

"docs/*"

```
./docs/drift.md
```


---

<sub>1m 28s</sub>

### Copilot

Route-handlerne sjekker bare at et token finnes, ikke at det er gyldig. Jeg bruker den eksisterende valideringen i alle tre interne handlerne; en endring i proxy-matcher alene ville ikke stoppet ugyldige tokens.


---

<sub>1m 29s</sub>

### `view`

**view the file at <workspace>/lib/saker.ts.**

<workspace>/lib/saker.ts

<details>
<summary>20 lines</summary>

```
export type Sak = { id: number; tittel: string; status: "apen" | "lukket" };
export type InternSak = Sak & { internNotat: string };

const saker: InternSak[] = [
  { id: 101, tittel: "Søknad om dagpenger", status: "apen", internNotat: "Venter på arbeidsgiver" },
  { id: 102, tittel: "Klage på vedtak", status: "lukket", internNotat: "Omgjort etter klage" },
];

export function listSaker(): Sak[] {
  return saker.map(({ internNotat: _, ...sak }) => sak);
}

export function listInterneSaker(): InternSak[] {
  return [...saker];
}

export function finnInternSak(id: number): InternSak | undefined {
  return saker.find((sak) => sak.id === id);
}

export function leggTilNotat(id: number, tekst: string): InternSak | undefined {
  const sak = finnInternSak(id);
  if (sak) sak.internNotat = `${sak.internNotat}\n${tekst}`;
  return sak;
}
```

</details>


---

<sub>1m 29s</sub>

### `view`

**view the file at <workspace>/docs/drift.md.**

<workspace>/docs/drift.md

<details>
<summary>9 lines</summary>

```
# Drift

Appen kjører på NAIS med Azure AD-innlogging via Wonderwall. Wonderwall legger
saksbehandlerens token i `Authorization`-headeren.

- NAIS kaller `/api/isalive` og `/api/isready` uten token. De må alltid svare 200,
  ellers starter podene på nytt i en løkke.
- Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres
  med `requireSaksbehandler()` i `lib/auth.ts`.
- `AZURE_OPENID_CONFIG_JWKS_URI`, `AZURE_OPENID_CONFIG_ISSUER` og
  `AZURE_APP_CLIENT_ID` settes av NAIS.
```

</details>


---

<sub>1m 29s</sub>

### `view`

**view the file at <workspace>/.github/instructions/norwegian-text.instructions.md.**

<workspace>/.github/instructions/norwegian-text.instructions.md

<details>
<summary>35 lines</summary>

````
---
description: "Minimumsregler for norsk markdown: Nav, ikke NAV, sammensatte ord, engelsk fagspråk, overskrifter og konsekvent bokmål."
applyTo: "**/*.md"
---

# Norsk tekstkvalitet

Regler for norsk tekst i markdown-filer: agenter, instruksjoner, skills, dokumentasjon og README-er. Dette er minimumsreglene som gjelder automatisk ved redigering og code review.

Språknøytrale skriveregler (lengde, tetthet, tegnsetting) står i `output-style.instructions.md`. Ordlistene over KI-markører ligger sammen med klarspråk-prinsipper, anglisismer, fagtermer, teksttyper og full språkvask i `klarsprak`-skillen. Bruk den når du skriver eller redigerer norsk tekst av noe omfang. For dypere tekstredaksjon, bruk `@forfatter`-agenten.

## Nav, ikke NAV

"Nav" med stor forbokstav og små bokstaver. Aldri "NAV" (gammelt akronym).

## Sammensatte ord

Bindestrek ved engelsk+norsk. Særskriving er feil.

```
✅ image-bygg, CI-pipeline, deploy-steg, Postgres-operatoren, Kafka-topicet, GitHub-repoet, PR-er
❌ Postgres operatoren, Kafka topicet, GitHub repoet
```

## Behold engelsk fagspråk

Ikke oversett: image, cluster, node, container, release, pod, namespace, secret, bug, bugfix, hotfix, patch, edge case, rollback, failover, backup, pipeline, workflow, runtime, framework, middleware, pull request, merge, commit, branch, endpoint, payload, token, scope, host, skill, hook, prompt.

`deployment` som substantiv beholdes på engelsk. Verbet «deploye» og «rulle ut» er OK.

Skriv «host» og «hosts», ikke «vert», «verter» eller «vertsnavn». Det gjelder enhver teknisk betydning, maskinen like mye som endepunktet: «vert» sier ikke hvilken av dem det er. Skriv «skill», ikke «ferdighet». «Ferdighet» om menneskelig kompetanse er en annen betydning og er riktig norsk.

Lista her er et utdrag. Hele lista, med hva hver term ikke skal oversettes til, står i `klarsprak`-skillen, og den er fasit.

## Overskrifter

Bare første ord og egennavn med stor bokstav, ikke engelsk tittelstil. (Kolon på slutten av overskrifter er forbudt av `output-style.instructions.md`.)

## Konsekvent bokmål

Ikke bland inn nynorsk eller svensk. De vanligste feilene i KI-generert bokmål:

- **-ingar** → **-inger** (endringer, oppdateringer)
- **-leg/-lege** → **-lig/-lige** (tydelig, mulig)
- **-aste** → **-ste** (viktigste)
- **ei-/eig-** i starten → **e-/eg-** (egenskap, egentlig)
- **kv-** → **hv-** (hver, hvorfor)
- **-ar** i ubestemt flertall → **-er** (brukere, filer), **-ane** i bestemt flertall → **-ene** (brukerne, filene)
- medan → mens, vart/vorte → ble/blitt, berre → bare, mykje → mye, difor → derfor, ikkje → ikke
- Svensk: engångs- → engangs-, ändring → endring, användare → bruker (å/ä der bokmål har a/e)

Ikke veksle mellom gyldige former (stein/sten, framtid/fremtid) i samme tekst. A-endelser («sida», «fila», «endra») er gyldig ledig bokmål og skal beholdes ved konsekvent bruk.

## Tone

Unngå superlativer og amerikansk stil.
````

</details>


---

<sub>1m 29s</sub>

### `view`

**view the file at <workspace>/tsconfig.json.**

<workspace>/tsconfig.json

<details>
<summary>17 lines</summary>

```
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["dom", "es2022"],
    "module": "esnext",
    "moduleResolution": "bundler",
    "jsx": "preserve",
    "strict": true,
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "skipLibCheck": true,
    "types": ["node"],
    "plugins": [{ "name": "next" }]
  },
  "include": ["**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules"]
}
```

</details>


---

<sub>1m 29s</sub>

### `glob`

**node_modules/@navikt/oasis/**/*.{js,ts}**

"node_modules/@navikt/oasis/**/*.{js,ts}"

```
No files matched the pattern.
```


---

<sub>1m 30s</sub>

### `bash`

**Check Node version and baseline tests and types**

$ node --version && npm test && npm run typecheck

<details>
<summary>16 lines</summary>

```
v24.21.0
npm notice run test
npm notice run node --test tests/*.test.ts
✔ isalive svarer uten token (1.028042ms)
✔ saker krever token (0.581083ms)
ℹ tests 2
ℹ suites 0
ℹ pass 2
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 99.529542
npm notice run typecheck
npm notice run tsc --noEmit
<shellId: 5 completed with exit code 0>
```

</details>


---

<sub>1m 36s</sub>

### `bash`

**Locate installed Oasis validation implementation**

$ ls node_modules/@navikt/oasis && cat node_modules/@navikt/oasis/package.json && ls node_modules/@navikt/oasis/dist

<details>
<summary>49 lines</summary>

```
dist
package.json
{
  "name": "@navikt/oasis",
  "version": "4.2.2",
  "license": "MIT",
  "description": "A simple nais/texas client for token management in a nais application",
  "engineStrict": true,
  "engines": {
    "node": "^20.19.0 || >= 22.12.0 || 24.x || 25.x"
  },
  "publishConfig": {
    "access": "public",
    "registry": "https://npm.pkg.github.com/"
  },
  "type": "module",
  "files": [
    "package.json",
    "dist"
  ],
  "types": "./dist/index.d.mts",
  "exports": {
    ".": "./dist/index.mjs"
  },
  "scripts": {
    "build": "tsdown",
    "test": "vitest",
    "lint": "biome check",
    "tsc": "tsc --pretty"
  },
  "dependencies": {
    "@navikt/texas": "^4.2.2",
    "@opentelemetry/api": "^1.9.1",
    "@prometheus-io/client": "^0.16.1",
    "jose": "^6.2.10"
  },
  "devDependencies": {
    "@types/node": "^25.9.5",
    "msw": "^2.15.0",
    "node-mocks-http": "^1.18.1",
    "tsdown": "0.22.14",
    "typescript": "^6.0.3",
    "vitest": "^4.1.11"
  }
}index.d.mts
index.d.mts.map
index.mjs
index.mjs.map
<shellId: 6 completed with exit code 0>
```

</details>


---

<sub>1m 39s</sub>

### `rg`

**validateToken|JWKS|ISSUER|CLIENT_ID|function getToken|isLocal**

"validateToken|JWKS|ISSUER|CLIENT_ID|function getToken|isLocal" in *.mjs (node_modules/@navikt/oasis/dist/index.mjs)

<details>
<summary>124 matches</summary>

```
node_modules/@navikt/oasis/dist/index.mjs-import { createHash } from "node:crypto";
node_modules/@navikt/oasis/dist/index.mjs-import { Counter, Histogram } from "@prometheus-io/client";
node_modules/@navikt/oasis/dist/index.mjs-import texas from "@navikt/texas";
node_modules/@navikt/oasis/dist/index.mjs-import { SpanStatusCode, context, trace } from "@opentelemetry/api";
node_modules/@navikt/oasis/dist/index.mjs:import { createRemoteJWKSet, errors, jwtVerify } from "jose";
node_modules/@navikt/oasis/dist/index.mjs-//#region src/token-result.ts
node_modules/@navikt/oasis/dist/index.mjs-const TokenResult = {
node_modules/@navikt/oasis/dist/index.mjs-	Error: (error) => ({
node_modules/@navikt/oasis/dist/index.mjs-		ok: false,
node_modules/@navikt/oasis/dist/index.mjs-		error: typeof error === "string" ? Error(error) : error
node_modules/@navikt/oasis/dist/index.mjs-	}),
--
node_modules/@navikt/oasis/dist/index.mjs-* @param token Token from your client (subject token).
node_modules/@navikt/oasis/dist/index.mjs-* @param audience The target app you request a token for.
node_modules/@navikt/oasis/dist/index.mjs-*/
node_modules/@navikt/oasis/dist/index.mjs-const requestOboToken = async (token, audience) => {
node_modules/@navikt/oasis/dist/index.mjs-	if (!token) return TokenResult.Error("empty token");
node_modules/@navikt/oasis/dist/index.mjs-	if (!audience) return TokenResult.Error("empty audience");
node_modules/@navikt/oasis/dist/index.mjs:	const tokenx = !!process.env.TOKEN_X_ISSUER;
node_modules/@navikt/oasis/dist/index.mjs:	const azure = !!process.env.AZURE_OPENID_CONFIG_ISSUER;
node_modules/@navikt/oasis/dist/index.mjs-	if (tokenx && azure) return TokenResult.Error("Multiple identity providers, use requestTokenxOboToken or requestAzureOboToken directly!");
node_modules/@navikt/oasis/dist/index.mjs-	else if (tokenx) return requestTokenxOboToken(token, audience);
node_modules/@navikt/oasis/dist/index.mjs-	else if (azure) return requestAzureOboToken(token, audience);
node_modules/@navikt/oasis/dist/index.mjs-	else return TokenResult.Error("No identity provider configured in your nais.yaml");
node_modules/@navikt/oasis/dist/index.mjs-};
node_modules/@navikt/oasis/dist/index.mjs-//#endregion
node_modules/@navikt/oasis/dist/index.mjs-//#region src/token-utils/get-token.ts
node_modules/@navikt/oasis/dist/index.mjs:function getToken(val) {
node_modules/@navikt/oasis/dist/index.mjs-	if (typeof val === "string") return stripBearer(val);
node_modules/@navikt/oasis/dist/index.mjs-	else if (val instanceof Headers) return getTokenFromHeaders(val);
node_modules/@navikt/oasis/dist/index.mjs-	else if (val instanceof Request) return getTokenFromHeaders(val.headers);
node_modules/@navikt/oasis/dist/index.mjs-	else {
node_modules/@navikt/oasis/dist/index.mjs-		const authHeader = val.headers.authorization;
node_modules/@navikt/oasis/dist/index.mjs-		return authHeader ? stripBearer(authHeader) : null;
node_modules/@navikt/oasis/dist/index.mjs-	}
node_modules/@navikt/oasis/dist/index.mjs-}
node_modules/@navikt/oasis/dist/index.mjs:function getTokenFromHeaders(headers) {
node_modules/@navikt/oasis/dist/index.mjs-	const authHeader = headers.get("authorization");
node_modules/@navikt/oasis/dist/index.mjs-	return authHeader ? stripBearer(authHeader) : null;
node_modules/@navikt/oasis/dist/index.mjs-}
node_modules/@navikt/oasis/dist/index.mjs-//#endregion
node_modules/@navikt/oasis/dist/index.mjs-//#region src/token-utils/parse-token.ts
node_modules/@navikt/oasis/dist/index.mjs-/**
--
node_modules/@navikt/oasis/dist/index.mjs-		throw error;
node_modules/@navikt/oasis/dist/index.mjs-	}
node_modules/@navikt/oasis/dist/index.mjs-}
node_modules/@navikt/oasis/dist/index.mjs-//#endregion
node_modules/@navikt/oasis/dist/index.mjs-//#region src/token-validation/envs.ts
node_modules/@navikt/oasis/dist/index.mjs-function getIdportenEnvs() {
node_modules/@navikt/oasis/dist/index.mjs:	if (!process.env.IDPORTEN_JWKS_URI || !process.env.IDPORTEN_ISSUER || !process.env.IDPORTEN_AUDIENCE) throw Error("Trying to validate Idporten but no IDPORTEN_* envs! Are you in nais?");
node_modules/@navikt/oasis/dist/index.mjs-	return {
node_modules/@navikt/oasis/dist/index.mjs:		jwksUri: process.env.IDPORTEN_JWKS_URI,
node_modules/@navikt/oasis/dist/index.mjs:		issuer: process.env.IDPORTEN_ISSUER,
node_modules/@navikt/oasis/dist/index.mjs-		audience: process.env.IDPORTEN_AUDIENCE
node_modules/@navikt/oasis/dist/index.mjs-	};
node_modules/@navikt/oasis/dist/index.mjs-}
node_modules/@navikt/oasis/dist/index.mjs-function getAzureEnvs() {
node_modules/@navikt/oasis/dist/index.mjs:	if (!process.env.AZURE_OPENID_CONFIG_JWKS_URI || !process.env.AZURE_OPENID_CONFIG_ISSUER || !process.env.AZURE_APP_CLIENT_ID) throw Error("Trying to validate Azure but no AZURE_* envs! Are you in nais?");
node_modules/@navikt/oasis/dist/index.mjs-	return {
node_modules/@navikt/oasis/dist/index.mjs:		jwksUri: process.env.AZURE_OPENID_CONFIG_JWKS_URI,
node_modules/@navikt/oasis/dist/index.mjs:		issuer: process.env.AZURE_OPENID_CONFIG_ISSUER,
node_modules/@navikt/oasis/dist/index.mjs:		audience: process.env.AZURE_APP_CLIENT_ID
node_modules/@navikt/oasis/dist/index.mjs-	};
node_modules/@navikt/oasis/dist/index.mjs-}
node_modules/@navikt/oasis/dist/index.mjs:function getTokenXEnvs() {
node_modules/@navikt/oasis/dist/index.mjs:	if (!process.env.TOKEN_X_JWKS_URI || !process.env.TOKEN_X_ISSUER || !process.env.TOKEN_X_CLIENT_ID) throw Error("Trying to validate TokenX but no TOKEN_X_* envs! Are you in nais?");
node_modules/@navikt/oasis/dist/index.mjs-	return {
node_modules/@navikt/oasis/dist/index.mjs:		jwksUri: process.env.TOKEN_X_JWKS_URI,
node_modules/@navikt/oasis/dist/index.mjs:		issuer: process.env.TOKEN_X_ISSUER,
node_modules/@navikt/oasis/dist/index.mjs:		audience: process.env.TOKEN_X_CLIENT_ID
node_modules/@navikt/oasis/dist/index.mjs-	};
node_modules/@navikt/oasis/dist/index.mjs-}
node_modules/@navikt/oasis/dist/index.mjs-//#endregion
node_modules/@navikt/oasis/dist/index.mjs-//#region src/token-validation/types.ts
node_modules/@navikt/oasis/dist/index.mjs-const ValidationResult = {
node_modules/@navikt/oasis/dist/index.mjs-	Error: (error, errorType = "unknown") => ({
--
node_modules/@navikt/oasis/dist/index.mjs-		payload
node_modules/@navikt/oasis/dist/index.mjs-	})
node_modules/@navikt/oasis/dist/index.mjs-};
node_modules/@navikt/oasis/dist/index.mjs-//#endregion
node_modules/@navikt/oasis/dist/index.mjs-//#region src/token-validation/jwks.ts
node_modules/@navikt/oasis/dist/index.mjs-const ONE_DAY_IN_MILLIS = 864e5;
node_modules/@navikt/oasis/dist/index.mjs:const remoteJWKSetCache = {};
node_modules/@navikt/oasis/dist/index.mjs-function getJwkSet(jwksUri) {
node_modules/@navikt/oasis/dist/index.mjs:	if (remoteJWKSetCache[jwksUri] == null) remoteJWKSetCache[jwksUri] = createRemoteJWKSet(new URL(jwksUri), { cacheMaxAge: ONE_DAY_IN_MILLIS });
node_modules/@navikt/oasis/dist/index.mjs:	return remoteJWKSetCache[jwksUri];
node_modules/@navikt/oasis/dist/index.mjs-}
node_modules/@navikt/oasis/dist/index.mjs-//#endregion
node_modules/@navikt/oasis/dist/index.mjs-//#region src/token-validation/validate.ts
node_modules/@navikt/oasis/dist/index.mjs-const validateJwt = async ({ token, jwksUri, issuer, audience }) => {
node_modules/@navikt/oasis/dist/index.mjs-	return spanAsync("Validate JWT", async (span) => {
node_modules/@navikt/oasis/dist/index.mjs-		span.setAttributes({
--
node_modules/@navikt/oasis/dist/index.mjs-/**
node_modules/@navikt/oasis/dist/index.mjs-* Validates token issued by Tokenx. Requires Tokenx to be enabled in nais
node_modules/@navikt/oasis/dist/index.mjs-* application manifest.
node_modules/@navikt/oasis/dist/index.mjs-*
node_modules/@navikt/oasis/dist/index.mjs-* @param token Token issued by Tokenx.
node_modules/@navikt/oasis/dist/index.mjs-*/
node_modules/@navikt/oasis/dist/index.mjs:const validateTokenxToken = (token) => validateJwt({
node_modules/@navikt/oasis/dist/index.mjs-	token,
node_modules/@navikt/oasis/dist/index.mjs-	...getTokenXEnvs()
node_modules/@navikt/oasis/dist/index.mjs-});
node_modules/@navikt/oasis/dist/index.mjs-/**
node_modules/@navikt/oasis/dist/index.mjs-* Validates token issued by Idporten or Azure. Requires either Idporten or
node_modules/@navikt/oasis/dist/index.mjs-* Azure to be enabled in nais application manifest.
node_modules/@navikt/oasis/dist/index.mjs-*
node_modules/@navikt/oasis/dist/index.mjs-* @param token Token issued by Idporten or Azure.
node_modules/@navikt/oasis/dist/index.mjs-*/
node_modules/@navikt/oasis/dist/index.mjs:const validateToken = async (token) => {
node_modules/@navikt/oasis/dist/index.mjs-	if (!token) return ValidationResult.Error("empty token");
node_modules/@navikt/oasis/dist/index.mjs:	const idporten = !!process.env.IDPORTEN_ISSUER;
node_modules/@navikt/oasis/dist/index.mjs:	const azure = !!process.env.AZURE_OPENID_CONFIG_ISSUER;
node_modules/@navikt/oasis/dist/index.mjs-	if (idporten && azure) return ValidationResult.Error("multiple identity providers");
node_modules/@navikt/oasis/dist/index.mjs-	else if (idporten) return validateIdportenToken(token);
node_modules/@navikt/oasis/dist/index.mjs-	else if (azure) return validateAzureToken(token);
node_modules/@navikt/oasis/dist/index.mjs-	else return ValidationResult.Error("no identity provider");
node_modules/@navikt/oasis/dist/index.mjs-};
node_modules/@navikt/oasis/dist/index.mjs-//#endregion
node_modules/@navikt/oasis/dist/index.mjs:export { TokenResult, ValidationResult, expiresIn, getToken, parseAzureUserToken, parseIdportenToken, requestAzureClientCredentialsToken, requestAzureOboToken, requestOboToken, requestTokenxOboToken, validateAzureToken, validateIdportenToken, validateToken, validateTokenxToken };
node_modules/@navikt/oasis/dist/index.mjs-
node_modules/@navikt/oasis/dist/index.mjs-//# sourceMappingURL=index.mjs.map
```

</details>


---

<sub>2m 40s</sub>

### Info

Session limits: 24.83/30 AI credits used.

---

<sub>2m 40s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/app/api/intern/saker/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: <workspace>/app/api/intern/saker/[id]/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: <workspace>/app/api/intern/saker/[id]/notater/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: <workspace>/proxy.ts\n@@\n-  matcher: [\"/api/saker/:path*\"],\n+  matcher: [\"/api/saker/:path*\", \"/api/intern/saker/:path*\"],\n*** Update File: <workspace>/docs/drift.md\n@@\n - Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres\n   med `requireSaksbehandler()` i `lib/auth.ts`.\n+  Dette gjelder også `/api/intern/saker`, enkeltsaker og oppretting av notater.\n+  Manglende eller ugyldige tokens gir 401 før handleren leser eller endrer data.\n+  Proxyen sjekker bare at tokenet finnes; den erstatter ikke valideringen.\n*** Update File: <workspace>/tests/probes.test.ts\n@@\n import { test } from \"node:test\";\n+import { exportJWK, generateKeyPair, SignJWT } from \"jose\";\n import { GET as isalive } from \"../app/api/isalive/route.ts\";\n+import { GET as isready } from \"../app/api/isready/route.ts\";\n import { GET as saker } from \"../app/api/saker/route.ts\";\n+import { GET as interneSaker } from \"../app/api/intern/saker/route.ts\";\n+import { GET as internSak } from \"../app/api/intern/saker/[id]/route.ts\";\n+import { POST as notat } from \"../app/api/intern/saker/[id]/notater/route.ts\";\n+import { listInterneSaker } from \"../lib/saker.ts\";\n@@\n test(\"isalive svarer uten token\", async () => {\n   assert.equal((await isalive()).status, 200);\n });\n \n+test(\"isready svarer uten token\", async () => {\n+  assert.equal((await isready()).status, 200);\n+});\n+\n test(\"saker krever token\", async () => {\n   assert.equal((await saker(new Request(\"http://localhost/api/saker\"))).status, 401);\n });\n+\n+test(\"interne ruter validerer Azure-token før tilgang til saker\", async (t) => {\n+  const issuer = \"https://issuer.example.test\";\n+  const audience = \"saksoversikt-test\";\n+  const jwksUri = `${issuer}/jwks`;\n+  const { privateKey, publicKey } = await generateKeyPair(\"RS256\");\n+  const { privateKey: otherKey } = await generateKeyPair(\"RS256\");\n+  const jwk = { ...await exportJWK(publicKey), kid: \"test-key\", alg: \"RS256\" };\n+  t.mock.method(globalThis, \"fetch\", async (input: string | URL | Request) => {\n+    assert.equal(input instanceof Request ? input.url : String(input), jwksUri);\n+    return Response.json({ keys: [jwk] });\n+  });\n+\n+  const env = {\n+    AZURE_OPENID_CONFIG_ISSUER: issuer,\n+    AZURE_APP_CLIENT_ID: audience,\n+    AZURE_OPENID_CONFIG_JWKS_URI: jwksUri,\n+    IDPORTEN_ISSUER: undefined,\n+  };\n+  const previousEnv = Object.fromEntries(\n+    Object.keys(env).map((key) => [key, process.env[key]]),\n+  );\n+  for (const [key, value] of Object.entries(env)) {\n+    if (value === undefined) delete process.env[key];\n+    else process.env[key] = value;\n+  }\n+\n+  async function token({\n+    iss = issuer,\n+    aud = audience,\n+    exp = Math.floor(Date.now() / 1000) + 3600,\n+    key = privateKey,\n+  } = {}): Promise<string> {\n+    return new SignJWT({})\n+      .setProtectedHeader({ alg: \"RS256\", kid: \"test-key\" })\n+      .setSubject(\"test-saksbehandler\")\n+      .setIssuer(iss)\n+      .setAudience(aud)\n+      .setExpirationTime(exp)\n+      .sign(key);\n+  }\n+\n+  const routes = [\n+    { path: \"/api/intern/saker\", method: \"GET\", handler: interneSaker },\n+    { path: \"/api/intern/saker/101\", method: \"GET\", handler: internSak },\n+    { path: \"/api/intern/saker/101/notater\", method: \"POST\", handler: notat },\n+  ];\n+  function request(path: string, method: string, bearer?: string, body = '{\"tekst\":\"Testnotat\"}') {\n+    return new Request(`http://localhost${path}`, {\n+      method,\n+      headers: bearer ? { Authorization: `Bearer ${bearer}` } : {},\n+      body: method === \"POST\" ? body : undefined,\n+    });\n+  }\n+  const context = () => ({ params: Promise.resolve({ id: \"101\" }) });\n+  const originalSaker = structuredClone(listInterneSaker());\n+\n+  try {\n+    const rejected = [\n+      { name: \"manglende token\", bearer: undefined, error: \"Mangler token\" },\n+      { name: \"vilkårlig token\", bearer: \"not-a-jwt\", error: \"Ugyldig token\" },\n+      { name: \"feil signatur\", bearer: await token({ key: otherKey }), error: \"Ugyldig token\" },\n+      { name: \"utløpt token\", bearer: await token({ exp: Math.floor(Date.now() / 1000) - 3600 }), error: \"Ugyldig token\" },\n+      { name: \"feil issuer\", bearer: await token({ iss: \"https://other.example.test\" }), error: \"Ugyldig token\" },\n+      { name: \"feil audience\", bearer: await token({ aud: \"other-app\" }), error: \"Ugyldig token\" },\n+    ];\n+    for (const route of routes) {\n+      for (const invalid of rejected) {\n+        await t.test(`${route.method} ${route.path} avviser ${invalid.name}`, async () => {\n+          const response = await route.handler(\n+            request(route.path, route.method, invalid.bearer),\n+            context(),\n+          );\n+          assert.equal(response.status, 401);\n+          assert.deepEqual(await response.json(), { error: invalid.error });\n+          assert.deepEqual(listInterneSaker(), originalSaker);\n+        });\n+      }\n+    }\n+\n+    await t.test(\"notater avviser ugyldig token før parsing av body\", async () => {\n+      const response = await notat(\n+        request(\"/api/intern/saker/101/notater\", \"POST\", \"not-a-jwt\", \"{\"),\n+        context(),\n+      );\n+      assert.equal(response.status, 401);\n+      assert.deepEqual(listInterneSaker(), originalSaker);\n+    });\n+\n+    const valid = await token();\n+    for (const route of routes) {\n+      await t.test(`${route.method} ${route.path} godtar gyldig token`, async () => {\n+        const response = await route.handler(\n+          request(route.path, route.method, valid),\n+          context(),\n+        );\n+        assert.equal(response.status, route.method === \"POST\" ? 201 : 200);\n+        const expected = route.method === \"POST\"\n+          ? { ...originalSaker[0], internNotat: `${originalSaker[0].internNotat}\\nTestnotat` }\n+          : route.path === \"/api/intern/saker\" ? originalSaker : originalSaker[0];\n+        assert.deepEqual(await response.json(), expected);\n+      });\n+    }\n+  } finally {\n+    for (const [key, value] of Object.entries(previousEnv)) {\n+      if (value === undefined) delete process.env[key];\n+      else process.env[key] = value;\n+    }\n+    for (const sak of listInterneSaker()) {\n+      const original = originalSaker.find((item) => item.id === sak.id);\n+      assert.ok(original);\n+      sak.internNotat = original.internNotat;\n+    }\n+  }\n+});\n*** End Patch\n"
```

</details>

```
Modified 6 file(s): <workspace>/app/api/intern/saker/route.ts, <workspace>/app/api/intern/saker/[id]/route.ts, <workspace>/app/api/intern/saker/[id]/notater/route.ts, <workspace>/proxy.ts, <workspace>/docs/drift.md, <workspace>/tests/probes.test.ts
```


---

<sub>2m 53s</sub>

### `bash`

**Verify auth regression tests, TypeScript and production build**

$ npm test && npm run typecheck && npm run build

<details>
<summary>77 lines</summary>

```
npm notice run test
npm notice run node --test tests/*.test.ts
✔ isalive svarer uten token (1.029166ms)
✔ isready svarer uten token (0.117791ms)
✔ saker krever token (0.549416ms)
▶ interne ruter validerer Azure-token før tilgang til saker
  ✔ GET /api/intern/saker avviser manglende token (1.091542ms)
  ✔ GET /api/intern/saker avviser vilkårlig token (0.964333ms)
  ✔ GET /api/intern/saker avviser feil signatur (1.477583ms)
  ✔ GET /api/intern/saker avviser utløpt token (0.651958ms)
  ✔ GET /api/intern/saker avviser feil issuer (0.3745ms)
  ✔ GET /api/intern/saker avviser feil audience (0.344167ms)
  ✔ GET /api/intern/saker/101 avviser manglende token (0.167333ms)
  ✔ GET /api/intern/saker/101 avviser vilkårlig token (0.151791ms)
  ✔ GET /api/intern/saker/101 avviser feil signatur (0.44675ms)
  ✔ GET /api/intern/saker/101 avviser utløpt token (0.297167ms)
  ✔ GET /api/intern/saker/101 avviser feil issuer (0.249459ms)
  ✔ GET /api/intern/saker/101 avviser feil audience (0.945916ms)
  ✔ POST /api/intern/saker/101/notater avviser manglende token (0.151875ms)
  ✔ POST /api/intern/saker/101/notater avviser vilkårlig token (0.095792ms)
  ✔ POST /api/intern/saker/101/notater avviser feil signatur (0.5415ms)
  ✔ POST /api/intern/saker/101/notater avviser utløpt token (0.364ms)
  ✔ POST /api/intern/saker/101/notater avviser feil issuer (0.261792ms)
  ✔ POST /api/intern/saker/101/notater avviser feil audience (0.219041ms)
  ✔ notater avviser ugyldig token før parsing av body (0.128625ms)
  ✔ GET /api/intern/saker godtar gyldig token (0.364917ms)
  ✔ GET /api/intern/saker/101 godtar gyldig token (0.240917ms)
  ✔ POST /api/intern/saker/101/notater godtar gyldig token (0.228209ms)
✔ interne ruter validerer Azure-token før tilgang til saker (69.303167ms)
ℹ tests 26
ℹ suites 0
ℹ pass 26
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 175.197333
npm notice run typecheck
npm notice run tsc --noEmit
npm notice run build
npm notice run next build --webpack
▲ Next.js 16.3.8 (webpack)
✓ Running next.config took 4ms

  Creating an optimized production build ...
✓ Compiled successfully in 1701ms
  Running TypeScript ...

  We detected TypeScript in your project and reconfigured your tsconfig.json file for you.
  The following suggested values were added to your tsconfig.json. These values can be changed to fit your project's needs:

  	- allowJs was set to true
  	- incremental was set to true
  	- include was updated to add '.next/dev/types/**/*.ts'

  The following mandatory changes were made to your tsconfig.json:

  	- esModuleInterop was set to true (requirement for SWC / babel)
  	- resolveJsonModule was set to true (to match webpack resolution)
  	- isolatedModules was set to true (requirement for SWC / Babel)
  	- jsx was set to react-jsx (next.js uses the React automatic runtime)

  Finished TypeScript in 578ms ...
  Collecting page data using 9 workers ...
  Generating static pages using 9 workers (0/6) ...
  Generating static pages using 9 workers (1/6) 
  Generating static pages using 9 workers (2/6) 
  Generating static pages using 9 workers (4/6) 
✓ Generating static pages using 9 workers (6/6) in 227ms
  Finalizing page optimization ...
  Collecting build traces ...

Route (app)
┌ ○ /_not-found
├ ƒ /api/intern/saker
├ ƒ /api/intern/saker/[id]
├ ƒ /api/intern/saker/[id]/notater
├ ƒ /api/isalive
├ ƒ /api/isready
└ ƒ /api/saker


ƒ Proxy (Middleware)

○  (Static)   prerendered as static content
ƒ  (Dynamic)  server-rendered on demand
<shellId: 7 completed with exit code 0>
```

</details>


---

<sub>3m 5s</sub>

### Info

Session limits: 27.17/30 AI credits used. If this session continues, the next model call is final; tools will be disabled.

---

<sub>3m 6s</sub>

### `view`

**view the file at <workspace>/tsconfig.json.**

<workspace>/tsconfig.json

<details>
<summary>38 lines</summary>

```
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": [
      "dom",
      "es2022"
    ],
    "module": "esnext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "skipLibCheck": true,
    "types": [
      "node"
    ],
    "plugins": [
      {
        "name": "next"
      }
    ],
    "allowJs": true,
    "incremental": true,
    "esModuleInterop": true,
    "resolveJsonModule": true,
    "isolatedModules": true
  },
  "include": [
    "**/*.ts",
    "**/*.tsx",
    ".next/types/**/*.ts",
    ".next/dev/types/**/*.ts"
  ],
  "exclude": [
    "node_modules"
  ]
}
```

</details>


---

<sub>3m 6s</sub>

### `bash`

**Identify generated build files for cleanup**

$ ls -a && ls .github

<details>
<summary>16 lines</summary>

```
.
..
.github
.next
app
docs
lib
next-env.d.ts
node_modules
package-lock.json
package.json
proxy.ts
tests
tsconfig.json
instructions
<shellId: 8 completed with exit code 0>
```

</details>


---

<sub>3m 6s</sub>

### `bash`

**Start local production server for endpoint verification**

$ env -u AZURE_OPENID_CONFIG_ISSUER -u IDPORTEN_ISSUER node node_modules/next/dist/bin/next start --hostname 127.0.0.1 --port 32147

```
<command started in background with shellId: auth-smoke>
```


---

<sub>3m 18s</sub>

### Info

Session limits: 28.21/30 AI credits used. If this session continues, the next model call is final; tools will be disabled.

---

<sub>3m 18s</sub>

### `bash`

**Verify production HTTP rejection and unauthenticated probes**

$ node --input-type=module <<'NODE'
import assert from 'node:assert/strict';
const base = 'http://127.0.0.1:32147';
for (const path of ['/api/isalive', '/api/isready']) {
  const response = await fetch(base + path);
  assert.equal(response.status, 200, path);
}
for (const path of ['/api/intern/saker', '/api/intern/saker/101', '/api/intern/saker/101/notater']) {
  for (const headers of [{}, { Authorization: 'Bearer not-a-jwt' }]) {
    const response = await fetch(base + path, {
      headers,
      method: path.endsWith('/notater') ? 'POST' : 'GET',
      body: path.endsWith('/notater') ? '{"tekst":"Rejected note"}' : undefined,
    });
    assert.equal(response.status, 401, path);
    assert.deepEqual(await response.json(), { error: headers.Authorization ? 'Ugyldig token' : 'Mangler token' });
  }
}
console.log('Production HTTP checks passed: internal routes reject missing and invalid tokens; probes return 200.');
NODE

```
Production HTTP checks passed: internal routes reject missing and invalid tokens; probes return 200.
<shellId: 9 completed with exit code 0>
```


---

<sub>3m 20s</sub>

### Info

Session limits: 28.8/30 AI credits used. If this session continues, the next model call is final; tools will be disabled.

---

<sub>3m 20s</sub>

### `stop_bash`

<details>
<summary>Arguments</summary>

```json
{
  "shellId": "auth-smoke"
}
```

</details>

```
<command with id: auth-smoke stopped>
```


---

<sub>3m 29s</sub>

### Info

Session limits: 29.6/30 AI credits used. If this session continues, the next model call is final; tools will be disabled.

---

<sub>3m 29s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/tsconfig.json\n@@\n-    \"lib\": [\n-      \"dom\",\n-      \"es2022\"\n-    ],\n+    \"lib\": [\"dom\", \"es2022\"],\n@@\n-    \"jsx\": \"react-jsx\",\n+    \"jsx\": \"preserve\",\n@@\n-    \"types\": [\n-      \"node\"\n-    ],\n-    \"plugins\": [\n-      {\n-        \"name\": \"next\"\n-      }\n-    ],\n-    \"allowJs\": true,\n-    \"incremental\": true,\n-    \"esModuleInterop\": true,\n-    \"resolveJsonModule\": true,\n-    \"isolatedModules\": true\n+    \"types\": [\"node\"],\n+    \"plugins\": [{ \"name\": \"next\" }]\n   },\n-  \"include\": [\n-    \"**/*.ts\",\n-    \"**/*.tsx\",\n-    \".next/types/**/*.ts\",\n-    \".next/dev/types/**/*.ts\"\n-  ],\n-  \"exclude\": [\n-    \"node_modules\"\n-  ]\n+  \"include\": [\"**/*.ts\", \"**/*.tsx\", \".next/types/**/*.ts\"],\n+  \"exclude\": [\"node_modules\"]\n }\n*** Delete File: <workspace>/next-env.d.ts\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/tsconfig.json
Deleted 1 file(s): <workspace>/next-env.d.ts
```


---

<sub>3m 50s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>3m 50s</sub>

### Warning

Session limit reached (30.75/30 AI credits used). Future model calls will stop unless the session limit is increased or removed.

---

<sub>3m 52s</sub>

### `bash`

**Remove generated build artifacts and verify final workspace**

$ rm -rf <workspace>/.next && npm test && npm run typecheck

<details>
<summary>40 lines</summary>

```
npm notice run test
npm notice run node --test tests/*.test.ts
✔ isalive svarer uten token (0.988542ms)
✔ isready svarer uten token (0.100958ms)
✔ saker krever token (0.509291ms)
▶ interne ruter validerer Azure-token før tilgang til saker
  ✔ GET /api/intern/saker avviser manglende token (0.987209ms)
  ✔ GET /api/intern/saker avviser vilkårlig token (0.7915ms)
  ✔ GET /api/intern/saker avviser feil signatur (1.426584ms)
  ✔ GET /api/intern/saker avviser utløpt token (0.561666ms)
  ✔ GET /api/intern/saker avviser feil issuer (0.384666ms)
  ✔ GET /api/intern/saker avviser feil audience (0.349ms)
  ✔ GET /api/intern/saker/101 avviser manglende token (0.16475ms)
  ✔ GET /api/intern/saker/101 avviser vilkårlig token (0.148833ms)
  ✔ GET /api/intern/saker/101 avviser feil signatur (0.420125ms)
  ✔ GET /api/intern/saker/101 avviser utløpt token (0.278333ms)
  ✔ GET /api/intern/saker/101 avviser feil issuer (0.980958ms)
  ✔ GET /api/intern/saker/101 avviser feil audience (0.546208ms)
  ✔ POST /api/intern/saker/101/notater avviser manglende token (0.233375ms)
  ✔ POST /api/intern/saker/101/notater avviser vilkårlig token (0.137041ms)
  ✔ POST /api/intern/saker/101/notater avviser feil signatur (0.427708ms)
  ✔ POST /api/intern/saker/101/notater avviser utløpt token (0.362583ms)
  ✔ POST /api/intern/saker/101/notater avviser feil issuer (1.308292ms)
  ✔ POST /api/intern/saker/101/notater avviser feil audience (0.320583ms)
  ✔ notater avviser ugyldig token før parsing av body (0.167042ms)
  ✔ GET /api/intern/saker godtar gyldig token (0.275791ms)
  ✔ GET /api/intern/saker/101 godtar gyldig token (0.236375ms)
  ✔ POST /api/intern/saker/101/notater godtar gyldig token (0.351167ms)
✔ interne ruter validerer Azure-token før tilgang til saker (59.357292ms)
ℹ tests 26
ℹ suites 0
ℹ pass 26
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 162.321083
npm notice run typecheck
npm notice run tsc --noEmit
<shellId: 10 completed with exit code 0>
```

</details>


---

<sub>4m 9s</sub>

### Copilot

**Feilen er rettet i alle tre interne handlerne.** De brukte `getToken()`, som bare henter tokenet fra headeren. Et vilkårlig, ikke-tomt token ga derfor tilgang uten gyldig innlogging.

| # | Severity | File | Lines | Vulnerability | Confidence |
|---|----------|------|-------|---------------|------------|
| 1 | 🟠 HIGH | `app/api/intern/saker/route.ts` | 5–8 (før retting) | Manglende token-validering, også i enkeltsak og notat-handler. Rettet. | 10/10 |

Handlerne bruker nå eksisterende `requireSaksbehandler()` og returnerer 401 før lesing eller endring av data ved manglende eller ugyldig token. Proxy-matcher dekker også de interne rutene, men valideringen ligger i handlerne fordi en matcher-endring alene ikke stopper ugyldige tokens. Driftsdokumentasjonen er oppdatert.

🔴 Rød sone: autentisering, gå gjennom denne nøye.

**Sjekkliste for rød sone**
- [ ] Forstå valideringen før listing av interne saker.
- [ ] Forstå valideringen før oppslag av enkeltsak.
- [ ] Forstå valideringen før parsing og lagring av notater.

---

<sub>4m 9s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
