# Phase 5-6 Migration Guide

## Overview

This guide provides step-by-step instructions for completing the final phases of migrating my-copilot to use the new copilot-api backend.

## Phase 5: Token Exchange and BFF Integration

### 5.1 Backend API Client (✅ Created)

**File:** `apps/my-copilot/src/lib/backend-api.ts`

The backend API client handles Azure AD OBO token exchange via the Texas sidecar.

**Key functions:**
- `exchangeToken(userToken)` - Exchanges user token for backend OBO token
- `backendRequest<T>(path, userToken, options)` - Makes authenticated requests to backend API

**Environment variables needed:**
- `NAIS_TOKEN_EXCHANGE_ENDPOINT` - Texas sidecar endpoint (auto-injected by NAIS)
- `COPILOT_API_URL` - Backend API URL (default: `http://copilot-api`)
- `NAIS_CLUSTER_NAME` - Cluster name for audience construction

### 5.2 Create BFF Proxy Routes

Create server-side API routes that proxy to the backend API. This keeps the client-side code unchanged while routing through the backend.

#### Example: BigQuery Usage Metrics Proxy

**File:** `apps/my-copilot/src/app/api/backend/usage/metrics/route.ts`

```typescript
import { NextRequest, NextResponse } from "next/server";
import { getToken } from "@/lib/auth";
import { backendRequest } from "@/lib/backend-api";

export async function GET(request: NextRequest) {
  const token = await getToken();
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const searchParams = request.nextUrl.searchParams;
  const days = searchParams.get("days");
  const queryString = days ? `?days=${days}` : "";

  try {
    const data = await backendRequest(
      `/api/v1/copilot/usage/metrics${queryString}`,
      token,
    );
    return NextResponse.json(data);
  } catch (error) {
    console.error("Backend API error:", error);
    return NextResponse.json(
      { error: "Failed to fetch usage metrics" },
      { status: 500 },
    );
  }
}
```

#### BFF Routes to Create

| Frontend Endpoint | Backend Endpoint | Method | Description |
|------------------|------------------|--------|-------------|
| `/api/backend/usage/metrics` | `/api/v1/copilot/usage/metrics` | GET | Daily usage metrics |
| `/api/backend/adoption/summary` | `/api/v1/copilot/adoption/summary` | GET | Adoption summary |
| `/api/backend/adoption/teams` | `/api/v1/copilot/adoption/teams` | GET | Team adoption |
| `/api/backend/adoption/languages` | `/api/v1/copilot/adoption/languages` | GET | Language adoption |
| `/api/backend/customizations/details` | `/api/v1/copilot/customizations/details` | GET | Customization details |
| `/api/backend/customizations/usage` | `/api/v1/copilot/customizations/usage` | GET | Customization usage |
| `/api/backend/billing` | `/api/v1/copilot/billing` | GET | Billing data |
| `/api/backend/seats/:username` | `/api/v1/copilot/seats/:username` | GET/POST/DELETE | Seat management |
| `/api/backend/saml/:identity` | `/api/v1/copilot/saml/:identity` | GET | SAML lookup |

### 5.3 Update Existing Code to Use BFF Routes

#### Option A: Minimal Changes (Recommended)

Keep existing imports and functions, but redirect them to use BFF routes instead of direct BigQuery/GitHub calls.

**Update `src/lib/cached-bigquery.ts`:**

