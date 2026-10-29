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

Texas then exposes these environment variables:

- `NAIS_TOKEN_EXCHANGE_ENDPOINT`
- `NAIS_TOKEN_INTROSPECTION_ENDPOINT`

## 2. Token Exchange

POST to `NAIS_TOKEN_EXCHANGE_ENDPOINT` ([docs](https://doc.nais.io/auth/tokenx/how-to/consume/)). Texas signs the client assertion, caches tokens and never returns an expired one, so you need no cache or key handling of your own.

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

The `HttpClient` needs `ContentNegotiation` with a JSON serializer.

## 3. Calling Another Service

```kotlin
class UserServiceClient(
    private val texas: TexasClient,
    private val httpClient: HttpClient,
    private val userServiceUrl: String
) {
    suspend fun getUser(userId: String, userToken: String): User {
        val exchangedToken = texas.exchange(userToken, target = "prod-gcp:team-user:user-service")

        return httpClient.get("$userServiceUrl/api/users/$userId") {
            bearerAuth(exchangedToken)
        }.body()
    }
}
```

## 4. Validating Inbound Tokens

POST the token to `NAIS_TOKEN_INTROSPECTION_ENDPOINT` ([docs](https://doc.nais.io/auth/tokenx/how-to/secure/)). The response is always HTTP 200: `active: true` with the token's claims, or `active: false` with an `error`. Texas checks the signature and the standard claims. Check any other claim you rely on (for example `acr`) yourself.

If you would rather use a library, use `token-validation-ktor` from [navikt/token-support](https://github.com/navikt/token-support). Never verify JWTs by hand.

## 5. Ktor Integration

```kotlin
import io.ktor.server.application.*
import io.ktor.server.auth.*
import io.ktor.server.response.*
import io.ktor.server.routing.*

data class TokenXPrincipal(val token: String, val claims: Map<String, Any?>)

fun Application.configureTokenX(texas: TexasClient) {
    install(Authentication) {
        bearer("tokenx") {
            authenticate { credential ->
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
    val httpClient = HttpClient { install(ContentNegotiation) { jackson() } }
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

Texas does not run locally. Stub its endpoints with Ktor's `MockEngine`:

```kotlin
import io.ktor.client.engine.mock.*

@Test
fun `exchanges token through Texas`() = runTest {
    val engine = MockEngine { request ->
        assertEquals("http://texas/exchange", request.url.toString())
        respond(
            """{"access_token":"exchanged","expires_in":3599,"token_type":"Bearer"}""",
            headers = headersOf(HttpHeaders.ContentType, "application/json")
        )
    }
    val client = HttpClient(engine) { install(ContentNegotiation) { jackson() } }
    val texas = TexasClient(client, "http://texas/exchange", "http://texas/introspect")

    assertEquals("exchanged", texas.exchange("user-token", "dev-gcp:team-user:user-service"))
}
```

For tests that need real signed JWTs, [navikt/mock-oauth2-server](https://github.com/navikt/mock-oauth2-server) issues them.

## Security Checklist

- [ ] TokenX enabled in Nais manifest
- [ ] Access policy defined for outbound calls
- [ ] Inbound tokens validated through Texas (or token-support) on all protected endpoints
- [ ] No hand-written JWT validation or client assertions
- [ ] Tokens not logged or exposed
- [ ] HTTPS enforced for all calls
