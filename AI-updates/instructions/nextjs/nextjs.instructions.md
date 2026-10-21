---
description: "Generelle Next.js App Router-regler: versjonstilpassede API-er, server-/klientgrenser, data, autentisering, feil og prosjektverktøy."
applyTo: "**/*.{tsx,ts}"
---

# Next.js application guidance

Apply only to Next.js projects. This is the generic guidance separated from the Aksel
instruction; the destination's Next.js owner should review it before installation.
Use the project's installed Next.js and React versions, not an assumed Nav-wide stack.

## Read the matching documentation

Before changing Next.js behavior, read the relevant installed documentation under
`node_modules/next/dist/docs/` when available. Otherwise use official Next.js docs for
the installed version. Check enabled features and configuration before applying caching
or routing advice. Aksel-specific setup belongs in `nextjs-aksel` and `aksel-builder`.

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
