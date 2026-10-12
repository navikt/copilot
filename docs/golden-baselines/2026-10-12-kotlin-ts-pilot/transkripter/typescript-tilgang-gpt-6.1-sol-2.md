# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `c607863d-6006-4723-a5ad-5a99361337f3`  
> - **Started:** 10/12/2026, 3:35:03 PM  
> - **Duration:** 4m 12s  
> - **Exported:** 10/12/2026, 3:39:16 PM  

---

<sub>0s</sub>

### User

En penetrasjonstest viser at `/api/intern/saker` svarer uten gyldig innlogging.
Finn årsaken og rett feilen. Bruk kun dette repoet.

---

<sub>7s</sub>

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


### `bash`

**Read local instructions and locate authentication files**

$ pwd; git --no-pager status; find . -name AGENTS.md -o -name CLAUDE.md; find app lib tests docs -type f; cat package.json; cat proxy.ts

<details>
<summary>49 lines</summary>

```
<workspace>
fatal: not a git repository (or any of the parent directories): .git
app/api/saker/route.ts
app/api/intern/saker/route.ts
app/api/intern/saker/[id]/route.ts
app/api/isalive/route.ts
app/api/isready/route.ts
lib/saker.ts
lib/auth.ts
tests/probes.test.ts
docs/drift.md
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
<shellId: 0 completed with exit code 0>
```

</details>


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


### `bash` — Failed

**Read auth flow, route handlers, tests and applicable instructions**

$ for p in . .. ../.. ../../.. ../../../.. ../../../../..; do for f in AGENTS.md CLAUDE.md; do if [ -f "$p/$f" ]; then printf '\n--- %s ---\n' "$p/$f"; cat "$p/$f"; fi; done; done; for f in lib/auth.ts lib/saker.ts app/api/intern/saker/route.ts 'app/api/intern/saker/[id]/route.ts' app/api/saker/route.ts tests/probes.test.ts docs/drift.md tsconfig.json; do printf '\n--- %s ---\n' "$f"; nl -ba "$f"; done; test -d node_modules && printf '\nnode_modules exists\n'; ls -a

```
Permission denied because no interactive user response was available. Retry in an interactive session so the user can approve it, or try an alternative that does not require this permission.
```


### `bash`

**Read authentication flow and existing tests within workspace**

$ for f in lib/auth.ts lib/saker.ts app/api/intern/saker/route.ts 'app/api/intern/saker/[id]/route.ts' app/api/saker/route.ts tests/probes.test.ts docs/drift.md tsconfig.json; do printf '\n--- %s ---\n' "$f"; nl -ba "$f"; done; test -d node_modules && printf '\nnode_modules exists\n'; ls -a

<details>
<summary>129 lines</summary>

```
--- lib/auth.ts ---
     1	import { getToken, validateToken } from "@navikt/oasis";
     2	
     3	/**
     4	 * Validerer Azure-tokenet til saksbehandleren. Returnerer et 401-svar som route
     5	 * handleren skal sende videre, eller null når tokenet er gyldig.
     6	 */
     7	export async function requireSaksbehandler(request: Request): Promise<Response | null> {
     8	  const token = getToken(request);
     9	  if (!token) {
    10	    return Response.json({ error: "Mangler token" }, { status: 401 });
    11	  }
    12	  const validation = await validateToken(token);
    13	  if (!validation.ok) {
    14	    return Response.json({ error: "Ugyldig token" }, { status: 401 });
    15	  }
    16	  return null;
    17	}

--- lib/saker.ts ---
     1	export type Sak = { id: number; tittel: string; status: "apen" | "lukket" };
     2	export type InternSak = Sak & { internNotat: string };
     3	
     4	const saker: InternSak[] = [
     5	  { id: 101, tittel: "Søknad om dagpenger", status: "apen", internNotat: "Venter på arbeidsgiver" },
     6	  { id: 102, tittel: "Klage på vedtak", status: "lukket", internNotat: "Omgjort etter klage" },
     7	];
     8	
     9	export function listSaker(): Sak[] {
    10	  return saker.map(({ internNotat: _, ...sak }) => sak);
    11	}
    12	
    13	export function listInterneSaker(): InternSak[] {
    14	  return [...saker];
    15	}
    16	
    17	export function finnInternSak(id: number): InternSak | undefined {
    18	  return saker.find((sak) => sak.id === id);
    19	}

--- app/api/intern/saker/route.ts ---
     1	import { getToken } from "@navikt/oasis";
     2	import { listInterneSaker } from "../../../../lib/saker.ts";
     3	
     4	export async function GET(request: Request): Promise<Response> {
     5	  if (!getToken(request)) {
     6	    return Response.json({ error: "Mangler token" }, { status: 401 });
     7	  }
     8	  return Response.json(listInterneSaker());
     9	}

--- app/api/intern/saker/[id]/route.ts ---
     1	import { getToken } from "@navikt/oasis";
     2	import { finnInternSak } from "../../../../../lib/saker.ts";
     3	
     4	export async function GET(
     5	  request: Request,
     6	  { params }: { params: Promise<{ id: string }> },
     7	): Promise<Response> {
     8	  if (!getToken(request)) {
     9	    return Response.json({ error: "Mangler token" }, { status: 401 });
    10	  }
    11	  const sak = finnInternSak(Number((await params).id));
    12	  if (!sak) {
    13	    return Response.json({ error: "Fant ikke saken" }, { status: 404 });
    14	  }
    15	  return Response.json(sak);
    16	}

--- app/api/saker/route.ts ---
     1	import { requireSaksbehandler } from "../../../lib/auth.ts";
     2	import { listSaker } from "../../../lib/saker.ts";
     3	
     4	export async function GET(request: Request): Promise<Response> {
     5	  const unauthorized = await requireSaksbehandler(request);
     6	  if (unauthorized) return unauthorized;
     7	  return Response.json(listSaker());
     8	}

--- tests/probes.test.ts ---
     1	import assert from "node:assert/strict";
     2	import { test } from "node:test";
     3	import { GET as isalive } from "../app/api/isalive/route.ts";
     4	import { GET as saker } from "../app/api/saker/route.ts";
     5	
     6	test("isalive svarer uten token", async () => {
     7	  assert.equal((await isalive()).status, 200);
     8	});
     9	
    10	test("saker krever token", async () => {
    11	  assert.equal((await saker(new Request("http://localhost/api/saker"))).status, 401);
    12	});

--- docs/drift.md ---
     1	# Drift
     2	
     3	Appen kjører på NAIS med Azure AD-innlogging via Wonderwall. Wonderwall legger
     4	saksbehandlerens token i `Authorization`-headeren.
     5	
     6	- NAIS kaller `/api/isalive` og `/api/isready` uten token. De må alltid svare 200,
     7	  ellers starter podene på nytt i en løkke.
     8	- Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres
     9	  med `requireSaksbehandler()` i `lib/auth.ts`.
    10	- `AZURE_OPENID_CONFIG_JWKS_URI`, `AZURE_OPENID_CONFIG_ISSUER` og
    11	  `AZURE_APP_CLIENT_ID` settes av NAIS.

--- tsconfig.json ---
     1	{
     2	  "compilerOptions": {
     3	    "target": "ES2022",
     4	    "lib": ["dom", "es2022"],
     5	    "module": "esnext",
     6	    "moduleResolution": "bundler",
     7	    "jsx": "preserve",
     8	    "strict": true,
     9	    "noEmit": true,
    10	    "allowImportingTsExtensions": true,
    11	    "skipLibCheck": true,
    12	    "types": ["node"],
    13	    "plugins": [{ "name": "next" }]
    14	  },
    15	  "include": ["**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
    16	  "exclude": ["node_modules"]
    17	}
.
..
.github
app
docs
lib
package-lock.json
package.json
proxy.ts
tests
tsconfig.json
<shellId: 2 completed with exit code 0>
```

</details>


### `bash`

**Inspect local configuration, proxy and health routes**

$ find .github -type f; node --version; nl -ba proxy.ts; nl -ba app/api/isalive/route.ts; nl -ba app/api/isready/route.ts

<details>
<summary>39 lines</summary>

```
.github/instructions/deliberate-ai-use.instructions.md
.github/instructions/performance.instructions.md
.github/instructions/docker.instructions.md
.github/instructions/accessibility.instructions.md
.github/instructions/output-style.instructions.md
.github/instructions/database.instructions.md
.github/instructions/nextjs-aksel.instructions.md
.github/instructions/github-actions.instructions.md
.github/instructions/golang.instructions.md
.github/instructions/testing-typescript.instructions.md
.github/instructions/security-core.instructions.md
.github/instructions/kotlin.instructions.md
.github/instructions/security-owasp.instructions.md
.github/instructions/testing-kotlin.instructions.md
.github/instructions/testing.instructions.md
.github/instructions/norwegian-text.instructions.md
v24.21.0
     1	import { getToken } from "@navikt/oasis";
     2	import { NextResponse, type NextRequest } from "next/server";
     3	
     4	// Rask sjekk før route handlerne: forespørsler uten token avvises med en gang.
     5	// Selve valideringen av tokenet gjør hver route handler med requireSaksbehandler().
     6	export function proxy(request: NextRequest): NextResponse {
     7	  if (!getToken(request)) {
     8	    return NextResponse.json({ error: "Mangler token" }, { status: 401 });
     9	  }
    10	  return NextResponse.next();
    11	}
    12	
    13	export const config = {
    14	  matcher: ["/api/saker/:path*"],
    15	};
     1	export async function GET(): Promise<Response> {
     2	  return new Response("OK");
     3	}
     1	export async function GET(): Promise<Response> {
     2	  return new Response("OK");
     3	}
<shellId: 3 completed with exit code 0>
```

