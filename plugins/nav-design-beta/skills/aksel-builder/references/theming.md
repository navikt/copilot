# Theming: light / dark & base color

Aksel theming is driven by the `.light` / `.dark` CSS classes plus the `data-color`
attribute. Together they set color mode and base color, and make the `--ax-*` tokens resolve
correctly. v8 defaults to light mode — if you don't need dark mode, no extra setup is needed.

## The `Theme` component

`Theme` manages those classes for you: it sets the mode, paints the background on the
top-most instance, and resets the base color.

```tsx
import { Theme } from "@navikt/ds-react";

<Theme theme="light" hasBackground>
  <App />
</Theme>;
```

Props:

| Prop            | Type                | Notes                                                                                                                                                         |
| --------------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `theme`         | `"light" \| "dark"` | Color mode. Inherits the parent `Theme`; defaults to `light` at the root.                                                                                     |
| `data-color`    | color role          | Base/brand color for descendants (default `accent`). Accepts a color role — `neutral`, `accent`, `success`, `warning`, `danger`, `info`, `brand-*`, `meta-*`. |
| `hasBackground` | `boolean`           | Paints the themed page background. Defaults to `true` when this is the root `Theme` and `theme` is set; otherwise `false`.                                    |
| `asChild`       | `boolean`           | Render as the child element instead of a wrapping `<div>`.                                                                                                    |
| `className`     | `string`            | Extra classes.                                                                                                                                                |

`Theme` must wrap your app (or the subtree you want themed) so the token variables cascade.
At minimum, set it up once at the root with `hasBackground`.

## Light / dark mode

`Theme` itself is controlled — it renders whatever `theme` you pass. To let the user (or
the OS) choose, drive the `theme` prop from your own state or a library like `next-themes`.

```tsx
"use client";
import { ThemeProvider as NextThemeProvider } from "next-themes";
import { Theme } from "@navikt/ds-react";

function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <NextThemeProvider
      attribute="class"
      storageKey="app-theme"
      enableSystem
      themes={["light", "dark"]}
      disableTransitionOnChange
    >
      <Theme hasBackground>{children}</Theme>
    </NextThemeProvider>
  );
}
```

For a simple in-app toggle without a library, hold the mode in state and pass it down:

```tsx
const [mode, setMode] = useState<"light" | "dark">("light");

<Theme theme={mode} hasBackground>
  <Button onClick={() => setMode((m) => (m === "light" ? "dark" : "light"))}>
    Toggle theme
  </Button>
  <App />
</Theme>;
```

## Nesting & base color

`Theme` is nestable. A nested `Theme` inherits `theme` and `data-color` from its parent and
can override either for a subtree — e.g. a panel that's always dark, or a section with a
different base color.

```tsx
<Theme theme="light" hasBackground>
  <App /> {/* light */}
  <Theme theme="dark">
    {" "}
    {/* this subtree is dark */}
    <PromoPanel />
  </Theme>
  <Theme data-color="success">
    {" "}
    {/* success becomes the base interactive color here */}
    <OnboardingFlow />
  </Theme>
</Theme>
```

Set `hasBackground` only where you actually want the themed background painted (typically
the root, or a nested `Theme` that should sit on its own surface). Leaving it off on nested
themes lets them sit transparently on the parent's background.

## Gotchas

- **`Theme` ≠ `Provider`.** `Theme` handles color mode and base color. `Provider` handles
  locale and portal roots ([setup-and-imports.md](setup-and-imports.md)). You often use both.
- **Don't hardcode mode-specific colors.** Use role tokens
  ([tokens-styling.md](tokens-styling.md)); they flip automatically between light and dark,
  which is the whole point of theming.
- "Darkside": If referencing anything with "darkside", this is just the project name for the theming system **before** it was launched in version 8. If project is on v8 or up, "darkside" is just baseline now.
