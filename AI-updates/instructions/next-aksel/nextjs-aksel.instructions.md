---
description: "Aksel i Next.js App Router: MCP-verifiserte API-er, CSS, server-/klientgrenser, sammensatte komponenter og språk."
applyTo: "**/*.{tsx,ts}"
---

# Aksel in Next.js

Apply only to Next.js projects that use Aksel. **Load `aksel-builder` first** and use
the Aksel MCP to verify APIs against the project's installed versions. If unavailable,
use the skill's Preflight fallback; don't guess imports, props, tokens, or icon names.

- **CSS:** import `@navikt/ds-css` once at the app entry, before application overrides.
  For a Tailwind integration, follow `references/tokens-styling.md` instead of importing
  the stylesheet twice. Native Tailwind layout and spacing are valid.
- **Server/client boundary:** keep the root layout server-side. Put interactive providers
  and locale objects in a client wrapper with `"use client"` first. Use the complete
  App Router example in `references/setup-and-imports.md`.
- **Compound components:** in Server Components, import named parts from the documented
  component subpath, such as `DialogHeader` from `@navikt/ds-react/Dialog`, instead of
  accessing client exports as `Dialog.Header`. Confirm installed exports; the MCP props
  tool does not return an export map.
- **Theme and language:** `Theme` controls color mode; `Provider` controls locale/portals.
  Match document `lang`, content, and locale. Semantic tokens work across themes; don't
  describe a token as a fixed light-mode color.
- **Accessible composition:** follow the skill's accessibility reference for labels,
  heading levels, error focus, and keyboard behavior. Aksel does not guarantee the
  accessibility of the complete page.

Generic Next.js routing, auth, data fetching, caching, and project tooling belong in the
project's Next.js instructions, not this Aksel integration layer.