</details>


### `rg`

**getToken|validateToken|requireSaksbehandler|Authorization|oasis|instructions**

"getToken|validateToken|requireSaksbehandler|Authorization|oasis|instructions"

<details>
<summary>37 matches</summary>

```
[grep content: 27 matches across 14 file(s) under <workspace>]

lib/auth.ts (4 match(es)):
   import { getToken, validateToken } from "@navikt/oasis";
   export async function requireSaksbehandler(request: Request): Promise<Response | null> {
     const token = getToken(request);
     const validation = await validateToken(token);

proxy.ts (3 match(es)):
   import { getToken } from "@navikt/oasis";
   // Selve valideringen av tokenet gjør hver route handler med requireSaksbehandler().
     if (!getToken(request)) {

.github/instructions/norwegian-text.instructions.md (2 match(es)):
   Språknøytrale skriveregler (lengde, tetthet, tegnsetting) står i `output-style.instructions.md`. Ordlistene over KI-markører ligger sammen med klarspråk-prinsipper, anglisismer, fagtermer, teksttyper og full språkvask i `klarsprak`-skillen. Bruk den når du skriver eller redigerer norsk tekst av noe omfang. For dypere tekstredaksjon, bruk `@forfatter`-agenten.
   Bare første ord og egennavn med stor bokstav, ikke engelsk tittelstil. (Kolon på slutten av overskrifter er forbudt av `output-style.instructions.md`.)
.github/instructions/testing.instructions.md: Common testing principles for Nav. Language-specific examples can be found in separate instructions for [Kotlin](testing-kotlin.instructions.md) and [TypeScript](testing-typescript.instructions.md).
.github/instructions/security-owasp.instructions.md: The rules that always apply (logging, secrets, queries, ownership, `azp`, TLS) are in `security-core.instructions.md`, which is loaded in every session. If it is not installed next to this file, tell the developer to install it: https://github.com/navikt/copilot/blob/main/instructions/security-core.instructions.md
.github/instructions/kotlin.instructions.md:             header("Authorization", "******")

.github/instructions/output-style.instructions.md (2 match(es)):
   and compound-word minimums live in `norwegian-text.instructions.md`.
   `deliberate-ai-use.instructions.md` is also `applyTo: "**"` and requires explaining architectural choices, showing tradeoffs, marking red-zone code and inviting follow-up questions. That requirement is scoped, not global. Apply it as follows:
.github/instructions/deliberate-ai-use.instructions.md: The explanation requirement below is scoped as `output-style.instructions.md` describes: explain when the response makes an architectural choice or touches the red zone, otherwise answer without explanation. The requirement for architectural choices and red-zone code is unchanged.
package.json:     "@navikt/oasis": "4.2.2",

package-lock.json (3 match(es)):
           "@navikt/oasis": "4.2.2",
       "node_modules/@navikt/oasis": {
         "resolved": "https://npm.pkg.github.com/download/@navikt/oasis/4.2.2/3c43e1e89bb2d46646a9a2aaf27972a024377a6f",

docs/drift.md (2 match(es)):
   saksbehandlerens token i `Authorization`-headeren.
     med `requireSaksbehandler()` i `lib/auth.ts`.

app/api/intern/saker/[id]/route.ts (2 match(es)):
   import { getToken } from "@navikt/oasis";
     if (!getToken(request)) {

app/api/intern/saker/route.ts (2 match(es)):
   import { getToken } from "@navikt/oasis";
     if (!getToken(request)) {

app/api/saker/route.ts (2 match(es)):
   import { requireSaksbehandler } from "../../../lib/auth.ts";
     const unauthorized = await requireSaksbehandler(request);
```

</details>


### `bash`

**Read applicable repository security, TypeScript and output instructions**

$ for f in .github/instructions/*.md; do printf '\n--- %s ---\n' "$f"; sed -n '1,12p' "$f"; done; for f in .github/instructions/security-core.instructions.md .github/instructions/output-style.instructions.md .github/instructions/deliberate-ai-use.instructions.md .github/instructions/testing.instructions.md .github/instructions/testing-typescript.instructions.md .github/instructions/security-owasp.instructions.md .github/instructions/nextjs-aksel.instructions.md .github/instructions/norwegian-text.instructions.md; do printf '\n--- %s ---\n' "$f"; cat "$f"; done

<details>
<summary>12 lines</summary>

```
Output too large to read at once (40.8 KB). Saved to: ~/Library/Caches/cplt/tmp/9c29ae57f9cfeb48bfc136ad28aeee97/1791812142911-copilot-tool-output-5914-15233f7c-872b-4f41-a254-26e353775125.txt
Consider using tools like rg (for searching), head/tail (for viewing start/end), view with view_range (for specific sections), or jq (for JSON) to examine portions of the output.

Preview (first 500 chars):

--- .github/instructions/accessibility.instructions.md ---
---
description: "Universell utforming i React-komponenter: WCAG 2.1 AA, Aksel-komponenter, semantisk HTML, overskriftshierarki, bilder, ARIA og tastaturnavigasjon."
applyTo: "src/**/*.{tsx,jsx}"
---

# Accessibility (UU) Standards

Universell utforming er lovpålagt i Norge. All frontend-kode i Nav skal oppfylle WCAG 2.1 AA.

> For comprehensive WCAG tables, Aksel component patterns, and manual testing checklists, use `@accessibility-ag
<shellId: 4 completed with exit code 0>
```

</details>


### `bash`

**Read global local instructions and determine file-specific applicability**

$ wc -l .github/instructions/*.md; for f in .github/instructions/*.md; do printf '\n%s ' "$f"; grep '^applyTo:' "$f"; done; cat .github/instructions/security-core.instructions.md .github/instructions/output-style.instructions.md .github/instructions/deliberate-ai-use.instructions.md

<details>
<summary>122 lines</summary>

```
121 .github/instructions/accessibility.instructions.md
     323 .github/instructions/database.instructions.md
      30 .github/instructions/deliberate-ai-use.instructions.md
     271 .github/instructions/docker.instructions.md
     220 .github/instructions/github-actions.instructions.md
     547 .github/instructions/golang.instructions.md
     741 .github/instructions/kotlin.instructions.md
     598 .github/instructions/nextjs-aksel.instructions.md
      56 .github/instructions/norwegian-text.instructions.md
      80 .github/instructions/output-style.instructions.md
     449 .github/instructions/performance.instructions.md
      16 .github/instructions/security-core.instructions.md
      12 .github/instructions/security-owasp.instructions.md
     185 .github/instructions/testing-kotlin.instructions.md
     100 .github/instructions/testing-typescript.instructions.md
     112 .github/instructions/testing.instructions.md
    3861 total

.github/instructions/accessibility.instructions.md applyTo: "src/**/*.{tsx,jsx}"

.github/instructions/database.instructions.md applyTo: "**/db/migration/**/*.sql"

.github/instructions/deliberate-ai-use.instructions.md applyTo: "**"

.github/instructions/docker.instructions.md applyTo: "**/Dockerfile"

.github/instructions/github-actions.instructions.md applyTo: ".github/workflows/*.{yml,yaml}"

.github/instructions/golang.instructions.md applyTo: "**/*.go"

.github/instructions/kotlin.instructions.md applyTo: "**/*.kt"

.github/instructions/nextjs-aksel.instructions.md applyTo: "src/**/*.{tsx,ts}"

.github/instructions/norwegian-text.instructions.md applyTo: "**/*.md"

.github/instructions/output-style.instructions.md applyTo: "**"

.github/instructions/performance.instructions.md applyTo: "src/**/*.{tsx,ts}"

.github/instructions/security-core.instructions.md applyTo: "**"

.github/instructions/security-owasp.instructions.md applyTo: "**/*.{kt,go,java,ts,tsx}"

.github/instructions/testing-kotlin.instructions.md applyTo: "**/*.test.{kt,kts}"

.github/instructions/testing-typescript.instructions.md applyTo: "**/*.test.{ts,tsx}"

.github/instructions/testing.instructions.md applyTo: "**/*.test.{ts,tsx,kt,kts}"
---
applyTo: "**"
---

# Security Core

These rules apply to every change, in every language. They are never optional.

