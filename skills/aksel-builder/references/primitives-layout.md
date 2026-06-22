# Primitives & layout

Reach for Aksel's layout primitives (from `@navikt/ds-react`) before custom CSS — they
encapsulate spacing, responsiveness, and the token system so layout stays consistent and
themeable.

## Which primitive?

| Need                                                               | Use                        |
| ------------------------------------------------------------------ | -------------------------- |
| Generic container with padding, background, border, radius, shadow | `Box`                      |
| Stack items horizontally with consistent gap                       | `HStack`                   |
| Stack items vertically with consistent gap                         | `VStack`                   |
| Stack with a runtime/responsive direction                          | `Stack` (`direction` prop) |
| Push siblings apart inside a stack                                 | `Spacer`                   |
| Multi-column grid with responsive columns                          | `HGrid`                    |
| Top-level page frame (max-width, gutters, footer)                  | `Page` + `Page.Block`      |
| Let content escape its parent's padding (negative margin)          | `Bleed`                    |
| Show an element only above/below a breakpoint                      | `Show`                     |
| Hide an element only above/below a breakpoint                      | `Hide`                     |

`HStack` and `VStack` are `Stack` with a fixed direction — prefer them when the direction
is static. They accept all the same layout props as `Box`, so write `<VStack padding="space-16">`
rather than wrapping a `Box` around a stack.

## The responsive prop system

Every primitive style prop accepts **either** a single token **or** an object keyed by
breakpoint. The breakpoint aliases are `xs`, `sm`, `md`, `lg`, `xl`, `2xl`.

```tsx
<Box padding="space-16" />
<Box padding={{ xs: "space-8", md: "space-16", lg: "space-24" }} />
```

Directional spacing props accept one or two tokens (`"<block/inline> <inline/...>"`):

```tsx
<Box paddingInline="space-16 space-32" />     // left/right differ
<HStack gap="space-32 space-16" />            // column-gap row-gap
```

`marginInline` / `marginBlock` additionally accept `"auto"` (e.g. `marginInline="auto"`
to center).

## Spacing scale (`space-*`)

Spacing props (`padding*`, `margin*`, `gap`, `inset`, `top/right/bottom/left`) take tokens
from this fixed scale — **never** arbitrary pixels:

```
space-0  space-1  space-2  space-4  space-6  space-8  space-12 space-16
space-20 space-24 space-28 space-32 space-36 space-40 space-44 space-48
space-56 space-64 space-72 space-80 space-96 space-128
```

Each maps to a `rem` value (e.g. `space-16` = `1rem`). Pick from the scale; if you think you
need a value that isn't there, you almost certainly want the nearest scale step. Confirm a
token's exact value with `aksel_get_token_details({ tokenName: "space-16" })` when it
matters.

These props are the **prop form** of the spacing tokens. Styling your own (non-Aksel)
element in CSS instead? Use the **CSS-variable form** — `padding: var(--ax-space-16)`, not a
raw `16px`. See [tokens-styling.md](tokens-styling.md) for when to use which.

## Radius scale (`borderRadius`)

`Box`'s `borderRadius` takes one to four values from: `2`, `4`, `8`, `12`, `16`, `full`
(and `0`). Like CSS shorthand, multiple values set individual corners, and it's responsive:

```tsx
<Box borderRadius="full" />
<Box borderRadius="0 full 12 2" />
<Box borderRadius={{ xs: "2", md: "8" }} />
```

## Box

The foundational container. Layout props (padding, margin, width, position, overflow,
flex/grid props, etc.) come from the shared primitive props; `Box` adds visual props:

```tsx
<Box
  background="raised" // background token, prefix dropped (see tokens-styling.md)
  borderColor="neutral-subtle" // border-color token; pair with borderWidth
  borderWidth="1" // 0–5 (px), required for a border to show
  borderRadius="8"
  shadow="dialog" // shadow token
  padding="space-16"
>
  <BodyShort>Content</BodyShort>
</Box>
```

`background`, `borderColor`, and `shadow` take **token names with the category prefix
dropped** (`background="accent-strong"` → `--ax-bg-accent-strong`,
`borderColor="neutral-subtle"` → `--ax-border-neutral-subtle`). Confirm valid names via
`aksel_get_token_details` or [tokens-styling.md](tokens-styling.md).

> **Deprecation:** `Box.New` / `BoxNew` is deprecated in v8 — use `Box` (same props).
> Run `pnx @navikt/aksel codemod v8-box-new` to migrate. See [migrations.md](migrations.md).

## Stack (HStack / VStack)

```tsx
<VStack gap="space-16" align="start">
  <Heading size="medium">Title</Heading>
  <BodyLong>Paragraph</BodyLong>
</VStack>

<HStack gap="space-8" justify="space-between" align="center" wrap>
  <Button>Save</Button>
  <Spacer />
  <Button variant="secondary">Cancel</Button>
</HStack>
```

- `gap` — spacing token (or two for column/row).
- `align` — `start | center | end | baseline | stretch` (default `stretch`).
- `justify` — `start | center | end | space-around | space-between | space-evenly`.
- `wrap` — boolean (default `true`).
- `Spacer` — an empty flexible element that pushes stack siblings apart.

## HGrid

```tsx
<HGrid gap="space-24" columns={3}>…</HGrid>
<HGrid gap="space-24" columns={{ xs: 1, md: 2, lg: "1fr auto" }}>…</HGrid>
```

- `columns` — a number, a CSS `grid-template-columns` string (`"1fr auto"`,
  `"repeat(3, minmax(0, 1fr))"`), or a responsive object.
- `gap` — spacing token(s).
- `align` — `start | center | end`.

## Page & Page.Block

`Page` is the top-level frame; `Page.Block` sets max-width, optional gutters, and centers
content horizontally.

```tsx
<Page
  footer={
    <Page.Block width="xl" gutters>
      …
    </Page.Block>
  }
>
  <Page.Block as="header" width="xl" gutters>
    …
  </Page.Block>
  <Page.Block as="main" width="xl" gutters>
    …
  </Page.Block>
</Page>
```

- `Page.Block` `width`: `text` (≈576px), `md` (768), `lg` (1024), `xl` (1280), `2xl` (1440);
  default is full width.
- `gutters` — adds responsive horizontal padding (3rem ≥ md, 1rem < md).
- For a full-bleed background behind a centered block, wrap `Page.Block` in a `Box` with a
  `background` token.

## Bleed

Lets a child extend past the parent's padding using negative margin.

```tsx
<Box padding="space-16">
  <Bleed marginInline="space-16" reflectivePadding>
    <img src="…" alt="" />
  </Bleed>
</Box>
```

- `marginInline` accepts spacing tokens **or** `"full"` to span the full viewport width.
- `marginBlock` accepts spacing tokens (no `"full"`).
- `reflectivePadding` re-adds matching padding so apparent width is preserved.

## Show / Hide

Render content conditionally by breakpoint. Use `asChild` to avoid an extra wrapper element.

```tsx
<Hide below="md" asChild>
  <DesktopNav />   {/* hidden below md, visible from md up */}
</Hide>

<Show below="md" asChild>
  <MobileMenuButton />  {/* visible only below md */}
</Show>
```

- `above` / `below` take a breakpoint (`sm`–`2xl`, inclusive); `xs` is not valid here.
- `as` is `"div"` (default) or `"span"`.
