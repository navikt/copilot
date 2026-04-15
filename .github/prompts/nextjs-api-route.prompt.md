---
name: nextjs-api-route
description: Scaffold en Next.js App Router API-rute med validering, feilhåndtering, auth og test
---

# Next.js API Route

Create a complete Next.js App Router API route with Nav standards.

## Questions

1. What is the endpoint path? (e.g. `/api/vedtak`, `/api/usage`)
2. Which HTTP methods? (GET, POST, PUT, DELETE)
3. Does it need authentication?
4. What is returned? (structure / type)

## Create these files

### 1. Route Handler

```typescript
// src/app/api/{{ressurs}}/route.ts
import { NextResponse } from "next/server";
import { getUser } from "@/lib/auth";

export async function GET() {
  const user = await getUser(false);
  if (!user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { data, error } = await fetchData();

  if (error) {
    console.error("Failed to fetch {{ressurs}}:", error);
    return NextResponse.json(
      { error: "Could not fetch data" },
      { status: 500 },
    );
  }

  return NextResponse.json(data);
}

export async function POST(request: Request) {
  const user = await getUser(false);
  if (!user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  let body: Create{{Ressurs}}Request;
  try {
    body = await request.json();
  } catch {
    return NextResponse.json(
      { error: "Invalid JSON" },
      { status: 400 },
    );
  }

  // Validation
  if (!body.requiredField) {
    return NextResponse.json(
      { error: "requiredField is required" },
      { status: 400 },
    );
  }

  const result = await createResource(body);
  return NextResponse.json(result, { status: 201 });
}
```

### 2. Route med dynamisk segment

```typescript
// src/app/api/{{ressurs}}/[id]/route.ts
import { NextResponse } from "next/server";
import { getUser } from "@/lib/auth";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const user = await getUser(false);
  if (!user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { id } = await params;
  const resource = await findById(id);

  if (!resource) {
    return NextResponse.json(
      { error: `{{Ressurs}} ${id} not found` },
      { status: 404 },
    );
  }

  return NextResponse.json(resource);
}
```

### 3. Test

```typescript
// src/app/api/{{ressurs}}/route.test.ts
import { GET, POST } from "./route";
import { vi } from "vitest";

vi.mock("@/lib/auth", () => ({
  getUser: vi.fn(),
}));

vi.mock("@/lib/{{ressurs}}", () => ({
  fetchData: vi.fn(),
  createResource: vi.fn(),
}));

import { getUser } from "@/lib/auth";
import { fetchData, createResource } from "@/lib/{{ressurs}}";

describe("GET /api/{{ressurs}}", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return 401 when not authenticated", async () => {
    vi.mocked(getUser).mockResolvedValue(null);

    const response = await GET();
    expect(response.status).toBe(401);
  });

  it("should return data when authenticated", async () => {
    vi.mocked(getUser).mockResolvedValue({ name: "test" });
    vi.mocked(fetchData).mockResolvedValue({
      data: [{ id: "1" }],
      error: null,
    });

    const response = await GET();
    const json = await response.json();

    expect(response.status).toBe(200);
    expect(json).toHaveLength(1);
  });

  it("should return 500 on fetch error", async () => {
    vi.mocked(getUser).mockResolvedValue({ name: "test" });
    vi.mocked(fetchData).mockResolvedValue({
      data: null,
      error: "DB error",
    });

    const response = await GET();
    expect(response.status).toBe(500);
  });
});

describe("POST /api/{{ressurs}}", () => {
  it("should return 400 for invalid JSON", async () => {
    vi.mocked(getUser).mockResolvedValue({ name: "test" });

    const request = new Request("http://localhost/api/{{ressurs}}", {
      method: "POST",
      body: "invalid json",
    });

    const response = await POST(request);
    expect(response.status).toBe(400);
  });

  it("should create resource", async () => {
    vi.mocked(getUser).mockResolvedValue({ name: "test" });
    vi.mocked(createResource).mockResolvedValue({ id: "1" });

    const request = new Request("http://localhost/api/{{ressurs}}", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ requiredField: "value" }),
    });

    const response = await POST(request);
    expect(response.status).toBe(201);
  });
});
```

## Rules

- **Always** check auth first
- **Always** return explicit status codes
- **Always** log errors server-side, return generic error messages to client
- **Never** return stack traces or internal error messages to client
- **Never** use `any` as type for request body

## Forstå koden

After generating the route, explain:

1. **Auth-mønsteret** — Why `getUser(false)` returns null instead of redirecting. When would you use `getUser()` (with redirect) instead?
2. **Feilhåndtering** — Why the try/catch around `request.json()`, and why generic error messages to the client but detailed logs server-side (information leakage).
3. **Next.js 15+ async APIs** — Why `params` is a Promise that must be awaited. What breaks if you don't?
4. **Teststrukturen** — Why mocking auth separately from data fetching. What would go wrong with integration tests instead of unit tests here?

🔴 **Rød sone**: Auth checks and input validation are security-critical — understand *why* each check exists before adapting this pattern.

Still gjerne spørsmål om valgene over.
