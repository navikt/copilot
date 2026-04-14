---
applyTo: "src/**/*.{tsx,ts}"
---

# Performance Optimization

Core Web Vitals targets and performance patterns for Next.js applications with Aksel Design System on NAIS.

## Core Web Vitals Targets

All user-facing pages must meet "Good" thresholds:

| Metric | Good | Needs Improvement | Poor |
| --- | --- | --- | --- |
| LCP (Largest Contentful Paint) | < 2.5s | 2.5s – 4.0s | > 4.0s |
| INP (Interaction to Next Paint) | < 200ms | 200ms – 500ms | > 500ms |
| CLS (Cumulative Layout Shift) | < 0.1 | 0.1 – 0.25 | > 0.25 |
| TTFB (Time to First Byte) | < 800ms | 800ms – 1800ms | > 1800ms |

## Server Components (Default)

Next.js 16 App Router defaults to Server Components — they render on the server and send zero JavaScript to the client. Use this to your advantage:

- Prefer server components for data fetching and static UI
- Push the `"use client"` boundary as low as possible in the component tree
- Only mark components as client when they need interactivity (`useState`, `useEffect`, event handlers)

```tsx
// ✅ Good — server component fetches data, thin client component for interactivity
// app/dashboard/page.tsx (server component)
export default async function DashboardPage() {
  const data = await fetchDashboardData();

  return (
    <VStack gap="8">
      <Heading size="large" level="1">Dashboard</Heading>
      <StaticSummary data={data} />
      <InteractiveFilter items={data.items} />
    </VStack>
  );
}

// components/interactive-filter.tsx
"use client";
import { useState } from "react";

export function InteractiveFilter({ items }: { items: Item[] }) {
  const [filter, setFilter] = useState("");
  // Only this component ships JS to the client
  return <TextField label="Filter" value={filter} onChange={(e) => setFilter(e.target.value)} />;
}
```

```tsx
// ❌ Bad — entire page is client-side, ships all JS to browser
"use client";

import { useState, useEffect } from "react";

export default function DashboardPage() {
  const [data, setData] = useState(null);
  useEffect(() => {
    fetch("/api/dashboard").then((r) => r.json()).then(setData);
  }, []);
  // All components here become client components
}
```

## Data Fetching

### Parallel data fetching

```tsx
// ✅ Good — parallel fetching with Promise.all
export default async function Page() {
  const [users, metrics, config] = await Promise.all([
    fetchUsers(),
    fetchMetrics(),
    fetchConfig(),
  ]);

  return <Dashboard users={users} metrics={metrics} config={config} />;
}

// ❌ Bad — sequential awaits block each other
export default async function Page() {
  const users = await fetchUsers();       // 200ms
  const metrics = await fetchMetrics();   // 300ms
  const config = await fetchConfig();     // 100ms
  // Total: 600ms instead of ~300ms
}
```

### Request-level deduplication

```tsx
import { cache } from "react";

// Deduplicated across the same render pass
const getUser = cache(async (id: string) => {
  const res = await fetch(`/api/users/${id}`);
  return res.json();
});
```

### Streaming with Suspense

```tsx
import { Suspense } from "react";
import { Skeleton } from "@navikt/ds-react";

export default function Page() {
  return (
    <VStack gap="8">
      <Heading size="large" level="1">Oversikt</Heading>
      {/* Critical content renders immediately */}
      <QuickSummary />
      {/* Non-critical content streams in */}
      <Suspense fallback={<Skeleton variant="rounded" height={300} />}>
        <SlowAnalytics />
      </Suspense>
    </VStack>
  );
}
```

### Fetch with caching

```tsx
// Revalidate every 60 seconds (ISR)
const data = await fetch("https://api.example.com/data", {
  next: { revalidate: 60 },
});

// Cache for the duration of the request
const data = await fetch("https://api.example.com/data", {
  cache: "force-cache",
});

// Always fresh
const data = await fetch("https://api.example.com/data", {
  cache: "no-store",
});
```

## Image Optimization

Always use `next/image` to get automatic WebP/AVIF conversion, resizing, and lazy loading:

