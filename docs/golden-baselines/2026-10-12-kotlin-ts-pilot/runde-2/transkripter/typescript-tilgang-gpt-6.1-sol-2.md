# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `1a7cbbf3-8028-4252-a6ad-77ac8aac03d0`  
> - **Started:** 10/12/2026, 11:24:14 PM  
> - **Duration:** 5m 1s  
> - **Exported:** 10/12/2026, 11:29:15 PM  

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

<sub>17s</sub>

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

**Read repository status and locate local instructions**

$ pwd; git --no-pager status --short -- .; find . -name node_modules -prune -o -name .git -prune -o \( -name AGENTS.md -o -name CLAUDE.md -o -name '*instructions*' \) -print; git --no-pager diff -- .; git --no-pager diff --staged -- .

<details>
<summary>22 lines</summary>

```
<workspace>
fatal: unable to get current working directory: Operation not permitted
./.github/instructions
./.github/instructions/deliberate-ai-use.instructions.md
./.github/instructions/performance.instructions.md
./.github/instructions/docker.instructions.md
./.github/instructions/accessibility.instructions.md
./.github/instructions/output-style.instructions.md
./.github/instructions/database.instructions.md
./.github/instructions/nextjs-aksel.instructions.md
./.github/instructions/github-actions.instructions.md
./.github/instructions/golang.instructions.md
./.github/instructions/testing-typescript.instructions.md
./.github/instructions/security-core.instructions.md
./.github/instructions/kotlin.instructions.md
./.github/instructions/security-owasp.instructions.md
./.github/instructions/testing-kotlin.instructions.md
./.github/instructions/testing.instructions.md
./.github/instructions/norwegian-text.instructions.md
fatal: unable to get current working directory: Operation not permitted
fatal: unable to get current working directory: Operation not permitted
<shellId: 0 completed with exit code 128>
```

</details>


### `bash`

**List endpoint, authentication, test and instruction files**

$ find <workspace>/.github -maxdepth 2 -type f; find <workspace>/app <workspace>/lib <workspace>/tests <workspace>/docs -type f

<details>
<summary>27 lines</summary>

```
<workspace>/.github/instructions/deliberate-ai-use.instructions.md
<workspace>/.github/instructions/performance.instructions.md
<workspace>/.github/instructions/docker.instructions.md
<workspace>/.github/instructions/accessibility.instructions.md
<workspace>/.github/instructions/output-style.instructions.md
<workspace>/.github/instructions/database.instructions.md
<workspace>/.github/instructions/nextjs-aksel.instructions.md
<workspace>/.github/instructions/github-actions.instructions.md
<workspace>/.github/instructions/golang.instructions.md
<workspace>/.github/instructions/testing-typescript.instructions.md
<workspace>/.github/instructions/security-core.instructions.md
<workspace>/.github/instructions/kotlin.instructions.md
<workspace>/.github/instructions/security-owasp.instructions.md
<workspace>/.github/instructions/testing-kotlin.instructions.md
<workspace>/.github/instructions/testing.instructions.md
<workspace>/.github/instructions/norwegian-text.instructions.md
<workspace>/app/api/saker/route.ts
<workspace>/app/api/intern/saker/route.ts
<workspace>/app/api/intern/saker/[id]/notater/route.ts
<workspace>/app/api/intern/saker/[id]/route.ts
<workspace>/app/api/isalive/route.ts
<workspace>/app/api/isready/route.ts
<workspace>/lib/saker.ts
<workspace>/lib/auth.ts
<workspace>/tests/probes.test.ts
<workspace>/docs/drift.md
<shellId: 1 completed with exit code 0>
```

</details>


### `bash`

**Read applicable local review and TypeScript instructions**

$ for f in .github/instructions/security-core.instructions.md .github/instructions/security-owasp.instructions.md .github/instructions/output-style.instructions.md .github/instructions/nextjs-aksel.instructions.md .github/instructions/testing.instructions.md .github/instructions/testing-typescript.instructions.md .github/instructions/norwegian-text.instructions.md; do printf '\n--- %s ---\n' "$f"; cat "$f"; done

<details>
<summary>13 lines</summary>

```
Output too large to read at once (29.5 KB). Saved to: ~/Library/Caches/cplt/tmp/26d098bc9eef710b50fa7e35b415a1f8/1791840289240-copilot-tool-output-99537-72efd1e1-ad5f-4323-b594-617a6021259e.txt
Consider using tools like rg (for searching), head/tail (for viewing start/end), view with view_range (for specific sections), or jq (for JSON) to examine portions of the output.

Preview (first 500 chars):

--- .github/instructions/security-core.instructions.md ---
---
applyTo: "**"
---

# Security Core

These rules apply to every change, in every language. They are never optional.

- **No secrets or personal data in logs.** Never log tokens, passwords, keys, fødselsnummer (FNR), name or address.
- **Secrets come from the environment.** Read tokens, passwords and keys from environment variables or the secret store. Never hardcode them.
- **Parameterized queries only.** Never concatenate user input
<shellId: 2 completed with exit code 0>
```

</details>


### `bash`

**Read endpoint authentication chain and current tests**

$ for f in proxy.ts lib/auth.ts app/api/intern/saker/route.ts lib/saker.ts package.json tests/probes.test.ts docs/drift.md tsconfig.json; do printf '\n--- %s ---\n' "$f"; nl -ba "$f"; done

<details>
<summary>139 lines</summary>

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
    20	
    21	export function leggTilNotat(id: number, tekst: string): InternSak | undefined {
    22	  const sak = finnInternSak(id);
    23	  if (sak) sak.internNotat = `${sak.internNotat}\n${tekst}`;
    24	  return sak;
    25	}

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
<shellId: 3 completed with exit code 0>
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


### `bash`

**Read local security and test instructions**

$ for f in .github/instructions/security-core.instructions.md .github/instructions/output-style.instructions.md .github/instructions/testing.instructions.md .github/instructions/testing-typescript.instructions.md; do printf '\n--- %s ---\n' "$f"; cat "$f"; done

<details>
<summary>234 lines</summary>

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
<shellId: 4 completed with exit code 0>
````

</details>


### `bash`

**Read remaining applicable local instructions**

$ for f in .github/instructions/security-owasp.instructions.md .github/instructions/nextjs-aksel.instructions.md .github/instructions/norwegian-text.instructions.md; do printf '\n--- %s ---\n' "$f"; cat "$f"; done

<details>
<summary>508 lines</summary>

````
--- .github/instructions/security-owasp.instructions.md ---
---
description: "Hvor OWASP Top 10:2025-mønstrene for Kotlin, Go, Java og TypeScript ligger: tilgangskontroll, feilkonfigurasjon, forsyningskjede, kryptografi, injeksjon, logging og feilhåndtering."
applyTo: "**/*.{kt,go,java,ts,tsx}"
---

# Security Essentials

The rules that always apply (logging, secrets, queries, ownership, `azp`, TLS) are in `security-core.instructions.md`, which is loaded in every session. If it is not installed next to this file, tell the developer to install it: https://github.com/navikt/copilot/blob/main/instructions/security-core.instructions.md

For detailed OWASP Top 10:2025 code-level patterns (Kotlin, Go, Java, Node.js), invoke the `$security-owasp` skill.
For scanning workflows (trivy, zizmor, govulncheck), use the `$security-review` skill.
For architecture-level threat modeling, use `@security-champion`.

--- .github/instructions/nextjs-aksel.instructions.md ---
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

// Page container
<main className="max-w-7xl mx-auto">
  <Box
    paddingBlock={{ xs: "space-16", md: "space-24" }}
    paddingInline={{ xs: "space-16", md: "space-40" }}
  >
    {children}
  </Box>
</main>

// Komponent med responsiv padding
<Box
  background="neutral-soft"
  padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
  borderRadius="8"
>
  <Heading size="large" level="2">Tittel</Heading>
  <BodyShort>Innhold</BodyShort>
</Box>

// Retningsbestemt padding
<Box
  paddingBlock="space-16"    // Topp og bunn
  paddingInline="space-24"   // Venstre og høyre
>
```

### ❌ Feil

```tsx
// Aldri bruk Tailwind padding/margin
<div className="p-4 md:p-6">  // ❌ Feil
<div className="mx-4 my-2">   // ❌ Feil
<Box padding="4">             // ❌ Feil — mangler space-prefiks
```

## Spacing-tokens

Token name = eksakt pikselverdi (`space-16` = 16px, `space-4` = 4px). Alltid `space-`-prefiks.

Fullstendig liste: [design-tokens](https://aksel.nav.no/grunnleggende/styling/design-tokens.md)

## Responsiv design

Mobil-først. Fullstendige breakpoints: [brekkpunkter](https://aksel.nav.no/grunnleggende/styling/brekkpunkter.md)

```tsx
<HGrid columns={{ xs: 1, md: 2, lg: 3 }} gap="space-16">
  {items.map(item => <Card key={item.id} {...item} />)}
</HGrid>

<Box
  padding={{ xs: "space-16", sm: "space-20", md: "space-24" }}
>
```

## Komponentmønstre

### Layout-komponenter

```tsx
import { Box, VStack, HStack, HGrid } from "@navikt/ds-react";

// Vertikal stack med mellomrom
<VStack gap="space-16">
  <Komponent1 />
  <Komponent2 />
  <Komponent3 />
</VStack>

// Horisontal stack
<HStack gap="space-8" align="center">
  <Icon />
  <Text />
</HStack>

// Responsiv grid
<HGrid columns={{ xs: 1, md: 2, lg: 3 }} gap="space-16">
  {/* Grid-elementer */}
</HGrid>
```

### Typografi

Full API: [typography](https://aksel.nav.no/komponenter/core/typography.md)

```tsx
import { Heading, BodyShort, Label } from "@navikt/ds-react";

<Heading size="xlarge|large|medium|small|xsmall" level="1-6">
  Tittel
</Heading>

<BodyShort size="large|medium|small">
  Vanlig tekstinnhold
</BodyShort>

<BodyShort weight="semibold">
  Halvfet tekst
</BodyShort>

<Label size="medium|small">
  Skjemaetikett
</Label>
```

### Bakgrunnsfarger

```tsx
<Box background="default">          {/* Hvit */}
<Box background="neutral-soft">     {/* Lys grå */}
<Box background="accent-soft">      {/* Lys blå */}
<Box background="success-soft">     {/* Lys grønn */}
<Box background="warning-soft">     {/* Lys oransje */}
<Box background="danger-soft">      {/* Lys rød */}
```

## Tallformatering

Bruk alltid norsk locale for tallformatering:

```typescript
import { formatNumber } from "@/lib/format";

// ✅ Correct
const formatted = formatNumber(151354); // "151 354"

// ❌ Wrong
const formatted = num.toLocaleString(); // Uses browser locale
```

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
  }

  const result = await processData(body);
  return NextResponse.json(result, { status: 201 });
}
```

## Async Request APIs (Next.js 15+)

In Next.js 15+, `cookies()`, `headers()`, and route `params` are **async** and require `await`:

```typescript
import { cookies, headers } from "next/headers";

export default async function Page() {
  // ✅ Next.js 15+: must await
  const cookieStore = await cookies();
  const headersList = await headers();

  const token = cookieStore.get("auth-token")?.value;
  const userAgent = headersList.get("user-agent");
}

// ✅ Route params are also Promises
export async function GET(
  request: Request,
  props: { params: Promise<{ id: string }> },
) {
  const { id } = await props.params;
  // ...
}

// ❌ Next.js 14 pattern — no longer works
const cookieStore = cookies(); // Error: must await
```

## Metadata API

```typescript
import { Metadata } from "next";

// ✅ Dynamic metadata for SEO
export async function generateMetadata(props: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await props.params;
  const vedtak = await fetchVedtak(id);

  return {
    title: `${vedtak.title} | Nav`,
    description: vedtak.summary,
    openGraph: { title: vedtak.title, description: vedtak.summary },
  };
}
```

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

## Testing

```typescript
import { formatNumber } from "./format";

describe("formatNumber", () => {
  it("should format numbers with Norwegian locale", () => {
    expect(formatNumber(151354)).toBe("151 354");
  });

  it("should handle decimal numbers", () => {
    expect(formatNumber(1234.56)).toBe("1 234,56");
  });
});
```

## Server Components (Next.js 16)

```tsx
// Server Component (default in App Router)
export default async function Page() {
  const data = await fetchData(); // Can use async/await

  return (
    <Box padding="space-24">
      <Heading size="large" level="1">
        {data.title}
      </Heading>
      <BodyShort>{data.description}</BodyShort>
    </Box>
  );
}
```

## Client Components

```tsx
"use client";

import { useState } from "react";
import { Button } from "@navikt/ds-react";

export function InteractiveComponent() {
  const [count, setCount] = useState(0);

  return <Button onClick={() => setCount(count + 1)}>Count: {count}</Button>;
}
```

## Loading and Error States

```tsx
// src/app/oversikt/loading.tsx — shown during server component loading
import { Loader, Box } from "@navikt/ds-react";

export default function Loading() {
  return (
    <Box padding="space-24" className="flex justify-center">
      <Loader size="xlarge" title="Laster data..." />
    </Box>
  );
}

// src/app/oversikt/error.tsx — shown on uncaught errors
"use client";

import { LocalAlert, Button, VStack } from "@navikt/ds-react";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <VStack gap="space-16">
      <LocalAlert status="error">
        <LocalAlert.Header>
          <LocalAlert.Title>Noe gikk galt</LocalAlert.Title>
        </LocalAlert.Header>
        <LocalAlert.Content>
          Det oppsto en feil under innlastingen av data. Prøv igjen senere. Hvis
          problemet vedvarer, kontakt support.
        </LocalAlert.Content>
      </LocalAlert>

      <Button variant="secondary" onClick={reset}>
        Prøv igjen
      </Button>
    </VStack>
  );
}
```

