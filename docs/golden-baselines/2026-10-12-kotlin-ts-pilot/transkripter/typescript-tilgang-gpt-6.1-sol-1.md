# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `4ff1839a-f998-412e-9f28-7652bf6ab18a`  
> - **Started:** 10/12/2026, 3:22:35 PM  
> - **Duration:** 4m 52s  
> - **Exported:** 10/12/2026, 3:27:28 PM  

---

<sub>0s</sub>

### User

En penetrasjonstest viser at `/api/intern/saker` svarer uten gyldig innlogging.
Finn årsaken og rett feilen. Bruk kun dette repoet.

---

<sub>13s</sub>

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

**Finn lokale instruksjoner og autentiseringsfiler**

$ pwd; git --no-pager status; find . -type f -not -path './node_modules/*' -not -path './.next/*' -not -path './.git/*' | sort

<details>
<summary>32 lines</summary>

```
<workspace>
fatal: not a git repository (or any of the parent directories): .git
./.github/instructions/accessibility.instructions.md
./.github/instructions/database.instructions.md
./.github/instructions/deliberate-ai-use.instructions.md
./.github/instructions/docker.instructions.md
./.github/instructions/github-actions.instructions.md
./.github/instructions/golang.instructions.md
./.github/instructions/kotlin.instructions.md
./.github/instructions/nextjs-aksel.instructions.md
./.github/instructions/norwegian-text.instructions.md
./.github/instructions/output-style.instructions.md
./.github/instructions/performance.instructions.md
./.github/instructions/security-core.instructions.md
./.github/instructions/security-owasp.instructions.md
./.github/instructions/testing-kotlin.instructions.md
./.github/instructions/testing-typescript.instructions.md
./.github/instructions/testing.instructions.md
./app/api/intern/saker/[id]/route.ts
./app/api/intern/saker/route.ts
./app/api/isalive/route.ts
./app/api/isready/route.ts
./app/api/saker/route.ts
./docs/drift.md
./lib/auth.ts
./lib/saker.ts
./package-lock.json
./package.json
./proxy.ts
./tests/probes.test.ts
./tsconfig.json
<shellId: 0 completed with exit code 0>
```

</details>


### `bash`

**Les lokale instrukser og endepunktets autentiseringsflyt**

$ for f in .github/instructions/security-core.instructions.md .github/instructions/security-owasp.instructions.md .github/instructions/testing.instructions.md .github/instructions/testing-typescript.instructions.md .github/instructions/nextjs-aksel.instructions.md .github/instructions/output-style.instructions.md .github/instructions/norwegian-text.instructions.md proxy.ts lib/auth.ts app/api/intern/saker/route.ts 'app/api/intern/saker/[id]/route.ts' app/api/saker/route.ts lib/saker.ts tests/probes.test.ts package.json tsconfig.json docs/drift.md; do printf '\n--- %s ---\n' "$f"; nl -ba "$f"; done

<details>
<summary>15 lines</summary>

```
Output too large to read at once (42.1 KB). Saved to: ~/Library/Caches/cplt/tmp/78c1b3886d41e3fb66376b224ed1916d/1791811382414-copilot-tool-output-94692-306b2255-f999-4a0c-9629-87d4839f8f96.txt
Consider using tools like rg (for searching), head/tail (for viewing start/end), view with view_range (for specific sections), or jq (for JSON) to examine portions of the output.

Preview (first 500 chars):

--- .github/instructions/security-core.instructions.md ---
     1	---
     2	applyTo: "**"
     3	---
     4	
     5	# Security Core
     6	
     7	These rules apply to every change, in every language. They are never optional.
     8	
     9	- **No secrets or personal data in logs.** Never log tokens, passwords, keys, fødselsnummer (FNR), name or address.
    10	- **Secrets come from the environment.** Read tokens, passwords and keys from environment variables or the secret store. Never hardcod
<shellId: 1 completed with exit code 0>
```

</details>


### `bash`

**Les auth, proxy, ruter, tester og driftsdokumentasjon**

$ for f in proxy.ts lib/auth.ts app/api/intern/saker/route.ts 'app/api/intern/saker/[id]/route.ts' app/api/saker/route.ts lib/saker.ts tests/probes.test.ts package.json tsconfig.json docs/drift.md; do printf '\n--- %s ---\n' "$f"; nl -ba "$f"; done

<details>
<summary>159 lines</summary>

```
--- proxy.ts ---
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

--- package.json ---
     1	{
     2	  "name": "saksoversikt",
     3	  "private": true,
     4	  "type": "module",
     5	  "engines": { "node": "24" },
     6	  "scripts": {
     7	    "build": "next build --webpack",
     8	    "start": "next start",
     9	    "test": "node --test tests/*.test.ts",
    10	    "typecheck": "tsc --noEmit"
    11	  },
    12	  "dependencies": {
    13	    "@navikt/oasis": "4.2.2",
    14	    "next": "16.3.8",
    15	    "react": "19.3.0",
    16	    "react-dom": "19.3.0"
    17	  },
    18	  "devDependencies": {
    19	    "@types/node": "26.6.2",
    20	    "@types/react": "19.3.0",
    21	    "jose": "6.2.12",
    22	    "typescript": "6.0.3"
    23	  }
    24	}

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
<shellId: 2 completed with exit code 0>
```

</details>


### `bash`

**Les relevante lokale regler og sjekk mulighet for lokal verifikasjon**

