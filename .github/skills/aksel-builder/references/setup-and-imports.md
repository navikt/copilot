# Setup & imports

Aksel ships a few `@navikt/*` packages with simple, stable import paths. This is the curated
rule set; confirm any specific component or token via the MCP ([mcp-workflow.md](mcp-workflow.md)).

## Packages

| Package               | What it provides                                          | Typical import                                    |
| --------------------- | --------------------------------------------------------- | ------------------------------------------------- |
| `@navikt/ds-react`    | React components, layout primitives etc                   | `import { Button } from "@navikt/ds-react"`       |
| `@navikt/ds-css`      | Compiled CSS for all components (required for styling)    | `import "@navikt/ds-css"`                         |
| `@navikt/ds-tokens`   | Raw token values for SCSS/JS/LESS (CSS already in ds-css) | `@use "@navikt/ds-tokens/scss";`                  |
| `@navikt/aksel-icons` | Icon components                                           | `import { TrashIcon } from "@navikt/aksel-icons"` |
| `@navikt/ds-tailwind` | Tailwind preset mapping Aksel tokens to `ax-*` utilities  | `presets: [require("@navikt/ds-tailwind")]`       |
| `@navikt/aksel`       | CLI: codemods                                             | `pnx @navikt/aksel <command>`                     |

All public `@navikt/*` runtime packages are kept **React 17-compatible**.

## React components — import from the package root

```tsx
import {
  BodyShort,
  Box,
  Button,
  HStack,
  Heading,
  Modal,
  TextField,
  VStack,
} from "@navikt/ds-react";
```

Almost everything lives on the package root: components, the layout primitives
(`Box`, `HStack`, `VStack`, `Stack`, `Spacer`, `HGrid`, `Page`, `Bleed`, `Show`, `Hide`),
typography (`Heading`, `BodyShort`, `BodyLong`, `Ingress`, `Detail`, `Label`,
`ErrorMessage`), `Theme`, and `Provider`.

Per-component entry points also exist for fine-grained code-splitting
(e.g. `@navikt/ds-react/Box`), and preview/unstable APIs live under a `PREVIEW` subpath
(e.g. `@navikt/ds-react/PREVIEW`). Prefer the root import unless you have a specific reason
to deep-import. **Never import from build output folders** (`esm/`, `cjs/`).

**Server Components (Next.js App Router):** compound components are `"use client"`, and
their subparts are exposed as **named exports on the component's own subpath** — not on the
package root (the root re-exports only the parent value and the subpart _types_). Import the
subparts from the subpath instead of relying on `Parent.Child` dot-notation:

```tsx
// Subparts come from the component subpath, not the package root:
import { Dialog, DialogHeader, DialogTitle } from "@navikt/ds-react/Dialog";
import { InfoCard, InfoCardTitle } from "@navikt/ds-react/InfoCard";

// import { DialogHeader } from "@navikt/ds-react";  ❌ root exports the TYPE only, not the value
```

Subpath names vary per component — confirm the exact import with `aksel_get_component_info`
or the component's `aksel_get_doc` page rather than guessing.

## CSS is required

Components are unstyled without the stylesheet. Import it **once** at your app entry, before
your own styles:

```tsx
// app entry (e.g. main.tsx, _app.tsx, layout.tsx)
import "@navikt/ds-css";
```

`@navikt/ds-css` **already bundles the design tokens** (fonts, `--ax-*` variables, reset,
baseline), so a typical React app needs nothing more — you do **not** import `@navikt/ds-tokens`
separately for CSS variables to resolve. Import `@navikt/ds-tokens` only when you need raw
token values in SCSS/JS/LESS (`@navikt/ds-tokens/scss`, `/js`, `/less`). To pull in only
parts of the global styles, see the `aksel_get_doc` page for "CSS-import".

## Provider — locale and portals

Wrap the app in `Provider` when you need non-default locale (Aksel defaults to Norwegian
bokmål, `nb`) or a shared portal root for `Modal`, `Tooltip`, and `ActionMenu`.

```tsx
import { Provider } from "@navikt/ds-react";
import { en } from "@navikt/ds-react/locales";

<Provider locale={en}>{app}</Provider>;
```

- `locale` accepts an Aksel locale object (`nb`, `nn`, `en`) from `@navikt/ds-react/locales`.
- `translations` lets you override individual strings (single object or array). It must be
  used together with `locale`.
- `rootElement` sets the global portal target.

`Provider` is about locale/portals. It is **not** the theming wrapper — light/dark and base
color are handled by `Theme` (see [theming.md](theming.md)).

## CSS-only consumers (no React)

Teams not on React can use `@navikt/ds-css` plus the token stylesheet and apply Aksel's
class names directly. Get the authoritative class names and markup from the component docs
via MCP rather than guessing them — `aksel_get_doc` pages include the rendered markup.
Tokens are available as `--ax-*` CSS variables (see [tokens-styling.md](tokens-styling.md)).

## Quick-start checklist

- [ ] `@navikt/ds-css` imported once at the app entry.
- [ ] App wrapped in `Theme` for color mode / base color (see [theming.md](theming.md)).
- [ ] `Provider` added only if you need non-`nb` locale or a shared portal root.
- [ ] Components imported from `@navikt/ds-react` root; icons from `@navikt/aksel-icons`.
- [ ] No imports from `esm/` or `cjs/` build folders.
