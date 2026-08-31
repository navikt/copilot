---
name: nav-auth
description: Azure AD, TokenX, ID-porten, Maskinporten og JWT-validering for Nav-applikasjoner
license: MIT
compatibility: Application on Nais with authentication needs
metadata:
  domain: auth
  tags: azure-ad tokenx id-porten maskinporten jwt auth oasis
---

# Nav Authentication Skill

Patterns for authentication and authorization in Nav applications. Covers Azure AD, TokenX, ID-porten, Maskinporten, and JWT validation.

## When to Use

- Adding authentication to a Nais application
- Implementing service-to-service calls with TokenX
- Validating JWT tokens (Azure AD, ID-porten)
- Setting up machine-to-machine auth with Maskinporten
- Debugging auth failures

## Commands

```bash
# Decode JWT token payload (without verification — note: uses tr for base64url)
echo "<token>" | cut -d'.' -f2 | tr '_-' '/+' | base64 -d 2>/dev/null | jq .

# Fetch Azure AD OpenID config
curl -s "https://login.microsoftonline.com/nav.no/.well-known/openid-configuration" | jq .

# Check auth env var names in pod (works with distroless/Chainguard, values hidden)
kubectl get pod <pod> -n <namespace> -o jsonpath='{range .spec.containers[0].env[*]}{.name}{"\n"}{end}' | grep -E 'AZURE|TOKEN_X|IDPORTEN'
# Or use Nais Console: https://console.nav.cloud.nais.io -> App -> Env vars

# Test if JWKS endpoint is reachable
curl -s "$AZURE_OPENID_CONFIG_JWKS_URI" | jq '.keys | length'
```

## Authentication Types

### 1. Azure AD (Internal Nav Users)

**Use when**: Internal Nav employees need to access the application.

**Nais Configuration**:

```yaml
azure:
  application:
    enabled: true
    tenant: nav.no
```

**Environment Variables** (auto-injected): `AZURE_APP_CLIENT_ID`, `AZURE_APP_CLIENT_SECRET`, `AZURE_APP_WELL_KNOWN_URL`, `AZURE_OPENID_CONFIG_ISSUER`, `AZURE_OPENID_CONFIG_JWKS_URI`

**Kotlin/Ktor**:

```kotlin
install(Authentication) {
    jwt("azureAd") {
        verifier(azureAdConfiguration.jwksUri)
        validate { credential ->
            val audience = credential.payload.audience
            val roles = credential.payload.getClaim("roles")?.asList(String::class.java)

            if (audience.contains(expectedAudience)) {
                JWTPrincipal(credential.payload)
            } else null
        }
    }
}

routing {
    authenticate("azureAd") {
        get("/api/internal") {
            val principal = call.principal<JWTPrincipal>()
            val userId = principal?.payload?.subject
            call.respond(data)
        }
    }
}
```

**TypeScript/Next.js with `@navikt/oasis`**:

```typescript
import { validateAzureToken } from "@navikt/oasis";

export async function GET(request: Request) {
  const token = getToken(request);
  if (!token) return new Response("Unauthorized", { status: 401 });

  const validation = await validateAzureToken(token);
  if (!validation.ok) return new Response("Forbidden", { status: 403 });

  const userId = validation.payload.sub;
  return Response.json({ userId });
}

function getToken(request: Request): string | null {
  const auth = request.headers.get("Authorization");
  return auth?.replace("Bearer ", "") ?? null;
}
```

### 2. TokenX (Service-to-Service)

**Use when**: One Nav service calls another on behalf of a user.

**Nais Configuration**:

```yaml
tokenx:
  enabled: true

accessPolicy:
  inbound:
    rules:
      - application: calling-service
        namespace: team-calling
  outbound:
    rules:
      - application: downstream-service
        namespace: team-downstream
```

**Environment Variables**: `TOKEN_X_WELL_KNOWN_URL`, `TOKEN_X_CLIENT_ID`, `TOKEN_X_PRIVATE_JWK`

**TypeScript with `@navikt/oasis`**:

```typescript
import { requestOboToken, getToken } from "@navikt/oasis";

export async function GET(request: Request) {
  const token = getToken(request);
  if (!token) return new Response("Unauthorized", { status: 401 });

  // TokenX audience: "cluster:namespace:app-name"
  const obo = await requestOboToken(token, "dev-gcp:team-namespace:downstream-service");
  if (!obo.ok) return new Response("Token exchange failed", { status: 403 });

  // Service-to-service calls within the cluster use HTTP (internal traffic never leaves the mesh)
  const response = await fetch("http://downstream-service/api/data", {
    headers: { Authorization: `Bearer ${obo.token}` },
  });
  return Response.json(await response.json());
}
```

