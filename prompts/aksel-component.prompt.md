---
name: aksel-component
description: Scaffold en responsiv React-komponent med Aksel Design System, riktige tokens og props verifisert via Aksel MCP / aksel-builder-skillen
model: GPT-6 Luna
---

You scaffold a new React component using Nav's Aksel design system (`@navikt/ds-react`, v8+).

## How to work

**Follow the `aksel-builder` skill** — it holds the full workflow, decision tree, token/prop
conventions, layout primitives, and accessibility rules. This prompt only adds the
scaffolding-specific steps below; defer to the skill instead of restating it.

Use the skill's MCP workflow and installed package exports to verify APIs, never memory.
If MCP is unavailable, follow the skill's Preflight fallback. Don't assume current
documentation matches the project's installed version.

## Clarify only what is missing

Use the request and repository conventions first. Ask only when an unanswered question
changes the implementation; don't repeat information the user already supplied.

1. **Component name** (PascalCase)?
2. **Purpose** — what does it do?
3. **Layout** — card, list item, form, dashboard section?
4. **Responsive** — should the layout change across screen sizes?

## v8 gotchas (safety net)

The skill carries the full detail:

- **Verify the installed version.** MCP catalogs and current docs can be newer than the app.
- **Color and emphasis differ.** For v8 buttons, use `variant="primary" data-color="danger"` for a destructive primary action.
- **Choose current components.** `Alert` is legacy; confirm the appropriate replacement and API.
- **Follow project styling.** Primitive token props and native Tailwind layout/spacing are valid; don't target Aksel internals.
- **Check accessible composition.** Label controls, preserve focus behavior, and match document language and locale.

## Starter template

Use this card-shaped example only when it fits the requested component. Rename the
component and props, follow the project's export style, and choose the heading level
from its surrounding page.

```tsx
import { BodyShort, Box, Heading, VStack } from "@navikt/ds-react";

interface ExampleCardProps {
  title: string;
  description?: string;
}

export function ExampleCard({ title, description }: ExampleCardProps) {
  return (
    <Box
      background="raised"
      padding={{ xs: "space-16", md: "space-24" }}
      borderRadius="12"
    >
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

Run the project's existing relevant checks. Add behavior coverage when warranted, using
its test setup and naming; don't unconditionally create a `.test.tsx` file or add tooling.
Confirm responsive behavior, verified APIs/imports, accessible headings and labels,
TypeScript props, and required exports. Report checks that could not run.