- **No secrets or personal data in logs.** Never log tokens, passwords, keys, fødselsnummer (FNR), name or address.
- **Secrets come from the environment.** Read tokens, passwords and keys from environment variables or the secret store. Never hardcode them.
- **Parameterized queries only.** Never concatenate user input into SQL, shell or any other command. Validate input at trust boundaries.
- **Verify resource ownership.** Check that the caller owns the resource, not only that the caller is authenticated.
- **Validate `azp` for machine-to-machine calls** against `AZURE_APP_PRE_AUTHORIZED_APPS`.
- **TLS 1.2 or newer.** Never set `InsecureSkipVerify: true` or disable certificate validation.

Code-level OWASP Top 10:2025 patterns: the `$security-owasp` skill.
---
applyTo: "**"
---

# Output Style

How output is written: chat answers, commit messages, PR descriptions, documentation and code comments. Never applies inside code blocks, program output or quoted error strings.

## Length

- No unrequested files. Do not add a README, summary document or test scaffold that was not asked for.
- No speculative scaffolding. Build what the task needs now, not what it might need later.
- Size the answer to the question. A one-line question gets a one-line answer.
- Lead with the answer, reasoning after. Never restate the question before answering it.

## Density

Drop these:

- Filler: "bare", "egentlig", "faktisk", "selvfølgelig", "simpelthen", "just", "really", "basically", "actually"
- Politeness: "gjerne", "med glede", "sure", "certainly", "happy to", "great question"
- Hedging: "kanskje", "muligens", "det kan hende", "maybe", "perhaps", "might"

Sentence fragments are fine. Prefer short synonyms: "fix", not "implement a solution for".

- One idea per sentence. If a sentence has to be read twice, split it.
- Short paragraphs, two to four sentences.
- Plain word over fancy word: "bruke" not "benytte", "use" not "utilise", "hjelpe" not "fasilitere", "start" not "commence".
- Cut adverbs. Use a stronger verb or the measured number instead: "runs quickly" becomes "is fast" or "runs in 40 ms"; "forbedrer betydelig" becomes the difference you measured.
- Active voice, and name the actor: "the compiler validates queries", not "queries are validated". Passive only when the actor is unknown or does not matter.
- Cut a sentence that only restates the one before it, and a closing paragraph that summarises what the reader just read. See the restatement and summary entries under **Structural tells**.

Keep exact: technical terms, code blocks, command output, error strings, file paths. Never paraphrase or compress these.

**Auto-clarity carve-out.** Terseness is suspended and full sentences are required for:

- Security warnings
- Irreversible actions (data deletion, force push, production deploy, dropped schema)
- Multi-step sequences where compression creates ambiguity about order or ownership
- Any answer where the user asked for an explanation or repeated the question

Resume the compact style once the warning or sequence is done.

## Anti-slop

The AI-marker lists live in the `klarsprak` skill: puffery, opening and closing
formulas, rhetorical patterns and structural tells, with the rewrite for each.
**Load `klarsprak` before writing or editing prose a human will read**:
documentation, README, ADR, commit message, PR description, issue, review comment,
and any chat answer longer than a couple of paragraphs. The lists are written in
Norwegian, but the markers are the same in English. Scrub both. Norwegian spelling
and compound-word minimums live in `norwegian-text.instructions.md`.

Recognise these four without the skill. Any one of them means the text needs the
full wash:

- A word that praises instead of describing: robust, seamless, banebrytende, helhetlig.
- An opening that delays the point: "it is worth noting", "i dagens verden".
- A closing that repeats the text above it: "in conclusion", "oppsummert kan man si at".
- Symmetry that was not in the facts: "not only X, but also Y", three nouns in series.

**Punctuation:**

- No em dashes (—) in prose. Use a colon, comma, parentheses, or a second sentence.
- Headings never end with a colon.
- A colon in every bullet is a tell. Vary the structure.
- No exclamation marks in technical text.
- Semicolons sparingly.

## Precedence over deliberate-ai-use

`deliberate-ai-use.instructions.md` is also `applyTo: "**"` and requires explaining architectural choices, showing tradeoffs, marking red-zone code and inviting follow-up questions. That requirement is scoped, not global. Apply it as follows:

1. Explain only when the response makes an **architectural choice** (structure, data model, API contract, auth design, dependency or migration strategy) or touches **red-zone code** (auth, authorization, input validation, core business logic, security). Otherwise answer without explanation.
2. When it applies, the explanation is two to three sentences: the choice, the main tradeoff, the alternative rejected. Not a section, not a table, unless the user asks for one.
3. Mark red-zone code with one line, for example `🔴 Rød sone: token-validering, gå gjennom denne nøye.`
4. Add the follow-up invitation ("Still gjerne spørsmål om valgene over") only in responses that already carry an architectural explanation. Never on trivial answers.
5. When the user asks "hvorfor" or "forklar", the Length rules above are lifted for that answer.

Phase gates in `nav-pilot` override this section. Never sacrifice phase integrity for brevity.
---
applyTo: "**"
---

# Deliberate AI Use

Nav's framework for using AI in a way that builds developer skill instead of eroding it. The rules below apply in every session. The full framework (research, what belongs in each zone, the three-attempt rule, experience level, generate-then-understand, follow-up questions) is in the `$deliberate-ai-use` skill: load it when the developer asks about AI use, learning or which zone a task belongs to. The zone definitions below are enough for declaring a zone in a plan.

**Red zone** (the developer should understand it deeply, and code it by hand first): debugging, new concepts, core business logic, security-critical code (authentication, authorization, input validation) and architecture decisions. **Green zone** (suited to AI): boilerplate, technology the developer already knows, configuration, refactoring with a known goal, test data.

## Boundaries

The explanation requirement below is scoped as `output-style.instructions.md` describes: explain when the response makes an architectural choice or touches the red zone, otherwise answer without explanation. The requirement for architectural choices and red-zone code is unchanged.

### ✅ Always

- Explain *why*, not only *what*, when you generate code, and show the tradeoffs: what is gained and what is given up
- Mark core logic and security code as «rød sone — forstå dette grundig»
- Encourage the developer to ask follow-up questions: end with «Still gjerne spørsmål om valgene over»

### ⚠️ Ask First

- Whether the developer wants full delegation or guided learning. For red-zone items the default is test skeletons and stubs; implement them only when the developer explicitly asks, and then end with a review checklist per item
- With unfamiliar technology: whether they want to learn the concepts first

### 🚫 Never

- Generate code without explaining the architectural choices
- Encourage blind copy-paste of generated code
- Skip error handling or security patterns in examples
<shellId: 5 completed with exit code 0>
```

</details>


### `bash`

**Read remaining applicable testing and security guidance**

$ cat .github/instructions/security-owasp.instructions.md .github/instructions/testing.instructions.md .github/instructions/testing-typescript.instructions.md .github/instructions/norwegian-text.instructions.md

<details>
<summary>205 lines</summary>

````
---
description: "Hvor OWASP Top 10:2025-mønstrene for Kotlin, Go, Java og TypeScript ligger: tilgangskontroll, feilkonfigurasjon, forsyningskjede, kryptografi, injeksjon, logging og feilhåndtering."
applyTo: "**/*.{kt,go,java,ts,tsx}"
---

# Security Essentials

The rules that always apply (logging, secrets, queries, ownership, `azp`, TLS) are in `security-core.instructions.md`, which is loaded in every session. If it is not installed next to this file, tell the developer to install it: https://github.com/navikt/copilot/blob/main/instructions/security-core.instructions.md

For detailed OWASP Top 10:2025 code-level patterns (Kotlin, Go, Java, Node.js), invoke the `$security-owasp` skill.
For scanning workflows (trivy, zizmor, govulncheck), use the `$security-review` skill.
For architecture-level threat modeling, use `@security-champion`.
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
<shellId: 6 completed with exit code 0>
````

</details>


### `bash`

**Verify route auth bypass in memory using explicit Oasis test double**

$ node --input-type=module -e '
import { registerHooks } from "node:module";
import assert from "node:assert/strict";
let validations = 0;
globalThis.__reviewValidate = async () => { validations++; return { ok: false }; };
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@navikt/oasis") return { url: "review:oasis", shortCircuit: true };
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url === "review:oasis") return { format: "module", shortCircuit: true, source: "export const getToken = request => request.headers.get(\"Authorization\")?.replace(/^Bearer /, \"\") ?? null; export const validateToken = token => globalThis.__reviewValidate(token);" };
    return nextLoad(url, context);
  }
});
const { GET: list } = await import("./app/api/intern/saker/route.ts");
const { GET: detail } = await import("./app/api/intern/saker/[id]/route.ts");
const { GET: ordinary } = await import("./app/api/saker/route.ts");
for (const [path, handler] of [["/api/intern/saker", list], ["/api/intern/saker/101", req => detail(req, { params: Promise.resolve({ id: "101" }) })]]) {
  const missing = await handler(new Request(`http://localhost${path}`));
  assert.equal(missing.status, 401);
  const invalid = await handler(new Request(`http://localhost${path}`, { headers: { Authorization: "Bearer definitely-not-a-valid-jwt" } }));
  assert.equal(invalid.status, 200);
  const body = await invalid.json();
  assert.ok((Array.isArray(body) ? body[0] : body).internNotat);
  console.log(`${path}: missing=401, nonempty invalid token=200, internNotat returned`);
}
assert.equal(validations, 0);
const denied = await ordinary(new Request("http://localhost/api/saker", { headers: { Authorization: "Bearer definitely-not-a-valid-jwt" } }));
assert.equal(denied.status, 401);
assert.equal(validations, 1);
console.log("Internal routes invoke validateToken 0 times; ordinary route invokes it and returns 401.");
console.log("Oasis was replaced with a test double; no real package validation or HTTP/Next.js integration was executed.");
'

