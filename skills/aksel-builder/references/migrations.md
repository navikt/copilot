# Migrations & codemods

Aksel ships codemods to automate version upgrades. Don't hand-migrate what a codemod can do,
and don't rely on memory for which exist — **list them from MCP**, since the set grows.

## Find the right codemod

```jsonc
// aksel_find_docs
{ "query": "v8", "kind": "migrations" } // or "7->8", a keyword, or omit query for all
```

The response includes the CLI version, the run command, and matching codemods (each with a
`name`, `description`, and sometimes a `warning`). You can also read the whole set from the
`aksel-migrations://catalog` resource.

For any upgrade/breaking-change/codemod question, use `kind:"migrations"` —
`find_docs` with `kind:"docs"` is the wrong tool and will hint you to switch.

## Run a codemod

```bash
pnx @navikt/aksel codemod <name>
```

After running:

- **Read the printed warnings.** Some codemods can't safely transform every case and insert
  a `TODO: Aksel …` comment where manual follow-up is needed — search for those.
- **Re-validate** the result against this skill's rules (tokens real, no raw values, a11y
  props intact). Codemods change syntax; they don't guarantee idiomatic final code.
- Commit before running so the diff is easy to review.

## v8 highlights (what changed, and the codemod for it)

These are the major v8 shifts you'll most often need to apply. Confirm the current list via
MCP, but as of v8:

| Change                                                                | Codemod                                                                                                                         |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Token usage moved to the new system; CSS prefix `--a-*` → `--ax-*`    | `v8-tokens`                                                                                                                     |
| Border-radius tokens renamed `--ax-border-radius-*` → `--ax-radius-*` | (part of token migration)                                                                                                       |
| `Box` legacy tokens → new token system                                | `v8-box`                                                                                                                        |
| `Box.New` / `BoxNew` → `Box`                                          | `v8-box-new`                                                                                                                    |
| Primitives updated to new `space-*` tokens                            | `v8-primitive-spacing`                                                                                                          |
| Deprecated props removed                                              | `v8-prop-deprecate`                                                                                                             |
| `List` restructured (wraps in `Box`, moves `title`/`description`)     | `v8-list`                                                                                                                       |
| `variant` split into `variant` + `data-color` on several components   | `v8-tag-variant`, `v8-toggle-group-variant`, `v8-accordion-variant`, `v8-chips-variant`, `v8-button-variant`, `v8-link-variant` |

The recurring v8 theme: **color moved out of `variant` into `data-color`**, and the **token
system was renamed** (`--a-` → `--ax-`, new `space-*` scale, `radius-*` instead of
`border-radius-*`). If you see `--a-*` variables, a `Box.New`, or a `variant` prop that
encodes a color, the code predates v8 — migrate it.

## Spotting outdated code

| Outdated signal                  | Modern equivalent                                    |
| -------------------------------- | ---------------------------------------------------- |
| `--a-*` CSS variables            | `--ax-*` ([tokens-styling.md](tokens-styling.md))    |
| `Box.New` / `BoxNew`             | `Box` ([primitives-layout.md](primitives-layout.md)) |
| `variant="…"` that names a color | `variant` (emphasis) + `data-color` (role)           |
| Numeric/px spacing on primitives | `space-*` tokens                                     |
| `LinkPanel`, `Panel`             | deprecated — check docs for the current component    |

When you encounter outdated patterns while editing, prefer running the matching codemod over
piecemeal hand-edits, then review the diff.