> **Note**: `@navikt/oasis` auto-caches OBO tokens. Azure AD audience uses different format: `"api://dev-gcp.namespace.app-name/.default"`

**Kotlin Token Exchange**:

```kotlin
suspend fun exchangeToken(token: String, targetApp: String): String {
    val response = httpClient.submitForm(
        url = System.getenv("TOKEN_X_TOKEN_ENDPOINT"),
        formParameters = Parameters.build {
            append("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
            append("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
            append("client_assertion", createClientAssertion())
            append("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
            append("subject_token", token)
            append("audience", "dev-gcp:team-namespace:$targetApp")
        }
    )
    return response.body<TokenResponse>().access_token
}
```

### 3. ID-porten (Citizens)

**Use when**: Norwegian citizens authenticate with BankID/MinID.

```yaml
idporten:
  enabled: true
  sidecar:
    enabled: true
    level: Level4 # or Level3
```

ID-porten sidecar handles authentication. Application receives validated JWT with fødselsnummer in claims.

### 4. Maskinporten (External Organizations)

**Use when**: External organizations need machine-to-machine access.

```yaml
maskinporten:
  enabled: true
  scopes:
    consumes:
      - name: "nav:example/scope"
```

## JWT Validation Pattern

### OpenID configuration

```kotlin
private val azureAdConfiguration: OpenIdConfiguration by lazy {
    runBlocking {
        httpClient.get(System.getenv("AZURE_APP_WELL_KNOWN_URL")).body()
    }
}

data class OpenIdConfiguration(
    val issuer: String,
    val jwks_uri: String,
    val token_endpoint: String
)
```

### Full validation: issuer, audience, expiration

```kotlin
install(Authentication) {
    jwt("azureAd") {
        verifier(JwkProvider(azureAdConfiguration.jwks_uri))

        validate { credential ->
            // Validate issuer
            if (credential.payload.issuer != azureAdConfiguration.issuer) {
                return@validate null
            }

            // Validate audience
            val audience = credential.payload.audience
            if (!audience.contains(expectedAudience)) {
                return@validate null
            }

            // Validate expiration
            if (credential.payload.expiresAt?.before(Date()) == true) {
                return@validate null
            }

            JWTPrincipal(credential.payload)
        }
    }
}
```

**TypeScript/Next.js with `@navikt/oasis`**:

```typescript
import { validateToken, parseAzureUserToken } from "@navikt/oasis";

// Simple validation (any issuer configured in Nais)
const validation = await validateToken(token);
if (!validation.ok) {
  return new Response("Invalid token", { status: 401 });
}

// Azure-specific validation with user info parsing
const azure = await parseAzureUserToken(token);
if (!azure.ok) {
  return new Response("Invalid Azure token", { status: 401 });
}

const { name, NAVident, preferred_username } = azure;
console.log(`User: ${name} (${NAVident})`);
```

## Authorization Patterns

### Role-based access control

```kotlin
fun Route.requireRole(role: String, build: Route.() -> Unit): Route {
    val route = createChild(object : RouteSelector() {
        override fun evaluate(context: RoutingResolveContext, segmentIndex: Int) = RouteSelectorEvaluation.Constant
    })

    route.intercept(ApplicationCallPipeline.Features) {
        val principal = call.principal<JWTPrincipal>()
        val roles = principal?.payload?.getClaim("roles")?.asList(String::class.java) ?: emptyList()

        if (!roles.contains(role)) {
            call.respond(HttpStatusCode.Forbidden, "Missing required role: $role")
            finish()
        }
    }

    route.build()
    return route
}

// Usage
authenticate("azureAd") {
    requireRole("admin") {
        post("/api/admin/users") {
            // Only accessible with admin role
        }
    }
}
```

## Machine-to-Machine (M2M) Validation

When accepting Azure AD M2M tokens (`sub == oid`), always validate `azp` against `AZURE_APP_PRE_AUTHORIZED_APPS`:

