# MCP workflow & anti-hallucination

The Aksel MCP server is the authoritative source for everything in this skill. This file
explains how to drive it efficiently and how to recover when a call comes back empty.

> **No MCP installed?** Recommend adding it (install config is in `SKILL.md` → Preflight),
> then fall back to the public docs: start at `https://aksel.nav.no/llm.md` (an index of
> every page as an individual `.md`) and fetch the specific `.md`. The Preflight section has
> the full tool → fetch map. Same anti-hallucination rules apply to fetched pages.

## The five tools

### `aksel_find_docs` — always start here

Searches documentation, tokens, or migrations depending on `kind`.

```jsonc
{ "query": "button", "kind": "docs", "limit": 8 } // kind defaults to "docs"
```

- `kind:"docs"` — documentation pages (components, primitives, patterns, guides). The
  `query` must be **≥ 3 characters**. One or two keywords work best; a bare component name
  is ideal. Norwegian and English both work (`"knapp"` and `"button"`).
- `kind:"tokens"` — browse the design-token catalog by keyword (`"danger"`, `"space"`,
  `"shadow"`). Follow up with `aksel_get_token_details` for the exact token.
- `kind:"migrations"` — list codemods for version upgrades. Pass a version (`"v8"`,
  `"7->8"`) or a keyword, or omit `query` to list everything.

The tool is smart about mismatches: a token-looking or migration-looking query under
`kind:"docs"` returns a hint telling you to switch `kind`. Read the hint and follow it.

**Output:** a JSON string with `results[]`. Each result carries `name`, `path`, `category`,
and `subcategory`. Feed `path` to the next tool — **never construct it yourself**. The
`subcategory` is a useful signal: a result under `legacy` is **deprecated** (e.g. `Alert`
resolves to `/komponenter/legacy/alert.md`, with `GlobalAlert` / `LocalAlert` as the current
replacements). Prefer non-legacy results unless the user is explicitly working on old code.

### `aksel_get_doc` — read a page

```jsonc
{ "path": "/komponenter/core/button.md" }
```

- `path` must come from `aksel_find_docs` or `aksel-docs://index`. It must start with `/`
  and end with `.md`.
- Returns rich markdown: a one-line `version="…"`/`packages="…"` header, runnable **examples**,
  usage guidelines (DO/DON'T), accessibility notes, **and a full Props table** plus a
  component-tokens note. So for many components `get_doc` alone already gives you props.
- The payload is prefixed with a "training-data is outdated, use this" banner — that's
  expected; trust the doc over memory.
- A `404` means the path is stale — re-run `aksel_find_docs` to get the current one. Do not
  twist the path by hand.

### `aksel_get_component_info` — get the prop API as JSON

```jsonc
{ "component": "komponenter/core/button" } // slug OR "/komponenter/core/button.md"
```

- Returns props as **structured JSON** (each prop: `name`, `type`, `defaultValue`,
  `required`, `deprecated`, `description`). Shape is `props.parts[].props[]` — `parts` is an
  array so compound components (e.g. `Modal.Header`) come back as separate parts.
- Use it when you want to **parse** the API (types, defaults, required) rather than read the
  prose table from `aksel_get_doc`. Both expose the same props; pick by what's easier to use.
- Accepts either the slug or the `.md` path. Returns **props only** (no examples — use
  `get_doc` for those).

### `aksel_get_token_details` — the token tool

```jsonc
{ "tokenName": "bg-accent-strong" }
```

- This is the correct tool for **any** token/color/spacing question — not `find_docs`
  with `kind:"docs"`.
- Returns a flat object: `name`, `value` (the `var(--ax-…)` reference), `rawValue` (the
  resolved value, e.g. `#ecedef`), per-language accessors (`cssValue`, `scssValue`,
  `lessValue`, `jsValue`), plus `comment`, `category`, `role`, and `modifier`.
- Unknown or outdated names (e.g. a v7 token) return the **closest existing tokens** as
  suggestions — use those instead of inventing one.

### `aksel_find_icons` — find an icon

```jsonc
{ "keyword": "trash", "variant": "Stroke", "limit": 20 } // also: category
```

- Fuzzy-matches across icon `name`, `keywords`, and `sub_category` (handles typos and
  synonyms, English/Norwegian).
- `variant` is **capitalized**: `"Stroke"`, `"Fill"`, or `"both"` (default `"both"`). The
  schema also offers `"Filled"`, but it returns **zero** results — use `"Fill"`.
- `category` is a fixed enum (`Home`, `Arrows`, `Interface`, `Money`, `Status`, …).
- **Critical mapping:** the result `name` (e.g. `"Trash"`) is _not_ the import. The export
  is `name + "Icon"` → `import { TrashIcon } from "@navikt/aksel-icons"`. Results have **no
  import string** — apply the rule yourself. The name often differs from the obvious guess
  (`home` → `House`, so it's `HouseIcon`, not `HomeIcon`). See [icons.md](icons.md).

## The four resources

Read a resource when you want the **whole catalog** rather than a ranked search:

| URI                              | Contents                                     |
| -------------------------------- | -------------------------------------------- |
| `aksel-docs://index`             | Full documentation index (every page + path) |
| `aksel-tokens://catalog`         | Lightweight list of all design tokens        |
| `aksel-icons://category-catalog` | Icon categories + subcategories + counts     |
| `aksel-migrations://catalog`     | Available migration codemods                 |

## Anti-hallucination rules

The failure modes that matter:

1. **Never hand-write a doc path** — get it from `aksel_find_docs`. A wrong path that
   silently resolves to another page is worse than a 404.
2. **Never assume a token name** — v7→v8 renamed tokens and switched `--a-` to `--ax-`.
   Confirm with `aksel_get_token_details`; trust its `similarTokens`.
3. **Never assume an icon export** — query `aksel_find_icons`, then append `Icon` to the
   returned `name`. Many intuitive names don't exist.
4. **Never assume a component still exists or has the same props** — verify with
   `aksel_get_component_info` before non-trivial props.
5. **MCP wins over memory.** If MCP truly returns nothing, say what you tried — don't paper
   over the gap with a plausible guess.

## Empty-result recovery

| Symptom                                     | Recovery                                                                               |
| ------------------------------------------- | -------------------------------------------------------------------------------------- |
| `find_docs` (`kind:"docs"`) returns nothing | Retry **once** with a single broader keyword — a component name, not a sentence.       |
| `find_docs` complains about `< 3 chars`     | Lengthen the query, or switch `kind` if the hint says it looks like a token/migration. |
| `get_doc` returns `404 / NOT_FOUND`         | The path is stale. Re-run `find_docs` and use the fresh path.                          |
| `get_component_info` returns `NOT_FOUND`    | Verify the slug/path via `find_docs` first, then retry.                                |
| `get_token_details` says "not found"        | Use the `similarTokens` it suggests; or browse via `find_docs` `kind:"tokens"`.        |
| `find_icons` returns nothing                | Try a synonym or the Norwegian word; broaden `variant` to `"both"`; drop `category`.   |

## Efficiency

- The default happy path is **two calls**: `find_docs` → (`get_doc` or
  `get_component_info`). Add a third for tokens or icons only when the task needs them.
- Don't re-fetch what you already have. Don't echo raw JSON back to the user.
- Aim for ≤ 4 MCP calls per task and stop as soon as you can build confidently.