```tsx
import Image from "next/image";

// ✅ Good — explicit dimensions prevent CLS, priority for LCP image
<Image
  src="/hero-banner.png"
  alt="Oversikt over tjenester"
  width={1200}
  height={630}
  priority
  sizes="(max-width: 768px) 100vw, 1200px"
/>

// ✅ Good — below-the-fold image, lazy loaded by default
<Image
  src="/chart.png"
  alt="Bruksstatistikk"
  width={800}
  height={400}
  sizes="(max-width: 768px) 100vw, 800px"
/>

// ❌ Bad — native img tag, no optimization, causes CLS
<img src="/hero-banner.png" />
```

Key rules:
- Set `priority` on above-the-fold images (LCP candidates)
- Always provide `width` and `height` to prevent layout shift
- Use `sizes` prop for responsive images to avoid downloading oversized images
- Default `loading="lazy"` is applied automatically — don't set it on `priority` images

## Font Optimization

Use `next/font` for zero-layout-shift font loading:

```tsx
// app/layout.tsx
import localFont from "next/font/local";

const navFont = localFont({
  src: [
    { path: "./fonts/Source-Sans-3-Regular.woff2", weight: "400" },
    { path: "./fonts/Source-Sans-3-SemiBold.woff2", weight: "600" },
  ],
  display: "swap", // Prevents FOIT (Flash of Invisible Text)
  preload: true,
});

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="nb" className={navFont.className}>
      <body>{children}</body>
    </html>
  );
}
```

## Bundle Optimization

### Lazy loading heavy components

```tsx
import dynamic from "next/dynamic";

// ✅ Good — chart library only loaded when needed
const HeavyChart = dynamic(() => import("@/components/heavy-chart"), {
  loading: () => <Skeleton variant="rounded" height={400} />,
  ssr: false,
});
```

### Tree-shaking: named imports

```tsx
// ✅ Good — tree-shakeable named import
import { Button, Heading } from "@navikt/ds-react";

// ❌ Bad — imports the entire package, defeats tree-shaking
import * as Aksel from "@navikt/ds-react";
```

### Barrel file anti-pattern

```tsx
// ❌ Bad — barrel export pulls in every component
// components/index.ts
export { Header } from "./header";
export { Footer } from "./footer";
export { Sidebar } from "./sidebar";
export { HeavyChart } from "./heavy-chart"; // Always bundled even if unused

// ✅ Good — import directly from the component file
import { Header } from "@/components/header";
```

### Analyze your bundle

```bash
# Install and run bundle analyzer
ANALYZE=true next build
```

Configure in `next.config.ts`:

```typescript
import bundleAnalyzer from "@next/bundle-analyzer";

const withBundleAnalyzer = bundleAnalyzer({
  enabled: process.env.ANALYZE === "true",
});

export default withBundleAnalyzer(nextConfig);
```

## Aksel-Specific Performance

```tsx
// ✅ Good — individual component imports (tree-shakeable)
import { Button } from "@navikt/ds-react";
import { Heading } from "@navikt/ds-react";

// ❌ Bad — wildcard import loads the entire library
import * as Aksel from "@navikt/ds-react";

// ✅ Good — specific icon import
import { ChevronRightIcon } from "@navikt/aksel-icons";

// ❌ Bad — barrel import of all icons
import * as Icons from "@navikt/aksel-icons";
```

CSS tokens from `@navikt/ds-css` are loaded once globally — no additional performance concern.

## Anti-Patterns

Common performance mistakes to avoid:

1. **Sequential `await`** when requests are independent — use `Promise.all()`
2. **Client-side data fetching** that could be done in a server component
3. **Missing `key` prop** on list items — causes unnecessary re-renders and DOM thrashing
4. **State in parent** when it belongs in child — triggers render cascades in the entire subtree
5. **Missing `React.memo` / `useMemo`** for expensive computations or stable references
6. **Unoptimized images** — no `next/image`, no explicit dimensions
7. **Synchronous `import()`** of large libraries — use `next/dynamic` or dynamic `import()`
8. **Layout shifts from dynamic content** without skeleton or placeholder
9. **Missing `loading.tsx`** for route segments — shows blank screen during navigation
10. **Over-fetching** — returning entire database rows when only 2 fields are needed

