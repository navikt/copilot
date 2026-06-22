# Icons (`@navikt/aksel-icons`)

Icon export names are the biggest source of hallucinated Aksel code — many intuitive names
don't exist, and others differ from your guess. **Always find the icon via MCP, then map the
result to the export name.**

## The name → import mapping (read this first)

`aksel_find_icons` returns each match's `name` in PascalCase **without** a suffix
(e.g. `"Trash"`, `"StarFill"`, `"ArrowRight"`). The actual export is that `name` **plus
`Icon`**:

```
find_icons name   →   import
"Trash"           →   import { TrashIcon } from "@navikt/aksel-icons";
"StarFill"        →   import { StarFillIcon } from "@navikt/aksel-icons";
"ArrowRight"      →   import { ArrowRightIcon } from "@navikt/aksel-icons";
```

Rule: **export = `${name}Icon`**, imported from `@navikt/aksel-icons`. Never write an icon
import you didn't get from a `find_icons` result.

> **The name rarely matches your first guess.** `find_icons({ keyword: "home" })` returns
> `House` and `HouseFill` — there is **no** `Home` icon, so `HomeIcon` does not exist; the
> correct import is `HouseIcon`. This is exactly why you query first instead of guessing.

## Finding an icon

```jsonc
{ "keyword": "trash", "variant": "Stroke", "limit": 20 }
```

- `keyword` — fuzzy-matched across name, keywords, and sub-category. Handles typos and
  synonyms, in English **and** Norwegian (`"søppel"` finds the trash icon).
- `variant` — **capitalized**: `"Stroke"` (outline, the default look), `"Fill"` (solid),
  or `"both"` (default). Fill icons carry `Fill` in their `name` → `…FillIcon`. (The tool
  schema also lists `"Filled"`, but it returns **zero** results — always use `"Fill"`.)
- `category` — optional filter from a fixed enum (e.g. `Home`, `Arrows`, `Interface`,
  `Money`, `Status`…).

Each result has `name`, `category`, and `variant` — **no import string**, so you must apply
the `${name}Icon` rule yourself. If a search returns nothing, try a synonym or the Norwegian
word, and widen `variant` to `"both"` before giving up. Do not substitute a guessed name.

## Using an icon

Aksel icons render at `1em` and inherit color via `currentColor`, so they scale with
**font size** and match surrounding text color automatically.

```tsx
import { TrashIcon } from "@navikt/aksel-icons";

// Size via fontSize (it maps to the SVG's 1em box); color is inherited
<TrashIcon fontSize="1.5rem" aria-hidden />;
```

- **Size** — set `fontSize` (e.g. `"1.5rem"`), or let it inherit from the parent's font
  size. You can also pass `width`/`height` for a fixed pixel size.
- **Color** — inherited via `currentColor`. To recolor, set `color` on the icon or a parent
  using a token (`style={{ color: "var(--ax-text-danger)" }}` or a Tailwind `text-ax-text-*`
  class).

## Icon accessibility (decide: meaningful or decorative?)

Every icon is either conveying information or purely decorative. Handle each correctly —
this is required for accessibility (uu):

- **Decorative** (next to a visible text label, e.g. inside a `Button` with text): hide it
  from assistive tech with `aria-hidden`.

  ```tsx
  <Button icon={<TrashIcon aria-hidden />}>Slett</Button>
  ```

- **Meaningful** (the icon is the only label, e.g. an icon-only button): give it an
  accessible name with the `title` prop, and make sure the interactive control is labelled
  too.

  ```tsx
  <Button
    icon={<TrashIcon title="Slett rad" />}
    // icon-only buttons still need their own accessible name — see accessibility.md
  />
  ```

The `title` prop renders an SVG `<title>` and wires `aria-labelledby`, turning the icon
into a labelled image. When in doubt about the exact accessibility pattern for a specific
component, confirm with `aksel_get_doc` (icon docs at `aksel.nav.no/ikoner`) and see
[accessibility.md](accessibility.md).
