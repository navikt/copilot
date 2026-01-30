# GitHub Copilot API Compliance

Last Updated: 2026-01-30

## Overview

This document outlines our compliance with GitHub's Copilot APIs and the migration from deprecated endpoints to current supported APIs.

## API Deprecation Timeline

### Deprecated APIs (DO NOT USE)

| API | Sunset Date | Status |
|-----|-------------|--------|
| `/orgs/{org}/copilot/usage` | February 2025 | ❌ Deprecated |
| User-level Feature Engagement Metrics API | March 2, 2026 | ❌ Deprecated |
| Direct Data Access API | March 2, 2026 | ❌ Deprecated |
| Legacy Copilot Metrics API | April 2, 2026 | ❌ Deprecated |

### Current Supported APIs (IN USE)

| API Endpoint | Purpose | Status | Version |
|--------------|---------|--------|---------|
| `GET /orgs/{org}/copilot/metrics` | Organization Copilot usage metrics | ✅ Supported | 2022-11-28 |
| `GET /orgs/{org}/copilot/billing` | Copilot billing information | ✅ Supported | 2022-11-28 |
| `GET /orgs/{org}/members/{username}/copilot` | Individual user Copilot seat status | ✅ Supported | 2022-11-28 |
| `POST /orgs/{org}/copilot/billing/selected_users` | Assign Copilot seats to users | ✅ Supported | 2022-11-28 |
| `DELETE /orgs/{org}/copilot/billing/selected_users` | Remove Copilot seats from users | ✅ Supported | 2022-11-28 |

## Implementation Details

### API Version Headers

All Copilot API requests include the `X-GitHub-Api-Version: 2022-11-28` header as recommended by GitHub's best practices. This ensures:

- **Stability**: API behavior won't change unexpectedly
- **Predictability**: Consistent response schemas
- **Migration Window**: 24-month support period for planned upgrades

### Code Location

All GitHub API integrations are located in:
- `/apps/my-copilot/src/lib/github.ts` - Main API functions
- `/apps/my-copilot/src/lib/cached-github.ts` - Cached wrappers

### Functions Using Current APIs

1. **`getCopilotUsage(org: string)`**
   - Endpoint: `GET /orgs/{org}/copilot/metrics`
   - Purpose: Retrieve daily Copilot usage metrics
   - Returns: Array of `CopilotMetrics` with up to 100 days of data
   - Features: Breakdowns by language, IDE, model, chat usage

2. **`getCopilotBilling(org: string)`**
   - Endpoint: `GET /orgs/{org}/copilot/billing`
   - Purpose: Retrieve billing and seat information
   - Returns: Seat breakdown, feature settings, plan type

3. **`getCopilotSeat(org: string, username: string)`**
   - Endpoint: `GET /orgs/{org}/members/{username}/copilot`
   - Purpose: Check individual user's Copilot seat status
   - Returns: User's seat assignment and activity details

4. **`assignUserToCopilot(org: string, username: string)`**
   - Endpoint: `POST /orgs/{org}/copilot/billing/selected_users`
   - Purpose: Assign a Copilot seat to a user
   - Returns: Number of seats created

5. **`unassignUserFromCopilot(org: string, username: string)`**
   - Endpoint: `DELETE /orgs/{org}/copilot/billing/selected_users`
   - Purpose: Remove a Copilot seat from a user
   - Returns: Number of seats cancelled

## Migration Status

### ✅ Completed

- [x] Audit all Copilot API endpoints in codebase
- [x] Verify usage of current, supported APIs
- [x] Add API version headers to all requests
- [x] Update TypeScript types to match latest schema
- [x] Test all API integrations
- [x] Document API compliance

### ❌ Not Required

- No migration needed from deprecated endpoints (already using correct APIs)
- No schema changes needed (types already match current API)

## Response Schema

### Copilot Metrics Response

The `GET /orgs/{org}/copilot/metrics` endpoint returns an array of daily metrics with the following structure:

```typescript
{
  date: string;                          // ISO date string
  total_active_users?: number;           // Users who used Copilot
  total_engaged_users?: number;          // Users who engaged with suggestions
  copilot_ide_code_completions?: {
    total_engaged_users?: number;
    languages?: Array<{
      name?: string;
      total_engaged_users?: number;
    }>;
    editors?: Array<{
      name?: string;
      total_engaged_users?: number;
      models?: Array<{
        name?: string;
        is_custom_model?: boolean;
        languages?: Array<{
          name?: string;
          total_code_suggestions?: number;
          total_code_acceptances?: number;
          total_code_lines_suggested?: number;
          total_code_lines_accepted?: number;
        }>;
      }>;
    }>;
  };
  copilot_ide_chat?: {...};              // IDE chat metrics
  copilot_dotcom_chat?: {...};           // GitHub.com chat metrics
  copilot_dotcom_pull_requests?: {...};  // PR summary metrics
}
```

## Best Practices

### 1. API Versioning
- Always include `X-GitHub-Api-Version` header
- Pin to specific version for stability
- Plan migrations 6+ months before version sunset

### 2. Caching
- Use Next.js caching for expensive API calls
- Cache billing data for 1 hour (changes infrequently)
- Cache usage metrics for 1 hour (updated daily)
- Cache user seat status for 1 minute (may change frequently)

### 3. Error Handling
- Handle 404 errors gracefully (e.g., user not assigned to Copilot)
- Return structured error responses
- Log errors for monitoring

### 4. Rate Limiting
- Authenticated requests: 5,000/hour
- Use caching to minimize API calls
- Implement exponential backoff for retries

## Monitoring

### API Health Checks

Monitor the following:
- API response times
- Error rates by endpoint
- Cache hit/miss ratios
- Rate limit consumption

### Deprecation Warnings

Watch for these headers in API responses:
- `Deprecation: true` - API version is deprecated
- `Sunset: <date>` - Final date the version will be supported

## References

- [GitHub Copilot Metrics API Documentation](https://docs.github.com/en/rest/copilot/copilot-metrics)
- [GitHub Copilot User Management API](https://docs.github.com/en/rest/copilot/copilot-user-management)
- [GitHub API Versioning](https://docs.github.com/en/rest/about-the-rest-api/api-versions)
- [Deprecation Notice (Jan 29, 2026)](https://github.blog/changelog/2026-01-29-closing-down-notice-of-legacy-copilot-metrics-apis/)

## Contact

For questions about this integration, contact the platform team or refer to the [repository documentation](../../README.md).