```
/api/intern/saker: missing=401, nonempty invalid token=200, internNotat returned
/api/intern/saker/101: missing=401, nonempty invalid token=200, internNotat returned
Internal routes invoke validateToken 0 times; ordinary route invokes it and returns 401.
Oasis was replaced with a test double; no real package validation or HTTP/Next.js integration was executed.
<shellId: 7 completed with exit code 0>
```


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


### Copilot

## Security Findings

### Alert 1
**File:** `<workspace>/app/api/intern/saker/route.ts:4-8`  
**Category:** AuthenticationFailure  
**Severity: HIGH | Confidence: 9/10**

**Problem:** Ruten kontrollerer bare at `getToken(request)` returnerer en verdi. Den validerer ikke tokenet før den returnerer interne saker, inkludert `internNotat`. En innsender som når applikasjonen, trenger derfor ingen gyldig innlogging dersom token-uttrekkingen returnerer den innsendte token-strengen.

Samme feil finnes i detaljruten:

`<workspace>/app/api/intern/saker/[id]/route.ts:8-15`

**Evidence:** Kildekoden viser at begge rutene importerer `getToken`, men aldri kaller token-validering. Jeg kjørte de uendrede handlerne i minnet med en eksplisitt Oasis-testdobbel som trekker ut Bearer-strengen og avviser tokenet ved validering:

| Forespørsel | Liste- og detaljrute |
|---|---|
| Uten token | `401` |
| `Bearer definitely-not-a-valid-jwt` | `200`, med `internNotat` |
| Antall kall til `validateToken` | `0` |

Den ordinære sakeruten returnerte `401` med samme testdobbel og kalte valideringen.

Dette verifiserer den manglende valideringen i handlerne. Det er **ikke** en ende-til-ende-test av Next.js, Wonderwall eller den faktiske Oasis-pakken: avhengighetene er ikke installert lokalt. Uten token avviser handlerne forespørselen; den konkrete feilen gjelder **tilstedeværende, men ugyldig token**.

**Relevante kontrollpunkter:**

- `<workspace>/lib/auth.ts:7-16` har allerede `requireSaksbehandler()`, som både krever token og avviser når `validateToken(token).ok` er falsk.
- `<workspace>/proxy.ts:6-14` matcher bare `/api/saker/:path*`. Proxyen beskytter ikke internrutene og kontrollerer uansett bare token-tilstedeværelse.

**Suggested fix:** Erstatt `getToken`-sjekken i **begge internrutene** med `await requireSaksbehandler(request)`, og returner avvisningssvaret før dataoppslag, slik den ordinære sakeruten allerede gjør. Det gjenbruker eksisterende validering uten å innføre en ny autentiseringsmekanisme. En endring i proxyens matcher alene retter ikke rotårsaken, fordi proxyen heller ikke validerer tokenet.

**Anbefalt testdekning:**

- Begge internruter gir `401` uten token og med ikke-tomt, ugyldig token.
- Ingen saksdata returneres ved avvisning. Detaljruten gir `401`, også for ukjent ID uten gyldig token.
- Gyldig token gir `200`; gyldig token med ukjent ID gir `404`.
- Integrasjonstester med lokal JWKS og reell validering avviser feil signatur, utløpt token, feil issuer og feil audience.
- `/api/isalive` og `/api/isready` beholder `200` uten token.

Lokale instruksjoner er lest. Ingen filer er endret, og ingen nettverksoppslag er gjort.


---

<sub>1m 32s</sub>

### Info

Session limits: 15.09/30 AI credits used.

---

<sub>1m 32s</sub>

### `glob`

****/***

"**/*"

<details>
<summary>29 matches</summary>

```
./tsconfig.json
./lib/auth.ts
./lib/saker.ts
./proxy.ts
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
./package.json
./package-lock.json
./docs/drift.md
./tests/probes.test.ts
./app/api/isready/route.ts
./app/api/isalive/route.ts
./app/api/intern/saker/[id]/route.ts
./app/api/intern/saker/route.ts
./app/api/saker/route.ts
```

</details>


---

<sub>1m 32s</sub>

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

<sub>1m 32s</sub>

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

<sub>1m 42s</sub>

### Copilot

Internrutene sjekker at et token finnes, men ikke at det er gyldig. Jeg retter begge med repoets eksisterende autentiseringsfunksjon og legger til regresjonstester.


---

<sub>1m 43s</sub>

### `view`

**view the file at ./package.json.**

./package.json

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

<sub>1m 43s</sub>

### `view`

**view the file at ./tsconfig.json.**

./tsconfig.json

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

<sub>1m 43s</sub>

### `view`

**view the file at ./lib/auth.ts.**

./lib/auth.ts

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

<sub>1m 43s</sub>

### `view`

**view the file at ./lib/saker.ts.**

./lib/saker.ts