```kotlin
// ✅ Correct — validate azp against pre-authorized apps
validate { credentials ->
    if (!erMaskinTilMaskin(credentials)) return@validate null

    val azpClaim = credentials.payload.getClaim("azp").asString()
    val preAuthorizedApp = preAuthorizedApps
        .firstOrNull { it.clientId == azpClaim }
        ?: return@validate null  // reject unknown callers

    JWTPrincipal(credentials.payload)
}

// ❌ Wrong — accepts ANY app in the Azure AD tenant
validate { credentials ->
    if (!erMaskinTilMaskin(credentials)) return@validate null
    JWTPrincipal(credentials.payload)  // no azp check!
}
```

Cross-check auth code against `.nais/nais.yaml` `accessPolicy.inbound.rules` — every app allowed at network level should also be validated at token level.

## Testing

### Kotlin with MockOAuth2Server

```kotlin
class AuthenticationTest {
    private val mockOAuth2Server = MockOAuth2Server()

    @BeforeEach fun setup() { mockOAuth2Server.start() }
    @AfterEach fun tearDown() { mockOAuth2Server.shutdown() }

    @Test
    fun `should authenticate with valid token`() {
        val token = mockOAuth2Server.issueToken(
            issuerId = "azuread",
            subject = "test-user",
            claims = mapOf("preferred_username" to "test@nav.no", "roles" to listOf("user"))
        )

        val response = client.get("/api/protected") { bearerAuth(token.serialize()) }
        response.status shouldBe HttpStatusCode.OK
    }

    @Test
    fun `should reject invalid token`() {
        val response = client.get("/api/protected") { bearerAuth("invalid-token") }
        response.status shouldBe HttpStatusCode.Unauthorized
    }
}
```

### TypeScript with Vitest

```typescript
import { vi, describe, it, expect } from "vitest";
import { validateAzureToken } from "@navikt/oasis";

vi.mock("@navikt/oasis", () => ({
  validateAzureToken: vi.fn(),
  requestOboToken: vi.fn(),
  getToken: vi.fn(),
}));

describe("auth middleware", () => {
  it("should accept valid Azure token", async () => {
    vi.mocked(validateAzureToken).mockResolvedValue({
      ok: true,
      payload: { sub: "user-123", aud: "client-id" },
    });

    const response = await GET(mockRequest("valid-token"));
    expect(response.status).toBe(200);
  });

  it("should reject invalid token", async () => {
    vi.mocked(validateAzureToken).mockResolvedValue({
      ok: false,
      error: new Error("Invalid signature"),
      errorType: "token validation failed",
    });

    const response = await GET(mockRequest("invalid-token"));
    expect(response.status).toBe(403);
  });
});
```

## Common Issues

| Problem | Solution |
|---------|----------|
| "Invalid audience" | Verify `AZURE_APP_CLIENT_ID` matches expected audience |
| "Token expired" | Implement token refresh; check system time sync |
| TokenX exchange fails | Check access policies, that target has TokenX enabled, and that the client assertion is correctly formed |
| JWKS retrieval fails | Cache JWKS with TTL; handle refresh on validation failure |

## Gotchas

- `accessPolicy` and auth validation must match — drift means dead code or missing rules
- `@navikt/oasis` auto-caches OBO tokens — don't add your own cache layer
- Azure AD M2M tokens have `sub == oid` — detect this to apply `azp` validation
- TokenX audience format differs from Azure AD OBO (`cluster:ns:app` vs `api://.../.default`)
- Never log full JWT tokens — only log claims you need for debugging

## Boundaries

### ✅ Always

- Validate JWT issuer, audience, expiration, and signature
- Validate `azp` against pre-authorized apps for M2M tokens
- Cross-check auth code against `.nais/` accessPolicy inbound rules
- Use HTTPS only for token transmission
- Define explicit `accessPolicy` for authenticated services
- Keep token lifetimes short and refresh rather than extend
- Apply least privilege: minimal access policies and scopes
- Support key rotation (refresh JWKS, never pin a single key)
- Log authentication attempts for monitoring, failures with context
- Use environment variables from Nais (never hardcode)

### ⚠️ Ask First

- Changing access policies in production
- Modifying token validation rules
- Adding new OAuth scopes or permissions
- Changing audience claims
- Implementing custom token refresh logic

### 🚫 Never

- Hardcode client secrets or tokens
- Log full JWT tokens or credentials
- Bypass authentication requirements
- Store tokens in localStorage (use httpOnly cookies)
- Skip token validation "for testing"

## Reference

- [sikkerhet.nav.no Golden Path](https://sikkerhet.nav.no/docs/goldenpath/)
