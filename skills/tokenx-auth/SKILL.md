---
name: tokenx-auth
description: Tjeneste-til-tjeneste-autentisering med TokenX token exchange i Nais
license: MIT
compatibility: Application on Nais with TokenX
metadata:
  domain: auth
  tags: tokenx auth service-to-service nais
---

# TokenX Authentication Skill

This skill provides patterns for secure service-to-service authentication using TokenX. Token exchange and validation go through [Texas](https://doc.nais.io/auth/reference/#texas), the Nais token sidecar. Do not sign client assertions or verify JWTs yourself.

The examples use Ktor 3 and need these artifacts besides Ktor server and client: `ktor-client-content-negotiation`, `ktor-serialization-jackson`, and for tests `ktor-client-mock` and `kotlinx-coroutines-test`.

## Workflow

1. Enable TokenX in the Nais manifest and define access policies
2. Exchange tokens through Texas
3. Call downstream services with the exchanged token
4. Validate inbound TokenX tokens through Texas
5. Integrate with Ktor authentication
6. Test with a stubbed Texas

## 1. Nais Manifest Setup

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  tokenx:
    enabled: true

  accessPolicy:
    outbound:
      rules:
        - application: user-service
          namespace: team-user
```

Nais injects these environment variables when `tokenx.enabled` is true:

- `NAIS_TOKEN_EXCHANGE_ENDPOINT`
- `NAIS_TOKEN_INTROSPECTION_ENDPOINT`

## 2. Token Exchange

POST to `NAIS_TOKEN_EXCHANGE_ENDPOINT` ([docs](https://doc.nais.io/auth/tokenx/how-to/consume/)). Texas signs the client assertion, caches tokens and never returns an expired one, so you need no cache or key handling of your own.

If the target rejects a token after its access policy changed, retry once with `"skip_cache": true` in the request body to get a fresh token.

```kotlin
import io.ktor.client.*
import io.ktor.client.call.*
import io.ktor.client.request.*
import io.ktor.http.*

class TexasClient(
    private val httpClient: HttpClient,
    private val exchangeEndpoint: String = System.getenv("NAIS_TOKEN_EXCHANGE_ENDPOINT"),
    private val introspectionEndpoint: String = System.getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"),
) {
    /** target: "<cluster>:<namespace>:<app>" */
    suspend fun exchange(userToken: String, target: String): String =
        httpClient.post(exchangeEndpoint) {
            contentType(ContentType.Application.Json)
            setBody(mapOf("identity_provider" to "tokenx", "target" to target, "user_token" to userToken))
        }.body<TokenResponse>().access_token

    suspend fun introspect(token: String): Map<String, Any?> =
        httpClient.post(introspectionEndpoint) {
            contentType(ContentType.Application.Json)
            setBody(mapOf("identity_provider" to "tokenx", "token" to token))
        }.body()
}

data class TokenResponse(val access_token: String, val expires_in: Int, val token_type: String)
```

Create the `HttpClient` with `expectSuccess = true` and `ContentNegotiation` with Jackson. Texas answers errors with 400 or 500 and a body with `error` and `error_description`, and `expectSuccess` turns those into exceptions instead of a half-parsed response.

## 3. Calling Another Service

```kotlin
class UserServiceClient(
    private val texas: TexasClient,
    private val httpClient: HttpClient,
    private val userServiceUrl: String
) {
    suspend fun getUser(userId: String, userToken: String): User {
        // NAIS_CLUSTER_NAME is injected by Nais, e.g. "dev-gcp" or "prod-gcp"
        val target = "${System.getenv("NAIS_CLUSTER_NAME")}:team-user:user-service"
        val exchangedToken = texas.exchange(userToken, target)

        return httpClient.get("$userServiceUrl/api/users/$userId") {
            bearerAuth(exchangedToken)
        }.body()
    }
}
```

## 4. Validating Inbound Tokens

POST the token to `NAIS_TOKEN_INTROSPECTION_ENDPOINT` ([docs](https://doc.nais.io/auth/tokenx/how-to/secure/)). The response is always HTTP 200: `active: true` with the token's claims, or `active: false` with an `error`.

Texas checks the signature, `iss`, `aud` (your TokenX client id), `exp`, `iat` and `nbf`. Your app must still check:

- `acr`, if you require a security level. Both `Level4` and `idporten-loa-high` occur.
- `client_id`, if you authorize per calling app.
- `pid`, against the resource the user asks for.

If Texas itself fails (network error, 5xx), let the request fail with 5xx. Do not treat it as 401: that hides an outage.

If you would rather use a library, use `token-validation-ktor` from [navikt/token-support](https://github.com/navikt/token-support). Never verify JWTs by hand.

## 5. Ktor Integration

```kotlin
import io.ktor.server.application.*
import io.ktor.server.auth.*
import io.ktor.server.response.*
import io.ktor.server.routing.*

// Ktor 3: a principal no longer needs to implement Principal
data class TokenXPrincipal(val token: String, val claims: Map<String, Any?>)

fun Application.configureTokenX(texas: TexasClient) {
    install(Authentication) {
        bearer("tokenx") {
            authenticate { credential ->
                // Exceptions from Texas propagate and give 5xx, not 401
                val result = texas.introspect(credential.token)
                if (result["active"] == true) TokenXPrincipal(credential.token, result) else null
            }
        }
    }

    routing {
        authenticate("tokenx") {
            get("/api/protected") {
                val principal = call.principal<TokenXPrincipal>()!!
                call.respond("Authenticated user: ${principal.claims["pid"]}")
            }
        }
    }
}
```

## Complete Example

```kotlin
fun main() {
    val httpClient = HttpClient {
        expectSuccess = true
        install(ContentNegotiation) { jackson() }
    }
    val texas = TexasClient(httpClient)
    val userServiceClient = UserServiceClient(texas, httpClient, System.getenv("USER_SERVICE_URL"))

    embeddedServer(Netty, port = 8080) {
        configureTokenX(texas)

        routing {
            authenticate("tokenx") {
                get("/api/users/{id}") {
                    val principal = call.principal<TokenXPrincipal>()!!
                    call.respond(userServiceClient.getUser(call.parameters["id"]!!, principal.token))
                }
            }
        }
    }.start(wait = true)
}
```

## Testing

Texas is not part of your local setup, so stub it with Ktor's `MockEngine`:

```kotlin
import io.ktor.client.engine.mock.*
import io.ktor.server.testing.*

private fun texasStub(body: String): TexasClient {
    val engine = MockEngine {
        respond(body, headers = headersOf(HttpHeaders.ContentType, "application/json"))
    }
    val client = HttpClient(engine) {
        expectSuccess = true
        install(ContentNegotiation) { jackson() }
    }
    return TexasClient(client, "http://texas/exchange", "http://texas/introspect")
}

@Test
fun `exchanges token through Texas`() = runTest {
    val texas = texasStub("""{"access_token":"exchanged","expires_in":3599,"token_type":"Bearer"}""")

    assertEquals("exchanged", texas.exchange("user-token", "dev-gcp:team-user:user-service"))
}

@Test
fun `rejects inactive token with 401`() = testApplication {
    val texas = texasStub("""{"active":false,"error":"token is expired"}""")
    application { configureTokenX(texas) }

    val response = client.get("/api/protected") { bearerAuth("expired-token") }

    assertEquals(HttpStatusCode.Unauthorized, response.status)
}
```

For tests that need real signed JWTs, [navikt/mock-oauth2-server](https://github.com/navikt/mock-oauth2-server) issues them.

## Security Checklist

- [ ] TokenX enabled in Nais manifest
- [ ] Access policy defined for outbound calls
- [ ] Inbound tokens validated through Texas (or token-support) on all protected endpoints
- [ ] App checks `acr`, `client_id` and `pid` where it relies on them
- [ ] Texas outages give 5xx, not 401
- [ ] No hand-written JWT validation or client assertions
- [ ] Tokens not logged or exposed
- [ ] HTTPS enforced for all calls