<details>
<summary>15 lines</summary>

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
```

</details>


---

<sub>1m 43s</sub>

### `view`

**view the file at ./app/api/intern/saker/route.ts.**

./app/api/intern/saker/route.ts

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

<sub>1m 43s</sub>

### `view`

**view the file at ./app/api/intern/saker/[id]/route.ts.**

./app/api/intern/saker/[id]/route.ts

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

<sub>1m 43s</sub>

### `view`

**view the file at ./app/api/saker/route.ts.**

./app/api/saker/route.ts

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


---

<sub>1m 43s</sub>

### `view`

**view the file at ./proxy.ts.**

./proxy.ts

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

<sub>1m 43s</sub>

### `view`

**view the file at ./tests/probes.test.ts.**

./tests/probes.test.ts

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

<sub>1m 43s</sub>

### `view`

**view the file at ./docs/drift.md.**

./docs/drift.md

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

<sub>1m 43s</sub>

### `view`

**view the file at ./.github/instructions/security-owasp.instructions.md.**

./.github/instructions/security-owasp.instructions.md

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

<sub>1m 43s</sub>

### `view`

**view the file at ./.github/instructions/testing.instructions.md.**

./.github/instructions/testing.instructions.md

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

<sub>1m 43s</sub>

### `view`

**view the file at ./.github/instructions/testing-typescript.instructions.md.**

./.github/instructions/testing-typescript.instructions.md

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

<sub>1m 43s</sub>

### `view`

**view the file at ./.github/instructions/norwegian-text.instructions.md.**

./.github/instructions/norwegian-text.instructions.md

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

<sub>1m 49s</sub>

### `bash`

**Kjør eksisterende tester og typekontroll**

$ npm test && npm run typecheck

<details>
<summary>33 lines</summary>

```
npm notice run test
npm notice run node --test tests/*.test.ts
node:internal/modules/package_json_reader:331
  throw new ERR_MODULE_NOT_FOUND(packageName, fileURLToPath(base), null);
        ^

Error [ERR_MODULE_NOT_FOUND]: Cannot find package '@navikt/oasis' imported from <workspace>/lib/auth.ts
    at Object.getPackageJSONURL (node:internal/modules/package_json_reader:331:9)
    at packageResolve (node:internal/modules/esm/resolve:784:25)
    at moduleResolve (node:internal/modules/esm/resolve:873:18)
    at defaultResolve (node:internal/modules/esm/resolve:1006:11)
    at #cachedDefaultResolve (node:internal/modules/esm/loader:705:20)
    at #resolveAndMaybeBlockOnLoaderThread (node:internal/modules/esm/loader:725:38)
    at ModuleLoader.resolveSync (node:internal/modules/esm/loader:763:56)
    at #resolve (node:internal/modules/esm/loader:687:17)
    at ModuleLoader.getOrCreateModuleJob (node:internal/modules/esm/loader:607:35)
    at ModuleJob.syncLink (node:internal/modules/esm/module_job:276:33) {
  code: 'ERR_MODULE_NOT_FOUND'
}

Node.js v24.21.0
✖ tests/probes.test.ts (66.939458ms)
ℹ tests 1
ℹ suites 0
ℹ pass 0
ℹ fail 1
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 69.885166

✖ failing tests:

test at tests/probes.test.ts:1:1
✖ tests/probes.test.ts (66.939458ms)
  'test failed'
<shellId: 8 completed with exit code 1>
```

</details>


---

<sub>1m 49s</sub>

### `view`

**view the file at ./package-lock.json.**

./package-lock.json (lines 1-100)

<details>
<summary>100 lines</summary>

```
{
  "name": "saksoversikt",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "name": "saksoversikt",
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
      },
      "engines": {
        "node": "24"
      }
    },
    "node_modules/@emnapi/runtime": {
      "version": "1.11.3",
      "resolved": "https://registry.npmjs.org/@emnapi/runtime/-/runtime-1.11.3.tgz",
      "integrity": "sha512-Xz4Tpyki7XyrpbUK1jR1AhdAdaXyhhY4lZ3neLodmhpuWfy2PAQN5B46sAiU4liOXGLkHypn/qU+jvfWSCYYLA==",
      "license": "MIT",
      "optional": true,
      "dependencies": {
        "tslib": "^2.4.0"
      }
    },
    "node_modules/@img/colour": {
      "version": "1.1.0",
      "resolved": "https://registry.npmjs.org/@img/colour/-/colour-1.1.0.tgz",
      "integrity": "sha512-Td76q7j57o/tLVdgS746cYARfSyxk8iEfRxewL9h4OMzYhbW4TAcppl0mT4eyqXddh6L/jwoM75mo7ixa/pCeQ==",
      "license": "MIT",
      "optional": true,
      "engines": {
        "node": ">=18"
      }
    },
    "node_modules/@img/sharp-darwin-arm64": {
      "version": "0.35.5",
      "resolved": "https://registry.npmjs.org/@img/sharp-darwin-arm64/-/sharp-darwin-arm64-0.35.5.tgz",
      "integrity": "sha512-QRUlFQ0WxvdWyqqG/WtI3iupfD5rBzmCHXSdPsY91sAtVtTo7Q4cb6zOccZ3gqEqkr0f1As1ehLqmEpDsRf+lg==",
      "cpu": [
        "arm64"
      ],
      "license": "Apache-2.0",
      "optional": true,
      "os": [
        "darwin"
      ],
      "engines": {
        "node": ">=20.9.0"
      },
      "funding": {
        "url": "https://opencollective.com/libvips"
      },
      "optionalDependencies": {
        "@img/sharp-libvips-darwin-arm64": "1.3.4"
      }
    },
    "node_modules/@img/sharp-darwin-x64": {
      "version": "0.35.5",
      "resolved": "https://registry.npmjs.org/@img/sharp-darwin-x64/-/sharp-darwin-x64-0.35.5.tgz",
      "integrity": "sha512-+BR255RhDlpygUpOc/Jdt1nT6DQ3XG/ERo5wbcdOf5Q320dKtPCKPLR1LJs9VGXRaMa8l1uUa0tkCNOXiAxZUw==",
      "cpu": [
        "x64"
      ],
      "license": "Apache-2.0",
      "optional": true,
      "os": [
        "darwin"
      ],
      "engines": {
        "node": ">=20.9.0"
      },
      "funding": {
        "url": "https://opencollective.com/libvips"
      },
      "optionalDependencies": {
        "@img/sharp-libvips-darwin-x64": "1.3.4"
      }
    },
    "node_modules/@img/sharp-freebsd-wasm32": {
      "version": "0.35.5",
      "resolved": "https://registry.npmjs.org/@img/sharp-freebsd-wasm32/-/sharp-freebsd-wasm32-0.35.5.tgz",
      "integrity": "sha512-Y/z91nEZ4uIBX5X3nfTovjU9lHNKFYbL2lpHCLVNmXQK03VIZvXBBt0KxbPGp2SdGSF+2mQU4e+hQaWOt86iAw==",
      "license": "Apache-2.0",
      "optional": true,
      "os": [
        "freebsd"
      ],
      "dependencies": {
        "@img/sharp-wasm32": "0.35.5"
      },
      "engines": {
```

</details>


---

<sub>1m 49s</sub>

### `view`

**view the file at ./app/api/isready/route.ts.**

./app/api/isready/route.ts

```
export async function GET(): Promise<Response> {
  return new Response("OK");
}

```


---

<sub>1m 49s</sub>

### `view`

**view the file at ./app/api/isalive/route.ts.**

./app/api/isalive/route.ts

```
export async function GET(): Promise<Response> {
  return new Response("OK");
}

```


---

<sub>1m 57s</sub>

### `bash`

**Installer låste avhengigheter fra lokal npm-cache**

$ npm ci --offline --ignore-scripts --no-audit --no-fund

```
npm error code EALLOWREMOTE
npm error Fetching packages of type "remote" have been disabled
npm error Refusing to fetch "@navikt/texas@https://npm.pkg.github.com/download/@navikt/texas/4.2.2/862d743a1df21630a007a0d2d2a36a74dd8b5187"
npm error A complete log of this run can be found in: ~/.npm/_logs/2026-10-12T13_36_57_793Z-debug-0.log
<shellId: 9 completed with exit code 1>
```


---

<sub>2m 2s</sub>

### `glob`

**node_modules/@navikt/**/***

"node_modules/@navikt/**/*"

```
No files matched the pattern.
```


---

<sub>2m 2s</sub>

### `glob`

**node_modules/{jose,typescript}/**/***

"node_modules/{jose,typescript}/**/*"

```
No files matched the pattern.
```


---

<sub>2m 2s</sub>

### `rg`

**node_modules/@navikt|node_modules/jose|node_modules/typescript**

"node_modules/@navikt|node_modules/jose|node_modules/typescript" in package-lock.json

<details>
<summary>90 matches</summary>

```
./package-lock.json:    "node_modules/@navikt/oasis": {
./package-lock.json-      "version": "4.2.2",
./package-lock.json-      "resolved": "https://npm.pkg.github.com/download/@navikt/oasis/4.2.2/3c43e1e89bb2d46646a9a2aaf27972a024377a6f",
./package-lock.json-      "integrity": "sha512-EVMsF8uKbLPNRWuPfH1IKVJhqtibFzSTSyNkzj5mTCVMZV2QPS5W3uYMD4r0rXsmLx7nv0HnLovDphb3AFEjvw==",
./package-lock.json-      "license": "MIT",
./package-lock.json-      "dependencies": {
./package-lock.json-        "@navikt/texas": "^4.2.2",
./package-lock.json-        "@opentelemetry/api": "^1.9.1",
./package-lock.json-        "@prometheus-io/client": "^0.16.1",
./package-lock.json-        "jose": "^6.2.10"
./package-lock.json-      },
./package-lock.json-      "engines": {
./package-lock.json-        "node": "^20.19.0 || >= 22.12.0 || 24.x || 25.x"
./package-lock.json-      }
./package-lock.json-    },
./package-lock.json:    "node_modules/@navikt/texas": {
./package-lock.json-      "version": "4.2.2",
./package-lock.json-      "resolved": "https://npm.pkg.github.com/download/@navikt/texas/4.2.2/862d743a1df21630a007a0d2d2a36a74dd8b5187",
./package-lock.json-      "integrity": "sha512-w/iTWLSdmvn2j2U3M+sJX9bSBvoV5UcMk+FuKs9EKz1QuaCoUksZrEfqX9/FtqyTUaOhwDnm/1iIuR6BudzFAg==",
./package-lock.json-      "license": "MIT",
./package-lock.json-      "engines": {
./package-lock.json-        "node": "^20.19.0 || >= 22.12.0 || 24.x || 25.x"
./package-lock.json-      }
./package-lock.json-    },
./package-lock.json-    "node_modules/@next/env": {
./package-lock.json-      "version": "16.3.8",
./package-lock.json-      "resolved": "https://registry.npmjs.org/@next/env/-/env-16.3.8.tgz",
./package-lock.json-      "integrity": "sha512-Al9zqHVV7TJv0eFuOU4U7Lvv74PTih4Ch63sk2xCIpSTkE3udFnaOcnzP2lQVymiL7yS9Cj2iClUXlR3EQ5sEw==",
./package-lock.json-      "license": "MIT"
./package-lock.json-    },
./package-lock.json-    "node_modules/@next/swc-darwin-arm64": {
./package-lock.json-      "version": "16.3.8",
./package-lock.json-      "resolved": "https://registry.npmjs.org/@next/swc-darwin-arm64/-/swc-darwin-arm64-16.3.8.tgz",
./package-lock.json-      "integrity": "sha512-2JPRMh2nmQG5CiL7cXGL9AGwnPWJQ//cTtAUCT+w511QHk79SYz3LGv/pc5X643B/WEO0rvu3Yww0hqwt3kgeA==",
./package-lock.json-      "cpu": [
./package-lock.json-        "arm64"
./package-lock.json-      ],
./package-lock.json-      "license": "MIT",
./package-lock.json-      "optional": true,
./package-lock.json-      "os": [
--
./package-lock.json:    "node_modules/jose": {
./package-lock.json-      "version": "6.2.12",
./package-lock.json-      "resolved": "https://registry.npmjs.org/jose/-/jose-6.2.12.tgz",
./package-lock.json-      "integrity": "sha512-9NiFmJEex0sy2Dk58j2UGBSHgUs2ypF9eZSu4L6vjOX3Dp96Sw1F3uL+H+D1sx02jZZdzUT0HgvCy59CuvXcWw==",
./package-lock.json-      "license": "MIT",
./package-lock.json-      "funding": {
./package-lock.json-        "url": "https://github.com/sponsors/panva"
./package-lock.json-      }
./package-lock.json-    },
./package-lock.json-    "node_modules/nanoid": {
./package-lock.json-      "version": "3.3.20",
./package-lock.json-      "resolved": "https://registry.npmjs.org/nanoid/-/nanoid-3.3.20.tgz",
./package-lock.json-      "integrity": "sha512-uKdg2G3GNCKQn9byYOpxbGqrT2fGO5KRt5J/8b3pok8rT6qxGWF6hxMyJiEYtAf+FVyYuD9hRaDqX5uPFYJ4ZQ==",
./package-lock.json-      "funding": [
./package-lock.json-        {
./package-lock.json-          "type": "github",
./package-lock.json-          "url": "https://github.com/sponsors/ai"
./package-lock.json-        }
./package-lock.json-      ],
./package-lock.json-      "license": "MIT",
./package-lock.json-      "bin": {
./package-lock.json-        "nanoid": "bin/nanoid.cjs"
./package-lock.json-      },
./package-lock.json-      "engines": {
./package-lock.json-        "node": "^10 || ^12 || ^13.7 || ^14 || >=15.0.1"
--
./package-lock.json:    "node_modules/typescript": {
./package-lock.json-      "version": "6.0.3",
./package-lock.json-      "resolved": "https://registry.npmjs.org/typescript/-/typescript-6.0.3.tgz",
./package-lock.json-      "integrity": "sha512-y2TvuxSZPDyQakkFRPZHKFm+KKVqIisdg9/CZwm9ftvKXLP8NRWj38/ODjNbr43SsoXqNuAisEf1GdCxqWcdBw==",
./package-lock.json-      "dev": true,
./package-lock.json-      "license": "Apache-2.0",
./package-lock.json-      "bin": {
./package-lock.json-        "tsc": "bin/tsc",
./package-lock.json-        "tsserver": "bin/tsserver"
./package-lock.json-      },
./package-lock.json-      "engines": {
./package-lock.json-        "node": ">=14.17"
./package-lock.json-      }
./package-lock.json-    },
./package-lock.json-    "node_modules/undici-types": {
./package-lock.json-      "version": "8.9.0",
./package-lock.json-      "resolved": "https://registry.npmjs.org/undici-types/-/undici-types-8.9.0.tgz",
./package-lock.json-      "integrity": "sha512-KTDyRTYX8sWmKXAikPHHSyc63CRPETMctyjKFupcC6OBLXT3xsN0e9aF7m+mIXutFWpUXuedtowG7iLOzp0kQg==",
./package-lock.json-      "dev": true,
./package-lock.json-      "license": "MIT"
./package-lock.json-    }
./package-lock.json-  }
./package-lock.json-}
```