## Measurement

### reportWebVitals

```tsx
// app/web-vitals.tsx
"use client";

import { useReportWebVitals } from "next/web-vitals";

export function WebVitals() {
  useReportWebVitals((metric) => {
    const { name, value, rating } = metric;

    // Log to analytics or monitoring
    console.log(`${name}: ${value} (${rating})`);

    // Send to your analytics endpoint
    if (rating === "poor") {
      fetch("/api/vitals", {
        method: "POST",
        body: JSON.stringify({ name, value, rating }),
      });
    }
  });

  return null;
}
```

### Tools

- **`web-vitals`** library for Real User Monitoring (RUM)
- **Lighthouse CI** in GitHub Actions for automated performance budgets
- **Next.js built-in analytics** via `useReportWebVitals`
- **Chrome DevTools Performance tab** for profiling renders and long tasks

## Caching

### HTTP Cache-Control for API routes

```tsx
export async function GET() {
  const data = await fetchData();

  return NextResponse.json(data, {
    headers: {
      "Cache-Control": "public, s-maxage=60, stale-while-revalidate=300",
    },
  });
}
```

### ISR (Incremental Static Regeneration)

```tsx
// Page revalidates every 60 seconds
export const revalidate = 60;

export default async function Page() {
  const data = await fetchData();
  return <Dashboard data={data} />;
}
```

### Per-request memoization with React cache

```tsx
import { cache } from "react";

export const getUser = cache(async (id: string) => {
  const res = await fetch(`/api/users/${id}`);
  return res.json();
});

// Called in multiple server components — only one fetch per request
```

### Cross-request caching

```tsx
import { unstable_cache } from "next/cache";

const getCachedMetrics = unstable_cache(
  async () => fetchMetrics(),
  ["metrics"],
  { revalidate: 300, tags: ["metrics"] },
);
```

## Backend Performance (API Routes)

Response time budget: **< 200ms** for user-facing APIs.

```tsx
// ✅ Good — paginated, indexed, lean response
export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const page = Number(searchParams.get("page") ?? "1");
  const limit = Math.min(Number(searchParams.get("limit") ?? "20"), 100);

  const data = await db.query(
    "SELECT id, name, status FROM users WHERE active = true ORDER BY name LIMIT $1 OFFSET $2",
    [limit, (page - 1) * limit],
  );

  return NextResponse.json(data, {
    headers: { "Cache-Control": "private, max-age=10" },
  });
}
```

Key rules:
- **Connection pooling** — reuse database connections across requests
- **Indexed queries** — add indexes for WHERE and ORDER BY columns
- **Pagination** — never return unbounded result sets
- **Avoid N+1 queries** — use JOINs or batch fetching instead of loops
- **Streaming responses** — use `ReadableStream` for large datasets
- **Cache-Control headers** — set appropriate caching for API responses

## Boundaries

### ✅ Always

- Meet Core Web Vitals "Good" thresholds
- Use server components by default
- Use `next/image` for all images with explicit dimensions
- Measure performance with `useReportWebVitals` or Lighthouse

### ⚠️ Ask First

- Adding client-side state management libraries (Zustand, Jotai)
- Custom caching strategies beyond Next.js built-ins
- Disabling SSR for components (`ssr: false`)

### 🚫 Never

- Barrel exports that pull in entire packages
- Sequential `await` for independent data fetches
- Images without explicit `width` and `height`
- `import *` from `@navikt/ds-react` or `@navikt/aksel-icons`

## Related

| Resource | Use For |
|----------|---------|
| `@aksel-agent` | Aksel Design System component patterns and spacing tokens |
| `@observability-agent` | Prometheus metrics and Grafana dashboards for Core Web Vitals |
| `nextjs-aksel` instruction | Next.js App Router patterns with Aksel |
| `playwright-testing` skill | E2E testing to validate performance optimizations |
