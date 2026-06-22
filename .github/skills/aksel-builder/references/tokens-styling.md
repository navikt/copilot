# Tokens & styling

Aksel styling is **token-based** — almost never write a raw hex or pixel value; a token
exists, and using it is what makes theming and light/dark work. A token can be applied two
ways: **token-props** on Aksel components/primitives, and **CSS variables** in your own CSS.

## The same token, two ways to apply it

Every Aksel token is a single design decision (e.g. "16px of space", "the `accent-strong`
background"). You apply that decision in one of **two forms**, depending on _what you are
styling_. Both forms resolve to the exact same value — they are **not** different tokens.

| Aspect          | **Prop form** (token-prop)                          | **CSS-variable form**                                        |
| --------------- | --------------------------------------------------- | ------------------------------------------------------------ |
| Looks like      | `padding="space-16"` · `background="accent-strong"` | `padding: var(--ax-space-16)` · `var(--ax-bg-accent-strong)` |
| You write it on | Aksel components & primitives, in JSX               | Your own CSS/SCSS, `style={{}}`, or styled-components        |
| Token name form | category prefix **dropped** (`accent-strong`)       | **full** variable name (`--ax-bg-accent-strong`)             |
| What you get    | type-safety + responsive `{ xs, md }` + theme-aware | a raw value usable in any selector/property                  |

**Decision rule — which form do I use?**

1. Styling an **Aksel component or primitive** (`Box`, `HStack`, `Button`, `Modal`…)? → use
   the **prop**. It already speaks the token system, is type-checked, and supports
   responsive objects. This is the default and covers most styling. Don't guess the prop
   name — confirm it with `aksel_get_component_info({ component })`, since which props a
   component exposes (and whether it takes a token) varies per component.
2. Styling **your own element**, a selector, a pseudo-state, or a CSS property Aksel doesn't
   expose as a prop? → use the **CSS variable** `var(--ax-…)`. This is the escape hatch.
3. **Either way, never hardcode the underlying value** (`16px`, `#0067c5`). One token flips
   correctly across light/dark and theming; a raw value can't.

> **#1 gotcha — the name differs by form.** A **prop drops** the category prefix
> (`background="accent-strong"`); the **CSS variable keeps** it (`--ax-bg-accent-strong`).
> When you call `aksel_get_token_details`, pass the **full** name (`bg-accent-strong`).

## Styling discipline (the rules that matter)

1. **Prefer the prop form** on Aksel components/primitives; drop to the CSS-variable form
   only for your own elements (decision rule above). Avoid `style`/CSS when a prop exists.
2. **No raw values.** Never hardcode `#0067c5` or `16px`. Use a token. If you don't know the
   token, look it up with `aksel_get_token_details` or browse with
   `aksel_find_docs({ kind: "tokens" })`.
3. **Don't target Aksel's internal class names.** They are implementation details and
   change between versions. Style your own wrappers instead.

## Token catalog: the `--ax-` CSS variables

Every token is also exposed as a CSS custom property prefixed `--ax-`. This is the canonical
**full name** of each token: the **CSS-variable form** uses it verbatim, and the **prop
form** is the same name with the category prefix dropped. The prefix changed from `--a-` to
`--ax-` in v8 — if you see `--a-*`, it's outdated (see [migrations.md](migrations.md)).

| Category   | CSS variable shape | Examples                                                                |
| ---------- | ------------------ | ----------------------------------------------------------------------- |
| Background | `--ax-bg-*`        | `--ax-bg-default`, `--ax-bg-accent-strong`, `--ax-bg-success-soft`      |
| Text       | `--ax-text-*`      | `--ax-text-accent`, `--ax-text-danger-subtle`, `--ax-text-default`      |
| Border     | `--ax-border-*`    | `--ax-border-accent`, `--ax-border-neutral-subtle`, `--ax-border-focus` |
| Spacing    | `--ax-space-*`     | `--ax-space-16`, `--ax-space-32`                                        |
| Radius     | `--ax-radius-*`    | `--ax-radius-8`, `--ax-radius-full`                                     |
| Shadow     | `--ax-shadow-*`    | `--ax-shadow-dialog`                                                    |
| Font       | `--ax-font-*`      | family, size, weight, line-height tokens                                |

These examples are representative, not exhaustive — **always confirm an exact token name
with `aksel_get_token_details`** before relying on it.

## Color roles

Semantic color tokens are organized by **role**. The role carries the meaning; pick the role
that matches intent, not the literal color you have in mind.

| Group  | Roles                                        | Use for                                                                    |
| ------ | -------------------------------------------- | -------------------------------------------------------------------------- |
| Main   | `neutral`, `accent`                          | `neutral` = doesn't stand out; `accent` = default for interactive elements |
| Status | `info`, `success`, `warning`, `danger`       | informational / positive / caution / destructive-or-error                  |
| Brand  | `brand-magenta`, `brand-beige`, `brand-blue` | Nav brand colors — use sparingly                                           |
| Meta   | `meta-purple`, `meta-lime`                   | metadata; teams define the meaning                                         |

Within a role, tokens follow an **emphasis ladder**:

- Backgrounds: `soft` → `moderate` → `strong` (plus alpha `…A` and states `-hover` /
  `-pressed`). Example token names: `bg-accent-soft`, `bg-accent-moderate`,
  `bg-accent-strong`.
- Borders & text: `subtle` / (base) / `strong`. Example: `border-neutral-subtle`,
  `text-accent`, `text-danger-subtle`.

Root/global tokens that aren't role-scoped also exist — backgrounds `default`, `input`,
`raised`, `sunken`, `overlay`; `border-focus`; `text-default`, `text-logo`.

**Note**

Some root names are **dynamic aliases**: `--ax-text-default` resolves correctly as a CSS
variable (to the active role's text color), but `aksel_get_token_details("text-default")`
returns _not found_ and suggests `text-neutral`. This will be the case for these names
"bg-soft, bg-softA, bg-moderate, bg-moderateA, bg-moderate-hover, bg-moderate-hoverA, bg-moderate-pressed, bg-moderate-pressedA, bg-strong, bg-strong-hover, bg-strong-pressed, text-default, text-subtle, text-decoration, text-contrast, border-default, border-subtle, border-subtleA, border-strong"

These tokens are colored based on the current `data-color` defined on closest parent node. By default, this will be `accent`, so no need to manually define.

## Prop form: token-props on components & primitives

The **default** way to apply a token. Passed as a prop value, the category prefix is
**dropped** (the prop already implies the category):

```tsx
// Box "background" prop → resolves to var(--ax-bg-<value>)
<Box background="accent-strong" />     // NOT background="bg-accent-strong"
<Box background="raised" />

// Box "borderColor" prop → resolves to var(--ax-border-<value>)
<Box borderColor="neutral-subtle" borderWidth="1" />   // NOT "border-neutral-subtle"

// Box "shadow" prop → resolves to var(--ax-shadow-<value>)
<Box shadow="dialog" />
```

## CSS-variable form: tokens in your own CSS

Use this form when you're styling **your own element**, or a property/selector Aksel doesn't
expose as a prop. Reference the full `var(--ax-…)` name — never the literal value behind it:

```css
.callout {
  background: var(--ax-bg-info-soft);
  color: var(--ax-text-default);
  border: 1px solid var(--ax-border-info);
  border-radius: var(--ax-radius-8);
  padding: var(--ax-space-16);
  box-shadow: var(--ax-shadow-dialog);
}
```

## Tailwind preset (`@navikt/ds-tailwind`)

Add the preset to your Tailwind config:

```jsx
// tailwind.config.js
module.exports = { presets: [require("@navikt/ds-tailwind")] };
```

**Utility names embed the token category** — the generated class is
`{tw-prefix}-ax-{token-name}`, and the token name already starts with its category
(`bg-…`, `text-…`, `border-…`). So the category appears twice:

```tsx
<div className="bg-ax-bg-accent-strong text-ax-text-danger border-ax-border-neutral-subtle shadow-ax-dialog">
```

- Background: `bg-ax-bg-*` (e.g. `bg-ax-bg-accent-strong`)
- Text color: `text-ax-text-*` (e.g. `text-ax-text-danger`)
- Border color: `border-ax-border-*` (e.g. `border-ax-border-neutral-subtle`)
- Shadow: `shadow-ax-dialog`
- Font size / weight: `text-ax-*` / `font-ax-*` (e.g. `text-ax-heading-large`, `font-ax-bold`)

**Not every token category has a utility.** Spacing (`space-*`) is **not** exposed as a
Tailwind utility — use native Tailwind spacing (`p-4`, `gap-2`) or, better, the layout
primitives' props (`padding="space-16"`). For radius and anything not listed above, check
the live config rather than guessing.

**v8 behavior:** colors map to CSS variables, so theming works automatically — no `dark:`
prefix needed. As a consequence you **cannot** manipulate the values (e.g. `bg-…/50` for
opacity won't work). `colors` and `screens` **replace** Tailwind's defaults, they don't
extend them.

**Setup differs by Tailwind major** (v4 uses `@import "@navikt/ds-css" layer(components)` +
`@config`; v3 needs the `postcss-import` plugin). The exact utility list and the current
setup are generated per version — fetch the live docs to confirm:
`aksel_find_docs({ query: "tailwind" })` → `aksel_get_doc` on `/grunnleggende/kode/tailwind.md`
and `/grunnleggende/styling/tailwind-config.md` (the full generated class list).

## CSS-only consumers

Without React, apply Aksel component classes (from the `aksel_get_doc` markup) and reference
`--ax-*` variables directly in your own CSS, exactly as in the custom-CSS example above.