</details>


---

<sub>2m 57s</sub>

### Info

Session limits: 24.88/30 AI credits used.

---

<sub>2m 57s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: app/api/intern/saker/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: app/api/intern/saker/[id]/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: docs/drift.md\n@@\n - Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres\n   med `requireSaksbehandler()` i `lib/auth.ts`.\n+- Både `/api/intern/saker` og `/api/intern/saker/[id]` validerer tokenet før\n+  saksdata hentes. Manglende eller ugyldig token gir 401, også for ukjent saks-ID.\n+  Proxyen sjekker bare om tokenet finnes på `/api/saker`. Den erstatter ikke\n+  valideringen i rutene.\n*** Add File: tests/intern-saker.test.ts\n+import assert from \"node:assert/strict\";\n+import { after, before, test } from \"node:test\";\n+import { createServer } from \"node:http\";\n+import { once } from \"node:events\";\n+import { exportJWK, generateKeyPair, SignJWT } from \"jose\";\n+import { GET as isalive } from \"../app/api/isalive/route.ts\";\n+import { GET as isready } from \"../app/api/isready/route.ts\";\n+\n+const issuer = \"https://issuer.example.test\";\n+const audience = \"saksoversikt-test\";\n+const keyId = \"test-key\";\n+const keys = await generateKeyPair(\"RS256\");\n+const otherKeys = await generateKeyPair(\"RS256\");\n+const jwk = { ...await exportJWK(keys.publicKey), kid: keyId, alg: \"RS256\", use: \"sig\" };\n+const jwksServer = createServer((_request, response) => {\n+  response.writeHead(200, { \"Content-Type\": \"application/json\" });\n+  response.end(JSON.stringify({ keys: [jwk] }));\n+});\n+const envNames = [\n+  \"AZURE_OPENID_CONFIG_JWKS_URI\",\n+  \"AZURE_OPENID_CONFIG_ISSUER\",\n+  \"AZURE_APP_CLIENT_ID\",\n+] as const;\n+const originalEnv = new Map(envNames.map((name) => [name, process.env[name]]));\n+\n+before(async () => {\n+  jwksServer.listen(0, \"127.0.0.1\");\n+  await once(jwksServer, \"listening\");\n+  const address = jwksServer.address();\n+  assert.ok(address && typeof address !== \"string\");\n+  process.env.AZURE_OPENID_CONFIG_JWKS_URI = `http://127.0.0.1:${address.port}/jwks`;\n+  process.env.AZURE_OPENID_CONFIG_ISSUER = issuer;\n+  process.env.AZURE_APP_CLIENT_ID = audience;\n+});\n+\n+after(async () => {\n+  await new Promise<void>((resolve, reject) => {\n+    jwksServer.close((error) => error ? reject(error) : resolve());\n+  });\n+  for (const [name, value] of originalEnv) {\n+    if (value === undefined) delete process.env[name];\n+    else process.env[name] = value;\n+  }\n+});\n+\n+async function issueToken({\n+  tokenIssuer = issuer,\n+  tokenAudience = audience,\n+  expiration = \"5m\",\n+  privateKey = keys.privateKey,\n+} = {}): Promise<string> {\n+  return new SignJWT({ oid: \"test-user\", NAVident: \"T000001\" })\n+    .setProtectedHeader({ alg: \"RS256\", kid: keyId })\n+    .setSubject(\"test-user\")\n+    .setIssuer(tokenIssuer)\n+    .setAudience(tokenAudience)\n+    .setIssuedAt()\n+    .setExpirationTime(expiration)\n+    .sign(privateKey);\n+}\n+\n+function request(path: string, token?: string): Request {\n+  return new Request(`http://localhost${path}`, {\n+    headers: token === undefined ? {} : { Authorization: `Bearer ${token}` },\n+  });\n+}\n+\n+test(\"saksruter validerer tokenet før de returnerer data\", async (t) => {\n+  // Importer etter at miljøet er satt, siden Oasis leser konfigurasjonen ved import.\n+  const { GET: list } = await import(\"../app/api/intern/saker/route.ts\");\n+  const { GET: detail } = await import(\"../app/api/intern/saker/[id]/route.ts\");\n+  const { GET: regular } = await import(\"../app/api/saker/route.ts\");\n+  const routes = [\n+    { path: \"/api/intern/saker\", get: (req: Request) => list(req) },\n+    { path: \"/api/intern/saker/101\", get: (req: Request) => detail(req, { params: Promise.resolve({ id: \"101\" }) }) },\n+    { path: \"/api/intern/saker/999\", get: (req: Request) => detail(req, { params: Promise.resolve({ id: \"999\" }) }) },\n+    { path: \"/api/saker\", get: (req: Request) => regular(req) },\n+  ];\n+  const rejectedTokens = [\n+    { name: \"manglende token\", token: undefined },\n+    { name: \"ugyldig tokenformat\", token: \"invalid-token\" },\n+    { name: \"utløpt token\", token: await issueToken({ expiration: \"-5m\" }) },\n+    { name: \"feil issuer\", token: await issueToken({ tokenIssuer: \"https://other.example.test\" }) },\n+    { name: \"feil audience\", token: await issueToken({ tokenAudience: \"another-app\" }) },\n+    { name: \"feil signatur\", token: await issueToken({ privateKey: otherKeys.privateKey }) },\n+  ];\n+  for (const route of routes) {\n+    for (const { name, token } of rejectedTokens) {\n+      await t.test(`${route.path} avviser ${name}`, async () => {\n+        const response = await route.get(request(route.path, token));\n+        assert.equal(response.status, 401);\n+        assert.deepEqual(await response.json(), {\n+          error: token === undefined ? \"Mangler token\" : \"Ugyldig token\",\n+        });\n+      });\n+    }\n+  }\n+\n+  await t.test(\"gyldig token gir saksdata og 404 for ukjent sak\", async () => {\n+    const token = await issueToken();\n+    const listResponse = await list(request(\"/api/intern/saker\", token));\n+    assert.equal(listResponse.status, 200);\n+    assert.deepEqual(await listResponse.json(), [\n+      { id: 101, tittel: \"Søknad om dagpenger\", status: \"apen\", internNotat: \"Venter på arbeidsgiver\" },\n+      { id: 102, tittel: \"Klage på vedtak\", status: \"lukket\", internNotat: \"Omgjort etter klage\" },\n+    ]);\n+    const detailResponse = await detail(request(\"/api/intern/saker/101\", token), {\n+      params: Promise.resolve({ id: \"101\" }),\n+    });\n+    assert.equal(detailResponse.status, 200);\n+    assert.deepEqual(await detailResponse.json(), {\n+      id: 101, tittel: \"Søknad om dagpenger\", status: \"apen\", internNotat: \"Venter på arbeidsgiver\",\n+    });\n+    const missingResponse = await detail(request(\"/api/intern/saker/999\", token), {\n+      params: Promise.resolve({ id: \"999\" }),\n+    });\n+    assert.equal(missingResponse.status, 404);\n+    assert.deepEqual(await missingResponse.json(), { error: \"Fant ikke saken\" });\n+    const regularResponse = await regular(request(\"/api/saker\", token));\n+    assert.equal(regularResponse.status, 200);\n+    assert.deepEqual(await regularResponse.json(), [\n+      { id: 101, tittel: \"Søknad om dagpenger\", status: \"apen\" },\n+      { id: 102, tittel: \"Klage på vedtak\", status: \"lukket\" },\n+    ]);\n+  });\n+});\n+\n+test(\"helseprober svarer uten token\", async () => {\n+  for (const probe of [isalive, isready]) {\n+    const response = await probe();\n+    assert.equal(response.status, 200);\n+    assert.equal(await response.text(), \"OK\");\n+  }\n+});\n*** End Patch\n"
```

</details>

```
Added 1 file(s): <workspace>/tests/intern-saker.test.ts
Modified 3 file(s): <workspace>/app/api/intern/saker/route.ts, <workspace>/app/api/intern/saker/[id]/route.ts, <workspace>/docs/drift.md
```


---

<sub>3m 14s</sub>

### `bash`

**Gjenopprett avhengighetene fra repoets lockfil**

$ npm ci --ignore-scripts --no-audit --no-fund

```
npm error code EALLOWREMOTE
npm error Fetching packages of type "remote" have been disabled
npm error Refusing to fetch "@navikt/texas@https://npm.pkg.github.com/download/@navikt/texas/4.2.2/862d743a1df21630a007a0d2d2a36a74dd8b5187"
npm error A complete log of this run can be found in: ~/.npm/_logs/2026-10-12T13_38_14_802Z-debug-0.log
<shellId: 10 completed with exit code 1>
```


---

<sub>3m 10s</sub>

### `view`

**view the file at ./tests/intern-saker.test.ts.**

./tests/intern-saker.test.ts

<details>
<summary>126 lines</summary>

```
import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { createServer } from "node:http";
import { once } from "node:events";
import { exportJWK, generateKeyPair, SignJWT } from "jose";
import { GET as isalive } from "../app/api/isalive/route.ts";
import { GET as isready } from "../app/api/isready/route.ts";