## Streaming with Suspense

```tsx
import { Suspense } from "react";
import { Skeleton } from "@navikt/ds-react";

export default function Page() {
  return (
    <VStack gap="space-24">
      <Heading size="large" level="1">
        Oversikt
      </Heading>
      <Suspense fallback={<Skeleton variant="rounded" height={200} />}>
        <SlowDataComponent />
      </Suspense>
    </VStack>
  );
}

// This component can load independently
async function SlowDataComponent() {
  const data = await fetchSlowData(); // streams in when ready
  return <DataTable data={data} />;
}
```

## Server Actions

```tsx
// src/app/vedtak/actions.ts
"use server";

import { revalidatePath } from "next/cache";
import { getUser } from "@/lib/auth";

export async function createVedtak(formData: FormData) {
  const user = await getUser();

  const tittel = formData.get("tittel") as string;
  if (!tittel) {
    return { error: "Tittel er påkrevd" };
  }

  await saveToDatabase({ tittel, opprettetAv: user.name });
  revalidatePath("/vedtak");
  return { success: true };
}
```

```tsx
// src/app/vedtak/page.tsx
"use client";

import { useActionState } from "react";
import { createVedtak } from "./actions";

export default function VedtakForm() {
  const [state, action, isPending] = useActionState(createVedtak, null);

  return (
    <form action={action}>
      <TextField name="tittel" label="Tittel" error={state?.error} />
      <Button type="submit" loading={isPending}>
        Opprett
      </Button>
    </form>
  );
}
```

## When Using React Query (Server State)

React Query (@tanstack/react-query) is the standard for server state management at Nav, replacing Redux/Context for API data.

```tsx
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader, LocalAlert, VStack } from "@navikt/ds-react";

export function ResourceList() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["resources"],
    queryFn: () => fetch("/api/resources").then((res) => res.json()),
  });

  if (isLoading) return <Loader title="Laster..." />;
  if (error)
    return (
      <LocalAlert status="error">
        <LocalAlert.Header>
          <LocalAlert.Title>Kunne ikke laste data</LocalAlert.Title>
        </LocalAlert.Header>
        <LocalAlert.Content>
          Det oppsto en feil under innlastingen av data. Prøv igjen senere. Hvis
          problemet vedvarer, kontakt support.
        </LocalAlert.Content>
      </LocalAlert>
    );

  return (
    <VStack gap="space-16">
      {data.map((resource) => (
        <ResourceCard key={resource.id} resource={resource} />
      ))}
    </VStack>
  );
}
```

```tsx
// ✅ Mutation with cache invalidation
const queryClient = useQueryClient();
const mutation = useMutation({
  mutationFn: (data: CreateRequest) =>
    fetch("/api/resources", { method: "POST", body: JSON.stringify(data) }),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ["resources"] }),
});
```

Do not introduce React Query into projects that don't already use it — Server Components with `fetch` may be sufficient.

## When Using React Hook Form (Form State)

React Hook Form is preferred for complex forms with validation.

```tsx
"use client";

import { useForm } from "react-hook-form";
import { TextField, Button, VStack } from "@navikt/ds-react";

interface FormData {
  name: string;
  email: string;
}

export function RegistrationForm() {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormData>();

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <VStack gap="space-16">
        <TextField
          label="Navn"
          {...register("name", { required: "Navn er påkrevd" })}
          error={errors.name?.message}
        />
        <TextField
          label="E-post"
          type="email"
          {...register("email", { required: "E-post er påkrevd" })}
          error={errors.email?.message}
        />
        <Button type="submit">Registrer</Button>
      </VStack>
    </form>
  );
}
```

For simple forms, Server Actions with `useActionState` (shown above) may be simpler.

## Package Manager

**pnpm** is the standard package manager for new Nav frontend projects.

```bash
# ✅ Use pnpm
pnpm install
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
- Skip responsive props
- Add code comments unless explicitly requested

## Related

| Resource                   | Use For                                                          |
| -------------------------- | ---------------------------------------------------------------- |
| `@aksel-agent`             | Aksel Design System component patterns and spacing tokens        |
| `@accessibility-agent`     | WCAG 2.1/2.2 compliance and accessibility testing                |
| `performance` instruction  | Core Web Vitals and bundle optimization                          |
| `aksel-builder` skill      | Aksel designsystem and ui-library reference                      |
| `playwright-testing` skill | E2E testing with Playwright and axe-core                         |
| aksel.nav.no/llm.md        | Full Aksel documentation index (components, tokens, foundations) |

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
<shellId: 5 completed with exit code 0>
````

</details>


### `bash`

**Locate installed token implementation and baseline handler**

$ printf '\n--- comparable handler ---\n'; nl -ba app/api/saker/route.ts; printf '\n--- auth package ---\n'; find node_modules/@navikt/oasis -maxdepth 3 -type f; printf '\n--- runtime ---\n'; node --version; printf '\n--- top-level files ---\n'; ls -la

<details>
<summary>32 lines</summary>

```
--- comparable handler ---
     1	import { requireSaksbehandler } from "../../../lib/auth.ts";
     2	import { listSaker } from "../../../lib/saker.ts";
     3	
     4	export async function GET(request: Request): Promise<Response> {
     5	  const unauthorized = await requireSaksbehandler(request);
     6	  if (unauthorized) return unauthorized;
     7	  return Response.json(listSaker());
     8	}

--- auth package ---
node_modules/@navikt/oasis/dist/index.d.mts
node_modules/@navikt/oasis/dist/index.d.mts.map
node_modules/@navikt/oasis/dist/index.mjs.map
node_modules/@navikt/oasis/dist/index.mjs
node_modules/@navikt/oasis/package.json

--- runtime ---
v24.21.0

--- top-level files ---
total 96
drwxr-xr-x  12 user  staff    384 Oct 12 23:24 .
drwxr-xr-x   7 user  staff    224 Oct 12 23:24 ..
drwxr-xr-x   3 user  staff     96 Oct 12 23:24 .github
drwxr-xr-x   3 user  staff     96 Oct 12 11:47 app
drwxr-xr-x   3 user  staff     96 Oct 12 11:47 docs
drwxr-xr-x   4 user  staff    128 Oct 12 18:11 lib
drwxr-xr-x  33 user  staff   1056 Oct 12 23:24 node_modules
-rw-r--r--   1 user  staff  36596 Oct 12 11:48 package-lock.json
-rw-r--r--   1 user  staff    519 Oct 12 11:47 package.json
-rw-r--r--   1 user  staff    534 Oct 12 11:47 proxy.ts
drwxr-xr-x   3 user  staff     96 Oct 12 11:47 tests
-rw-r--r--   1 user  staff    424 Oct 12 11:47 tsconfig.json
<shellId: 6 completed with exit code 0>
```

</details>


### `bash`

**Read local review constraints and actual Oasis implementation**

$ cat .github/instructions/deliberate-ai-use.instructions.md; printf '\n--- installed Oasis ---\n'; nl -ba node_modules/@navikt/oasis/dist/index.mjs; cat node_modules/@navikt/oasis/package.json

<details>
<summary>9 lines</summary>

```
Output too large to read at once (23.1 KB). Saved to: ~/Library/Caches/cplt/tmp/26d098bc9eef710b50fa7e35b415a1f8/1791840309692-copilot-tool-output-99537-7a6adf3d-c625-4f6d-925e-11ebdf44f437.txt
Consider using tools like rg (for searching), head/tail (for viewing start/end), view with view_range (for specific sections), or jq (for JSON) to examine portions of the output.

Preview (first 500 chars):
---
applyTo: "**"
---

# Deliberate AI Use

Nav's framework for using AI in a way that builds developer skill instead of eroding it. The rules below apply in every session. The full framework (research, what belongs in each zone, the three-attempt rule, experience level, generate-then-understand, follow-up questions) is in the `$deliberate-ai-use` skill: load it when the developer asks about AI use, learning or which zone a task belongs to. The zone definitions below are enough for declaring a z
<shellId: 7 completed with exit code 0>
```

</details>


### `bash`

**Trace authorization guards and locate local framework documentation**

$ printf '\n--- local Next auth documentation ---\n'; find node_modules/next/dist/docs -type f -iname '*proxy*' -o -iname '*auth*'; printf '\n--- request auth references ---\n'; rg -n 'requireSaksbehandler|getToken|validateToken|Authorization|azure|roles|groups|matcher' app lib proxy.ts docs tests; printf '\n--- Next package ---\n'; cat node_modules/next/package.json

<details>
<summary>174 lines</summary>

```
--- local Next auth documentation ---
node_modules/next/dist/docs/02-pages/02-guides/authentication.md
node_modules/next/dist/docs/02-pages/04-api-reference/04-config/01-next-config-js/skipProxyUrlNormalize.md
node_modules/next/dist/docs/02-pages/04-api-reference/04-config/01-next-config-js/proxyClientMaxBodySize.md
node_modules/next/dist/docs/02-pages/04-api-reference/02-file-conventions/proxy.md
node_modules/next/dist/docs/01-app/01-getting-started/16-proxy.md
node_modules/next/dist/docs/01-app/02-guides/authentication.md
node_modules/next/dist/docs/01-app/02-guides/authentication-with-cache-components.md
node_modules/next/dist/docs/01-app/03-api-reference/04-functions/unauthorized.md
node_modules/next/dist/docs/01-app/03-api-reference/05-config/01-next-config-js/skipProxyUrlNormalize.md
node_modules/next/dist/docs/01-app/03-api-reference/05-config/01-next-config-js/proxyClientMaxBodySize.md
node_modules/next/dist/docs/01-app/03-api-reference/05-config/01-next-config-js/authInterrupts.md
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/unauthorized.md
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md

