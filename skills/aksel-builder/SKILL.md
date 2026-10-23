---
name: aksel-builder
description: Build and refactor UI with Aksel, Nav's design system. Use for frontend tasks involving Aksel, Nav/Navikt, or @navikt/ds- and @navikt/aksel-packages, and for Figma-to-code with Aksel. Covers components, layout, tokens, theming, icons, Tailwind, accessibility, and migrations. Do not use when the user opts out of Aksel.
license: MIT
metadata:
  domain: frontend
  tags: aksel design-system nav react spacing tokens layout responsive primitives icons tailwind figma figma-to-code design-to-code
---

# Aksel design system skill

## Mission

Build correct, accessible, production-quality UI with the **Aksel design system** (Nav).
Lead the user to the right component, token, and primitive — and back every choice with the
**Aksel MCP server**, not memory. Its 5 tools and 4 resources combine cached live docs
from `aksel.nav.no` with token, icon, and migration catalogs bundled with the server.
Check the project's installed versions before applying newer APIs. Aim for the
_fewest correct calls_.

---

## MCP-First rule (hard rule)

> **Never generate, edit, or debug Aksel code from training memory alone.**

Aksel moves fast — token names, props, and whole components change between majors, and your
weights are stale. Before writing any import, prop, token, or icon name:

1. **Confirm it exists** and how it's used via the MCP.
2. **Never invent** doc paths, prop names, token names, or icon exports.
3. **Verified sources win** over memory. The MCP describes current docs and its bundled
   catalogs, not necessarily the project's installed version. Resolve mismatches against
   installed package types/exports and version-specific documentation. If no source
   confirms an API, say what you tried — don't fall back to a guess.

This one rule prevents the most common failure: confidently shipping an Aksel API that no
longer exists.

---

## Preflight: is the MCP available?

This skill is built around the Aksel MCP. **Before your first lookup, check whether the
`aksel_*` tools exist**, and branch:

**✅ MCP available (strongly preferred).** Use the tools below for structured props,
token values, and icon names. They do not inspect the consuming project's dependencies:
confirm version-sensitive APIs against the installed packages.

The environment must allow `https://aksel-mcp.nav.no`. If access is blocked, report it
and use the fallback below; don't work around access restrictions.

**❌ MCP not available → recommend installing it, then fall back to fetching docs.**

1. **Recommend the MCP.** Find `io.github.navikt/aksel-mcp` in Nav's registry
   (`https://mcp-registry.nav.no`) and use the client's installation flow. The endpoint is
   `https://aksel-mcp.nav.no/mcp`. For VS Code, `.vscode/mcp.json` uses:

```json
{
  "servers": {
    "io.github.navikt/aksel-mcp": {
      "type": "http",
      "url": "https://aksel-mcp.nav.no/mcp"
    }
  }
}
```