$ for f in .github/instructions/security-core.instructions.md .github/instructions/testing.instructions.md .github/instructions/testing-typescript.instructions.md .github/instructions/output-style.instructions.md .github/instructions/norwegian-text.instructions.md; do printf '\n--- %s ---\n' "$f"; cat "$f"; done; printf '\n--- lokale avhengigheter ---\n'; test -d node_modules && printf 'node_modules finnes\n'; node --version; wc -l .github/instructions/security-owasp.instructions.md .github/instructions/nextjs-aksel.instructions.md; ls -a

<details>
<summary>286 lines</summary>

````
--- .github/instructions/security-core.instructions.md ---
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

--- .github/instructions/testing.instructions.md ---
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

--- .github/instructions/testing-typescript.instructions.md ---
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

--- .github/instructions/output-style.instructions.md ---
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

--- .github/instructions/norwegian-text.instructions.md ---
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

--- lokale avhengigheter ---
v24.21.0
      12 .github/instructions/security-owasp.instructions.md
     598 .github/instructions/nextjs-aksel.instructions.md
     610 total
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
<shellId: 3 completed with exit code 0>
````

</details>


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


### `bash`

**Les auth-relaterte lokale regler og kjør eksisterende tester**

$ cat .github/instructions/security-owasp.instructions.md; cat .github/instructions/deliberate-ai-use.instructions.md; sed -n '1,35p' .github/instructions/nextjs-aksel.instructions.md; rg -n -i 'auth|token|security|api route|middleware|proxy' .github/instructions/nextjs-aksel.instructions.md; for f in app/api/isalive/route.ts app/api/isready/route.ts; do printf '\n--- %s ---\n' "$f"; nl -ba "$f"; done; npm test

<details>
<summary>114 lines</summary>

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
---
description: "Next.js med Aksel: spacing-tokens i stedet for Tailwind p-/m-, riktige token-navn, responsiv layout, App Router, Server Components og auth."
applyTo: "src/**/*.{tsx,ts}"
---

# Next.js med Aksel Design System

Standarder for Next.js-apper med Aksel: spacing-tokens, responsive props og komponentmønstre.

<!-- BEGIN:nextjs-agent-rules -->

# Next.js: ALWAYS read docs before coding

Before any Next.js work, find and read the relevant doc in `node_modules/next/dist/docs/`. Your training data is outdated — the docs are the source of truth.

<!-- END:nextjs-agent-rules -->

## Aksel-dokumentasjon

For komponent-APIer, tokens, theming, layout og mønstre — hent dokumentasjon fra:

```
https://aksel.nav.no/llm.md
```

Filen er et indeks over alle tilgjengelige Aksel-dokumentasjonssider som individuelle `.md`-filer. Hent individuelle sider ved behov fremfor å anta API fra treningsdata.

## Spacing-regler

**VIKTIG**: Bruk alltid Nav DS spacing-tokens, aldri Tailwind padding/margin.

### ✅ Riktig

```tsx
import { Box, VStack, HGrid } from "@navikt/ds-react";
2:description: "Next.js med Aksel: spacing-tokens i stedet for Tailwind p-/m-, riktige token-navn, responsiv layout, App Router, Server Components og auth."
8:Standarder for Next.js-apper med Aksel: spacing-tokens, responsive props og komponentmønstre.
20:For komponent-APIer, tokens, theming, layout og mønstre — hent dokumentasjon fra:
30:**VIKTIG**: Bruk alltid Nav DS spacing-tokens, aldri Tailwind padding/margin.
73:## Spacing-tokens
75:Token name = eksakt pikselverdi (`space-16` = 16px, `space-4` = 4px). Alltid `space-`-prefiks.
77:Fullstendig liste: [design-tokens](https://aksel.nav.no/grunnleggende/styling/design-tokens.md)
168:## API Routes (App Router)
213:  const token = cookieStore.get("auth-token")?.value;
250:## Middleware
253:// src/middleware.ts
256:export function middleware(request: NextRequest) {
259:  // Security headers
271:## Authentication
274:import { getUser } from "@/lib/auth";
276:// Redirect if not authenticated
279:// Return null if not authenticated
282:  return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
414:import { getUser } from "@/lib/auth";
566:- Use spacing tokens with `space-` prefix
569:- Explicit error handling in API routes
577:- Changing authentication flow
593:| `@aksel-agent`             | Aksel Design System component patterns and spacing tokens        |
598:| aksel.nav.no/llm.md        | Full Aksel documentation index (components, tokens, foundations) |

--- app/api/isalive/route.ts ---
     1	export async function GET(): Promise<Response> {
     2	  return new Response("OK");
     3	}

--- app/api/isready/route.ts ---
     1	export async function GET(): Promise<Response> {
     2	  return new Response("OK");
     3	}
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
✖ tests/probes.test.ts (75.956792ms)
ℹ tests 1
ℹ suites 0
ℹ pass 0
ℹ fail 1
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 78.807167

✖ failing tests:

test at tests/probes.test.ts:1:1
✖ tests/probes.test.ts (75.956792ms)
  'test failed'
<shellId: 4 completed with exit code 1>
````

</details>


### `skill`

**security-owasp**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "security-owasp"
}
```

</details>

```
Skill "security-owasp" loaded successfully. Follow the instructions in the skill context.
```


### `bash`

**Bekreft eneste auth-kontroller og les relevante ruteregler**

