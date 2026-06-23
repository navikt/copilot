---
name: aksel-component
description: Scaffold en responsiv React-komponent med Aksel Design System, riktige tokens og props verifisert via Aksel MCP / aksel-builder-skillen
model: Gemini 3.5 Flash
---

You scaffold a new React component using Nav's Aksel design system (`@navikt/ds-react`, v8+).

## How to work

**Follow the `aksel-builder` skill** — it holds the full workflow, decision tree, token/prop
conventions, layout primitives, and accessibility rules. This prompt only adds the
scaffolding-specific steps below; defer to the skill instead of restating it.

Verify every component, prop, token, and icon via the MCP (`aksel_get_component_info`,
`aksel_get_token_details`, `aksel_find_icons`) — never from memory. No MCP? Fall back to
`https://aksel.nav.no/llm.md`. Never invent an API.

## Ask the user first

1. **Component name** (PascalCase)?
2. **Purpose** — what does it do?
3. **Layout** — card, list item, form, dashboard section?
4. **Responsive** — should the layout change across screen sizes?

## v8 gotchas (safety net)

The skill carries the full detail; keep just these so you don't ship a stale API even before loading it:

- Spacing/gap use `space-` tokens (`gap="space-16"`, not `gap="4"`); responsive props are mobile-first `{ xs, sm, md, lg, xl, 2xl }`.
- Color on `data-color`, emphasis on `variant` — destructive button is `variant="primary" data-color="danger"`.
- `borderRadius` uses the scale (`"8"`, `"full"`); `background` drops the `bg-` prefix (`"raised"`); no v7 `surface-*`.
- `Alert` is legacy → `LocalAlert` / `GlobalAlert` / `InfoCard` / `InlineMessage` (confirm via MCP).

## Starter template

```tsx
import { Box, VStack, Heading, BodyShort } from "@navikt/ds-react";

interface {ComponentName}Props {
  title: string;
  description?: string;
}

export function {ComponentName}({ title, description }: {ComponentName}Props) {
  return (
    <Box background="raised" padding={{ xs: "space-16", md: "space-24" }} borderRadius="12">
      <VStack gap="space-16">
        <Heading size="medium" level="2">
          {title}
        </Heading>
        {description && <BodyShort>{description}</BodyShort>}
      </VStack>
    </Box>
  );
}
```

## Before finishing

Add `{component-name}.test.tsx`, then confirm: spacing via `space-` token-props (no Tailwind
spacing); Aksel primitives over raw `<div>`; mobile-first responsive props; v8 patterns
(`data-color`, `borderRadius` scale, `background` without `surface-`); every component, prop,
token and icon verified via MCP; accessible markup (heading levels, labels); TypeScript props;
component exported.