--- request auth references ---
proxy.ts:1:import { getToken } from "@navikt/oasis";
proxy.ts:5:// Selve valideringen av tokenet gjør hver route handler med requireSaksbehandler().
proxy.ts:7:  if (!getToken(request)) {
proxy.ts:14:  matcher: ["/api/saker/:path*"],
lib/auth.ts:1:import { getToken, validateToken } from "@navikt/oasis";
lib/auth.ts:7:export async function requireSaksbehandler(request: Request): Promise<Response | null> {
lib/auth.ts:8:  const token = getToken(request);
lib/auth.ts:12:  const validation = await validateToken(token);
docs/drift.md:4:saksbehandlerens token i `Authorization`-headeren.
docs/drift.md:9:  med `requireSaksbehandler()` i `lib/auth.ts`.
app/api/saker/route.ts:1:import { requireSaksbehandler } from "../../../lib/auth.ts";
app/api/saker/route.ts:5:  const unauthorized = await requireSaksbehandler(request);
app/api/intern/saker/[id]/route.ts:1:import { getToken } from "@navikt/oasis";
app/api/intern/saker/[id]/route.ts:8:  if (!getToken(request)) {
app/api/intern/saker/route.ts:1:import { getToken } from "@navikt/oasis";
app/api/intern/saker/route.ts:5:  if (!getToken(request)) {
app/api/intern/saker/[id]/notater/route.ts:1:import { getToken } from "@navikt/oasis";
app/api/intern/saker/[id]/notater/route.ts:8:  if (!getToken(request)) {

--- Next package ---
{
  "name": "next",
  "version": "16.3.8",
  "description": "The React Framework",
  "main": "./dist/server/next.js",
  "license": "MIT",
  "repository": "vercel/next.js",
  "bugs": "https://github.com/vercel/next.js/issues",
  "homepage": "https://nextjs.org",
  "types": "index.d.ts",
  "files": [
    "AGENTS.md",
    "dist",
    "app.js",
    "app.d.ts",
    "babel.js",
    "babel.d.ts",
    "client.js",
    "client.d.ts",
    "compat",
    "cache.js",
    "cache.d.ts",
    "constants.js",
    "constants.d.ts",
    "document.js",
    "document.d.ts",
    "dynamic.js",
    "dynamic.d.ts",
    "error.js",
    "error.d.ts",
    "future",
    "legacy",
    "script.js",
    "script.d.ts",
    "server.js",
    "server.d.ts",
    "head.js",
    "head.d.ts",
    "image.js",
    "image.d.ts",
    "link.js",
    "link.d.ts",
    "offline.js",
    "offline.d.ts",
    "form.js",
    "form.d.ts",
    "router.js",
    "router.d.ts",
    "jest.js",
    "jest.d.ts",
    "og.js",
    "og.d.ts",
    "root-params.js",
    "root-params.d.ts",
    "types.d.ts",
    "types.js",
    "index.d.ts",
    "types/global.d.ts",
    "types/compiled.d.ts",
    "image-types/global.d.ts",
    "navigation-types/navigation.d.ts",
    "navigation-types/compat/navigation.d.ts",
    "font",
    "navigation.js",
    "navigation.d.ts",
    "headers.js",
    "headers.d.ts",
    "navigation-types",
    "web-vitals.js",
    "web-vitals.d.ts",
    "experimental/testing/server.js",
    "experimental/testing/server.d.ts",
    "experimental/testmode/playwright.js",
    "experimental/testmode/playwright.d.ts",
    "experimental/testmode/playwright/msw.js",
    "experimental/testmode/playwright/msw.d.ts",
    "experimental/testmode/proxy.js",
    "experimental/testmode/proxy.d.ts"
  ],
  "bin": {
    "next": "./dist/bin/next"
  },
  "dependencies": {
    "@next/env": "16.3.8",
    "@swc/helpers": "0.5.23",
    "baseline-browser-mapping": "^2.9.19",
    "caniuse-lite": "^1.0.30001579",
    "postcss": "8.5.23",
    "styled-jsx": "5.1.6"
  },
  "peerDependencies": {
    "@opentelemetry/api": "^1.1.0",
    "@playwright/test": "^1.51.1",
    "babel-plugin-react-compiler": "*",
    "react": "^18.2.0 || 19.0.0-rc-de68d2f4-20241204 || ^19.0.0",
    "react-dom": "^18.2.0 || 19.0.0-rc-de68d2f4-20241204 || ^19.0.0",
    "sass": "^1.3.0"
  },
  "peerDependenciesMeta": {
    "babel-plugin-react-compiler": {
      "optional": true
    },
    "sass": {
      "optional": true
    },
    "@opentelemetry/api": {
      "optional": true
    },
    "@playwright/test": {
      "optional": true
    }
  },
  "optionalDependencies": {
    "sharp": "^0.35.4",
    "@next/swc-darwin-arm64": "16.3.8",
    "@next/swc-darwin-x64": "16.3.8",
    "@next/swc-linux-arm64-gnu": "16.3.8",
    "@next/swc-linux-arm64-musl": "16.3.8",
    "@next/swc-linux-x64-gnu": "16.3.8",
    "@next/swc-linux-x64-musl": "16.3.8",
    "@next/swc-win32-arm64-msvc": "16.3.8",
    "@next/swc-win32-x64-msvc": "16.3.8"
  },
  "keywords": [
    "react",
    "framework",
    "nextjs",
    "web",
    "server",
    "node",
    "front-end",
    "backend",
    "cli",
    "vercel"
  ],
  "engines": {
    "node": ">=20.9.0"
  }
}
<shellId: 8 completed with exit code 0>
```

</details>


### `bash`

**Read remaining local instructions and locate validator guards**

$ cat .github/instructions/deliberate-ai-use.instructions.md; printf '\n--- Oasis auth symbols ---\n'; rg -n 'function getToken|function validateToken|function validateAzureToken|function getAzure|jwtVerify|issuer|audience|isLocal|LOCAL|export' node_modules/@navikt/oasis/dist/index.mjs

<details>
<summary>55 lines</summary>

```
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

- Whether the developer wants full delegation or guided learning. For red-zone items the default is test skeletons and stubs; implement them only when the developer explicitly asks, in any language, and then end the reply that delivers the code with the heading «**Sjekkliste for rød sone**», one `- [ ]` line per implemented item, and the explain-back question. A promise to review on its own («jeg reviewer», «I'll review everything») is not enough, and a request that names some items covers only those
- With unfamiliar technology: whether they want to learn the concepts first

### 🚫 Never

- Generate code without explaining the architectural choices
- Encourage blind copy-paste of generated code
- Skip error handling or security patterns in examples

--- Oasis auth symbols ---
5:import { createRemoteJWKSet, errors, jwtVerify } from "jose";
160:	return async (provider, scopeOrToken, audienceOrNothing) => {
162:		const key = sha256(scopeOrToken + audienceOrNothing);
169:		return oboProvider(provider, scopeOrToken, audienceOrNothing).then((result) => {
202:	TokenVerifyIssuer: "token.verify.issuer",
203:	TokenVerifyAudience: "token.verify.audience",
286:	return async (provider, token, audience) => {
288:		const oboToken = await oboProvider(provider, token, audience);
302:* @param audience The target app you request a token for (scope).
304:const requestAzureOboToken = async (token, audience) => withCache(withPrometheus((provider, token, target) => grantOboToken(provider, token, target)))("azuread", token, audience);
310:* @param audience The target app you request a token for.
312:const requestTokenxOboToken = async (token, audience) => withCache(withPrometheus((provider, token, audience) => grantOboToken(provider, token, audience)))("tokenx", token, audience);
318:* @param audience The target app you request a token for.
320:const requestOboToken = async (token, audience) => {
322:	if (!audience) return TokenResult.Error("empty audience");
326:	else if (tokenx) return requestTokenxOboToken(token, audience);
327:	else if (azure) return requestAzureOboToken(token, audience);
332:function getToken(val) {
341:function getTokenFromHeaders(headers) {
408:		issuer: process.env.IDPORTEN_ISSUER,
409:		audience: process.env.IDPORTEN_AUDIENCE
412:function getAzureEnvs() {
416:		issuer: process.env.AZURE_OPENID_CONFIG_ISSUER,
417:		audience: process.env.AZURE_APP_CLIENT_ID
420:function getTokenXEnvs() {
424:		issuer: process.env.TOKEN_X_ISSUER,
425:		audience: process.env.TOKEN_X_CLIENT_ID
451:const validateJwt = async ({ token, jwksUri, issuer, audience }) => {
454:			[OtelTaxonomy.TokenVerifyIssuer]: issuer,
455:			[OtelTaxonomy.TokenVerifyAudience]: audience
458:			const { payload } = await jwtVerify(stripBearer(token), getJwkSet(jwksUri), {
459:				issuer,
460:				audience,
519:export { TokenResult, ValidationResult, expiresIn, getToken, parseAzureUserToken, parseIdportenToken, requestAzureClientCredentialsToken, requestAzureOboToken, requestOboToken, requestTokenxOboToken, validateAzureToken, validateIdportenToken, validateToken, validateTokenxToken };
<shellId: 9 completed with exit code 0>
```

</details>


### `bash`

**Read relevant proxy guidance and run current tests**

$ printf '\n--- proxy documentation ---\n'; rg -n 'matcher|authentication|authorization|Route Handlers|security|unstable_doesProxyMatch' node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md node_modules/next/dist/docs/01-app/02-guides/authentication.md; printf '\n--- test helper exports ---\n'; cat node_modules/next/experimental/testing/server.js; printf '\n--- current tests ---\n'; node --test tests/probes.test.ts

<details>
<summary>93 lines</summary>

```
--- proxy documentation ---
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:15:Proxy executes before routes are rendered. It's particularly useful for implementing custom server-side logic like authentication, logging, or handling redirects.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:37:  matcher: '/about/:path*',
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:50:  matcher: '/about/:path*',
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:69:Optionally, a config object can be exported alongside the Proxy function. This object includes the [matcher](#matcher) to specify paths where the Proxy applies.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:73:The `matcher` option allows you to target specific paths for the Proxy to run on.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:75:Without a `matcher`, Proxy runs on **every request**, including static files (`_next/static`), image optimizations (`_next/image`), and assets in the `public/` folder. Consider using a [negative match pattern](#negative-matching) to exclude these paths, otherwise auth logic or redirects can unintentionally block CSS, JS, or images from loading.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:80:- For multiple paths: Use an array to list multiple paths, such as `matcher: ['/about', '/contact']`, which applies the Proxy to both `/about` and `/contact`.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:84:  matcher: ['/about/:path*', '/dashboard/:path*'],
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:88:Additionally, the `matcher` option supports complex path specifications using regular expressions. For example, you can exclude certain paths with a regular expression matcher:
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:92:  matcher: [
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:101:The `matcher` option accepts an array of objects with the following keys:
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:110:  matcher: [
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:136:> - The `matcher` values need to be constants so they can be statically analyzed at build-time. Dynamic values such as variables will be ignored.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:137:> - For backward compatibility, Next.js always considers `/public` as `/public/index`. Therefore, a matcher of `/public/:path` will match.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:238:Proxy will be invoked for **every route in your project**. Given this, it's crucial to use [matchers](#matcher) to precisely target or exclude specific routes. The following is the execution order:
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:249:> **Good to know:** [Server Functions](/docs/app/api-reference/directives/use-server) are not separate routes in this chain. They are handled as POST requests to the route where they are used, so a Proxy matcher that excludes a path will also skip Server Function calls on that path.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:251:> A matcher change or a refactor that moves a Server Function to a different route can silently remove Proxy coverage. Always verify authentication and authorization inside each Server Function rather than relying on Proxy alone. See the [Data Security guide](/docs/app/guides/data-security#authentication-and-authorization) for recommended patterns.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:534:  matcher: '/api/:path*',
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:579:  matcher: '/api/:path*',
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:585:> **Good to know:** You can configure CORS headers for individual routes in [Route Handlers](/docs/app/api-reference/file-conventions/route#cors).
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:599:  matcher: '/api/:function*',
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:603:  // Call our authentication function to check the request
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:607:      { success: false, message: 'authentication failed' },
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:619:  matcher: '/api/:function*',
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:623:  // Call our authentication function to check the request
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:627:      { success: false, message: 'authentication failed' },
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:636:The `matcher` config allows full regex so matching like negative lookaheads or character matching is supported. An example of a negative lookahead to match all except specific paths can be seen here:
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:640:  matcher: [
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:657:  matcher: [
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:695:> Even when `_next/data` is excluded in a negative matcher pattern, proxy will still be invoked for `_next/data` routes. This is intentional behavior to prevent accidental security issues where you might protect a page but forget to protect the corresponding data route.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:699:  matcher:
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:732:The `unstable_doesProxyMatch` function can be used to assert whether proxy will run for the provided URL, headers, and cookies.
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:735:import { unstable_doesProxyMatch } from 'next/experimental/testing/server'
node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md:738:  unstable_doesProxyMatch({
node_modules/next/dist/docs/01-app/02-guides/authentication.md:2:title: How to implement authentication in Next.js
node_modules/next/dist/docs/01-app/02-guides/authentication.md:4:description: Learn how to implement authentication in your Next.js application.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:7:Understanding authentication is crucial for protecting your application's data. This page will guide you through what React and Next.js features to use to implement auth.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:11:1. **[Authentication](#authentication)**: Verifies if the user is who they say they are. It requires the user to prove their identity with something they have, such as a username and password.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:13:3. **[Authorization](#authorization)**: Decides what routes and data the user can access.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:15:This diagram shows the authentication flow using React and Next.js features:
node_modules/next/dist/docs/01-app/02-guides/authentication.md:18:  alt="Diagram showing the authentication flow with React and Next.js features"
node_modules/next/dist/docs/01-app/02-guides/authentication.md:19:  srcLight="/docs/light/authentication-overview-new.png"
node_modules/next/dist/docs/01-app/02-guides/authentication.md:20:  srcDark="/docs/dark/authentication-overview-new.png"
node_modules/next/dist/docs/01-app/02-guides/authentication.md:25:The examples on this page walk through basic username and password auth for educational purposes. While you can implement a custom auth solution, for increased security and simplicity, we recommend using an authentication library. These offer built-in solutions for authentication, session management, and authorization, as well as additional features such as social logins, multi-factor authentication, and role-based access control. You can find a list in the [Auth Libraries](#auth-libraries) section.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:27:> **Good to know:** With [Cache Components](/docs/app/api-reference/config/next-config-js/cacheComponents) enabled, reading the session and caching per-user data follow specific rules. See [Authentication with Cache Components](/docs/app/guides/authentication-with-cache-components).
node_modules/next/dist/docs/01-app/02-guides/authentication.md:37:Since Server Actions always execute on the server, they provide a secure environment for handling authentication logic.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:103:Use the Server Action to validate the form fields on the server. If your authentication provider doesn't provide form validation, you can use a schema validation library like [Zod](https://zod.dev/), [Valibot](https://valibot.dev/) or [Yup](https://github.com/jquense/yup).
node_modules/next/dist/docs/01-app/02-guides/authentication.md:160:To prevent unnecessary calls to your authentication provider's API or database, you can `return` early in the Server Action if any form fields do not match the defined schema.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:301:> - Before mutating data, you should always ensure a user is also authorized to perform the action. See [Authentication and Authorization](#authorization).
node_modules/next/dist/docs/01-app/02-guides/authentication.md:305:After validating form fields, you can create a new user account or check if the user exists by calling your authentication provider's API or database.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:381:> - The example above is verbose since it breaks down the authentication steps for the purpose of education. This highlights that implementing your own secure solution can quickly become complex. Consider using an [Auth Library](#auth-libraries) to simplify the process.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:392:3. Upon successful verification, the process is completed, indicating the user's successful authentication.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:473:You can then call your Authentication Provider's API in the API route to handle authentication:
node_modules/next/dist/docs/01-app/02-guides/authentication.md:716:> - 🎥 Watch: Learn more about stateless sessions and authentication with Next.js → [YouTube (11 minutes)](https://www.youtube.com/watch?v=DJvM2lSPn6w).
node_modules/next/dist/docs/01-app/02-guides/authentication.md:957:After implementing session management, you'll need to add authorization logic to control what users can access and do within your application. Continue to the [Authorization](#authorization) section to learn more.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1013:Once a user is authenticated and a session is created, you can implement authorization to control what the user can access and do within your application.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1015:There are two main types of authorization checks:
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1022:- Creating a [Data Access Layer](#creating-a-data-access-layer-dal) to centralize your authorization logic
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1075:  matcher: ['/((?!api|_next/static|_next/image|.*\\.png$).*)'],
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1117:  matcher: ['/((?!api|_next/static|_next/image|.*\\.png$).*)'],
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1121:While Proxy can be useful for initial checks, it should not be your only line of defense in protecting your data. The majority of security checks should be performed as close as possible to your data source, see [Data Access Layer](#creating-a-data-access-layer-dal) for more information.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1127:> - You can use the `matcher` property in the Proxy to specify which routes Proxy should run on. Although, for auth, it's recommended Proxy runs on all routes.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1133:We recommend creating a DAL to centralize your data requests and authorization logic.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1175:You can then invoke the `verifySession()` function in your data requests, Server Actions, Route Handlers:
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1303:By centralizing your data requests and authorization logic in a DAL and using DTOs, you can ensure that all data requests are secure and consistent, making it easier to maintain, audit, and debug as your application scales.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1308:> - Learn more about security best practices in our [Security in Next.js article](/blog/security-nextjs-server-components-actions).
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1459:> - Ensure that any Server Actions called from these components also perform their own authorization checks, as client-side UI restrictions alone are not sufficient for security.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1463:Treat [Server Actions](/docs/app/guides/server-actions) with the same security considerations as public-facing API endpoints, and verify if the user is allowed to perform a mutation.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1501:### Route Handlers
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1503:Treat [Route Handlers](/docs/app/api-reference/file-conventions/route) with the same security considerations as public-facing API endpoints, and verify if the user is allowed to access the Route Handler.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1511:  // User authentication and role verification
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1534:  // User authentication and role verification
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1553:The example above demonstrates a Route Handler with a two-tier security check. It first checks for an active session, and then verifies if the logged-in user is an 'admin'.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1563:API Routes in Next.js are essential for handling server-side logic and data management. It's crucial to secure these routes to ensure that only authorized users can access specific functionalities. This typically involves verifying the user's authentication status and their role-based permissions.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1622:This example demonstrates an API Route with a two-tier security check for authentication and authorization. It first checks for an active session, and then verifies if the logged-in user is an 'admin'. This approach ensures secure access, limited to authenticated and authorized users, maintaining robust security for request processing.
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1628:Now that you've learned about authentication in Next.js, here are Next.js-compatible libraries and resources to help you implement secure authentication and session management:
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1652:To continue learning about authentication and security, check out the following resources:
node_modules/next/dist/docs/01-app/02-guides/authentication.md:1654:- [How to think about security in Next.js](/blog/security-nextjs-server-components-actions)

--- test helper exports ---
module.exports = require('../../dist/experimental/testing/server')

--- current tests ---
✔ isalive svarer uten token (0.949625ms)
✔ saker krever token (0.745667ms)
ℹ tests 2
ℹ suites 0
ℹ pass 2
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 97.878167
<shellId: 10 completed with exit code 0>
```

</details>


### `bash`

**Inspect extraction, JWT checks and matcher test API**

$ nl -ba node_modules/@navikt/oasis/dist/index.mjs | sed -n '329,365p;397,478p;485,519p'; printf '\n--- validation exports ---\n'; rg -n 'const validateToken|const validateAzureToken|const stripBearer' node_modules/@navikt/oasis/dist/index.mjs; printf '\n--- matcher documentation ---\n'; sed -n '69,100p;721,748p' node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md

<details>
<summary>203 lines</summary>

````
329	};
   330	//#endregion
   331	//#region src/token-utils/get-token.ts
   332	function getToken(val) {
   333		if (typeof val === "string") return stripBearer(val);
   334		else if (val instanceof Headers) return getTokenFromHeaders(val);
   335		else if (val instanceof Request) return getTokenFromHeaders(val.headers);
   336		else {
   337			const authHeader = val.headers.authorization;
   338			return authHeader ? stripBearer(authHeader) : null;
   339		}
   340	}
   341	function getTokenFromHeaders(headers) {
   342		const authHeader = headers.get("authorization");
   343		return authHeader ? stripBearer(authHeader) : null;
   344	}
   345	//#endregion
   346	//#region src/token-utils/parse-token.ts
   347	/**
   348	* Parses Idporten token, returning PID if present. This does not support parsing
   349	* tokenx tokens.
   350	*
   351	* @param token Validated token issued by Idporten.
   352	*/
   353	function parseIdportenToken(token) {
   354		try {
   355			const payload = decodeJwt(token);
   356			if (typeof payload.pid === "string") return {
   357				ok: true,
   358				pid: payload.pid
   359			};
   360			return {
   361				ok: false,
   362				error: /* @__PURE__ */ new Error("Invalid or missing values in token")
   363			};
   364		} catch (error) {
   365			if (error instanceof Error) return {
   397				error
   398			};
   399			throw error;
   400		}
   401	}
   402	//#endregion
   403	//#region src/token-validation/envs.ts
   404	function getIdportenEnvs() {
   405		if (!process.env.IDPORTEN_JWKS_URI || !process.env.IDPORTEN_ISSUER || !process.env.IDPORTEN_AUDIENCE) throw Error("Trying to validate Idporten but no IDPORTEN_* envs! Are you in nais?");
   406		return {
   407			jwksUri: process.env.IDPORTEN_JWKS_URI,
   408			issuer: process.env.IDPORTEN_ISSUER,
   409			audience: process.env.IDPORTEN_AUDIENCE
   410		};
   411	}
   412	function getAzureEnvs() {
   413		if (!process.env.AZURE_OPENID_CONFIG_JWKS_URI || !process.env.AZURE_OPENID_CONFIG_ISSUER || !process.env.AZURE_APP_CLIENT_ID) throw Error("Trying to validate Azure but no AZURE_* envs! Are you in nais?");
   414		return {
   415			jwksUri: process.env.AZURE_OPENID_CONFIG_JWKS_URI,
   416			issuer: process.env.AZURE_OPENID_CONFIG_ISSUER,
   417			audience: process.env.AZURE_APP_CLIENT_ID
   418		};
   419	}
   420	function getTokenXEnvs() {
   421		if (!process.env.TOKEN_X_JWKS_URI || !process.env.TOKEN_X_ISSUER || !process.env.TOKEN_X_CLIENT_ID) throw Error("Trying to validate TokenX but no TOKEN_X_* envs! Are you in nais?");
   422		return {
   423			jwksUri: process.env.TOKEN_X_JWKS_URI,
   424			issuer: process.env.TOKEN_X_ISSUER,
   425			audience: process.env.TOKEN_X_CLIENT_ID
   426		};
   427	}
   428	//#endregion
   429	//#region src/token-validation/types.ts
   430	const ValidationResult = {
   431		Error: (error, errorType = "unknown") => ({
   432			ok: false,
   433			error: typeof error === "string" ? Error(error) : error,
   434			errorType
   435		}),
   436		Ok: (payload) => ({
   437			ok: true,
   438			payload
   439		})
   440	};
   441	//#endregion
   442	//#region src/token-validation/jwks.ts
   443	const ONE_DAY_IN_MILLIS = 864e5;
   444	const remoteJWKSetCache = {};
   445	function getJwkSet(jwksUri) {
   446		if (remoteJWKSetCache[jwksUri] == null) remoteJWKSetCache[jwksUri] = createRemoteJWKSet(new URL(jwksUri), { cacheMaxAge: ONE_DAY_IN_MILLIS });
   447		return remoteJWKSetCache[jwksUri];
   448	}
   449	//#endregion
   450	//#region src/token-validation/validate.ts
   451	const validateJwt = async ({ token, jwksUri, issuer, audience }) => {
   452		return spanAsync("Validate JWT", async (span) => {
   453			span.setAttributes({
   454				[OtelTaxonomy.TokenVerifyIssuer]: issuer,
   455				[OtelTaxonomy.TokenVerifyAudience]: audience
   456			});
   457			try {
   458				const { payload } = await jwtVerify(stripBearer(token), getJwkSet(jwksUri), {
   459					issuer,
   460					audience,
   461					algorithms: ["RS256"]
   462				});
   463				return ValidationResult.Ok(payload);
   464			} catch (e) {
   465				const errorType = e instanceof errors.JWTExpired ? "token expired" : "unknown";
   466				failSpan(span, `Token validation failed (${errorType})`, e);
   467				return ValidationResult.Error(e, errorType);
   468			}
   469		});
   470	};
   471	//#endregion
   472	//#region src/token-validation/index.ts
   473	/**
   474	* Validates token issued by Idporten. Requires Idporten to be enabled in nais
   475	* application manifest.
   476	*
   477	* @param token Token issued by Idporten.
   478	*/
   485	* application manifest.
   486	*
   487	* @param token Token issued by Azure.
   488	*/
   489	const validateAzureToken = (token) => validateJwt({
   490		token,
   491		...getAzureEnvs()
   492	});
   493	/**
   494	* Validates token issued by Tokenx. Requires Tokenx to be enabled in nais
   495	* application manifest.
   496	*
   497	* @param token Token issued by Tokenx.
   498	*/
   499	const validateTokenxToken = (token) => validateJwt({
   500		token,
   501		...getTokenXEnvs()
   502	});
   503	/**
   504	* Validates token issued by Idporten or Azure. Requires either Idporten or
   505	* Azure to be enabled in nais application manifest.
   506	*
   507	* @param token Token issued by Idporten or Azure.
   508	*/
   509	const validateToken = async (token) => {
   510		if (!token) return ValidationResult.Error("empty token");
   511		const idporten = !!process.env.IDPORTEN_ISSUER;
   512		const azure = !!process.env.AZURE_OPENID_CONFIG_ISSUER;
   513		if (idporten && azure) return ValidationResult.Error("multiple identity providers");
   514		else if (idporten) return validateIdportenToken(token);
   515		else if (azure) return validateAzureToken(token);
   516		else return ValidationResult.Error("no identity provider");
   517	};
   518	//#endregion
   519	export { TokenResult, ValidationResult, expiresIn, getToken, parseAzureUserToken, parseIdportenToken, requestAzureClientCredentialsToken, requestAzureOboToken, requestOboToken, requestTokenxOboToken, validateAzureToken, validateIdportenToken, validateToken, validateTokenxToken };

--- validation exports ---
31:const stripBearer = (token) => token.replace(/^****** "");
489:const validateAzureToken = (token) => validateJwt({
499:const validateTokenxToken = (token) => validateJwt({
509:const validateToken = async (token) => {

--- matcher documentation ---
Optionally, a config object can be exported alongside the Proxy function. This object includes the [matcher](#matcher) to specify paths where the Proxy applies.

### Matcher

The `matcher` option allows you to target specific paths for the Proxy to run on.

Without a `matcher`, Proxy runs on **every request**, including static files (`_next/static`), image optimizations (`_next/image`), and assets in the `public/` folder. Consider using a [negative match pattern](#negative-matching) to exclude these paths, otherwise auth logic or redirects can unintentionally block CSS, JS, or images from loading.

You can specify paths in several ways:

- For a single path: Directly use a string to define the path, like `'/about'`.
- For multiple paths: Use an array to list multiple paths, such as `matcher: ['/about', '/contact']`, which applies the Proxy to both `/about` and `/contact`.

```js filename="proxy.js"
export const config = {
  matcher: ['/about/:path*', '/dashboard/:path*'],
}
```

Additionally, the `matcher` option supports complex path specifications using regular expressions. For example, you can exclude certain paths with a regular expression matcher:

```js filename="proxy.js"
export const config = {
  matcher: [
    // Exclude API routes, static files, image optimizations, and .png files
    '/((?!api|_next/static|_next/image|.*\\.png$).*)',
  ],
}
```

This enables precise control over which paths to include or exclude.

    })
  )

  return NextResponse.next()
}
```

### Unit testing (experimental)

Starting in Next.js 15.1, the `next/experimental/testing/server` package contains utilities to help unit test proxy files. Unit testing proxy can help ensure that it's only run on desired paths and that custom routing logic works as intended before code reaches production.

The `unstable_doesProxyMatch` function can be used to assert whether proxy will run for the provided URL, headers, and cookies.

```js
import { unstable_doesProxyMatch } from 'next/experimental/testing/server'

expect(
  unstable_doesProxyMatch({
    config,
    nextConfig,
    url: '/test',
  })
).toEqual(false)
```

The entire proxy function can also be tested.

```js
<shellId: 11 completed with exit code 0>
````

</details>


### `bash`

**Reproduce internal list access with fake tokens in memory**

$ node --input-type=module -e 'import {GET} from "./app/api/intern/saker/route.ts"; import {requireSaksbehandler} from "./lib/auth.ts"; for (const authorization of [undefined, "Bearer", "Bearer invalid-token", "Bearer a.b.c"]) { const request = new Request("http://localhost/api/intern/saker", {headers: authorization ? {authorization} : {}}); const response = await GET(request); console.log(JSON.stringify({authorization: authorization ?? "(missing)", status: response.status, body: await response.json()})); if (authorization === "Bearer invalid-token") { const guard = await requireSaksbehandler(request); console.log(JSON.stringify({sameRequestViaSharedGuard:guard?.status, body: guard ? await guard.json() : null})); }}'

<details>
<summary>6 lines</summary>

```
{"authorization":"(missing)","status":401,"body":{"error":"Mangler token"}}
{"authorization":"Bearer","status":200,"body":[{"id":101,"tittel":"Søknad om dagpenger","status":"apen","internNotat":"Venter på arbeidsgiver"},{"id":102,"tittel":"Klage på vedtak","status":"lukket","internNotat":"Omgjort etter klage"}]}
{"authorization":"******","status":200,"body":[{"id":101,"tittel":"Søknad om dagpenger","status":"apen","internNotat":"Venter på arbeidsgiver"},{"id":102,"tittel":"Klage på vedtak","status":"lukket","internNotat":"Omgjort etter klage"}]}
{"sameRequestViaSharedGuard":401,"body":{"error":"Ugyldig token"}}
{"authorization":"******","status":200,"body":[{"id":101,"tittel":"Søknad om dagpenger","status":"apen","internNotat":"Venter på arbeidsgiver"},{"id":102,"tittel":"Klage på vedtak","status":"lukket","internNotat":"Omgjort etter klage"}]}
<shellId: 12 completed with exit code 0>
```

</details>


### `bash`

**Verify real framework matching for the target endpoint**

$ node --input-type=module -e 'import {createRequire} from "node:module"; import {readFileSync} from "node:fs"; const require = createRequire(import.meta.url); const {unstable_doesProxyMatch} = require("next/experimental/testing/server"); const text = readFileSync("./proxy.ts", "utf8"); const matcher = JSON.parse(text.match(/matcher:\s*(\[[^\]]+\])/)[1]); for (const url of ["/api/saker", "/api/intern/saker", "/api/isalive", "/api/isready"]) console.log(JSON.stringify({url, proxyMatches: unstable_doesProxyMatch({config:{matcher},nextConfig:{},url})}));'

<details>
<summary>11 lines</summary>

```
<workspace>/[eval1]:1
import {createRequire} from "node:module"; import {readFileSync} from "node:fs"; const require = createRequire(import.meta.url); const {unstable_doesProxyMatch} = require("next/experimental/testing/server"); const text = readFileSync("./proxy.ts", "utf8"); const matcher = JSON.parse(text.match(/matcher:\s*(\[[^\]]+\])/)[1]); for (const url of ["/api/saker", "/api/intern/saker", "/api/isalive", "/api/isready"]) console.log(JSON.stringify({url, proxyMatches: unstable_doesProxyMatch({config:{matcher},nextConfig:{},url})}));
                                                                                                                                                                                                                                                                                                                                                                                                                                                                             ^

TypeError: unstable_doesProxyMatch is not a function
    at <workspace>/[eval1]:1:462
    at ModuleJob.run (node:internal/modules/esm/module_job:561:25)
    at async node:internal/modules/esm/loader:223:26
    at async ModuleLoader.executeModuleJob (node:internal/modules/esm/loader:220:20)
    at async asyncRunEntryPointWithESMLoader (node:internal/modules/run_main:101:5)

Node.js v24.21.0
<shellId: 14 completed with exit code 1>
```

</details>


### `bash`

**Compare target route and shared guard using local signed JWTs**

$ node --input-type=module -e '
import assert from "node:assert/strict";
import {generateKeyPair, exportJWK, SignJWT} from "jose";
import {GET} from "./app/api/intern/saker/route.ts";
import {requireSaksbehandler} from "./lib/auth.ts";
const issuer = "https://issuer.example.invalid/azure";
const audience = "local-test-client";
process.env.AZURE_OPENID_CONFIG_ISSUER = issuer;
process.env.AZURE_APP_CLIENT_ID = audience;
process.env.AZURE_OPENID_CONFIG_JWKS_URI = "https://jwks.example.invalid/keys";
delete process.env.IDPORTEN_ISSUER;
const trusted = await generateKeyPair("RS256");
const other = await generateKeyPair("RS256");
const jwk = {...await exportJWK(trusted.publicKey), kid: "local-test", alg: "RS256", use: "sig"};
let fetches = 0;
globalThis.fetch = async (url) => {assert.equal(String(url), process.env.AZURE_OPENID_CONFIG_JWKS_URI); fetches++; return Response.json({keys: [jwk]});};
async function mint({key=trusted.privateKey, iss=issuer, aud=audience, exp="5m"} = {}) {return new SignJWT({}).setProtectedHeader({alg:"RS256",kid:"local-test"}).setSubject("test-user").setIssuer(iss).setAudience(aud).setIssuedAt().setExpirationTime(exp).sign(key);}
const cases = [
["missing", undefined, 401],
["arbitrary token", "not-a-jwt", 401],
["valid signed token", await mint(), 200],
["expired token", await mint({exp:Math.floor(Date.now()/1000)-60}), 401],
["wrong audience", await mint({aud:"other-client"}), 401],
["wrong issuer", await mint({iss:"https://other.example.invalid"}), 401],
["invalid signature", await mint({key:other.privateKey}), 401]
];
for (const [name, token, expectedGuard] of cases) {
const request = new Request("http://localhost/api/intern/saker", {headers:token ? {authorization:`Bearer ${token}`} : {}});
const result = await GET(request);
const guard = await requireSaksbehandler(request);
assert.equal(guard?.status ?? 200, expectedGuard);
const body = await result.json();
console.log(JSON.stringify({case:name, endpointStatus:result.status, sharedGuardStatus:guard?.status ?? 200, exposesInternalNotes: Array.isArray(body) && body.some(s=>"internNotat" in s)}));
}
console.log(JSON.stringify({jwksFetchesServedInMemory:fetches, externalNetworkRequests:0}));
'

<details>
<summary>9 lines</summary>

```
{"case":"missing","endpointStatus":401,"sharedGuardStatus":401,"exposesInternalNotes":false}
{"case":"arbitrary token","endpointStatus":200,"sharedGuardStatus":401,"exposesInternalNotes":true}
{"case":"valid signed token","endpointStatus":200,"sharedGuardStatus":200,"exposesInternalNotes":true}
{"case":"expired token","endpointStatus":200,"sharedGuardStatus":401,"exposesInternalNotes":true}
{"case":"wrong audience","endpointStatus":200,"sharedGuardStatus":401,"exposesInternalNotes":true}
{"case":"wrong issuer","endpointStatus":200,"sharedGuardStatus":401,"exposesInternalNotes":true}
{"case":"invalid signature","endpointStatus":200,"sharedGuardStatus":401,"exposesInternalNotes":true}
{"jwksFetchesServedInMemory":1,"externalNetworkRequests":0}
<shellId: 13 completed with exit code 0>
```

</details>


### `bash`

**Resolve installed matcher API and verify token extraction**

$ printf '\n--- installed Next test exports ---\n'; node -e 'console.log(Object.keys(require("next/experimental/testing/server")))'; printf '\n--- matcher helper implementation ---\n'; rg -n 'does.*Match|getMiddlewareMatchers|function.*Match' node_modules/next/dist/experimental/testing/server/config-testing-utils.js node_modules/next/dist/experimental/testing/server/index.js; printf '\n--- bearer extraction ---\n'; sed -n '24,35p' node_modules/@navikt/oasis/dist/index.mjs

<details>
<summary>23 lines</summary>

```
--- installed Next test exports ---
[
  'getRedirectUrl',
  'getRewrittenUrl',
  'isRewrite',
  'unstable_getResponseFromNextConfig',
  'unstable_doesMiddlewareMatch'
]

--- matcher helper implementation ---

--- bearer extraction ---
const decodeJwt = (token) => {
	try {
		return JSON.parse(Buffer.from(token.split(".")[1], "base64url").toString("utf8"));
	} catch {
		throw new Error("Invalid JWT");
	}
};
const stripBearer = (token) => token.replace(/^****** "");
//#endregion
//#region src/token-utils/expires-in.ts
/**
* Returns seconds until token expires.
<shellId: 15 completed with exit code 0>
```

</details>


### `bash`

**Confirm target route bypasses configured proxy matcher**

$ node --input-type=module -e 'import {createRequire} from "node:module"; import {readFileSync} from "node:fs"; const require = createRequire(import.meta.url); const {unstable_doesMiddlewareMatch} = require("next/experimental/testing/server"); const text = readFileSync("./proxy.ts", "utf8"); const matcher = JSON.parse(text.match(/matcher:\s*(\[[^\]]+\])/)[1]); for (const url of ["/api/saker", "/api/intern/saker", "/api/isalive", "/api/isready"]) console.log(JSON.stringify({url, proxyMatches:unstable_doesMiddlewareMatch({config:{matcher},nextConfig:{},url})}));'

<details>
<summary>16 lines</summary>

```
<workspace>/node_modules/next/dist/server/app-render/async-local-storage.js:27
const sharedAsyncLocalStorageNotAvailableError = Object.defineProperty(new Error('Invariant: AsyncLocalStorage accessed in runtime where it is not available'), "__NEXT_ERROR_CODE", {
                                                                       ^

Error: Invariant: AsyncLocalStorage accessed in runtime where it is not available
    at Object.<anonymous> (<workspace>/node_modules/next/dist/server/app-render/async-local-storage.js:27:72)
    at Module._compile (node:internal/modules/cjs/loader:1929:14)
    at Object..js (node:internal/modules/cjs/loader:2060:10)
    at Module.load (node:internal/modules/cjs/loader:1651:32)
    at Module._load (node:internal/modules/cjs/loader:1443:12)
    at wrapModuleLoad (node:internal/modules/cjs/loader:261:19)
    at Module.<anonymous> (node:internal/modules/cjs/loader:1674:12)
    at require (node:internal/modules/helpers:157:16)
    at Object.<anonymous> (<workspace>/node_modules/next/dist/server/app-render/work-async-storage-instance.js:11:28)
    at Module._compile (node:internal/modules/cjs/loader:1929:14)

Node.js v24.21.0
<shellId: 16 completed with exit code 1>
```

</details>


### `bash`

**Run matcher check with required Next runtime async storage**

$ node --input-type=module -e 'import {AsyncLocalStorage} from "node:async_hooks"; import {createRequire} from "node:module"; import {readFileSync} from "node:fs"; globalThis.AsyncLocalStorage = AsyncLocalStorage; const require = createRequire(import.meta.url); const {unstable_doesMiddlewareMatch} = require("next/experimental/testing/server"); const text = readFileSync("./proxy.ts", "utf8"); const matcher = JSON.parse(text.match(/matcher:\s*(\[[^\]]+\])/)[1]); for (const url of ["/api/saker", "/api/intern/saker", "/api/isalive", "/api/isready"]) console.log(JSON.stringify({url, proxyMatches:unstable_doesMiddlewareMatch({config:{matcher},nextConfig:{},url})}));'

```
{"url":"/api/saker","proxyMatches":true}
{"url":"/api/intern/saker","proxyMatches":false}
{"url":"/api/isalive","proxyMatches":false}
{"url":"/api/isready","proxyMatches":false}
<shellId: 17 completed with exit code 0>
```


### Copilot

## Sikkerhetsfunn

### 1. Et vilkårlig token gir tilgang til interne saker

**File:** `<workspace>/app/api/intern/saker/route.ts:1–8`  
**Category:** AuthenticationFailure  
**Severity: HIGH | Confidence: 10/10**

**Problem:** `GET` kontrollerer at `getToken(request)` returnerer en streng, men validerer ikke tokenet. En angriper som kan nå appen, kan sende `Authorization: Bearer invalid-token` og få HTTP 200 med hele den interne sakslisten, inkludert `internNotat`.

Årsaken er denne kontrollen:

```ts
if (!getToken(request)) {
  return Response.json({ error: "Mangler token" }, { status: 401 });
}
return Response.json(listInterneSaker());
```

`getToken` henter headerverdien og fjerner Bearer-prefikset. Den kontrollerer ikke signatur, utløpstid, issuer eller audience. Dette er bekreftet i den installerte implementasjonen:

`<workspace>/node_modules/@navikt/oasis/dist/index.mjs:332–344`

**Evidence:** Jeg kalte den faktiske route-handleren med `Request`-objekter. Signerte testtokens og JWKS ble laget i minnet, uten eksterne nettverkskall.

| Forespørsel | Endepunktet nå | Eksisterende `requireSaksbehandler` |
|---|---:|---:|
| Ingen token | 401 | 401 |
| Vilkårlig streng, ikke JWT | **200** | 401 |
| Gyldig signert token | 200 | 200 |
| Utløpt token | **200** | 401 |
| Feil audience | **200** | 401 |
| Feil issuer | **200** | 401 |
| Ugyldig signatur | **200** | 401 |

Alle 200-svar fra endepunktet inneholdt interne notater. **Påstanden er dermed bekreftet for ugyldig innlogging, men ikke for en forespørsel helt uten tokenheader**, som allerede får 401.

Proxyen gir heller ingen ekstra beskyttelse:

`<workspace>/proxy.ts:6–14`

```ts
matcher: ["/api/saker/:path*"],
```

Nexts installerte matcher-test bekreftet at `/api/intern/saker` ikke omfattes. Å utvide matcheren alene retter ikke sårbarheten: proxyen kontrollerer også kun at tokenet finnes.

## Anbefalt minimal retting

I den berørte route-filen: erstatt `getToken`-kontrollen med den eksisterende helperen. Foreslått innhold, **ikke skrevet til disk**:

```ts
import { requireSaksbehandler } from "../../../../lib/auth.ts";
import { listInterneSaker } from "../../../../lib/saker.ts";

export async function GET(request: Request): Promise<Response> {
  const unauthorized = await requireSaksbehandler(request);
  if (unauthorized) return unauthorized;
  return Response.json(listInterneSaker());
}
```

Helperen finnes i:

`<workspace>/lib/auth.ts:7–16`

Den avviser manglende token og resultatet `validation.ok === false`. Med Azure-konfigurasjonen beskrevet i `<workspace>/docs/drift.md:3–11` bruker Oasis signaturvalidering og kontrollerer issuer, audience og tidskrav.

Kontrollen må ligge i handleren før data hentes. Dette gjenbruker eksisterende sikkerhetskode og beskytter endepunktet uavhengig av proxy-matcheren.

**Rød sone:** Helperen validerer tokenet, men sjekker ikke saksbehandlerroller, grupper eller saksrettigheter. Lokale filer definerer ingen slik tilgangspolicy. Rettingen gjenoppretter derfor det dokumenterte autentiseringskravet; den etablerer ikke en ny rollepolicy.

## Relevante tester

Legg til tester for **akkurat `/api/intern/saker`**, med tilfellene i tabellen over:

- Alle ugyldige eller manglende tokens skal gi 401 uten saksdata.
- Et gyldig Azure-token med riktig audience skal gi 200.
- Signatur-, issuer-, audience- og utløpstestene bør bruke ekte signerte testtokens og lokal/in-memory JWKS, ikke mocke hele validatoren.
- Behold tester som sikrer at `/api/isalive` og `/api/isready` svarer uten token dersom proxyen senere endres.

Eksisterende `<workspace>/tests/probes.test.ts:6–12` har to tester. Begge bestod, men ingen dekker det interne endepunktet eller ugyldige tokens.

Undersøkelsen gjelder nåværende lokale kode. Git nektet tilgang til arbeidskatalogen, så diff kunne ikke bekreftes. Wonderwall-beskyttelse og faktisk produksjonseksponering er ikke verifisert. Ingen filer er endret.


---

<sub>1m 11s</sub>

### Info

Session limits: 15.24/30 AI credits used.

---

<sub>2m 42s</sub>

### Info

Session limits: 23.38/30 AI credits used.

---

<sub>2m 44s</sub>

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

<sub>2m 55s</sub>

### Copilot

Årsaken er at endepunktet sjekker om et token finnes, uten å validere det. Jeg erstatter kontrollen med repoets eksisterende autentiseringsfunksjon og legger til regresjonstester for ugyldige tokens.


---

<sub>2m 55s</sub>

### `glob`

**{AGENTS.md,package.json,tsconfig.json,vitest.config.*,tests/**,.github/instructions/**,docs/**}**

"{AGENTS.md,package.json,tsconfig.json,vitest.config.*,tests/**,.github/instructions/**,docs/**}"

<details>
<summary>20 matches</summary>

```
./tsconfig.json
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
./docs/drift.md
./tests/probes.test.ts
```

</details>


---

<sub>2m 55s</sub>

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

<sub>2m 55s</sub>

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

<sub>2m 55s</sub>

### `bash`

**Kontrollere lokale endringer**

$ git --no-pager diff -- . && git status --short -- .

```
fatal: unable to get current working directory: Operation not permitted
<shellId: 18 completed with exit code 128>
```


---

<sub>3m 5s</sub>

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

<sub>3m 5s</sub>

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

<sub>3m 5s</sub>

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

<sub>3m 5s</sub>

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

<sub>3m 5s</sub>

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

<sub>3m 5s</sub>

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

<sub>3m 5s</sub>

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

<sub>3m 5s</sub>

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

<sub>3m 12s</sub>

### Info

Session limits: 28.17/30 AI credits used. If this session continues, the next model call is final; tools will be disabled.

---

<sub>3m 12s</sub>

### `rg`

**jwks|JWKS|issuer|validateToken|customFetch**

"jwks|JWKS|issuer|validateToken|customFetch" in *.{mjs,ts} (node_modules/@navikt/oasis/dist)

<details>
<summary>25 matches</summary>

```
[grep content: 23 matches across 1 file(s) under node_modules/@navikt/oasis/dist]

index.mjs (23 match(es)):
   import { createRemoteJWKSet, errors, jwtVerify } from "jose";
   	TokenVerifyIssuer: "token.verify.issuer",
   	if (!process.env.IDPORTEN_JWKS_URI || !process.env.IDPORTEN_ISSUER || !process.env.IDPORTEN_AUDIENCE) throw Error("Trying to validate Idporten but no IDPORTEN_* envs! Are you in nais?");
   		jwksUri: process.env.IDPORTEN_JWKS_URI,
   		issuer: process.env.IDPORTEN_ISSUER,
   	if (!process.env.AZURE_OPENID_CONFIG_JWKS_URI || !process.env.AZURE_OPENID_CONFIG_ISSUER || !process.env.AZURE_APP_CLIENT_ID) throw Error("Trying to validate Azure but no AZURE_* envs! Are you in nais?");
   		jwksUri: process.env.AZURE_OPENID_CONFIG_JWKS_URI,
   		issuer: process.env.AZURE_OPENID_CONFIG_ISSUER,
   	if (!process.env.TOKEN_X_JWKS_URI || !process.env.TOKEN_X_ISSUER || !process.env.TOKEN_X_CLIENT_ID) throw Error("Trying to validate TokenX but no TOKEN_X_* envs! Are you in nais?");
   		jwksUri: process.env.TOKEN_X_JWKS_URI,
   		issuer: process.env.TOKEN_X_ISSUER,
   //#region src/token-validation/jwks.ts
   const remoteJWKSetCache = {};
   function getJwkSet(jwksUri) {
   	if (remoteJWKSetCache[jwksUri] == null) remoteJWKSetCache[jwksUri] = createRemoteJWKSet(new URL(jwksUri), { cacheMaxAge: ONE_DAY_IN_MILLIS });
   	return remoteJWKSetCache[jwksUri];
   const validateJwt = async ({ token, jwksUri, issuer, audience }) => {
   			[OtelTaxonomy.TokenVerifyIssuer]: issuer,
   			const { payload } = await jwtVerify(stripBearer(token), getJwkSet(jwksUri), {
   				issuer,
   const validateTokenxToken = (token) => validateJwt({
   const validateToken = async (token) => {
   export { TokenResult, ValidationResult, expiresIn, getToken, parseAzureUserToken, parseIdportenToken, requestAzureClientCredentialsToken, requestAzureOboToken, requestOboToken, requestTokenxOboToken, validateAzureToken, validateIdportenToken, validateToken, validateTokenxToken };
```

</details>


---

<sub>3m 13s</sub>

### `bash`

**Kjøre eksisterende tester og typesjekk**

$ pwd; node --version; npm test && npm run typecheck

<details>
<summary>17 lines</summary>

```
<workspace>
v24.21.0
npm notice run test
npm notice run node --test tests/*.test.ts
✔ isalive svarer uten token (0.969042ms)
✔ saker krever token (0.550292ms)
ℹ tests 2
ℹ suites 0
ℹ pass 2
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 101.959958
npm notice run typecheck
npm notice run tsc --noEmit
<shellId: 19 completed with exit code 0>
```

</details>


---

<sub>3m 12s</sub>

### `glob`

**app/api/**/route.ts**

"app/api/**/route.ts"

<details>
<summary>6 matches</summary>

```
./app/api/isready/route.ts
./app/api/isalive/route.ts
./app/api/intern/saker/[id]/route.ts
./app/api/intern/saker/[id]/notater/route.ts
./app/api/intern/saker/route.ts
./app/api/saker/route.ts
```

</details>


---

<sub>3m 22s</sub>

### Info

Session limits: 29.01/30 AI credits used. If this session continues, the next model call is final; tools will be disabled.

---

<sub>3m 22s</sub>

### `view`

**view the file at <workspace>/node_modules/@navikt/oasis/dist/index.mjs.**

<workspace>/node_modules/@navikt/oasis/dist/index.mjs (lines 175-330)

<details>
<summary>156 lines</summary>

```
			}
			return result;
		});
	};
}
//#endregion
//#region src/lib/otel.ts
const LIB_NAME = "@navikt/oasis";
/**
* Wraps an async function in a span. Lets you receive the active
* span to add additional attributes or events.
*/
async function spanAsync(name, fn) {
	const span = trace.getTracer(LIB_NAME, "4.2.2").startSpan(`${name}`);
	return context.with(trace.setSpan(context.active(), span), async () => fn(span).finally(() => span.end()));
}
function failSpan(span, what, error) {
	if (error && error instanceof Error) {
		span.recordException(error);
		if (error.cause != null) span.recordException(error.cause instanceof Error ? error.cause : new Error(error.cause));
	}
	span.setStatus({
		code: SpanStatusCode.ERROR,
		message: what
	});
}
const OtelTaxonomy = {
	TokenVerifyIssuer: "token.verify.issuer",
	TokenVerifyAudience: "token.verify.audience",
	OboExchangeTarget: "token.exchange.obo.target",
	OboExchangeProvider: "token.exchange.obo.provider",
	M2MExchangeTarget: "token.exchange.m2m.target",
	M2MExchangeProvider: "token.exchange.m2m.provider"
};
//#endregion
//#region src/token-exchange/m2m/exchange.ts
const grantClientCredentialsToken = async (provider, target) => {
	return spanAsync("Token exchange (m2m)", async (span) => {
		span.setAttributes({
			[OtelTaxonomy.M2MExchangeTarget]: target,
			[OtelTaxonomy.M2MExchangeProvider]: provider
		});
		try {
			const { access_token } = await texas.token({
				identity_provider: provider,
				target
			});
			return access_token ? TokenResult.Ok(access_token) : TokenResult.Error(Error("TokenSet does not contain an access_token"));
		} catch (e) {
			failSpan(span, "M2M token exchange failed", e);
			return TokenResult.Error(e);
		}
	});
};
//#endregion
//#region src/token-exchange/m2m/index.ts
/**
* Requests client credentials token from Azure. Requires Azure to be enabled in
* nais application manifest.
*
* @param target The target app you request a token for.
*/
const requestAzureClientCredentialsToken = async (target) => withCache((provider, target) => grantClientCredentialsToken(provider, target), "client-credentials")("azuread", target);
//#endregion
//#region src/token-exchange/obo/exchange.ts
const grantOboToken = async (provider, token, target) => {
	return spanAsync("Token exchange (OBO)", async (span) => {
		span.setAttributes({
			[OtelTaxonomy.OboExchangeTarget]: target,
			[OtelTaxonomy.OboExchangeProvider]: provider
		});
		try {
			const { access_token } = await texas.exchange(provider === "azuread" ? {
				identity_provider: provider,
				user_token: stripBearer(token),
				target
			} : {
				identity_provider: provider,
				user_token: stripBearer(token),
				target
			});
			return access_token ? TokenResult.Ok(access_token) : TokenResult.Error(Error("TokenSet does not contain an access_token"));
		} catch (e) {
			failSpan(span, "OBO token exchange failed", e);
			return TokenResult.Error(e);
		}
	});
};
//#endregion
//#region src/token-exchange/obo/prometheus.ts
var AuthMetrics = class {
	tokenExchangeDurationHistogram = new Histogram({
		name: "oasis_token_exchange_duration_seconds",
		help: "Duration of token exchange in seconds",
		labelNames: ["provider"]
	});
	tokenExchangeFailures = new Counter({
		name: "oasis_token_exchange_failures",
		help: "Number of failed token exchanges",
		labelNames: ["provider"]
	});
	tokenExchanges = new Counter({
		name: "oasis_token_exchanges",
		help: "Number of token exchanges",
		labelNames: ["provider"]
	});
};
const authMetricsSymbol = Symbol.for("AuthMetrics");
global[authMetricsSymbol] = global[authMetricsSymbol] || new AuthMetrics();
const prometheus = global[authMetricsSymbol];
function withPrometheus(oboProvider) {
	return async (provider, token, audience) => {
		const measureTokenExchange = prometheus.tokenExchangeDurationHistogram.labels({ provider }).startTimer();
		const oboToken = await oboProvider(provider, token, audience);
		measureTokenExchange();
		if (!oboToken.ok) prometheus.tokenExchangeFailures.labels({ provider }).inc();
		prometheus.tokenExchanges.labels({ provider }).inc();
		return oboToken;
	};
}
//#endregion
//#region src/token-exchange/obo/index.ts
/**
* Requests on-behalf-of token from Azure. Requires Azure to be enabled in nais
* application manifest.
*
* @param token Token from your client (assertion).
* @param audience The target app you request a token for (scope).
*/
const requestAzureOboToken = async (token, audience) => withCache(withPrometheus((provider, token, target) => grantOboToken(provider, token, target)))("azuread", token, audience);
/**
* Requests on-behalf-of token from Tokenx. Requires Tokenx to be enabled in
* nais application manifest.
*
* @param token Token from your client (subject token).
* @param audience The target app you request a token for.
*/
const requestTokenxOboToken = async (token, audience) => withCache(withPrometheus((provider, token, audience) => grantOboToken(provider, token, audience)))("tokenx", token, audience);
/**
* Requests on-behalf-of token from Tokenx or Azure. Requires either Tokenx or
* Azure to be enabled in nais application manifest.
*
* @param token Token from your client (subject token).
* @param audience The target app you request a token for.
*/
const requestOboToken = async (token, audience) => {
	if (!token) return TokenResult.Error("empty token");
	if (!audience) return TokenResult.Error("empty audience");
	const tokenx = !!process.env.TOKEN_X_ISSUER;
	const azure = !!process.env.AZURE_OPENID_CONFIG_ISSUER;
	if (tokenx && azure) return TokenResult.Error("Multiple identity providers, use requestTokenxOboToken or requestAzureOboToken directly!");
	else if (tokenx) return requestTokenxOboToken(token, audience);
	else if (azure) return requestAzureOboToken(token, audience);
	else return TokenResult.Error("No identity provider configured in your nais.yaml");
};
//#endregion
```

</details>


---

<sub>3m 23s</sub>

### `view`

**view the file at <workspace>/node_modules/jose/dist/webapi/jwks/remote.js.**

<workspace>/node_modules/jose/dist/webapi/jwks/remote.js (lines 1-100)

<details>
<summary>100 lines</summary>

```
import { JOSEError, JWKSNoMatchingKey, JWKSTimeout } from "../util/errors.js";
import { createLocalJWKSet } from "./local.js";
import { isJwkSet } from "../lib/validate.js";
function isCloudflareWorkers() {
  return typeof WebSocketPair < "u" || typeof navigator < "u" && navigator.userAgent === "Cloudflare-Workers" || typeof EdgeRuntime < "u" && EdgeRuntime === "vercel";
}
let USER_AGENT;
(typeof navigator > "u" || !navigator.userAgent?.startsWith?.("Mozilla/5.0 ")) && (USER_AGENT = "jose/v6.2.12");
const customFetch = /* @__PURE__ */ Symbol();
async function fetchJwks(url, headers, signal, fetchImpl = fetch) {
  const response = await fetchImpl(url, {
    method: "GET",
    signal,
    redirect: "manual",
    headers
  }).catch((err) => {
    throw err.name === "TimeoutError" ? new JWKSTimeout() : err;
  });
  if (response.status !== 200)
    throw new JOSEError("Expected 200 OK from the JSON Web Key Set HTTP response");
  try {
    return await response.json();
  } catch {
    throw new JOSEError("Failed to parse the JSON Web Key Set HTTP response as JSON");
  }
}
const jwksCache = /* @__PURE__ */ Symbol();
function isFreshFor(timestamp, duration) {
  return Number.isFinite(timestamp) && Date.now() < timestamp + duration;
}
function validateDuration(value, fallback, option) {
  if (Number.isNaN(value))
    throw new TypeError(`"${option}" option must not be NaN`);
  return typeof value == "number" ? value : fallback;
}
function createRemoteJWKSet(url, options) {
  if (!(url instanceof URL))
    throw new TypeError("url must be an instance of URL");
  const href = new URL(url.href).href, opts = options ?? {}, timeoutOption = opts.timeoutDuration;
  if (typeof timeoutOption == "number" && (!Number.isInteger(timeoutOption) || timeoutOption < 0))
    throw new TypeError('"timeoutDuration" option must be a non-negative integer');
  const timeoutDuration = typeof timeoutOption == "number" ? timeoutOption : 5e3, cooldownDuration = validateDuration(opts.cooldownDuration, 3e4, "cooldownDuration"), cacheMaxAge = validateDuration(opts.cacheMaxAge, 6e5, "cacheMaxAge"), headers = new Headers(opts.headers);
  USER_AGENT && !headers.has("User-Agent") && headers.set("User-Agent", USER_AGENT), headers.has("accept") || headers.set("accept", "application/json, application/jwk-set+json");
  const fetchImpl = opts[customFetch], cache = opts[jwksCache];
  let jwksTimestamp, pendingFetch, reloadSequence = 0, appliedSequence = 0, local;
  if (cache && typeof cache == "object") {
    const { uat, jwks } = cache;
    isFreshFor(uat, cacheMaxAge) && isJwkSet(jwks) && (jwksTimestamp = uat, local = createLocalJWKSet(jwks));
  }
  const reload = async () => {
    if (pendingFetch && isCloudflareWorkers() && (pendingFetch = void 0), !pendingFetch) {
      const sequence = ++reloadSequence, current = pendingFetch = fetchJwks(href, headers, AbortSignal.timeout(timeoutDuration), fetchImpl).then((json) => {
        const next = createLocalJWKSet(json);
        if (sequence <= appliedSequence)
          return;
        local = next;
        const updatedAt = Date.now();
        cache && (cache.uat = updatedAt, cache.jwks = json), jwksTimestamp = updatedAt, appliedSequence = sequence;
      }).finally(() => {
        pendingFetch === current && (pendingFetch = void 0);
      });
    }
    await pendingFetch;
  };
  return Object.defineProperties(async (protectedHeader, token) => {
    (!local || !isFreshFor(jwksTimestamp, cacheMaxAge)) && await reload();
    try {
      return await local(protectedHeader, token);
    } catch (err) {
      if (err instanceof JWKSNoMatchingKey && !isFreshFor(jwksTimestamp, cooldownDuration))
        return await reload(), local(protectedHeader, token);
      throw err;
    }
  }, {
    coolingDown: {
      get: () => isFreshFor(jwksTimestamp, cooldownDuration),
      enumerable: !0
    },
    fresh: {
      get: () => isFreshFor(jwksTimestamp, cacheMaxAge),
      enumerable: !0
    },
    reload: {
      value: reload,
      enumerable: !0
    },
    reloading: {
      get: () => !!pendingFetch,
      enumerable: !0
    },
    jwks: {
      value: () => local?.jwks(),
      enumerable: !0
    }
  });
}
export {
  createRemoteJWKSet,
  customFetch,
  jwksCache
```

</details>


---

<sub>3m 23s</sub>

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

<sub>3m 29s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>3m 29s</sub>

### Warning

Session limit reached (30.18/30 AI credits used). Future model calls will stop unless the session limit is increased or removed.

---

<sub>3m 29s</sub>

### `rg`

**const validateToken|const validateJwt|getAzureConfiguration|validateAzureToken|clockTolerance**

"const validateToken|const validateJwt|getAzureConfiguration|validateAzureToken|clockTolerance" (node_modules/@navikt/oasis/dist/index.mjs)

<details>
<summary>72 matches</summary>

```
[grep content: 6 matches across 1 file(s) under node_modules/@navikt/oasis/dist]

index.mjs (6 match(es)):
  439- 	})
  440- };
  441- //#endregion
  442- //#region src/token-validation/jwks.ts
  443- const ONE_DAY_IN_MILLIS = 864e5;
  444- const remoteJWKSetCache = {};
  445- function getJwkSet(jwksUri) {
  446- 	if (remoteJWKSetCache[jwksUri] == null) remoteJWKSetCache[jwksUri] = createRemoteJWKSet(new URL(jwksUri), { cacheMaxAge: ONE_DAY_IN_MILLIS });
  447- 	return remoteJWKSetCache[jwksUri];
  448- }
  449- //#endregion
  450- //#region src/token-validation/validate.ts
  451: const validateJwt = async ({ token, jwksUri, issuer, audience }) => {
  452- 	return spanAsync("Validate JWT", async (span) => {
  453- 		span.setAttributes({
  454- 			[OtelTaxonomy.TokenVerifyIssuer]: issuer,
  455- 			[OtelTaxonomy.TokenVerifyAudience]: audience
  456- 		});
  457- 		try {
  458- 			const { payload } = await jwtVerify(stripBearer(token), getJwkSet(jwksUri), {
  459- 				issuer,
  460- 				audience,
  461- 				algorithms: ["RS256"]
  462- 			});
  463- 			return ValidationResult.Ok(payload);
  477- * @param token Token issued by Idporten.
  478- */
  479- const validateIdportenToken = (token) => validateJwt({
  480- 	token,
  481- 	...getIdportenEnvs()
  482- });
  483- /**
  484- * Validates token issued by Azure. Requires Azure to be enabled in nais
  485- * application manifest.
  486- *
  487- * @param token Token issued by Azure.
  488- */
  489: const validateAzureToken = (token) => validateJwt({
  490- 	token,
  491- 	...getAzureEnvs()
  492- });
  493- /**
  494- * Validates token issued by Tokenx. Requires Tokenx to be enabled in nais
  495- * application manifest.
  496- *
  497- * @param token Token issued by Tokenx.
  498- */
  499: const validateTokenxToken = (token) => validateJwt({
  500- 	token,
  501- 	...getTokenXEnvs()
  502- });
  503- /**
  504- * Validates token issued by Idporten or Azure. Requires either Idporten or
  505- * Azure to be enabled in nais application manifest.
  506- *
  507- * @param token Token issued by Idporten or Azure.
  508- */
  509: const validateToken = async (token) => {
  510- 	if (!token) return ValidationResult.Error("empty token");
  511- 	const idporten = !!process.env.IDPORTEN_ISSUER;
  512- 	const azure = !!process.env.AZURE_OPENID_CONFIG_ISSUER;
  513- 	if (idporten && azure) return ValidationResult.Error("multiple identity providers");
  514- 	else if (idporten) return validateIdportenToken(token);
  515: 	else if (azure) return validateAzureToken(token);
  516- 	else return ValidationResult.Error("no identity provider");
  517- };
  518- //#endregion
  519: export { TokenResult, ValidationResult, expiresIn, getToken, parseAzureUserToken, parseIdportenToken, requestAzureClientCredentialsToken, requestAzureOboToken, requestOboToken, requestTokenxOboToken, validateAzureToken, validateIdportenToken, validateToken, validateTokenxToken };
  520- 
  521- //# sourceMappingURL=index.mjs.map
```

</details>


---

<sub>4m 5s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 5s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Add File: <workspace>/tests/intern-saker.test.ts\n+import assert from \"node:assert/strict\";\n+import { test } from \"node:test\";\n+import { exportJWK, generateKeyPair, SignJWT } from \"jose\";\n+import { GET } from \"../app/api/intern/saker/route.ts\";\n+import { listInterneSaker } from \"../lib/saker.ts\";\n+\n+test(\"interne saker krever et gyldig Azure-token\", async (t) => {\n+  const issuer = \"https://login.test/tenant/v2.0\";\n+  const audience = \"saksoversikt-test\";\n+  const jwksUri = \"https://login.test/tenant/keys\";\n+  const env = {\n+    AZURE_OPENID_CONFIG_ISSUER: issuer,\n+    AZURE_APP_CLIENT_ID: audience,\n+    AZURE_OPENID_CONFIG_JWKS_URI: jwksUri,\n+    IDPORTEN_ISSUER: undefined,\n+  };\n+  for (const [name, value] of Object.entries(env)) {\n+    const original = process.env[name];\n+    t.after(() => {\n+      if (original === undefined) delete process.env[name];\n+      else process.env[name] = original;\n+    });\n+    if (value === undefined) delete process.env[name];\n+    else process.env[name] = value;\n+  }\n+\n+  const { privateKey, publicKey } = await generateKeyPair(\"RS256\");\n+  const { privateKey: untrustedKey } = await generateKeyPair(\"RS256\");\n+  const jwk = await exportJWK(publicKey);\n+  t.mock.method(globalThis, \"fetch\", async (input: string | URL | Request) => {\n+    assert.equal(input, jwksUri);\n+    return Response.json({ keys: [{ ...jwk, kid: \"test-key\", alg: \"RS256\" }] });\n+  });\n+\n+  const now = Math.floor(Date.now() / 1000);\n+  const issueToken = ({\n+    tokenIssuer = issuer,\n+    tokenAudience = audience,\n+    expiresAt = now + 300,\n+    notBefore = now - 60,\n+    key = privateKey,\n+  } = {}) =>\n+    new SignJWT({})\n+      .setProtectedHeader({ alg: \"RS256\", kid: \"test-key\" })\n+      .setIssuer(tokenIssuer)\n+      .setAudience(tokenAudience)\n+      .setIssuedAt(now - 60)\n+      .setNotBefore(notBefore)\n+      .setExpirationTime(expiresAt)\n+      .sign(key);\n+\n+  const rejectedTokens = [\n+    { name: \"manglende token\", token: undefined, error: \"Mangler token\" },\n+    { name: \"tomt token\", token: \"\", error: \"Mangler token\" },\n+    { name: \"vilkårlig streng\", token: \"ikke-et-jwt\", error: \"Ugyldig token\" },\n+    { name: \"utløpt token\", token: await issueToken({ expiresAt: now - 30 }), error: \"Ugyldig token\" },\n+    { name: \"feil issuer\", token: await issueToken({ tokenIssuer: \"https://wrong.test\" }), error: \"Ugyldig token\" },\n+    { name: \"feil audience\", token: await issueToken({ tokenAudience: \"annen-app\" }), error: \"Ugyldig token\" },\n+    { name: \"ugyldig signatur\", token: await issueToken({ key: untrustedKey }), error: \"Ugyldig token\" },\n+    { name: \"token som ikke er gyldig ennå\", token: await issueToken({ notBefore: now + 300 }), error: \"Ugyldig token\" },\n+  ];\n+  for (const { name, token, error } of rejectedTokens) {\n+    await t.test(`avviser ${name} uten saksdata`, async () => {\n+      const headers = token === undefined ? {} : { Authorization: `Bearer ${token}` };\n+      const response = await GET(new Request(\"http://localhost/api/intern/saker\", { headers }));\n+      assert.equal(response.status, 401);\n+      assert.deepEqual(await response.json(), { error });\n+    });\n+  }\n+\n+  await t.test(\"gyldig token gir den interne sakslisten\", async () => {\n+    const token = await issueToken();\n+    const response = await GET(new Request(\"http://localhost/api/intern/saker\", {\n+      headers: { Authorization: `Bearer ${token}` },\n+    }));\n+    assert.equal(response.status, 200);\n+    assert.deepEqual(await response.json(), listInterneSaker());\n+  });\n+});\n*** End Patch\n"
```

</details>

```
Added 1 file(s): <workspace>/tests/intern-saker.test.ts
```


---

<sub>4m 9s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 9s</sub>

### `bash`

**Bekrefte regresjonstest mot sårbar kode**

$ node --test tests/intern-saker.test.ts

<details>
<summary>133 lines</summary>

```
▶ interne saker krever et gyldig Azure-token
  ✔ avviser manglende token uten saksdata (1.969625ms)
  ✖ avviser tomt token uten saksdata (0.585167ms)
  ✖ avviser vilkårlig streng uten saksdata (0.168208ms)
  ✖ avviser utløpt token uten saksdata (0.15825ms)
  ✖ avviser feil issuer uten saksdata (0.231ms)
  ✖ avviser feil audience uten saksdata (0.126459ms)
  ✖ avviser ugyldig signatur uten saksdata (0.102916ms)
  ✖ avviser token som ikke er gyldig ennå uten saksdata (0.097875ms)
  ✔ gyldig token gir den interne sakslisten (1.831458ms)
✖ interne saker krever et gyldig Azure-token (63.230375ms)
ℹ tests 10
ℹ suites 0
ℹ pass 2
ℹ fail 8
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 151.043

✖ failing tests:

test at tests/intern-saker.test.ts:63:13
✖ avviser tomt token uten saksdata (0.585167ms)
  AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
  
  200 !== 401
  
      at TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:66:14)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:63:5)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
    generatedMessage: true,
    code: 'ERR_ASSERTION',
    actual: 200,
    expected: 401,
    operator: 'strictEqual',
    diff: 'simple'
  }

test at tests/intern-saker.test.ts:63:13
✖ avviser vilkårlig streng uten saksdata (0.168208ms)
  AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
  
  200 !== 401
  
      at TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:66:14)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:63:5)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
    generatedMessage: true,
    code: 'ERR_ASSERTION',
    actual: 200,
    expected: 401,
    operator: 'strictEqual',
    diff: 'simple'
  }

test at tests/intern-saker.test.ts:63:13
✖ avviser utløpt token uten saksdata (0.15825ms)
  AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
  
  200 !== 401
  
      at TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:66:14)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:63:5)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
    generatedMessage: true,
    code: 'ERR_ASSERTION',
    actual: 200,
    expected: 401,
    operator: 'strictEqual',
    diff: 'simple'
  }

test at tests/intern-saker.test.ts:63:13
✖ avviser feil issuer uten saksdata (0.231ms)
  AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
  
  200 !== 401
  
      at TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:66:14)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:63:5)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
    generatedMessage: true,
    code: 'ERR_ASSERTION',
    actual: 200,
    expected: 401,
    operator: 'strictEqual',
    diff: 'simple'
  }

test at tests/intern-saker.test.ts:63:13
✖ avviser feil audience uten saksdata (0.126459ms)
  AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
  
  200 !== 401
  
      at TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:66:14)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:63:5)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
    generatedMessage: true,
    code: 'ERR_ASSERTION',
    actual: 200,
    expected: 401,
    operator: 'strictEqual',
    diff: 'simple'
  }

test at tests/intern-saker.test.ts:63:13
✖ avviser ugyldig signatur uten saksdata (0.102916ms)
  AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
  
  200 !== 401
  
      at TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:66:14)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:63:5)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
    generatedMessage: true,
    code: 'ERR_ASSERTION',
    actual: 200,
    expected: 401,
    operator: 'strictEqual',
    diff: 'simple'
  }

test at tests/intern-saker.test.ts:63:13
✖ avviser token som ikke er gyldig ennå uten saksdata (0.097875ms)
  AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
  
  200 !== 401
  
      at TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:66:14)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async TestContext.<anonymous> (<workspace>/tests/intern-saker.test.ts:63:5)
      at async Test.run (node:internal/test_runner/test:1409:7)
      at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
    generatedMessage: true,
    code: 'ERR_ASSERTION',
    actual: 200,
    expected: 401,
    operator: 'strictEqual',
    diff: 'simple'
  }
<shellId: 20 completed with exit code 1>
```

</details>


---

<sub>4m 22s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 22s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/app/api/intern/saker/route.ts\n@@\n-import { getToken } from \"@navikt/oasis\";\n+import { requireSaksbehandler } from \"../../../../lib/auth.ts\";\n@@\n-  if (!getToken(request)) {\n-    return Response.json({ error: \"Mangler token\" }, { status: 401 });\n-  }\n+  const unauthorized = await requireSaksbehandler(request);\n+  if (unauthorized) return unauthorized;\n*** Update File: <workspace>/tests/intern-saker.test.ts\n@@\n-    { name: \"tomt token\", token: \"\", error: \"Mangler token\" },\n+    { name: \"tomt token\", token: \"\", error: \"Ugyldig token\" },\n*** Update File: <workspace>/docs/drift.md\n@@\n - Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres\n   med `requireSaksbehandler()` i `lib/auth.ts`.\n+- `/api/intern/saker` validerer tokenet i route-handleren før saksdata hentes.\n+  Manglende eller ugyldig token gir 401 uten saksdata. `getToken()` henter\n+  tokenet fra forespørselen, men validerer det ikke.\n*** End Patch\n"
```

</details>

```
Modified 3 file(s): <workspace>/app/api/intern/saker/route.ts, <workspace>/tests/intern-saker.test.ts, <workspace>/docs/drift.md
```


---

<sub>4m 25s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 26s</sub>

### `bash`

**Verifisere autentisering og TypeScript etter retting**

$ npm test && npm run typecheck

<details>
<summary>31 lines</summary>

```
npm notice run test
npm notice run node --test tests/*.test.ts
▶ interne saker krever et gyldig Azure-token
  ✔ avviser manglende token uten saksdata (1.985875ms)
  ✔ avviser tomt token uten saksdata (0.779333ms)
  ✔ avviser vilkårlig streng uten saksdata (0.214625ms)
  ✔ avviser utløpt token uten saksdata (1.795917ms)
  ✔ avviser feil issuer uten saksdata (0.5325ms)
  ✔ avviser feil audience uten saksdata (0.353334ms)
  ✔ avviser ugyldig signatur uten saksdata (0.329458ms)
  ✔ avviser token som ikke er gyldig ennå uten saksdata (0.326666ms)
  ✔ gyldig token gir den interne sakslisten (0.908667ms)
✔ interne saker krever et gyldig Azure-token (68.7585ms)
✔ isalive svarer uten token (0.974458ms)
✔ saker krever token (0.523ms)
ℹ tests 12
ℹ suites 0
ℹ pass 12
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 168.7815
npm notice run typecheck
npm notice run tsc --noEmit
tests/intern-saker.test.ts(65,85): error TS2322: Type '{ Authorization?: undefined; } | { Authorization: string; }' is not assignable to type 'HeadersInit | undefined'.
  Type '{ Authorization?: undefined; }' is not assignable to type 'HeadersInit | undefined'.
    Type '{ Authorization?: undefined; }' is not assignable to type 'Record<string, string>'.
      Property 'Authorization' is incompatible with index signature.
        Type 'undefined' is not assignable to type 'string'.
<shellId: 21 completed with exit code 2>
```

</details>


---

<sub>4m 30s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 30s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/tests/intern-saker.test.ts\n@@\n-      const headers = token === undefined ? {} : { Authorization: `Bearer ${token}` };\n+      const headers = new Headers();\n+      if (token !== undefined) headers.set(\"Authorization\", `Bearer ${token}`);\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/tests/intern-saker.test.ts
```


---

<sub>4m 37s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>4m 39s</sub>

### `bash`

**Verifisere tester og typesikkerhet**

$ npm test && npm run typecheck

<details>
<summary>26 lines</summary>

```
npm notice run test
npm notice run node --test tests/*.test.ts
▶ interne saker krever et gyldig Azure-token
  ✔ avviser manglende token uten saksdata (1.943292ms)
  ✔ avviser tomt token uten saksdata (0.754291ms)
  ✔ avviser vilkårlig streng uten saksdata (0.202708ms)
  ✔ avviser utløpt token uten saksdata (1.7475ms)
  ✔ avviser feil issuer uten saksdata (0.478709ms)
  ✔ avviser feil audience uten saksdata (0.338083ms)
  ✔ avviser ugyldig signatur uten saksdata (0.324875ms)
  ✔ avviser token som ikke er gyldig ennå uten saksdata (0.294459ms)
  ✔ gyldig token gir den interne sakslisten (0.996416ms)
✔ interne saker krever et gyldig Azure-token (39.27225ms)
✔ isalive svarer uten token (0.96525ms)
✔ saker krever token (0.552125ms)
ℹ tests 12
ℹ suites 0
ℹ pass 12
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 141.292959
npm notice run typecheck
npm notice run tsc --noEmit
<shellId: 22 completed with exit code 0>
```

</details>


---

<sub>4m 38s</sub>

### `view`

**view the file at <workspace>/app/api/intern/saker/route.ts.**

<workspace>/app/api/intern/saker/route.ts

<details>
<summary>7 lines</summary>

```
import { requireSaksbehandler } from "../../../../lib/auth.ts";
import { listInterneSaker } from "../../../../lib/saker.ts";

export async function GET(request: Request): Promise<Response> {
  const unauthorized = await requireSaksbehandler(request);
  if (unauthorized) return unauthorized;
  return Response.json(listInterneSaker());
}
```

</details>


---

<sub>4m 38s</sub>

### `view`

**view the file at <workspace>/tests/intern-saker.test.ts.**

<workspace>/tests/intern-saker.test.ts

<details>
<summary>75 lines</summary>

```
import assert from "node:assert/strict";
import { test } from "node:test";
import { exportJWK, generateKeyPair, SignJWT } from "jose";
import { GET } from "../app/api/intern/saker/route.ts";
import { listInterneSaker } from "../lib/saker.ts";

test("interne saker krever et gyldig Azure-token", async (t) => {
  const issuer = "https://login.test/tenant/v2.0";
  const audience = "saksoversikt-test";
  const jwksUri = "https://login.test/tenant/keys";
  const env = {
    AZURE_OPENID_CONFIG_ISSUER: issuer,
    AZURE_APP_CLIENT_ID: audience,
    AZURE_OPENID_CONFIG_JWKS_URI: jwksUri,
    IDPORTEN_ISSUER: undefined,
  };
  for (const [name, value] of Object.entries(env)) {
    const original = process.env[name];
    t.after(() => {
      if (original === undefined) delete process.env[name];
      else process.env[name] = original;
    });
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }

  const { privateKey, publicKey } = await generateKeyPair("RS256");
  const { privateKey: untrustedKey } = await generateKeyPair("RS256");
  const jwk = await exportJWK(publicKey);
  t.mock.method(globalThis, "fetch", async (input: string | URL | Request) => {
    assert.equal(input, jwksUri);
    return Response.json({ keys: [{ ...jwk, kid: "test-key", alg: "RS256" }] });
  });

  const now = Math.floor(Date.now() / 1000);
  const issueToken = ({
    tokenIssuer = issuer,
    tokenAudience = audience,
    expiresAt = now + 300,
    notBefore = now - 60,
    key = privateKey,
  } = {}) =>
    new SignJWT({})
      .setProtectedHeader({ alg: "RS256", kid: "test-key" })
      .setIssuer(tokenIssuer)
      .setAudience(tokenAudience)
      .setIssuedAt(now - 60)
      .setNotBefore(notBefore)
      .setExpirationTime(expiresAt)
      .sign(key);

  const rejectedTokens = [
    { name: "manglende token", token: undefined, error: "Mangler token" },
    { name: "tomt token", token: "", error: "Ugyldig token" },
    { name: "vilkårlig streng", token: "ikke-et-jwt", error: "Ugyldig token" },
    { name: "utløpt token", token: await issueToken({ expiresAt: now - 30 }), error: "Ugyldig token" },
    { name: "feil issuer", token: await issueToken({ tokenIssuer: "https://wrong.test" }), error: "Ugyldig token" },
    { name: "feil audience", token: await issueToken({ tokenAudience: "annen-app" }), error: "Ugyldig token" },
    { name: "ugyldig signatur", token: await issueToken({ key: untrustedKey }), error: "Ugyldig token" },
    { name: "token som ikke er gyldig ennå", token: await issueToken({ notBefore: now + 300 }), error: "Ugyldig token" },
  ];
  for (const { name, token, error } of rejectedTokens) {
    await t.test(`avviser ${name} uten saksdata`, async () => {
      const headers = new Headers();
      if (token !== undefined) headers.set("Authorization", `******;
      const response = await GET(new Request("http://localhost/api/intern/saker", { headers }));
      assert.equal(response.status, 401);
      assert.deepEqual(await response.json(), { error });
    });
  }

  await t.test("gyldig token gir den interne sakslisten", async () => {
    const token = await issueToken();
    const response = await GET(new Request("http://localhost/api/intern/saker", {
      headers: { Authorization: `****** },
    }));
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), listInterneSaker());
  });
});
```

</details>


---

<sub>5m 1s</sub>

### Copilot

**Rettet `/api/intern/saker`.** Endepunktet godtok en vilkårlig tokenstreng fordi `getToken()` bare hentet tokenet, uten å validere det.

| # | Severity | File | Lines | Vulnerability | Confidence |
|---|----------|------|-------|---------------|------------|
| 1 | 🟠 HIGH | `app/api/intern/saker/route.ts` | 4-7 før retting | Uvalidert token ga tilgang til interne saksdata | 10/10 |

Handleren bruker nå eksisterende `requireSaksbehandler()` før saksdata hentes. Manglende eller ugyldig token gir 401, mens gyldig token beholder samme svar. Kontrollen ligger i handleren fremfor proxyen, slik at beskyttelsen ikke avhenger av proxy-matcheren; eksisterende tilgangspolicy er uendret.

Lagt til regresjonstester med lokalt genererte, signerte tokens og JWKS uten eksterne kall. Oppdatert `docs/drift.md`.

🔴 Rød sone: autentisering, forstå denne kontrollen grundig.

**Sjekkliste for rød sone**
- [ ] Gå gjennom token-valideringen før saksdata hentes. Forklar for deg selv hvorfor `getToken()` ikke beviser gyldig innlogging.

---

<sub>5m 1s</sub>

### Info

Session limit reached. Increase or unset the session limit to continue.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