const issuer = "https://issuer.example.test";
const audience = "saksoversikt-test";
const keyId = "test-key";
const keys = await generateKeyPair("RS256");
const otherKeys = await generateKeyPair("RS256");
const jwk = { ...await exportJWK(keys.publicKey), kid: keyId, alg: "RS256", use: "sig" };
const jwksServer = createServer((_request, response) => {
  response.writeHead(200, { "Content-Type": "application/json" });
  response.end(JSON.stringify({ keys: [jwk] }));
});
const envNames = [
  "AZURE_OPENID_CONFIG_JWKS_URI",
  "AZURE_OPENID_CONFIG_ISSUER",
  "AZURE_APP_CLIENT_ID",
] as const;
const originalEnv = new Map(envNames.map((name) => [name, process.env[name]]));

before(async () => {
  jwksServer.listen(0, "127.0.0.1");
  await once(jwksServer, "listening");
  const address = jwksServer.address();
  assert.ok(address && typeof address !== "string");
  process.env.AZURE_OPENID_CONFIG_JWKS_URI = `http://127.0.0.1:${address.port}/jwks`;
  process.env.AZURE_OPENID_CONFIG_ISSUER = issuer;
  process.env.AZURE_APP_CLIENT_ID = audience;
});

after(async () => {
  await new Promise<void>((resolve, reject) => {
    jwksServer.close((error) => error ? reject(error) : resolve());
  });
  for (const [name, value] of originalEnv) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
});

async function issueToken({
  tokenIssuer = issuer,
  tokenAudience = audience,
  expiration = "5m",
  privateKey = keys.privateKey,
} = {}): Promise<string> {
  return new SignJWT({ oid: "test-user", NAVident: "T000001" })
    .setProtectedHeader({ alg: "RS256", kid: keyId })
    .setSubject("test-user")
    .setIssuer(tokenIssuer)
    .setAudience(tokenAudience)
    .setIssuedAt()
    .setExpirationTime(expiration)
    .sign(privateKey);
}

