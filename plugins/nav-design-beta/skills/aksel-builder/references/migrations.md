# Migrations & codemods

Aksel ships codemods to automate version upgrades. Don't hand-migrate what a codemod can do,
and don't rely on memory for which exist — **list them from MCP**, since the set grows.

## Find the right codemod

```jsonc
// aksel_find_docs
{ "query": "v8", "kind": "migrations" } // or "7->8", a keyword, or omit query for all
```

The response includes a run command and matching codemods (each with a `name`,
`description`, and sometimes a `warning`). Its `cliVersion` field may not match the installed
`@navikt/aksel` CLI; don't use it to select a CLI release. Check the intended CLI version and read the matching migration guide.
You can also read the catalog from `aksel-migrations://catalog`.

Use `kind:"migrations"` to find codemods. Use `kind:"docs"` or the docs index to find
the migration guide and manual steps; a codemod catalog is not a complete upgrade guide.

## Run a codemod

```bash
npx @navikt/aksel codemod <name>
```

Use the project's approved package runner and chosen CLI version. Review the working
tree before running a codemod; preserve unrelated changes and don't commit without a
request.

After running:

- **Read the printed warnings.** Some codemods can't safely transform every case and insert
  a `TODO: Aksel …` comment where manual follow-up is needed — search for those.
- **Re-validate** the result against this skill's rules (tokens real, installed APIs, a11y
  props intact). Codemods change syntax; they don't guarantee idiomatic final code.
- Review the diff and run the project's relevant checks.

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
encodes a color, check the installed major version and upgrade scope before migrating it.

## Spotting outdated code

| Outdated signal                  | Modern equivalent                                    |
| -------------------------------- | ---------------------------------------------------- |
| `--a-*` CSS variables            | `--ax-*` ([tokens-styling.md](tokens-styling.md))    |
| `Box.New` / `BoxNew`             | `Box` ([primitives-layout.md](primitives-layout.md)) |
| `variant="…"` that names a color | `variant` (emphasis) + `data-color` (role)           |
| Numeric/px spacing on primitives | `space-*` tokens                                     |
| `Alert`, `LinkPanel`, `Panel`    | deprecated — check docs for the current component    |

For an agreed migration, prefer the matching codemod over piecemeal hand-edits, then review
the diff. Don't turn a small feature request into an unrelated app-wide upgrade.