$ sed -n '168,193p' .github/instructions/nextjs-aksel.instructions.md; sed -n '250,285p' .github/instructions/nextjs-aksel.instructions.md; sed -n '555,585p' .github/instructions/nextjs-aksel.instructions.md; rg -n 'getToken|validateToken|requireSaksbehandler|matcher|Authorization|AZURE_' app lib proxy.ts tests docs; node --input-type=module -e 'import fs from "node:fs"; const lock=JSON.parse(fs.readFileSync("package-lock.json","utf8")); console.log(JSON.stringify(lock.packages?.["node_modules/@navikt/oasis"],null,2));'

<details>
<summary>103 lines</summary>

````
## API Routes (App Router)

```typescript
import { NextResponse } from "next/server";

// GET endpoint with error handling
export async function GET() {
  const { data, error } = await fetchData();

  if (error) {
    return NextResponse.json({ error: error.message }, { status: 500 });
  }

  return NextResponse.json(data);
}

// POST endpoint
export async function POST(request: Request) {
  const body = await request.json();

  // Validation
  if (!body.requiredField) {
    return NextResponse.json(
      { error: "requiredField is missing" },
      { status: 400 },
    );
## Middleware

```typescript
// src/middleware.ts
import { NextRequest, NextResponse } from "next/server";

export function middleware(request: NextRequest) {
  const response = NextResponse.next();

  // Security headers
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");

  return response;
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
};
```

## Authentication

```typescript
import { getUser } from "@/lib/auth";

// Redirect if not authenticated
const user = await getUser();

// Return null if not authenticated
const user = await getUser(false);
if (!user) {
  return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
}
```

pnpm add @navikt/ds-react
pnpm test

# Lock file: pnpm-lock.yaml (commit this)
```

## Boundaries

### ✅ Always

- Use Aksel Design System components
- Use spacing tokens with `space-` prefix
- Mobile-first responsive design
- Norwegian number formatting
- Explicit error handling in API routes
- pnpm for new projects
- Preserve existing code structure when making targeted fixes — don't rename, restructure, or refactor working code beyond the task at hand

### ⚠️ Ask First

- Adding custom Tailwind utilities
- Deviating from Aksel patterns
- Changing authentication flow
- Modifying data aggregation logic
- Introducing React Query or React Hook Form into existing projects

### 🚫 Never