Merge the entry into existing configuration; don't replace other servers.
Other clients use different configuration keys; see the
[VS Code format reference](https://code.visualstudio.com/docs/agents/reference/mcp-configuration).

2. **Until it's installed, fall back to the public LLM docs over HTTP.** Start from the
   index at `https://aksel.nav.no/llm.md` — it lists every page as an individual `.md` file
   — then fetch the specific `.md` page you need. These are the docs used by the MCP's
   documentation tools, not a replacement for its structured token/icon catalogs.
   Inspect installed exports and token files when prose is insufficient; never invent
   paths, props, tokens, or icon names.

### Tool → manual-fetch fallback

When the MCP is absent, map each tool to a direct fetch:

| MCP tool (preferred)             | Without MCP, fetch instead                                                                                    |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `aksel_find_docs` (`docs`)       | `https://aksel.nav.no/llm.md` index → follow the matching `.md` link                                          |
| `aksel_get_doc`                  | the page's `.md` URL directly (e.g. `https://aksel.nav.no/komponenter/core/button.md`)                        |
| `aksel_get_component_info`       | the component's `.md` page — it documents props & usage (prose, not a structured table)                       |
| `aksel_get_token_details`        | `https://aksel.nav.no/grunnleggende/styling/design-tokens.md`                                                 |
| `aksel_find_icons`               | `https://aksel.nav.no/komponenter/ikoner.md`                                                                  |
| `aksel_find_docs` (`migrations`) | `https://aksel.nav.no/grunnleggende/kode/codemods-config.md`; discover the version-specific guide in `llm.md` |

Prefer fetching individual `.md` pages over the full collection — only pull `llm.md` (or
`komponenter.md`) when you need to discover the right page.

---

## Tool capability matrix

Route by _intent_. Pick the narrowest tool. The default loop is **Find → Fetch**.

| Intent                                  | Tool                       | Key args                           | Returns                        | Common failure to avoid                         |
| --------------------------------------- | -------------------------- | ---------------------------------- | ------------------------------ | ----------------------------------------------- |
| Find the doc/component path for a topic | `aksel_find_docs`          | `{ query, kind:"docs" }`           | ranked `results[]` with `path` | Guessing a path instead of searching first      |
| Read a full doc page                    | `aksel_get_doc`            | `{ path }` (exact, from find)      | markdown                       | Hand-writing a path; must start `/`, end `.md`  |
| Get a component's props/API             | `aksel_get_component_info` | `{ component }` slug or path       | structured props               | Reading prose docs when you need the prop table |
| Look up a design token's value/usage    | `aksel_get_token_details`  | `{ tokenName }`                    | value, accessors, semantics    | Using `find_docs` for tokens — wrong tool       |
| Browse tokens by keyword                | `aksel_find_docs`          | `{ query, kind:"tokens" }`         | token catalog matches          | Then call `get_token_details` for the exact one |
| Find an icon                            | `aksel_find_icons`         | `{ keyword, category?, variant? }` | `icons[]` with `name`          | Assuming an icon export name from memory        |
| Find a migration/codemod                | `aksel_find_docs`          | `{ query, kind:"migrations" }`     | codemods + run command         | Using `kind:"docs"` for upgrade questions       |

**Resources** (read directly when you want the whole catalog, not a search):
`aksel-docs://index`, `aksel-tokens://catalog`, `aksel-icons://category-catalog`,
`aksel-migrations://catalog`.

---

## Core workflow: Find → Fetch → Build → Validate

Most tasks need 2–4 MCP calls.

1. **Find** — `aksel_find_docs({ query })` with 1–2 keywords (component names work best;
   Norwegian and English both work). Use the returned `path`; never build one yourself.
   Switch `kind`: tokens/colors/spacing → `"tokens"`, upgrades/codemods → `"migrations"`.
2. **Fetch** — `get_doc` (usage, examples, props as prose), `get_component_info` (props as
   JSON), `get_token_details` (one token), `find_icons` (then map `name` → `${name}Icon`).
3. **Build** — use semantic tokens for design-system styling; primitive props, CSS, and
   the project's existing Tailwind utilities are valid choices, subject to local rules
   ([tokens-styling.md](references/tokens-styling.md)); reach for layout primitives before
   custom CSS ([primitives-layout.md](references/primitives-layout.md)); add the a11y props
   Aksel requires ([accessibility.md](references/accessibility.md)).
4. **Validate** — run the checklist at the end of this file.

---

## Decision tree: route the task

The Find → Fetch → Build → Validate loop is _how_ you work; this is _where_ to go for a
given task. Orient once, then take the matching branch(es). **Load a reference the first
time a task touches its domain — don't preload all of them.** All reference files live in
`references/` (e.g. `references/components.md`).

**0. Orient (once per task).** Glance at the project before building:

- **Are the `aksel_*` MCP tools available?** If not, see **Preflight** above — encourage
  installing the MCP, and until then fall back to fetching `https://aksel.nav.no/llm.md`.
- Is Aksel installed and **which major version**? Check `package.json` for `@navikt/ds-react`
  (its version drives which API is current). Unsure about setup, imports, or SSR/RSC?
  → load **setup-and-imports.md**.

**Then branch by intent** (reference to load → tools to call):

| If the task is…                                  | Load                   | Then call                                                                                        |
| ------------------------------------------------ | ---------------------- | ------------------------------------------------------------------------------------------------ |
| Pick/build a component (form, modal, table, …)   | `components.md`        | `find_docs` → `get_component_info` → `get_doc` (usage) → build                                   |
| Implement a **Figma design** → Aksel code        | `figma-to-code.md`     | Figma MCP (`get_design_context`/`get_screenshot`) → map to Aksel via the `aksel_*` tools → build |
| Lay out / space / make responsive                | `primitives-layout.md` | build with primitives (`get_component_info` for a primitive's API)                               |
| Color / token / spacing value / Tailwind styling | `tokens-styling.md`    | `get_token_details` (browse via `find_docs` `kind:"tokens"`)                                     |
| Light/dark mode or base color                    | `theming.md`           | `get_component_info` / `get_doc` for `Theme` when not already verified                           |
| An icon                                          | `icons.md`             | `find_icons` → map `name` → `${name}Icon`                                                        |
| Accessibility / labels / a11y review             | `accessibility.md`     | `get_component_info` to find the label/description props                                         |
| Upgrade / codemod / “why is this deprecated?”    | `migrations.md`        | `find_docs` `kind:"migrations"`                                                                  |
| Project setup / imports / packages / SSR         | `setup-and-imports.md` | `find_docs` `kind:"docs"` (`"kom i gang"`)                                                       |
| A tool call misbehaves / returns nothing         | `mcp-workflow.md`      | re-route per its recovery tables                                                                 |

**Complex tasks chain branches.** “Dark-mode contact form with a trash icon” touches
`components.md` + `tokens-styling.md` + `theming.md` + `icons.md` + `accessibility.md` —
load each as you reach that part, and build in that order (structure → style → theme →
icons → a11y pass).

---

## Reference files

The decision tree above says _when_ to load each file; this is the index of _what each
contains_. Load on demand to keep context small.

- [references/mcp-workflow.md](references/mcp-workflow.md) — tool mechanics, `kind` switching, empty-result recovery, anti-hallucination rules.
- [references/setup-and-imports.md](references/setup-and-imports.md) — packages, import paths, locale `Provider`, SSR/RSC, CSS-only.
- [references/primitives-layout.md](references/primitives-layout.md) — `Box`/`HStack`/`VStack`/`HGrid`/`Page`/`Bleed`, responsive props, spacing & radius scales.
- [references/tokens-styling.md](references/tokens-styling.md) — token-props vs `--ax-` variables, color roles, styling discipline, Tailwind preset.
- [references/theming.md](references/theming.md) — light/dark mode, the `Theme` component, `data-color`.
- [references/components.md](references/components.md) — choosing the right component; form & composition patterns.
- [references/figma-to-code.md](references/figma-to-code.md) — Figma → Aksel code: Code Connect, confidence matrix, HTML→primitive/token mapping, validation. Needs the Figma MCP.
- [references/icons.md](references/icons.md) — finding icons, `name` → `${name}Icon`, sizing, a11y.
- [references/accessibility.md](references/accessibility.md) — required a11y props, semantic structure, Norwegian content.
- [references/migrations.md](references/migrations.md) — upgrading versions, codemods, v7→v8 highlights.

---

## Call-budget & token conservation

- Prefer **Find → Fetch** (2 calls) over scattershot searching; stop once you can build.
- `find_docs` (`kind:"docs"`) needs **≥ 3 characters**; if it returns nothing, retry once
  with a single broader keyword (a component name) before changing approach.
- Don't re-fetch a doc you already have. Don't echo raw tool JSON back — extract and build.
- ≤ 4 calls is the target, not a limit. Still stuck after ~7? Stop and ask the user rather
  than guessing.

---

## Final validation checklist

- [ ] Aksel components and icons were confirmed via MCP or the documented fallback, and match installed versions.
- [ ] Doc paths came from `aksel_find_docs`, `aksel-docs://index`, or `llm.md` — none were guessed.
- [ ] Non-trivial props were checked with `aksel_get_component_info` or, in fallback mode, official docs and installed types.
- [ ] Aksel tokens are verified; custom values have a real requirement rather than replacing an existing token.
- [ ] Icon imports use the `${name}Icon` export from `@navikt/aksel-icons`.
- [ ] Required accessibility props are present (labels, descriptions, alt text).
- [ ] Imports resolve against installed packages. React compatibility follows the app; React 17 compatibility is required when contributing to Aksel's published React library.
