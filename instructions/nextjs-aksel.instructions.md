---
description: "Next.js App Router og Aksel: versjonstilpassede API-er, server-/klientgrenser, data, autentisering, CSS og tilgjengelighet via aksel-builder."
applyTo: "src/**/*.{tsx,ts}"
---

# Next.js and Aksel

Apply only to Next.js projects; the Aksel section applies only when the project uses
Aksel. Keep framework guidance and Aksel integration in this single instruction.
Use the project's installed Next.js and React versions, not an assumed Nav-wide stack.

## Read the matching documentation

Before changing Next.js behavior, read the relevant installed documentation under
`node_modules/next/dist/docs/` when available. Otherwise use official Next.js docs for
the installed version. Check enabled features and configuration before applying caching
or routing advice. For Aksel APIs and implementation workflows, load `aksel-builder`.

## Aksel integration

Load `aksel-builder` and use the Aksel MCP to verify APIs against the project's installed
versions. If MCP is unavailable, use the skill's Preflight fallback; don't guess imports,
props, tokens, or icon names. Reference paths below belong to that skill.

- **CSS:** import `@navikt/ds-css` once at the app entry, before application overrides.
  For Tailwind, follow `references/tokens-styling.md` instead of importing the stylesheet
  twice. Native Tailwind layout and spacing are valid unless the project restricts them.
- **Server/client boundary:** keep the root layout server-side. Put interactive providers
  and locale objects in a client wrapper with `"use client"` first. Use the App Router
  example in `references/setup-and-imports.md`.
- **Compound components:** in Server Components, use named parts from the documented
  component subpath instead of accessing client exports through `Parent.Child`.
  Confirm installed exports; the MCP props tool does not return an export map.
- **Theme and language:** `Theme` controls color mode; `Provider` controls locale/portals.
  Match document `lang`, content, and locale. Semantic tokens work across themes; don't
  describe a token as a fixed light-mode color.
- **Accessible composition:** follow `references/accessibility.md` for labels, heading
  levels, error focus, and keyboard behavior. Aksel does not guarantee the accessibility
  of the complete page.

## Server and client boundaries

- Prefer Server Components for server data and secrets. Use `"use client"` as the first
  statement in a separate file for event handlers, state, or browser APIs.
- Pass serializable data across the boundary; don't pass secrets or arbitrary functions.
- On current Next.js versions, await request APIs such as `cookies()`, `headers()`,
  route `params`, and page `searchParams`. Check the installed version's signatures.
- Keep `metadata` and `generateMetadata` in server files; don't expose private data in
  titles, descriptions, or social metadata.

## Data, mutations, and authentication

- Reuse the project's server data layer. Validate inputs and authorize access at each
  Route Handler, Server Action, or data boundary; redirects in Proxy are not sufficient.
- For Next.js 16, use the `proxy.ts` / `proxy` convention; older projects may still use
  `middleware.ts`. Don't rename files without checking the installed version.
- Use the existing auth helper only after reading its implementation. Don't assume that
  a helper named `getUser` exists or that a boolean argument controls unauthorized access.
- Route Handlers should return intentional status codes and safe public error messages.
  Log internal errors through the project's logger without leaking credentials or data.
- Treat `FormData.get()` as `string | File | null`; validate it instead of casting.
  If using `useActionState`, the action receives previous state before the payload;
  check React support and match the action's signature.
- Select cache/revalidation behavior explicitly from the app's configuration and data
  requirements. Don't assume framework defaults or introduce a caching library by habit.

## Loading and errors

Keep `loading.tsx` and `error.tsx` in separate files. An error boundary is a Client
Component with `"use client"` first; provide a useful recovery action and report errors
through the existing observability setup. Don't display raw server exception messages.

Place Suspense around the part that can stream independently. Keep headings, navigation,
and other stable content outside a boundary when appropriate; loading states still need
accessible names and useful layout.

## Project conventions

- Use the repository's package manager, lockfile, scripts, and file naming. Don't impose
  pnpm, React Query, React Hook Form, or a comments policy across all Nav applications.
- If React Query already exists, reuse its query keys and client setup, check HTTP status
  before parsing responses, and invalidate affected queries after successful mutations.
  Server Components may be sufficient without it.
- If React Hook Form already exists, use its established validation and submit pattern.
  For simpler forms, consider the project's existing server-action approach instead.
- Match the UI locale. For Norwegian number formatting, use
  `new Intl.NumberFormat("nb-NO").format(value)` or a verified shared helper; don't assume
  a particular Unicode space in formatting tests.
- Run relevant existing lint, type, and behavior checks. Cover meaningful behavior and
  failure paths; don't add test infrastructure for an isolated example.
- Keep changes scoped. Confirm consequential auth, caching, or dependency changes;
  preserve unrelated code and explain blockers.
