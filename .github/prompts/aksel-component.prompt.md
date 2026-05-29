---
name: aksel-component
description: Scaffold en responsiv React-komponent med Aksel Design System og riktige spacing-tokens
model: Claude Haiku 4.5
---

You scaffold a new React component using Nav's Aksel Design System.

## Rules

1. **Never use Tailwind padding/margin** (`p-*`, `m-*`, `px-*`, `py-*`)
2. **Always use Aksel spacing tokens** with `space-` prefix
3. **Mobile-first**, responsive design with breakpoints: `xs`, `sm`, `md`, `lg`, `xl`
4. **Use Aksel components**: Box, VStack, HGrid, Heading, BodyShort, Button, etc.

## Ask the user

1. **Component name**: What is the component called? (PascalCase)
2. **Purpose**: What does the component do?
3. **Layout**: Card, list item, form, dashboard section, etc.?
4. **Responsive**: Should the layout change across screen sizes?

## Component template

```tsx
import { Box, VStack, Heading, BodyShort } from "@navikt/ds-react";

interface {ComponentName}Props {
  title: string;
  description?: string;
  // Add more props as needed
}

export function {ComponentName}({
  title,
  description
}: {ComponentName}Props) {
  return (
    <Box
      background="surface-subtle"
      padding={{ xs: "space-16", md: "space-24" }}
      borderRadius="large"
    >
      <VStack gap="4">
        <Heading size="medium" level="2">
          {title}
        </Heading>
        {description && (
          <BodyShort>
            {description}
          </BodyShort>
        )}
      </VStack>
    </Box>
  );
}
```

## Common patterns

### Card component

```tsx
<Box
  background="surface-subtle"
  padding={{ xs: "space-16", md: "space-24" }}
  borderRadius="large"
  className="hover:shadow-lg transition-shadow"
>
  <VStack gap="4">
    <Heading size="medium" level="3">
      {title}
    </Heading>
    <BodyShort>{description}</BodyShort>
  </VStack>
</Box>
```

### Responsive grid layout

```tsx
<HGrid columns={{ xs: 1, md: 2, lg: 3 }} gap="4">
  {items.map((item) => (
    <Card key={item.id} {...item} />
  ))}
</HGrid>
```

### Form section

```tsx
<Box paddingBlock="space-24">
  <VStack gap="8">
    <Heading size="large" level="2">
      Form Title
    </Heading>
    <VStack gap="4">
      <TextField label="Field 1" />
      <TextField label="Field 2" />
      <Button>Submit</Button>
    </VStack>
  </VStack>
</Box>
```

### Dashboard Section

```tsx
<Box background="surface-default" padding={{ xs: "space-16", md: "space-24" }} borderRadius="medium">
  <VStack gap="6">
    <div className="flex items-center justify-between">
      <Heading size="large" level="2">
        Section Title
      </Heading>
      <Button variant="secondary" size="small">
        Action
      </Button>
    </div>
    <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="4">
      {metrics.map((metric) => (
        <MetricCard key={metric.id} {...metric} />
      ))}
    </HGrid>
  </VStack>
</Box>
```

### Page Container

```tsx
<main className="max-w-7xl mx-auto">
  <Box paddingBlock={{ xs: "space-16", md: "space-24" }} paddingInline={{ xs: "space-16", md: "space-40" }}>
    <VStack gap={{ xs: "space-16", md: "space-24" }}>{/* Page content */}</VStack>
  </Box>
</main>
```

## Available Aksel Components

### Layout

- `Box` - Container with spacing, background, radius
- `VStack` - Vertical stack with gap
- `HStack` - Horizontal stack with gap
- `HGrid` - Responsive grid

### Typography

- `Heading` - size: "large" | "medium" | "small", level: 1-6
- `BodyShort` - size: "large" | "medium" | "small"
- `BodyLong` - For longer text blocks
- `Label` - For form labels
- `Detail` - For supplementary info

### Interactive

- `Button` - variant: "primary" | "secondary" | "tertiary"
- `TextField` - Text input
- `Select` - Dropdown
- `Checkbox`, `Radio`, `Switch`

### Feedback

- `Alert` - variant: "info" | "success" | "warning" | "error"
- `Loader` - Loading spinner
- `HelpText` - Contextual help

## Spacing Tokens

Always use these tokens:

- `space-4` (4px)
- `space-8` (8px)
- `space-12` (12px)
- `space-16` (16px) - Common default
- `space-20` (20px)
- `space-24` (24px) - Common for cards
- `space-32` (32px)
- `space-40` (40px) - Common for page padding

## Background Colors

```tsx
background = "surface-default"; // White
background = "surface-subtle"; // Light gray
background = "surface-action-subtle"; // Light blue
background = "surface-success-subtle"; // Light green
background = "surface-warning-subtle"; // Light orange
background = "surface-danger-subtle"; // Light red
```

## Responsive Breakpoints

```tsx
// Mobile-first approach
padding={{ xs: "space-16" }}                          // All sizes
padding={{ xs: "space-16", md: "space-24" }}         // Mobile + tablet
padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}  // All breakpoints

columns={{ xs: 1, md: 2, lg: 3 }}  // Responsive grid
gap={{ xs: "4", md: "6" }}          // Responsive gap
```

Breakpoints:

- `xs`: 0px (mobile)
- `sm`: 480px
- `md`: 768px (tablet)
- `lg`: 1024px (desktop)
- `xl`: 1280px (large desktop)

## Testing

Create a test file `{component-name}.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { ComponentName } from "./component-name";

describe("ComponentName", () => {
  it("should render title", () => {
    render(<ComponentName title="Test Title" />);
    expect(screen.getByText("Test Title")).toBeInTheDocument();
  });
});
```

## Checklist

After generating the component, verify:

- ✅ No Tailwind padding/margin utilities
- ✅ All spacing uses `space-` prefix tokens
- ✅ Responsive design with breakpoints
- ✅ TypeScript props interface
- ✅ Accessible markup (proper heading levels, labels)
- ✅ Component exported from file

## Forstå koden

After generating the component, explain:

1. **Spacing-tokens vs Tailwind** — Why Aksel uses semantic tokens (`space-16`) instead of utility classes (`p-4`). What happens to consistency when teams mix approaches?
2. **Responsive design** — Why mobile-first (`xs` as base, overrides at `md`/`lg`)? What's the cognitive difference between mobile-first and desktop-first?
3. **Component choice** — Why `Box`/`VStack`/`HGrid` instead of raw `<div>` with CSS. What accessibility and maintainability benefits do these provide?
4. **Heading levels** — Why `level` matters for accessibility (screen readers use heading hierarchy for navigation). What breaks with wrong heading levels?

🔴 **Rød sone**: Accessibility markup (heading levels, ARIA attributes, labels) is worth understanding deeply — automated tools only catch ~30% of accessibility issues.

Still gjerne spørsmål om valgene over.