- Use Tailwind padding/margin utilities (`p-*`, `m-*`)
- Use numeric spacing without `space-` prefix
- Ignore accessibility requirements
app/api/saker/route.ts:1:import { requireSaksbehandler } from "../../../lib/auth.ts";
app/api/saker/route.ts:5:  const unauthorized = await requireSaksbehandler(request);
lib/auth.ts:1:import { getToken, validateToken } from "@navikt/oasis";
lib/auth.ts:7:export async function requireSaksbehandler(request: Request): Promise<Response | null> {
lib/auth.ts:8:  const token = getToken(request);
lib/auth.ts:12:  const validation = await validateToken(token);
app/api/intern/saker/route.ts:1:import { getToken } from "@navikt/oasis";
app/api/intern/saker/route.ts:5:  if (!getToken(request)) {
docs/drift.md:4:saksbehandlerens token i `Authorization`-headeren.
docs/drift.md:9:  med `requireSaksbehandler()` i `lib/auth.ts`.
docs/drift.md:10:- `AZURE_OPENID_CONFIG_JWKS_URI`, `AZURE_OPENID_CONFIG_ISSUER` og
docs/drift.md:11:  `AZURE_APP_CLIENT_ID` settes av NAIS.
app/api/intern/saker/[id]/route.ts:1:import { getToken } from "@navikt/oasis";
app/api/intern/saker/[id]/route.ts:8:  if (!getToken(request)) {
proxy.ts:1:import { getToken } from "@navikt/oasis";
proxy.ts:5:// Selve valideringen av tokenet gjør hver route handler med requireSaksbehandler().
proxy.ts:7:  if (!getToken(request)) {
proxy.ts:14:  matcher: ["/api/saker/:path*"],
{
  "version": "4.2.2",
  "resolved": "https://npm.pkg.github.com/download/@navikt/oasis/4.2.2/3c43e1e89bb2d46646a9a2aaf27972a024377a6f",
  "integrity": "sha512-EVMsF8uKbLPNRWuPfH1IKVJhqtibFzSTSyNkzj5mTCVMZV2QPS5W3uYMD4r0rXsmLx7nv0HnLovDphb3AFEjvw==",
  "license": "MIT",
  "dependencies": {
    "@navikt/texas": "^4.2.2",
    "@opentelemetry/api": "^1.9.1",
    "@prometheus-io/client": "^0.16.1",
    "jose": "^6.2.10"
  },
  "engines": {
    "node": "^20.19.0 || >= 22.12.0 || 24.x || 25.x"
  }
}
<shellId: 5 completed with exit code 0>
````

</details>


### Copilot

## Security Findings

### Alert 1: Token-tilstedeværelse erstatter autentisering

**Severity: HIGH | Confidence: 9/10**  
**Category:** AuthenticationFailure

**Berørte filer og linjer**
- `<workspace>/app/api/intern/saker/route.ts:1–8`
- `<workspace>/app/api/intern/saker/[id]/route.ts:1–15`

**Problem og rotårsak:** Begge interne GET-handlere kontrollerer bare at `getToken(request)` returnerer et token. Ingen av dem kaller token-valideringen. En klient som når appen med et token som kan hentes ut, trenger derfor ikke bevise gyldig innlogging før handleren returnerer interne saker, inkludert `internNotat`.

Dette gjelder **ugyldig token**, ikke nødvendigvis fravær av token: Begge handlerne har allerede en eksplisitt 401-retur når `getToken()` ikke finner noe token.

**Bekreftende kode**
- `<workspace>/lib/auth.ts:7–16` skiller mellom token-uttrekk og validering. `requireSaksbehandler()` kaller `validateToken()` og returnerer 401 når `validation.ok` er falsk. De interne handlerne bruker ikke denne funksjonen.
- `<workspace>/proxy.ts:4–14` forutsetter validering i handlerne. Matcher dekker bare `/api/saker/:path*`, ikke de interne rutene. Proxyen kontrollerer dessuten bare token-tilstedeværelse, så utvidet matcher alene løser ikke feilen.

### Anbefalt kirurgisk retting

Erstatt den direkte `getToken()`-kontrollen i **begge interne handlere** med et avventet kall til eksisterende `requireSaksbehandler(request)`. Returner avvisningssvaret før saksoppslag eller uthenting av data. Listehandleren skal ikke hente saker ved avvist autentisering; detaljhandleren skal heller ikke avsløre 404 før autentisering er godkjent.

Dette gjenbruker appens etablerte validering uten ny auth-logikk eller avhengigheter. En bredere proxy-matcher er ikke nødvendig for denne rettingen, og helseendepunktene må fortsatt være tilgjengelige uten token.

### Tester hovedagenten bør legge til

Bruk eksisterende `node:test`-oppsett. Test begge interne handlere direkte, slik at autentisering ikke avhenger av proxyen.

| Tilfelle | Forventning |
|---|---|
| Ingen `Authorization`-header | 401, ingen saksdata |
| `Bearer` med ugyldig token | 401, ingen saksdata |
| JWT med feil signatur, utløpt `exp`, feil `iss` eller feil `aud` | 401 for hvert tilfelle |
| Gyldig token med riktig signatur, issuer, audience og levetid | 200 med forventet saksliste eller sak |
| Ugyldig token og ukjent saks-ID | 401, ikke 404 |
| Gyldig token og ukjent saks-ID | 404 |
| Begge helseendepunkter uten token | Fortsatt 200 |

For JWT-testene: bruk lokalt genererte testnøkler og lokal JWKS-server med testkonfigurasjon satt før auth-modulen lastes. Behold den reelle `validateToken()`-implementasjonen i disse testene; ikke mock godkjenning som eneste bevis på gyldig innlogging.

**Verifikasjonsbegrensning:** Funnene er bekreftet ved statisk lesing. `npm test` stopper med `ERR_MODULE_NOT_FOUND` for `@navikt/oasis`, fordi avhengighetene ikke er installert. Ingen HTTP-reproduksjon eller vellykket testkjøring er derfor bekreftet. Arbeidsmappen mangler også Git-metadata, så ingen diff kunne avgrenses. Ingen kode er endret.


---

<sub>49s</sub>

### Info

Session limits: 16.41/30 AI credits used.

---

<sub>1m 34s</sub>

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

<sub>1m 33s</sub>

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

<sub>1m 55s</sub>

### Info

Session limits: 23.41/30 AI credits used.

---

<sub>1m 55s</sub>

### Copilot

Rotårsaken er at de interne handlerne bare sjekker om et token finnes, ikke om det er gyldig. Jeg retter begge interne endepunkter med den eksisterende valideringen, slik at beskyttelsen ikke avhenger av proxyen.


---

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

### `view`

**view the file at <workspace>/lib/saker.ts.**

<workspace>/lib/saker.ts

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>1m 57s</sub>

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

<sub>2m 8s</sub>

### `bash`

**Kjør eksisterende tester**

$ npm test

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
✖ tests/probes.test.ts (69.5515ms)
ℹ tests 1
ℹ suites 0
ℹ pass 0
ℹ fail 1
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 72.386916

✖ failing tests:

test at tests/probes.test.ts:1:1
✖ tests/probes.test.ts (69.5515ms)
  'test failed'
<shellId: 6 completed with exit code 1>
```

</details>


---

<sub>2m 8s</sub>

### `view`

**view the file at <workspace>/.github/instructions/security-core.instructions.md.**

<workspace>/.github/instructions/security-core.instructions.md

<details>
<summary>12 lines</summary>

```
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
```

</details>


---

<sub>2m 8s</sub>

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


---

<sub>2m 8s</sub>

### `view`

**view the file at <workspace>/app/api/isready/route.ts.**

<workspace>/app/api/isready/route.ts

```
export async function GET(): Promise<Response> {
  return new Response("OK");
}

```


---

<sub>2m 17s</sub>

### `bash`

**Installer låste prosjektavhengigheter**

$ npm ci --ignore-scripts --no-audit --no-fund

```
npm error code EALLOWREMOTE
npm error Fetching packages of type "remote" have been disabled
npm error Refusing to fetch "@navikt/texas@https://npm.pkg.github.com/download/@navikt/texas/4.2.2/862d743a1df21630a007a0d2d2a36a74dd8b5187"
npm error A complete log of this run can be found in: ~/.npm/_logs/2026-10-12T13_24_49_877Z-debug-0.log
<shellId: 7 completed with exit code 1>
```


---

<sub>2m 26s</sub>

### `view`

**view the file at <workspace>/package-lock.json.**

<workspace>/package-lock.json (lines 1-180)

<details>
<summary>180 lines</summary>

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
        "node": ">=20.9.0"
      },
      "funding": {
        "url": "https://opencollective.com/libvips"
      }
    },
    "node_modules/@img/sharp-libvips-darwin-arm64": {
      "version": "1.3.4",
      "resolved": "https://registry.npmjs.org/@img/sharp-libvips-darwin-arm64/-/sharp-libvips-darwin-arm64-1.3.4.tgz",
      "integrity": "sha512-5R89nBYiRdUlSWJxPhO+GVtaXzXSxKnRu/xqMn3KTA3L9EB9Oy/P+Nn2f2vlhPuUdy/Zusb2DarbyTpGCfEDuw==",
      "cpu": [
        "arm64"
      ],
      "license": "LGPL-3.0-or-later",
      "optional": true,
      "os": [
        "darwin"
      ],
      "funding": {
        "url": "https://opencollective.com/libvips"
      }
    },
    "node_modules/@img/sharp-libvips-darwin-x64": {
      "version": "1.3.4",
      "resolved": "https://registry.npmjs.org/@img/sharp-libvips-darwin-x64/-/sharp-libvips-darwin-x64-1.3.4.tgz",
      "integrity": "sha512-iR2OKH80yi0U+dUplyh3/xdpFvps6YkCwsXenIJxqxR1v9o+xtKTGbS9H7cps+2Vxjc8B1j96p75NmTGjIhtpQ==",
      "cpu": [
        "x64"
      ],
      "license": "LGPL-3.0-or-later",
      "optional": true,
      "os": [
        "darwin"
      ],
      "funding": {
        "url": "https://opencollective.com/libvips"
      }
    },
    "node_modules/@img/sharp-libvips-linux-arm": {
      "version": "1.3.4",
      "resolved": "https://registry.npmjs.org/@img/sharp-libvips-linux-arm/-/sharp-libvips-linux-arm-1.3.4.tgz",
      "integrity": "sha512-LmRtTsOHuvM2+wlO2Db37dx5MiZhB0FvSunciw48YjdOkZz9KAiRbm8ujeMOA1INqmei5NapFxYEK1D1ZSidmw==",
      "cpu": [
        "arm"
      ],
      "libc": [
        "glibc"
      ],
      "license": "LGPL-3.0-or-later",
      "optional": true,
      "os": [
        "linux"
      ],
      "funding": {
        "url": "https://opencollective.com/libvips"
      }
    },
    "node_modules/@img/sharp-libvips-linux-arm64": {
      "version": "1.3.4",
      "resolved": "https://registry.npmjs.org/@img/sharp-libvips-linux-arm64/-/sharp-libvips-linux-arm64-1.3.4.tgz",
      "integrity": "sha512-Y3dgX/6lE2QhQb+Gxy0WZxfg9MEm/JBjamZpS2IklP7xIQoKN4hzAm7KcMVGtaVDt3neE9OKBC7vAfonA/Lr1A==",
      "cpu": [
        "arm64"
      ],
      "libc": [
        "glibc"
      ],
      "license": "LGPL-3.0-or-later",
      "optional": true,
      "os": [
        "linux"
      ],
      "funding": {
        "url": "https://opencollective.com/libvips"
      }
    },
    "node_modules/@img/sharp-libvips-linux-ppc64": {
      "version": "1.3.4",
      "resolved": "https://registry.npmjs.org/@img/sharp-libvips-linux-ppc64/-/sharp-libvips-linux-ppc64-1.3.4.tgz",
      "integrity": "sha512-Le6boB8Tai0Nis+gIxIpKx68UDVVIqdR8Tin5Yf1z2LJJQLDJvCDRqRu+jC2qCoD+eIomonmOwB4smBRxfVpYQ==",
```

</details>


---

<sub>2m 30s</sub>

### `bash`

**Prøv å installere fra lokal pakkecache**

$ npm ci --offline --ignore-scripts --no-audit --no-fund

```
npm error code EALLOWREMOTE
npm error Fetching packages of type "remote" have been disabled
npm error Refusing to fetch "@navikt/texas@https://npm.pkg.github.com/download/@navikt/texas/4.2.2/862d743a1df21630a007a0d2d2a36a74dd8b5187"
npm error A complete log of this run can be found in: ~/.npm/_logs/2026-10-12T13_25_02_512Z-debug-0.log
<shellId: 8 completed with exit code 1>
```


---

<sub>3m 15s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>3m 15s</sub>

### Warning

Session limit reached (30.13/30 AI credits used). Future model calls will stop unless the session limit is increased or removed.

---

<sub>3m 15s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: app/api/intern/saker/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: app/api/intern/saker/[id]/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: docs/drift.md\n@@\n - Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres\n-  med `requireSaksbehandler()` i `lib/auth.ts`.\n+  med `requireSaksbehandler()` i `lib/auth.ts` før handleren henter saksdata,\n+  også for `/api/intern/saker` og `/api/intern/saker/[id]`.\n+  `getToken()` henter bare tokenet og er ikke en validering. Handlerne må\n+  validere tokenet selv, uavhengig av proxyens matcher.\n*** Add File: tests/interne-saker.test.ts\n+import assert from \"node:assert/strict\";\n+import { once } from \"node:events\";\n+import { createServer } from \"node:http\";\n+import { after, before, test } from \"node:test\";\n+import { exportJWK, generateKeyPair, SignJWT, type JWTPayload } from \"jose\";\n+import { GET as listSaker } from \"../app/api/intern/saker/route.ts\";\n+import { GET as finnSak } from \"../app/api/intern/saker/[id]/route.ts\";\n+import { GET as isalive } from \"../app/api/isalive/route.ts\";\n+import { GET as isready } from \"../app/api/isready/route.ts\";\n+\n+const issuer = \"https://issuer.example.test\";\n+const audience = \"saksoversikt-test\";\n+const keyId = \"test-key\";\n+const originalEnv = {\n+  AZURE_OPENID_CONFIG_JWKS_URI: process.env.AZURE_OPENID_CONFIG_JWKS_URI,\n+  AZURE_OPENID_CONFIG_ISSUER: process.env.AZURE_OPENID_CONFIG_ISSUER,\n+  AZURE_APP_CLIENT_ID: process.env.AZURE_APP_CLIENT_ID,\n+};\n+let keys: Awaited<ReturnType<typeof generateKeyPair>>;\n+let otherKeys: Awaited<ReturnType<typeof generateKeyPair>>;\n+const jwksServer = createServer();\n+\n+before(async () => {\n+  keys = await generateKeyPair(\"RS256\");\n+  otherKeys = await generateKeyPair(\"RS256\");\n+  const jwk = { ...await exportJWK(keys.publicKey), kid: keyId, alg: \"RS256\", use: \"sig\" };\n+  jwksServer.on(\"request\", (_request, response) => {\n+    response.writeHead(200, { \"Content-Type\": \"application/json\" });\n+    response.end(JSON.stringify({ keys: [jwk] }));\n+  });\n+  jwksServer.listen(0, \"127.0.0.1\");\n+  await once(jwksServer, \"listening\");\n+  const address = jwksServer.address();\n+  assert.ok(address && typeof address !== \"string\");\n+  process.env.AZURE_OPENID_CONFIG_JWKS_URI = `http://127.0.0.1:${address.port}/jwks`;\n+  process.env.AZURE_OPENID_CONFIG_ISSUER = issuer;\n+  process.env.AZURE_APP_CLIENT_ID = audience;\n+});\n+\n+after(async () => {\n+  for (const [name, value] of Object.entries(originalEnv)) {\n+    if (value === undefined) delete process.env[name];\n+    else process.env[name] = value;\n+  }\n+  if (jwksServer.listening) {\n+    await new Promise<void>((resolve, reject) => {\n+      jwksServer.close((error) => error ? reject(error) : resolve());\n+      jwksServer.closeAllConnections();\n+    });\n+  }\n+});\n+\n+async function token(\n+  claims: JWTPayload = {},\n+  privateKey = keys.privateKey,\n+): Promise<string> {\n+  return new SignJWT({\n+    iss: issuer,\n+    aud: audience,\n+    sub: \"test-saksbehandler\",\n+    iat: Math.floor(Date.now() / 1000),\n+    exp: Math.floor(Date.now() / 1000) + 300,\n+    ...claims,\n+  })\n+    .setProtectedHeader({ alg: \"RS256\", kid: keyId })\n+    .sign(privateKey);\n+}\n+\n+const routes = [\n+  {\n+    path: \"/api/intern/saker\",\n+    get: (request: Request) => listSaker(request),\n+    expected: [\n+      { id: 101, tittel: \"Søknad om dagpenger\", status: \"apen\", internNotat: \"Venter på arbeidsgiver\" },\n+      { id: 102, tittel: \"Klage på vedtak\", status: \"lukket\", internNotat: \"Omgjort etter klage\" },\n+    ],\n+  },\n+  {\n+    path: \"/api/intern/saker/101\",\n+    get: (request: Request) => finnSak(request, { params: Promise.resolve({ id: \"101\" }) }),\n+    expected: { id: 101, tittel: \"Søknad om dagpenger\", status: \"apen\", internNotat: \"Venter på arbeidsgiver\" },\n+  },\n+];\n+\n+function request(path: string, bearer?: string): Request {\n+  return new Request(`http://localhost${path}`, {\n+    headers: bearer === undefined ? {} : { Authorization: `Bearer ${bearer}` },\n+  });\n+}\n+\n+for (const route of routes) {\n+  test(`${route.path} avviser manglende token`, async () => {\n+    const response = await route.get(request(route.path));\n+    assert.equal(response.status, 401);\n+    assert.deepEqual(await response.json(), { error: \"Mangler token\" });\n+  });\n+\n+  const invalidTokens = [\n+    { name: \"ugyldig format\", make: async () => \"ikke-et-jwt\" },\n+    { name: \"feil signatur\", make: () => token({}, otherKeys.privateKey) },\n+    { name: \"utløpt token\", make: () => token({ exp: Math.floor(Date.now() / 1000) - 3600 }) },\n+    { name: \"feil issuer\", make: () => token({ iss: \"https://wrong.example.test\" }) },\n+    { name: \"feil audience\", make: () => token({ aud: \"annen-app\" }) },\n+  ];\n+  for (const invalid of invalidTokens) {\n+    test(`${route.path} avviser ${invalid.name}`, async () => {\n+      const response = await route.get(request(route.path, await invalid.make()));\n+      assert.equal(response.status, 401);\n+      assert.deepEqual(await response.json(), { error: \"Ugyldig token\" });\n+    });\n+  }\n+\n+  test(`${route.path} returnerer saksdata med gyldig token`, async () => {\n+    const response = await route.get(request(route.path, await token()));\n+    assert.equal(response.status, 200);\n+    assert.deepEqual(await response.json(), route.expected);\n+  });\n+}\n+\n+test(\"ukjent sak avviser ugyldig token før saksoppslag\", async () => {\n+  const response = await finnSak(request(\"/api/intern/saker/999\", \"ikke-et-jwt\"), {\n+    params: Promise.resolve({ id: \"999\" }),\n+  });\n+  assert.equal(response.status, 401);\n+  assert.deepEqual(await response.json(), { error: \"Ugyldig token\" });\n+});\n+\n+test(\"ukjent sak gir 404 med gyldig token\", async () => {\n+  const response = await finnSak(request(\"/api/intern/saker/999\", await token()), {\n+    params: Promise.resolve({ id: \"999\" }),\n+  });\n+  assert.equal(response.status, 404);\n+  assert.deepEqual(await response.json(), { error: \"Fant ikke saken\" });\n+});\n+\n+test(\"helseendepunktene svarer uten token\", async () => {\n+  assert.equal((await isalive()).status, 200);\n+  assert.equal((await isready()).status, 200);\n+});\n*** End Patch\n"
```

</details>

```
Added 1 file(s): <workspace>/tests/interne-saker.test.ts
Modified 3 file(s): <workspace>/app/api/intern/saker/route.ts, <workspace>/app/api/intern/saker/[id]/route.ts, <workspace>/docs/drift.md
```


---

<sub>3m 37s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>3m 37s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: tests/interne-saker.test.ts\n@@\n-import { GET as listSaker } from \"../app/api/intern/saker/route.ts\";\n-import { GET as finnSak } from \"../app/api/intern/saker/[id]/route.ts\";\n import { GET as isalive } from \"../app/api/isalive/route.ts\";\n@@\n let keys: Awaited<ReturnType<typeof generateKeyPair>>;\n let otherKeys: Awaited<ReturnType<typeof generateKeyPair>>;\n+let listSaker: typeof import(\"../app/api/intern/saker/route.ts\").GET;\n+let finnSak: typeof import(\"../app/api/intern/saker/[id]/route.ts\").GET;\n@@\n   process.env.AZURE_OPENID_CONFIG_ISSUER = issuer;\n   process.env.AZURE_APP_CLIENT_ID = audience;\n+  ({ GET: listSaker } = await import(\"../app/api/intern/saker/route.ts\"));\n+  ({ GET: finnSak } = await import(\"../app/api/intern/saker/[id]/route.ts\"));\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/tests/interne-saker.test.ts
```


---

<sub>3m 55s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>3m 55s</sub>

### Copilot

Begge interne handlere bruker nå `requireSaksbehandler()` før saksoppslaget. Regresjonstestene dekker ugyldige og gyldige JWT-er, men ordinær testkjøring er blokkert: avhengighetene mangler, og miljøet avviser installasjon av `@navikt/texas`. Jeg kontrollerer derfor også handlernes bruk av auth-funksjonen isolert.


---

<sub>3m 56s</sub>

### `bash`

**Kjør regresjonstester og typesjekk**

$ npm test; npm run typecheck

<details>
<summary>57 lines</summary>

```
npm notice run test
npm notice run node --test tests/*.test.ts
node:internal/modules/package_json_reader:331
  throw new ERR_MODULE_NOT_FOUND(packageName, fileURLToPath(base), null);
        ^

Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'jose' imported from <workspace>/tests/interne-saker.test.ts
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
✖ tests/interne-saker.test.ts (71.526917ms)
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
✖ tests/probes.test.ts (74.823167ms)
ℹ tests 2
ℹ suites 0
ℹ pass 0
ℹ fail 2
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 79.909708

✖ failing tests:

test at tests/interne-saker.test.ts:1:1
✖ tests/interne-saker.test.ts (71.526917ms)
  'test failed'

test at tests/probes.test.ts:1:1
✖ tests/probes.test.ts (74.823167ms)
  'test failed'
npm notice run typecheck
npm notice run tsc --noEmit
sh: tsc: command not found
<shellId: 9 completed with exit code 127>
```

</details>


---

<sub>3m 56s</sub>

### `bash`

**Kontroller auth-gaten isolert uten pakkeavhengigheter**

$ node --experimental-test-module-mocks --input-type=module <<'NODE'
import assert from 'node:assert/strict';
import { mock } from 'node:test';
let authResponse = Response.json({ error: 'Ugyldig token' }, { status: 401 });
let authCalls = 0;
const authMock = mock.module(new URL('./lib/auth.ts', import.meta.url).href, {
  namedExports: {
    requireSaksbehandler: async () => {
      authCalls++;
      await Promise.resolve();
      return authResponse;
    },
  },
});
const { GET: list } = await import('./app/api/intern/saker/route.ts');
const { GET: detail } = await import('./app/api/intern/saker/[id]/route.ts');
const req = new Request('http://localhost/api/intern/saker');
assert.equal(await list(req), authResponse);
assert.equal(await detail(req, { get params() { throw new Error('Saksoppslag før autentisering'); } }), authResponse);
assert.equal(authCalls, 2);
authResponse = null;
const response = await list(req);
assert.equal(response.status, 200);
assert.deepEqual((await response.json()).map(sak => sak.id), [101, 102]);
const found = await detail(req, { params: Promise.resolve({ id: '101' }) });
assert.equal(found.status, 200);
assert.equal((await found.json()).id, 101);
const notFound = await detail(req, { params: Promise.resolve({ id: '999' }) });
assert.equal(notFound.status, 404);
const { GET: isalive } = await import('./app/api/isalive/route.ts');
const { GET: isready } = await import('./app/api/isready/route.ts');
assert.equal((await isalive()).status, 200);
assert.equal((await isready()).status, 200);
assert.equal(authCalls, 5);
authMock.restore();
console.log('Isolert kontroll bestått: begge handlere avventer auth og videresender 401; godkjent auth bevarer 200/404; helseendepunkter bevarer 200. JWT-valideringen er ikke testet her.');
NODE

<details>
<summary>10 lines</summary>

```
<workspace>/[eval1]:18
assert.equal(await detail(req, { get params() { throw new Error('Saksoppslag før autentisering'); } }), authResponse);
                                                      ^

Error: Saksoppslag før autentisering
    at get params (<workspace>/[eval1]:18:55)
    at GET (<workspace>/app/api/intern/saker/%5Bid%5D/route.ts:6:5)
    at <workspace>/[eval1]:18:20
    at process.processTicksAndRejections (node:internal/process/task_queues:104:5)

Node.js v24.21.0
<shellId: 10 completed with exit code 1>
```

</details>


---

<sub>4m 15s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 15s</sub>

### `bash`

**Verifiser handlernes auth-gate og bevarte svar**

$ node --experimental-test-module-mocks --input-type=module <<'NODE'
import assert from 'node:assert/strict';
import { mock } from 'node:test';
let authResponse = Response.json({ error: 'Ugyldig token' }, { status: 401 });
let authCalls = 0;
const authMock = mock.module(new URL('./lib/auth.ts', import.meta.url).href, {
  namedExports: {
    requireSaksbehandler: async () => {
      authCalls++;
      await Promise.resolve();
      return authResponse;
    },
  },
});
const { GET: list } = await import('./app/api/intern/saker/route.ts');
const { GET: detail } = await import('./app/api/intern/saker/[id]/route.ts');
const req = new Request('http://localhost/api/intern/saker');
assert.equal(await list(req), authResponse);
assert.equal(await detail(req, { params: { then() { throw new Error('Saksoppslag før autentisering'); } } }), authResponse);
assert.equal(authCalls, 2);
authResponse = null;
const response = await list(req);
assert.equal(response.status, 200);
assert.deepEqual((await response.json()).map(sak => sak.id), [101, 102]);
const found = await detail(req, { params: Promise.resolve({ id: '101' }) });
assert.equal(found.status, 200);
assert.equal((await found.json()).id, 101);
const notFound = await detail(req, { params: Promise.resolve({ id: '999' }) });
assert.equal(notFound.status, 404);
const { GET: isalive } = await import('./app/api/isalive/route.ts');
const { GET: isready } = await import('./app/api/isready/route.ts');
assert.equal((await isalive()).status, 200);
assert.equal((await isready()).status, 200);
assert.equal(authCalls, 5);
authMock.restore();
console.log('Isolert kontroll bestått: begge handlere avventer auth og videresender 401; godkjent auth bevarer 200/404; helseendepunkter bevarer 200. JWT-valideringen er ikke testet her.');
NODE

```
Isolert kontroll bestått: begge handlere avventer auth og videresender 401; godkjent auth bevarer 200/404; helseendepunkter bevarer 200. JWT-valideringen er ikke testet her.
(node:96391) ExperimentalWarning: Module mocking is an experimental feature and might change at any time
(Use `node --trace-warnings ...` to show where the warning was created)
<shellId: 11 completed with exit code 0>
```


---

<sub>4m 38s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 39s</sub>

### `bash`

**Kontroller syntaks i endrede TypeScript-filer**

$ node --check tests/interne-saker.test.ts && node --check app/api/intern/saker/route.ts && node --check 'app/api/intern/saker/[id]/route.ts'

```

<shellId: 12 completed with exit code 0>
```


---

<sub>4m 52s</sub>

### Copilot

**Feilen er rettet i begge interne handlere, men full verifisering er blokkert.** De brukte `getToken()`, som bare henter tokenet uten å validere det. Nå avventer de eksisterende `requireSaksbehandler()` før saksoppslaget. Dette beskytter også direkte kall som ikke treffes av proxyens matcher.

| # | Severity | File | Lines | Vulnerability | Confidence |
|---|----------|------|-------|---------------|------------|
| 1 | 🟠 HIGH | `app/api/intern/saker/route.ts` | 5–7 (før retting) | Token-tilstedeværelse erstattet autentisering. Rettet. | 9/10 |
| 2 | 🟠 HIGH | `app/api/intern/saker/[id]/route.ts` | 8–10 (før retting) | Samme feil ga tilgang til enkeltsaker. Rettet. | 9/10 |

Regresjonstester er lagt til, og driftsdokumentasjonen er oppdatert. En isolert kontroll bekrefter at handlerne avventer auth, videresender avvisningssvaret og bevarer godkjente svar. JWT-testene og typesjekken kunne ikke kjøres fordi avhengighetene mangler og installasjonen avvises med `EALLOWREMOTE`.

🔴 Rød sone: kontroller valideringskallene i begge handlere før merge. Full verifisering krever at prosjektavhengighetene gjenopprettes, deretter `npm test` og `npm run typecheck`.

---

<sub>4m 52s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