```typescript
// Old: Direct BigQuery calls
export async function getCachedDailyMetrics(days?: number) {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 3600 });
  cacheTag("daily-metrics");

  // NEW: Call BFF route instead
  const queryString = days ? `?days=${days}` : "";
  const response = await fetch(`/api/backend/usage/metrics${queryString}`, {
    headers: { "Content-Type": "application/json" },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch daily metrics: ${response.status}`);
  }

  return response.json();
}
```

**Update `src/lib/cached-github.ts`:**

```typescript
export async function getCachedCopilotBilling() {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 3600 });
  cacheTag("copilot-billing");

  // NEW: Call BFF route instead
  const response = await fetch("/api/backend/billing", {
    headers: { "Content-Type": "application/json" },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch billing: ${response.status}`);
  }

  return response.json();
}
```

#### Option B: Create New Backend-Specific Functions

Create parallel functions with `-backend` suffix, gradually migrate pages, then delete old functions.

### 5.4 Test Token Exchange Flow

**Test script:** `apps/my-copilot/scripts/test-token-exchange.ts`

```typescript
import { exchangeToken } from "@/lib/backend-api";

async function testTokenExchange() {
  const testToken = process.env.TEST_USER_TOKEN;
  if (!testToken) {
    console.error("TEST_USER_TOKEN not set");
    process.exit(1);
  }

  try {
    const oboToken = await exchangeToken(testToken);
    console.log("✅ Token exchange successful");
    console.log("OBO token length:", oboToken.length);

    // Decode JWT to verify claims (without signature validation)
    const [, payload] = oboToken.split(".");
    const claims = JSON.parse(Buffer.from(payload, "base64").toString());
    console.log("Audience:", claims.aud);
    console.log("Issuer:", claims.iss);
    console.log("Authorized party:", claims.azp);
  } catch (error) {
    console.error("❌ Token exchange failed:", error);
    process.exit(1);
  }
}

testTokenExchange();
```

---

## Phase 6: Cleanup Next.js

### 6.1 Remove Direct BigQuery Access

**Remove from `package.json`:**
```json
{
  "dependencies": {
    "@google-cloud/bigquery": "^X.X.X"  // DELETE
  }
}
```

**Delete files:**
- `src/lib/bigquery.ts`
- `src/lib/bigquery-config.ts`
- `src/lib/bigquery-config.test.ts`

**Update `src/lib/cached-bigquery.ts`:**
- Remove all BigQuery client imports
- Replace with fetch calls to BFF routes (see 5.3)

### 6.2 Remove Direct GitHub Access

**Remove from `package.json`:**
```json
{
  "dependencies": {
    "@octokit/rest": "^X.X.X",          // DELETE
    "@octokit/auth-app": "^X.X.X"       // DELETE
  }
}
```

**Delete files:**
- `src/lib/github.ts` (keep types if needed)

**Update `src/lib/cached-github.ts`:**
- Remove all Octokit imports
- Replace with fetch calls to BFF routes

### 6.3 Update Environment Variables

**Remove from `.env` and NAIS configs:**
- ❌ `GITHUB_APP_ID`
- ❌ `GITHUB_APP_PRIVATE_KEY`
- ❌ `GITHUB_APP_INSTALLATION_ID`
- ❌ `GCP_TEAM_PROJECT_ID`
- ❌ `COPILOT_METRICS_DATASET`
- ❌ `COPILOT_METRICS_TABLE`
- ❌ `COPILOT_ADOPTION_DATASET`

**Add to `.env` and NAIS configs:**
- ✅ `COPILOT_API_URL=http://copilot-api` (internal)
- ✅ `NAIS_TOKEN_EXCHANGE_ENDPOINT` (auto-injected by Texas)
- ✅ `NAIS_CLUSTER_NAME` (auto-injected by NAIS)

### 6.4 Update NAIS Configuration

**File:** `apps/my-copilot/.nais/app.yaml`

```yaml
spec:
  # REMOVE BigQuery access
  gcp:
    bigQueryDatasets: []  # DELETE entire section

  # UPDATE access policy
  accessPolicy:
    outbound:
      rules:
        - application: copilot-api  # Already added
          namespace: copilot
      external:
        # REMOVE:
        # - host: api.github.com
        # - host: bigquery.googleapis.com
        # - host: storage.googleapis.com

        # KEEP only:
        - host: login.microsoftonline.com  # Azure AD
        - host: mcp-registry.intern.dev.nav.no  # MCP registry (if used)
```

### 6.5 Validation Checklist

Before deploying to production:

- [ ] All BFF routes created and tested
- [ ] All pages use BFF routes (not direct BigQuery/GitHub)
- [ ] Token exchange works in dev environment
- [ ] Backend API validates tokens correctly
- [ ] Audit logs show correct user identity for mutations
- [ ] No references to `@google-cloud/bigquery` in code
- [ ] No references to `@octokit` in code
- [ ] NAIS config updated (no direct BigQuery/GitHub access)
- [ ] Environment variables cleaned up
- [ ] All tests pass (`pnpm test`)
- [ ] Build succeeds (`pnpm build`)
- [ ] Smoke test in dev-gcp
- [ ] Load test critical endpoints
- [ ] Monitor logs for errors (24h in dev)

### 6.6 Rollback Plan

If issues arise in production:

1. **Immediate:** Revert NAIS config to restore direct access
2. **Short-term:** Update code to use direct clients again
3. **Long-term:** Fix backend API issues, re-deploy

**Rollback diff:**
```diff
# apps/my-copilot/.nais/app.yaml
+ gcp:
+   bigQueryDatasets:
+     - name: copilot_metrics
+       permission: READ
+     - name: copilot_adoption
+       permission: READ

  accessPolicy:
    outbound:
+     external:
+       - host: api.github.com
+       - host: bigquery.googleapis.com
```

---

## Testing Strategy

### Unit Tests

Test BFF routes in isolation:

```typescript
// apps/my-copilot/src/app/api/backend/usage/metrics/route.test.ts
import { GET } from "./route";
import { NextRequest } from "next/server";

jest.mock("@/lib/backend-api");

describe("/api/backend/usage/metrics", () => {
  it("proxies to backend API", async () => {
    const request = new NextRequest("http://localhost/api/backend/usage/metrics");
    const response = await GET(request);

    expect(response.status).toBe(200);
    const data = await response.json();
    expect(Array.isArray(data)).toBe(true);
  });
});
```

### Integration Tests

Test end-to-end flow with real tokens (in dev):

```bash
# Get a real token from dev environment
export TEST_USER_TOKEN=$(curl -s https://my-copilot.intern.dev.nav.no/api/debug/token)

# Test token exchange
pnpm test:token-exchange

# Test backend API call
curl -H "Authorization: Bearer $TEST_USER_TOKEN" \
  https://my-copilot.intern.dev.nav.no/api/backend/billing
```

### Load Tests

Simulate production traffic:

```bash
# Install k6
brew install k6  # or use Docker

# Run load test
k6 run apps/my-copilot/k6/load-test.js
```

**Sample k6 script:**

```javascript
// apps/my-copilot/k6/load-test.js
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 10 },   // Ramp up
    { duration: '5m', target: 50 },   // Steady load
    { duration: '1m', target: 0 },    // Ramp down
  ],
};

export default function () {
  const res = http.get('https://my-copilot.intern.dev.nav.no/api/backend/billing');

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
```

---

## Monitoring and Observability

### Key Metrics to Track

**Backend API (copilot-api):**
- Request rate by endpoint
- Response times (p50, p95, p99)
- Error rate (4xx, 5xx)
- Token validation failures
- BigQuery query times
- GitHub API rate limit usage

**BFF (my-copilot):**
- Token exchange success rate
- Token exchange latency
- Backend API call success rate
- Cache hit rate (Next.js cache)

### Grafana Dashboards

Create dashboards for:

1. **Backend API Health**
   - Request rate
   - Error rate
   - Response time percentiles
   - GitHub metrics freshness

2. **Token Exchange Flow**
   - Exchange success rate
   - Exchange latency
   - Token validation failures

3. **User Experience**
   - End-to-end page load times
   - API call success rates
   - Error rates by endpoint

### Alerts

Set up alerts for:

- **Critical:** Backend API error rate > 5%
- **Warning:** Backend API p95 > 1s
- **Warning:** Token exchange failure rate > 1%
- **Info:** GitHub metrics stale > 10min

---

## Common Issues and Solutions

### Issue 1: Token Exchange Fails with 401

**Symptom:** `Token exchange failed (401): Unauthorized`

**Cause:** Texas sidecar not configured or wrong audience

**Solution:**
1. Verify `NAIS_TOKEN_EXCHANGE_ENDPOINT` is set
2. Check audience format: `api://{cluster}.copilot.copilot-api/.default`
3. Verify NAIS config has `azure.application.enabled: true`

### Issue 2: Backend Rejects Token (403 Forbidden)

**Symptom:** `Backend API returned 403`

**Cause:** Backend doesn't trust my-copilot as caller (azp check failed)

**Solution:**
1. Verify backend NAIS config has `accessPolicy.inbound.rules` with my-copilot
2. Check `AZURE_APP_PRE_AUTHORIZED_APPS` in backend includes my-copilot client ID
3. Decode token and verify `azp` claim matches my-copilot

### Issue 3: High Latency After Migration

**Symptom:** Page load times increased from 200ms to 800ms

**Cause:** Extra network hop (my-copilot → copilot-api) + no caching

**Solution:**
1. Enable Next.js caching on BFF routes (`"use cache"`)
2. Set appropriate `cacheLife` (1h for dashboards, 60s for seat status)
3. Implement cache warming for critical endpoints
4. Consider parallel requests where possible

### Issue 4: Cache Invalidation Not Working

**Symptom:** Seat assignments don't show up immediately

**Cause:** BFF cache not invalidated after mutations

**Solution:**
1. After seat mutations, call `revalidateTag('copilot-seats')`
2. Backend should return cache control headers
3. Consider shorter TTL for mutable data (60s vs 1h)

---

## Performance Optimization

### 1. Parallel Backend Calls

When fetching multiple independent datasets, call backend API in parallel:

```typescript
// ❌ Bad: Sequential calls
const billing = await backendRequest("/api/v1/copilot/billing", token);
const adoption = await backendRequest("/api/v1/copilot/adoption/summary", token);

// ✅ Good: Parallel calls
const [billing, adoption] = await Promise.all([
  backendRequest("/api/v1/copilot/billing", token),
  backendRequest("/api/v1/copilot/adoption/summary", token),
]);
```

### 2. Token Exchange Caching

Cache OBO tokens to avoid repeated exchanges:

```typescript
// Cache OBO token for same user token (short TTL)
const oboTokenCache = new Map<string, { token: string; expiry: number }>();

async function exchangeTokenCached(userToken: string): Promise<string> {
  const cached = oboTokenCache.get(userToken);
  if (cached && cached.expiry > Date.now()) {
    return cached.token;
  }

  const oboToken = await exchangeToken(userToken);
  oboTokenCache.set(userToken, {
    token: oboToken,
    expiry: Date.now() + 300000, // 5min cache
  });

  return oboToken;
}
```

### 3. Backend Response Caching

Let backend set cache headers, respect them in BFF:

```typescript
// Backend sets:
res.setHeader("Cache-Control", "public, max-age=3600");

// BFF respects:
const backendRes = await fetch(...);
const cacheControl = backendRes.headers.get("cache-control");
// Pass through to Next.js response
```

---

## Success Criteria

Migration is complete when:

1. ✅ All pages load successfully using BFF routes
2. ✅ No direct BigQuery/GitHub API calls from my-copilot
3. ✅ Token exchange succeeds for all authenticated users
4. ✅ Backend validates tokens and extracts user identity
5. ✅ Audit logs show correct user for seat mutations
6. ✅ Performance is comparable to pre-migration (±10%)
7. ✅ Error rate < 0.1% in production
8. ✅ All dependencies removed from package.json
9. ✅ NAIS config updated (no direct access)
10. ✅ Documentation complete and validated

---

## Timeline Estimate

**Phase 5 (BFF Integration):** 3-5 days
- Day 1: Create all BFF proxy routes
- Day 2: Update cached-bigquery.ts and cached-github.ts
- Day 3: Test token exchange and backend calls
- Day 4-5: Integration testing and bug fixes

**Phase 6 (Cleanup):** 2-3 days
- Day 1: Remove dependencies and unused files
- Day 2: Update NAIS configs and environment variables
- Day 3: Final validation and smoke testing

**Total:** 5-8 days for full migration

---

## Next Steps

1. **Create BFF proxy routes** (see 5.2)
2. **Test token exchange** in dev (see 5.4)
3. **Migrate one page** as proof-of-concept
4. **Validate end-to-end** flow
5. **Migrate remaining pages** incrementally
6. **Remove dependencies** (see 6.1-6.2)
7. **Update NAIS config** (see 6.4)
8. **Final validation** (see 6.5)
9. **Deploy to prod** with monitoring
10. **Monitor for 48h** before declaring success
