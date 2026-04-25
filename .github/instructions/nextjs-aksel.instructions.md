---
applyTo: "src/**/*.{tsx,ts}"
---

# Next.js med Aksel Design System

Standarder for Next.js-apper med Aksel: spacing-tokens, responsive props og komponentmønstre.

<!-- BEGIN:nextjs-agent-rules -->

# Next.js: ALWAYS read docs before coding

Before any Next.js work, find and read the relevant doc in `node_modules/next/dist/docs/`. Your training data is outdated — the docs are the source of truth.

<!-- END:nextjs-agent-rules -->

## Aksel-dokumentasjon

For komponent-APIer, tokens, theming, layout og mønstre — hent dokumentasjon fra:

```
https://aksel.nav.no/llm.md
```

Filen er et indeks over alle tilgjengelige Aksel-dokumentasjonssider som individuelle `.md`-filer. Hent individuelle sider ved behov fremfor å anta API fra treningsdata.

## Spacing-regler

**VIKTIG**: Bruk alltid Nav DS spacing-tokens, aldri Tailwind padding/margin.

### ✅ Riktig

```tsx
import { Box, VStack, HGrid } from "@navikt/ds-react";

// Page container
<main className="max-w-7xl mx-auto">
  <Box
    paddingBlock={{ xs: "space-16", md: "space-24" }}
    paddingInline={{ xs: "space-16", md: "space-40" }}
  >
    {children}
  </Box>
</main>

// Komponent med responsiv padding
<Box
  background="neutral-soft"
  padding={{ xs: "space-12", sm: "space-16", md: "space-24" }}
  borderRadius="8"
>
  <Heading size="large" level="2">Tittel</Heading>
  <BodyShort>Innhold</BodyShort>
</Box>

// Retningsbestemt padding
<Box
  paddingBlock="space-16"    // Topp og bunn
  paddingInline="space-24"   // Venstre og høyre
>
```

### ❌ Feil

```tsx
// Aldri bruk Tailwind padding/margin
<div className="p-4 md:p-6">  // ❌ Feil
<div className="mx-4 my-2">   // ❌ Feil
<Box padding="4">             // ❌ Feil — mangler space-prefiks
```

## Spacing-tokens

Token name = eksakt pikselverdi (`space-16` = 16px, `space-4` = 4px). Alltid `space-`-prefiks.