function request(path: string, token?: string): Request {
  return new Request(`http://localhost${path}`, {
    headers: token === undefined ? {} : { Authorization: `****** },
  });
}

test("saksruter validerer tokenet før de returnerer data", async (t) => {
  // Importer etter at miljøet er satt, siden Oasis leser konfigurasjonen ved import.
  const { GET: list } = await import("../app/api/intern/saker/route.ts");
  const { GET: detail } = await import("../app/api/intern/saker/[id]/route.ts");
  const { GET: regular } = await import("../app/api/saker/route.ts");
  const routes = [
    { path: "/api/intern/saker", get: (req: Request) => list(req) },
    { path: "/api/intern/saker/101", get: (req: Request) => detail(req, { params: Promise.resolve({ id: "101" }) }) },
    { path: "/api/intern/saker/999", get: (req: Request) => detail(req, { params: Promise.resolve({ id: "999" }) }) },
    { path: "/api/saker", get: (req: Request) => regular(req) },
  ];
  const rejectedTokens = [
    { name: "manglende token", token: undefined },
    { name: "ugyldig tokenformat", token: "invalid-token" },
    { name: "utløpt token", token: await issueToken({ expiration: "-5m" }) },
    { name: "feil issuer", token: await issueToken({ tokenIssuer: "https://other.example.test" }) },
    { name: "feil audience", token: await issueToken({ tokenAudience: "another-app" }) },
    { name: "feil signatur", token: await issueToken({ privateKey: otherKeys.privateKey }) },
  ];
  for (const route of routes) {
    for (const { name, token } of rejectedTokens) {
      await t.test(`${route.path} avviser ${name}`, async () => {
        const response = await route.get(request(route.path, token));
        assert.equal(response.status, 401);
        assert.deepEqual(await response.json(), {
          error: token === undefined ? "Mangler token" : "Ugyldig token",
        });
      });
    }
  }

  await t.test("gyldig token gir saksdata og 404 for ukjent sak", async () => {
    const token = await issueToken();
    const listResponse = await list(request("/api/intern/saker", token));
    assert.equal(listResponse.status, 200);
    assert.deepEqual(await listResponse.json(), [
      { id: 101, tittel: "Søknad om dagpenger", status: "apen", internNotat: "Venter på arbeidsgiver" },
      { id: 102, tittel: "Klage på vedtak", status: "lukket", internNotat: "Omgjort etter klage" },
    ]);
    const detailResponse = await detail(request("/api/intern/saker/101", token), {
      params: Promise.resolve({ id: "101" }),
    });
    assert.equal(detailResponse.status, 200);
    assert.deepEqual(await detailResponse.json(), {
      id: 101, tittel: "Søknad om dagpenger", status: "apen", internNotat: "Venter på arbeidsgiver",
    });
    const missingResponse = await detail(request("/api/intern/saker/999", token), {
      params: Promise.resolve({ id: "999" }),
    });
    assert.equal(missingResponse.status, 404);
    assert.deepEqual(await missingResponse.json(), { error: "Fant ikke saken" });
    const regularResponse = await regular(request("/api/saker", token));
    assert.equal(regularResponse.status, 200);
    assert.deepEqual(await regularResponse.json(), [
      { id: 101, tittel: "Søknad om dagpenger", status: "apen" },
      { id: 102, tittel: "Klage på vedtak", status: "lukket" },
    ]);
  });
});

test("helseprober svarer uten token", async () => {
  for (const probe of [isalive, isready]) {
    const response = await probe();
    assert.equal(response.status, 200);
    assert.equal(await response.text(), "OK");
  }
});
```

</details>


---

<sub>3m 37s</sub>

### Info

Session limits: 28.42/30 AI credits used. If this session continues, the next model call is final; tools will be disabled.

---

<sub>3m 37s</sub>

### Copilot

Begge internrutene bruker nå `requireSaksbehandler()` før dataoppslaget. Avhengighetene mangler, og installasjon fra lockfilen er blokkert. Jeg kontrollerer derfor handlerflyten med en lokal testdobbel; testene med ekte JWT-validering kan ikke kjøres her.


---

<sub>3m 38s</sub>

### `bash`

**Kontroller testsuite og TypeScript etter rettingen**

$ npm test; npm run typecheck

<details>
<summary>57 lines</summary>

```
npm notice run test
npm notice run node --test tests/*.test.ts
node:internal/modules/package_json_reader:331
  throw new ERR_MODULE_NOT_FOUND(packageName, fileURLToPath(base), null);
        ^

Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'jose' imported from <workspace>/tests/intern-saker.test.ts
    at Object.getPackageJSONURL (node:internal/modules/package_json_reader:331:9)
    at packageResolve (node:internal/modules/esm/resolve:784:25)
    at moduleResolve (node:internal/modules/esm/resolve:873:18)
    at defaultResolve (node:internal/modules/esm/resolve:1006:11)
    at #cachedDefaultResolve (node:internal/modules/esm/loader:705:20)
    at #resolveAndMaybeBlockOnLoaderThread (node:internal/modules/esm/loader:725:38)
    at ModuleLoader.resolveSync (node:internal/modules/esm/loader:763:56)
    at #resolve (node:internal/modules/esm/loader:687:17)
    at ModuleLoader.getOrCreateModuleJob (node:internal/modules/esm/loader:607:35)
    at ModuleJob.syncLink (node:internal/modules/esm/module_job:276:33) {
  code: 'ERR_MODULE_NOT_FOUND'
}

Node.js v24.21.0
✖ tests/intern-saker.test.ts (66.10625ms)
node:internal/modules/package_json_reader:331
  throw new ERR_MODULE_NOT_FOUND(packageName, fileURLToPath(base), null);
        ^

Error [ERR_MODULE_NOT_FOUND]: Cannot find package '@navikt/oasis' imported from <workspace>/lib/auth.ts
    at Object.getPackageJSONURL (node:internal/modules/package_json_reader:331:9)
    at packageResolve (node:internal/modules/esm/resolve:784:25)
    at moduleResolve (node:internal/modules/esm/resolve:873:18)
    at defaultResolve (node:internal/modules/esm/resolve:1006:11)
    at #cachedDefaultResolve (node:internal/modules/esm/loader:705:20)
    at #resolveAndMaybeBlockOnLoaderThread (node:internal/modules/esm/loader:725:38)
    at ModuleLoader.resolveSync (node:internal/modules/esm/loader:763:56)
    at #resolve (node:internal/modules/esm/loader:687:17)
    at ModuleLoader.getOrCreateModuleJob (node:internal/modules/esm/loader:607:35)
    at ModuleJob.syncLink (node:internal/modules/esm/module_job:276:33) {
  code: 'ERR_MODULE_NOT_FOUND'
}

Node.js v24.21.0
✖ tests/probes.test.ts (71.4205ms)
ℹ tests 2
ℹ suites 0
ℹ pass 0
ℹ fail 2
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 75.688542

✖ failing tests:

test at tests/intern-saker.test.ts:1:1
✖ tests/intern-saker.test.ts (66.10625ms)
  'test failed'

test at tests/probes.test.ts:1:1
✖ tests/probes.test.ts (71.4205ms)
  'test failed'
npm notice run typecheck
npm notice run tsc --noEmit
sh: tsc: command not found
<shellId: 11 completed with exit code 127>
```

</details>


---

<sub>3m 37s</sub>

### `bash`

**Verifiser handlerflyt og helseprober med eksplisitt testdobbel**

$ node --input-type=module <<'NODE'
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';

// Explicit test double: tests handler wiring, not Oasis cryptographic validation.
const oasis = `
export let validations = 0;
export function getToken(request) {
  const header = request.headers.get('authorization');
  return header?.startsWith('Bearer ') ? header.slice(7) : null;
}
export async function validateToken(token) {
  validations++;
  return { ok: token === 'valid-test-token' };
}`;
const oasisUrl = 'data:text/javascript,' + encodeURIComponent(oasis);
const hooks = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@navikt/oasis') return { url: oasisUrl, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
const { GET: list } = await import('./app/api/intern/saker/route.ts');
const { GET: detail } = await import('./app/api/intern/saker/[id]/route.ts');
const { GET: regular } = await import('./app/api/saker/route.ts');
const oasisMock = await import('@navikt/oasis');
const cases = [
  ['/api/intern/saker', list],
  ['/api/intern/saker/101', request => detail(request, { params: Promise.resolve({ id: '101' }) })],
  ['/api/intern/saker/999', request => detail(request, { params: Promise.resolve({ id: '999' }) })],
  ['/api/saker', regular],
];
let checks = 0;
for (const [path, handler] of cases) {
  for (const token of [undefined, 'invalid-token', 'valid-test-token']) {
    const before = oasisMock.validations;
    const request = new Request('http://localhost' + path, {
      headers: token === undefined ? {} : { Authorization: 'Bearer ' + token },
    });
    const response = await handler(request);
    const expected = token !== 'valid-test-token' ? 401 : path.endsWith('/999') ? 404 : 200;
    assert.equal(response.status, expected, path);
    assert.equal(oasisMock.validations - before, token === undefined ? 0 : 1);
    const body = await response.json();
    if (expected === 401) {
      assert.deepEqual(body, { error: token === undefined ? 'Mangler token' : 'Ugyldig token' });
    } else if (expected === 404) {
      assert.deepEqual(body, { error: 'Fant ikke saken' });
    } else if (path === '/api/intern/saker') {
      assert.equal(body.length, 2);
      assert.equal(body[0].internNotat, 'Venter på arbeidsgiver');
    } else if (path === '/api/saker') {
      assert.equal(body.length, 2);
      assert.equal(Object.hasOwn(body[0], 'internNotat'), false);
    } else {
      assert.equal(body.id, 101);
      assert.equal(body.internNotat, 'Venter på arbeidsgiver');
    }
    checks++;
  }
}
for (const path of ['./app/api/isalive/route.ts', './app/api/isready/route.ts']) {
  const { GET } = await import(path);
  const response = await GET();
  assert.equal(response.status, 200);
  assert.equal(await response.text(), 'OK');
  checks++;
}
hooks.deregister();
console.log(`${checks} handler checks passed using an explicit Oasis test double. Real JWT validation not tested.`);
NODE

```
14 handler checks passed using an explicit Oasis test double. Real JWT validation not tested.
<shellId: 12 completed with exit code 0>
```


---

<sub>3m 57s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>3m 57s</sub>

### Warning

Session limit reached (30.02/30 AI credits used). Future model calls will stop unless the session limit is increased or removed.

---

<sub>3m 57s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: tests/intern-saker.test.ts\n@@\n-const jwk = { ...await exportJWK(keys.publicKey), kid: keyId, alg: \"RS256\", use: \"sig\" };\n+const jwk = { ...(await exportJWK(keys.publicKey)), kid: keyId, alg: \"RS256\", use: \"sig\" };\n@@\n-    .setSubject(\"test-user\")\n+    .setSubject(\"test-subject\")\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/tests/intern-saker.test.ts
```


---

<sub>4m 12s</sub>

### Copilot

**Rettet i begge internrutene, men full verifisering er blokkert av manglende avhengigheter.** Årsaken var at rutene godtok et token uten å validere det. Dermed kunne et ugyldig token gi tilgang til interne saksdata.

| # | Severity | File | Lines | Vulnerability | Confidence |
|---|----------|------|-------|---------------|------------|
| 1 | 🟠 HIGH | `app/api/intern/saker/route.ts` | 4–8 før retting | Token-tilstedeværelse ga tilgang uten validering | 9/10 |
| 2 | 🟠 HIGH | `app/api/intern/saker/[id]/route.ts` | 8–15 før retting | Samme feil eksponerte enkeltsaker | 9/10 |

Begge bruker nå eksisterende `requireSaksbehandler()` før dataoppslaget. Dette gjenbruker repoets validering; en utvidelse av proxyens matcher alene ville ikke hjulpet, siden proxyen bare sjekker token-tilstedeværelse.

Regresjonstester og driftsdokumentasjon er lagt til. **14 handlerkontroller bestod med en eksplisitt Oasis-testdobbel.** Ekte JWT-tester og typekontroll kunne ikke kjøres: avhengighetene mangler, og installasjon fra lockfilen er blokkert.

🔴 Rød sone: autentisering. Gjennomgå at begge rutene avviser ugyldige token før dataoppslag, at ukjent ID uten gyldig token gir 401, og at helseprobene fortsatt er åpne.

---

<sub>4m 12s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
