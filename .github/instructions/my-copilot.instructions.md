---
applyTo: "apps/my-copilot/**"
---

# my-copilot instructions

## Authentication and BFF boundary

- **Route protection (`src/proxy.ts` and `.nais/app.yaml`)**: Every new page or `route.ts` handler under `src/app/` must either be protected by `isPrivatePath` in `src/proxy.ts` or explicitly classified in `src/proxy.test.ts` and `autoLoginIgnorePaths` in `apps/my-copilot/.nais/app.yaml`.
- **Defense in depth in API routes**: Private `/api/*` route handlers (`usage`, `credits`, `budget`, `team-year`, `survey/[id]/responses`) must verify `await getUser(false)` and return `401` when unauthenticated, in addition to the proxy check.
- **Local dev bypass safety**: `isLocalDev()` in `src/lib/backend-api.ts` must require `process.env.NODE_ENV === "development"` and absence of `NAIS_CLUSTER_NAME` before falling back to `DEV_COPILOT_API_TOKEN`.
- **Upstream error sanitization**: Do not forward raw `error.message` strings from upstream fetches to browser clients in JSON responses. Log the error server-side and return a generic status message.
- **Install redirect validation (`src/lib/install-redirect.ts`)**: `/install/[type]` redirects to `vscode:` or `vscode-insiders:` URIs must pass `isAllowedInstallUrl`, which enforces a single `url` query parameter pointing to `https://raw.githubusercontent.com/navikt/` with no path traversal (`..`, encoded slashes, backslashes), credentials, query string, or fragment.

## UI, Aksel, and content conventions

- **Aksel spacing tokens**: Use Aksel primitives (`Box`, `VStack`, `HStack`, `Bleed`) with Aksel spacing tokens (`space-*`). Do not use Tailwind `p-*` or `m-*` spacing classes.
- **Accessibility and ARIA**: Prefer native HTML elements (`<button>`, `<nav>`, `<dialog>`, `<details>`) and Aksel components over custom `role=` attributes. `hooks/ask-first-aria.py` blocks unapproved interactive ARIA roles in `src/**/*.tsx`.
- **Language and terminology**: Follow `apps/my-copilot/ORDBOK.md` for Norwegian domain terms (`ferdighet` for skill, `instruksjon` for instruction, `agent` for agent). Keep `/cplt` (`src/app/cplt/`) in English (`lang="en"`) with its dark theme.
- **Link inventory**: If a change adds or updates routes, anchors, or site links, run `pnpm test src/link-inventory.test.ts` (and `pnpm link-inventory:update` when adding valid new links).