Fullstendig liste: [design-tokens](https://aksel.nav.no/grunnleggende/styling/design-tokens.md)

## Responsiv design

Mobil-først. Fullstendige breakpoints: [brekkpunkter](https://aksel.nav.no/grunnleggende/styling/brekkpunkter.md)

```tsx
<HGrid columns={{ xs: 1, md: 2, lg: 3 }} gap="space-16">
  {items.map(item => <Card key={item.id} {...item} />)}
</HGrid>

<Box
  padding={{ xs: "space-16", sm: "space-20", md: "space-24" }}
>
```

## Komponentmønstre

### Layout-komponenter

```tsx
import { Box, VStack, HStack, HGrid } from "@navikt/ds-react";

// Vertikal stack med mellomrom
<VStack gap="space-16">
  <Komponent1 />
  <Komponent2 />
  <Komponent3 />
</VStack>

// Horisontal stack
<HStack gap="space-8" align="center">
  <Icon />
  <Text />
</HStack>

// Responsiv grid
<HGrid columns={{ xs: 1, md: 2, lg: 3 }} gap="space-16">
  {/* Grid-elementer */}
</HGrid>
```

### Typografi

Full API: [typography](https://aksel.nav.no/komponenter/core/typography.md)

```tsx
import { Heading, BodyShort, Label } from "@navikt/ds-react";

<Heading size="xlarge|large|medium|small|xsmall" level="1-6">
  Tittel
</Heading>

<BodyShort size="large|medium|small">
  Vanlig tekstinnhold
</BodyShort>

<BodyShort weight="semibold">
  Halvfet tekst
</BodyShort>

<Label size="medium|small">
  Skjemaetikett
</Label>
```

### Bakgrunnsfarger

```tsx
<Box background="default">          {/* Hvit */}
<Box background="neutral-soft">     {/* Lys grå */}
<Box background="accent-soft">      {/* Lys blå */}
<Box background="success-soft">     {/* Lys grønn */}
<Box background="warning-soft">     {/* Lys oransje */}
<Box background="danger-soft">      {/* Lys rød */}
```

## Tallformatering

Bruk alltid norsk locale for tallformatering:

```typescript
import { formatNumber } from "@/lib/format";

// ✅ Correct
const formatted = formatNumber(151354); // "151 354"

// ❌ Wrong
const formatted = num.toLocaleString(); // Uses browser locale
```

## API Routes (App Router)

```typescript
import { NextResponse } from "next/server";

// GET endpoint with error handling
export async function GET() {
  const { data, error } = await fetchData();

  if (error) {
    return NextResponse.json({ error: error.message }, { status: 500 });
  }

  return NextResponse.json(data);
}

// POST endpoint
export async function POST(request: Request) {
  const body = await request.json();

  // Validation
  if (!body.requiredField) {
    return NextResponse.json(
      { error: "requiredField is missing" },
      { status: 400 },
    );
  }

  const result = await processData(body);
  return NextResponse.json(result, { status: 201 });
}
```

## Async Request APIs (Next.js 15+)

In Next.js 15+, `cookies()`, `headers()`, and route `params` are **async** and require `await`:

```typescript
import { cookies, headers } from "next/headers";

export default async function Page() {
  // ✅ Next.js 15+: must await
  const cookieStore = await cookies();
  const headersList = await headers();

  const token = cookieStore.get("auth-token")?.value;
  const userAgent = headersList.get("user-agent");
}

// ✅ Route params are also Promises
export async function GET(
  request: Request,
  props: { params: Promise<{ id: string }> },
) {
  const { id } = await props.params;
  // ...
}

// ❌ Next.js 14 pattern — no longer works
const cookieStore = cookies(); // Error: must await
```

## Metadata API

```typescript
import { Metadata } from "next";

// ✅ Dynamic metadata for SEO
export async function generateMetadata(props: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await props.params;
  const vedtak = await fetchVedtak(id);

  return {
    title: `${vedtak.title} | Nav`,
    description: vedtak.summary,
    openGraph: { title: vedtak.title, description: vedtak.summary },
  };
}
```

## Middleware

```typescript
// src/middleware.ts
import { NextRequest, NextResponse } from "next/server";

export function middleware(request: NextRequest) {
  const response = NextResponse.next();

  // Security headers
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");

  return response;
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
};
```

## Authentication

```typescript
import { getUser } from "@/lib/auth";

// Redirect if not authenticated
const user = await getUser();

// Return null if not authenticated
const user = await getUser(false);
if (!user) {
  return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
}
```

## Testing

```typescript
import { formatNumber } from "./format";

describe("formatNumber", () => {
  it("should format numbers with Norwegian locale", () => {
    expect(formatNumber(151354)).toBe("151 354");
  });

  it("should handle decimal numbers", () => {
    expect(formatNumber(1234.56)).toBe("1 234,56");
  });
});
```

## Server Components (Next.js 16)

```tsx
// Server Component (default in App Router)
export default async function Page() {
  const data = await fetchData(); // Can use async/await

  return (
    <Box padding="space-24">
      <Heading size="large" level="1">
        {data.title}
      </Heading>
      <BodyShort>{data.description}</BodyShort>
    </Box>
  );
}
```

## Client Components

```tsx
"use client";

import { useState } from "react";
import { Button } from "@navikt/ds-react";

export function InteractiveComponent() {
  const [count, setCount] = useState(0);

  return <Button onClick={() => setCount(count + 1)}>Count: {count}</Button>;
}
```

## Loading and Error States

```tsx
// src/app/oversikt/loading.tsx — shown during server component loading
import { Loader, Box } from "@navikt/ds-react";

export default function Loading() {
  return (
    <Box padding="space-24" className="flex justify-center">
      <Loader size="xlarge" title="Laster data..." />
    </Box>
  );
}

// src/app/oversikt/error.tsx — shown on uncaught errors
"use client";

import { LocalAlert, Button, VStack } from "@navikt/ds-react";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <VStack gap="space-16">
      <LocalAlert status="error">
        <LocalAlert.Header>
          <LocalAlert.Title>Noe gikk galt</LocalAlert.Title>
        </LocalAlert.Header>
        <LocalAlert.Content>
          Det oppsto en feil under innlastingen av data. Prøv igjen senere. Hvis
          problemet vedvarer, kontakt support.
        </LocalAlert.Content>
      </LocalAlert>

      <Button variant="secondary" onClick={reset}>
        Prøv igjen
      </Button>
    </VStack>
  );
}
```

## Streaming with Suspense

```tsx
import { Suspense } from "react";
import { Skeleton } from "@navikt/ds-react";

export default function Page() {
  return (
    <VStack gap="space-24">
      <Heading size="large" level="1">
        Oversikt
      </Heading>
      <Suspense fallback={<Skeleton variant="rounded" height={200} />}>
        <SlowDataComponent />
      </Suspense>
    </VStack>
  );
}

// This component can load independently
async function SlowDataComponent() {
  const data = await fetchSlowData(); // streams in when ready
  return <DataTable data={data} />;
}
```

## Server Actions

```tsx
// src/app/vedtak/actions.ts
"use server";

import { revalidatePath } from "next/cache";
import { getUser } from "@/lib/auth";

export async function createVedtak(formData: FormData) {
  const user = await getUser();

  const tittel = formData.get("tittel") as string;
  if (!tittel) {
    return { error: "Tittel er påkrevd" };
  }

  await saveToDatabase({ tittel, opprettetAv: user.name });
  revalidatePath("/vedtak");
  return { success: true };
}
```

```tsx
// src/app/vedtak/page.tsx
"use client";

import { useActionState } from "react";
import { createVedtak } from "./actions";

export default function VedtakForm() {
  const [state, action, isPending] = useActionState(createVedtak, null);

  return (
    <form action={action}>
      <TextField name="tittel" label="Tittel" error={state?.error} />
      <Button type="submit" loading={isPending}>
        Opprett
      </Button>
    </form>
  );
}
```

## When Using React Query (Server State)

React Query (@tanstack/react-query) is the standard for server state management at Nav, replacing Redux/Context for API data.

```tsx
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader, LocalAlert, VStack } from "@navikt/ds-react";

export function ResourceList() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["resources"],
    queryFn: () => fetch("/api/resources").then((res) => res.json()),
  });

  if (isLoading) return <Loader title="Laster..." />;
  if (error)
    return (
      <LocalAlert status="error">
        <LocalAlert.Header>
          <LocalAlert.Title>Kunne ikke laste data</LocalAlert.Title>
        </LocalAlert.Header>
        <LocalAlert.Content>
          Det oppsto en feil under innlastingen av data. Prøv igjen senere. Hvis
          problemet vedvarer, kontakt support.
        </LocalAlert.Content>
      </LocalAlert>
    );

  return (
    <VStack gap="space-16">
      {data.map((resource) => (
        <ResourceCard key={resource.id} resource={resource} />
      ))}
    </VStack>
  );
}
```

```tsx
// ✅ Mutation with cache invalidation
const queryClient = useQueryClient();
const mutation = useMutation({
  mutationFn: (data: CreateRequest) =>
    fetch("/api/resources", { method: "POST", body: JSON.stringify(data) }),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ["resources"] }),
});
```

Do not introduce React Query into projects that don't already use it — Server Components with `fetch` may be sufficient.

## When Using React Hook Form (Form State)

React Hook Form is preferred for complex forms with validation.

```tsx
"use client";

import { useForm } from "react-hook-form";
import { TextField, Button, VStack } from "@navikt/ds-react";

interface FormData {
  name: string;
  email: string;
}

export function RegistrationForm() {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormData>();

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <VStack gap="space-16">
        <TextField
          label="Navn"
          {...register("name", { required: "Navn er påkrevd" })}
          error={errors.name?.message}
        />
        <TextField
          label="E-post"
          type="email"
          {...register("email", { required: "E-post er påkrevd" })}
          error={errors.email?.message}
        />
        <Button type="submit">Registrer</Button>
      </VStack>
    </form>
  );
}
```

For simple forms, Server Actions with `useActionState` (shown above) may be simpler.

## Package Manager

**pnpm** is the standard package manager for new Nav frontend projects.

```bash
# ✅ Use pnpm
pnpm install
pnpm add @navikt/ds-react
pnpm test

# Lock file: pnpm-lock.yaml (commit this)
```

## Boundaries

### ✅ Always

- Use Aksel Design System components
- Use spacing tokens with `space-` prefix
- Mobile-first responsive design
- Norwegian number formatting
- Explicit error handling in API routes
- pnpm for new projects
- Preserve existing code structure when making targeted fixes — don't rename, restructure, or refactor working code beyond the task at hand

### ⚠️ Ask First

- Adding custom Tailwind utilities
- Deviating from Aksel patterns
- Changing authentication flow
- Modifying data aggregation logic
- Introducing React Query or React Hook Form into existing projects

### 🚫 Never

- Use Tailwind padding/margin utilities (`p-*`, `m-*`)
- Use numeric spacing without `space-` prefix
- Ignore accessibility requirements
- Skip responsive props
- Add code comments unless explicitly requested

## Related

| Resource                   | Use For                                                          |
| -------------------------- | ---------------------------------------------------------------- |
| `@aksel-agent`             | Aksel Design System component patterns and spacing tokens        |
| `@accessibility-agent`     | WCAG 2.1/2.2 compliance and accessibility testing                |
| `performance` instruction  | Core Web Vitals and bundle optimization                          |
| `aksel-spacing` skill      | Responsive spacing token reference                               |
| `playwright-testing` skill | E2E testing with Playwright and axe-core                         |
| aksel.nav.no/llm.md        | Full Aksel documentation index (components, tokens, foundations) |
