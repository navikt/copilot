# Copilot Code Review Instructions

## Norwegian text (`.github/**/*.md`, `docs/**/*.md`)

Flag these in Norwegian markdown files:

**AI-generated text markers — remove or rewrite:**

- Svulstige ord: "banebrytende", "revolusjonerende", "robust", "sømløs", "holistisk"
- Åpningsfraser: "det er verdt å merke seg", "i dagens verden", "la oss dykke ned i"
- Avslutningsfraser: "oppsummert kan man si at", "kort sagt", "avslutningsvis"
- Overgangsord som avsnittåpnere: "Videre", "Dessuten", "I tillegg"
- Overskrifter som alle ender med kolon er et AI-tegn — varier

**Klarspråk:**

- Substantivsyke: "gjennomføring av vurdering" → "vi vurderer"
- Passiv form: "det benyttes" → "vi bruker"
- Fyllord: "i bunn og grunn", "i stor grad", "på mange måter"
- Start med poenget, ikke bakgrunn

**Anglismer — bruk norsk:**

- "adressere et problem" → "løse", "fikse"
- "delivere" → "levere"
- "ta eierskap til" → "ha ansvar for"
- "per dags dato" → "nå", "i dag"
- "involvere" (overbrukt) → "ta med", "inkludere"

**Behold engelsk fagspråk:** deploy, pipeline, cluster, pod, container, endpoint, token, pull request, merge, commit, branch, workflow, runtime, framework. Ikke oversett disse.

**Sammensatte ord:** Bindestrek ved engelsk+norsk: "image-bygg", "CI-pipeline", "Postgres-operatoren", "PR-er". Aldri særskriving: ❌ "Postgres operatoren".

**Nav** skrives alltid "Nav", aldri "NAV".

## TypeScript/React (`apps/my-copilot/**`)

- **Spacing:** Use Aksel spacing props/tokens (`Box paddingBlock="space-16"`, `VStack gap="space-8"`), not Tailwind `p-*`/`m-*` for component spacing. Layout utilities like `mx-auto`, `max-w-7xl` are OK.
- **UI text:** Follow `apps/my-copilot/ORDBOK.md` for Norwegian terminology. Use existing format helpers (e.g., `formatNumber`) — don't introduce ad hoc locale formatting.
- **Accessibility:** Semantic HTML, heading hierarchy without skipping levels, accessible names on icon buttons (`title` or sr-only text), no `<div onClick>` without `role` and `tabIndex`.
- **Server components by default.** Push `"use client"` as low as possible. Use `Promise.all()` for independent data fetches.

## Go (`apps/mcp-*/**`, `apps/copilot-*/**`)

- Wrap errors: `fmt.Errorf("context: %w", err)` — never `%v` for wrapping
- Structured logging with `slog` — never `fmt.Println` or `log.Println`
- No PII in logs (fødselsnummer, name, address — use opaque IDs)
- Sanitized error responses: log details server-side, return generic messages to clients

## Security (all code)

- SQL queries must be parameterized (`$1`, `?`) — never string concatenation
- No secrets, tokens, or credentials in code
- No `InsecureSkipVerify: true`
- GitHub Actions pinned to full SHA with version comment, not tags

## Generated files

Do not edit manually — run `mise run generate` to regenerate:

- `apps/my-copilot/src/lib/copilot-manifest.json`
- `apps/mcp-onboarding/internal/discovery/copilot-manifest.json`
- `docs/README*.md`
