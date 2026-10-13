# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `ffab91f2-3501-46f9-828a-6178d2e98169`  
> - **Started:** 10/13/2026, 1:17:51 AM  
> - **Duration:** 2m 27s  
> - **Exported:** 10/13/2026, 1:20:18 AM  

---

<sub>2s</sub>

### Warning

1 MCP server was blocked by policy: 'github-copilot-browser'

---

<sub>3s</sub>

### User

Brukere får av og til 401 fra `vedtak-api` etter omtrent en time, og noen ganger
ser vi i loggen at én bruker har fått data som er hentet med en annen brukers token.
Texas-teamet melder i tillegg om topper med mange samtidige vekslinger for samme
bruker. Finn årsaken og rett feilen. Bruk kun dette repoet.

---

<sub>9s</sub>

### `bash`

**Check repository worktree state**

$ git status --short --branch

```
fatal: unable to get current working directory: Operation not permitted
<shellId: 0 completed with exit code 128>
```


---

<sub>9s</sub>

### `glob`

****/*.{kt,kts,java,md,yml,yaml}**

"**/*.{kt,kts,java,md,yml,yaml}"

<details>
<summary>27 matches</summary>

```
./src/main/kotlin/no/nav/benchmark/TokenCache.kt
./src/main/kotlin/no/nav/benchmark/VedtakClient.kt
./src/main/kotlin/no/nav/benchmark/TokenConfig.kt
./src/main/kotlin/no/nav/benchmark/Token.kt
./src/main/kotlin/no/nav/benchmark/TexasClient.kt
./src/main/kotlin/no/nav/benchmark/ArkivClient.kt
./src/main/kotlin/no/nav/benchmark/Application.kt
./src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt
./settings.gradle.kts
./.github/instructions/norwegian-text.instructions.md
./.github/instructions/testing.instructions.md
./.github/instructions/testing-kotlin.instructions.md
./.github/instructions/security-owasp.instructions.md
./.github/instructions/kotlin.instructions.md
./.github/instructions/security-core.instructions.md
./.github/instructions/testing-typescript.instructions.md
./.github/instructions/golang.instructions.md
./.github/instructions/github-actions.instructions.md
./.github/instructions/nextjs-aksel.instructions.md
./.github/instructions/database.instructions.md
./.github/instructions/output-style.instructions.md
./.github/instructions/accessibility.instructions.md
./.github/instructions/docker.instructions.md
./.github/instructions/performance.instructions.md
./.github/instructions/deliberate-ai-use.instructions.md
./build.gradle.kts
./docs/adr/0002-tokenx-via-texas.md
```

</details>


---

<sub>9s</sub>

### `rg`

**token|Token|401|Texas|vedtak-api|exchange**

"token|Token|401|Texas|vedtak-api|exchange" in **/*.{kt,kts,java,yml,yaml,md}

<details>
<summary>106 matches</summary>

```
[nav-pilot] Some values in this output were masked ([REDACTED:…]). The file on disk is unchanged. Do not write the placeholders back; ask the user if you need a value.

[grep content: 90 matches across 18 file(s) under .]

src/main/kotlin/no/nav/benchmark/TokenCache.kt (10 match(es)):
   /** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */
   class TokenCache(
       private val texas: TexasClient,
       private val tokens = ConcurrentHashMap<String, OboToken>()
       suspend fun token(userToken: String, audience: String): String {
           val cached = tokens[audience]
               return cached.accessToken
           val fresh = texas.exchange(userToken, audience)
           tokens[audience] = fresh
           return fresh.accessToken

src/main/kotlin/no/nav/benchmark/VedtakClient.kt (6 match(es)):
   const val VEDTAK_AUDIENCE = "dev-gcp:vedtak:vedtak-api"
       private val tokens: TokenCache,
       private val baseUrl: String = "http://vedtak-api",
       suspend fun hent(userToken: String, id: String): HttpResponse {
           val token = tokens.token(userToken, VEDTAK_AUDIENCE)
           return http.get("$baseUrl/api/vedtak/$id") { bearerAuth(token) }

src/main/kotlin/no/nav/benchmark/TokenConfig.kt (4 match(es)):
   /** Innstillinger for TokenX-veksling. Se docs/adr/0002-tokenx-via-texas.md. */
   data class TokenConfig(
       val texasEndpoint: String = "http://texas/api/v1/token/exchange",
       /** Hvor lang tid før utløp et token regnes som brukt opp. */
src/main/kotlin/no/nav/benchmark/Token.kt: data class OboToken(val accessToken: String, val expiresAt: Instant) {

src/main/kotlin/no/nav/benchmark/TexasClient.kt (10 match(es)):
   class TexasException(message: String) : RuntimeException(message)
   /** Bytter innkommende brukertoken mot et TokenX-token via Texas-sidecaren. */
   class TexasClient(
       private val endpoint: String = "http://texas/api/v1/token/exchange",
       suspend fun exchange(userToken: String, audience: String): OboToken {
                   append("identity_provider", "tokenx")
                   append("user_token", userToken)
               throw TexasException("Texas svarte ${response.status.value}")
           val accessToken = body.getValue("access_token").jsonPrimitive.content
           return OboToken(accessToken, clock.instant().plusSeconds(expiresIn))

src/main/kotlin/no/nav/benchmark/ArkivClient.kt (4 match(es)):
       private val tokens: TokenCache,
       suspend fun dokumenter(userToken: String, vedtakId: String): HttpResponse {
           val token = tokens.token(userToken, ARKIV_AUDIENCE)
           return http.get("$baseUrl/api/dokumenter?vedtak=$vedtakId") { bearerAuth(token) }

src/main/kotlin/no/nav/benchmark/Application.kt (10 match(es)):
       config: TokenConfig = TokenConfig(),
       val texas = TexasClient(HttpClient(texasEngine), clock, config.texasEndpoint)
       val tokens = TokenCache(texas, clock)
       val vedtak = VedtakClient(HttpClient(vedtakEngine), tokens)
       val arkiv = ArkivClient(HttpClient(arkivEngine), tokens)
               val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
               call.forward(vedtak.hent(userToken, id), "vedtak-api")
               val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
               call.forward(arkiv.dokumenter(userToken, call.parameters["id"]!!), "arkiv-api")
   private fun RoutingCall.userToken(): String? =

src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt (6 match(es)):
       fun henterVedtakMedVeksletToken() = testApplication {
               respond("""{"access_token":"[REDACTED:secret]","expires_in":3600,"token_type":"Bearer"}""", headers = json)
               respond("""{"id":"42","token":"${request.headers[HttpHeaders.Authorization]}"}""", headers = json)
           val response = client.get("/api/vedtak/42") { bearerAuth("bruker-token") }
           assertEquals("""{"id":"42","token":"******"}""", response.bodyAsText())
       fun avviserForespørselUtenToken() = testApplication {
.github/instructions/norwegian-text.instructions.md: Ikke oversett: image, cluster, node, container, release, pod, namespace, secret, bug, bugfix, hotfix, patch, edge case, rollback, failover, backup, pipeline, workflow, runtime, framework, middleware, pull request, merge, commit, branch, endpoint, payload, token, scope, host, skill, hook, prompt.

.github/instructions/testing-kotlin.instructions.md (3 match(es)):
       fun `should authenticate with valid token`() {
           val token = mockOAuth2Server.issueToken(
               bearerAuth(token.serialize())

.github/instructions/kotlin.instructions.md (5 match(es)):
       fun `should return 401 without token`() = testApplication {
   ### Auth (token-validation-spring)
           // Token validation is handled automatically by the filter
               header("Authorization", "******")
   | Skill | [$nav-auth](../skills/nav-auth/) | JWT validation, TokenX, ID-porten, Azure AD |

.github/instructions/security-core.instructions.md (2 match(es)):
   - **No secrets or personal data in logs.** Never log tokens, passwords, keys, fødselsnummer (FNR), name or address.
   - **Secrets come from the environment.** Read tokens, passwords and keys from environment variables or the secret store. Never hardcode them.

.github/instructions/github-actions.instructions.md (3 match(es)):
     id-token: write      # For OIDC/Nais deploy
     id-token: write
         id-token: write

.github/instructions/nextjs-aksel.instructions.md (12 match(es)):
   description: "Next.js med Aksel: spacing-tokens i stedet for Tailwind p-/m-, riktige token-navn, responsiv layout, App Router, Server Components og auth."
   Standarder for Next.js-apper med Aksel: spacing-tokens, responsive props og komponentmønstre.
   For komponent-APIer, tokens, theming, layout og mønstre — hent dokumentasjon fra:
   **VIKTIG**: Bruk alltid Nav DS spacing-tokens, aldri Tailwind padding/margin.
   ## Spacing-tokens
   Token name = eksakt pikselverdi (`space-16` = 16px, `space-4` = 4px). Alltid `space-`-prefiks.
   Fullstendig liste: [design-tokens](https://aksel.nav.no/grunnleggende/styling/design-tokens.md)
     const token = cookieStore.get("auth-token")?.value;
     return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
   - Use spacing tokens with `space-` prefix
   | `@aksel-agent`             | Aksel Design System component patterns and spacing tokens        |
   | aksel.nav.no/llm.md        | Full Aksel documentation index (components, tokens, foundations) |
.github/instructions/docker.instructions.md:       id-token: write
.github/instructions/output-style.instructions.md: 3. Mark red-zone code with one line, for example `🔴 Rød sone: token-validering, gå gjennom denne nøye.`

.github/instructions/performance.instructions.md (2 match(es)):
   CSS tokens from `@navikt/ds-css` are loaded once globally — no additional performance concern.
   | `@aksel-agent` | Aksel Design System component patterns and spacing tokens |

docs/adr/0002-tokenx-via-texas.md (9 match(es)):
   # 2. TokenX-veksling via Texas med cache
   `vedtak-proxy` kaller `vedtak-api` og `arkiv-api` på vegne av innlogget bruker.
   Hvert kall trenger et TokenX-token for API-ets audience. Texas-sidecaren veksler
   brukerens token mot et slikt token. Uten cache går det ett kall til Texas per
   - Vekslede tokener caches i minnet per pod, felles for alle klientene.
   - Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en
   - Tokener fornyes når tiden som er igjen før de utløper, er kortere enn
     `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.
   - Texas har en grense for antall vekslinger per sekund. Samtidige forespørsler fra
```

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md

```
File too large to read at once (22.1 KB). Consider using the grep tool to search within the file, or view with view_range to read specific sections.
```


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/.github/instructions/security-owasp.instructions.md.**

<workspace>/.github/instructions/security-owasp.instructions.md

<details>
<summary>9 lines</summary>

```
---
description: "Hvor OWASP Top 10:2025-mønstrene for Kotlin, Go, Java og TypeScript ligger: tilgangskontroll, feilkonfigurasjon, forsyningskjede, kryptografi, injeksjon, logging og feilhåndtering."
applyTo: "**/*.{kt,go,java,ts,tsx}"
---

# Security Essentials

The rules that always apply (logging, secrets, queries, ownership, `azp`, TLS) are in `security-core.instructions.md`, which is loaded in every session. If it is not installed next to this file, tell the developer to install it: https://github.com/navikt/copilot/blob/main/instructions/security-core.instructions.md

For detailed OWASP Top 10:2025 code-level patterns (Kotlin, Go, Java, Node.js), invoke the `$security-owasp` skill.
For scanning workflows (trivy, zizmor, govulncheck), use the `$security-review` skill.
For architecture-level threat modeling, use `@security-champion`.
```

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/.github/instructions/security-core.instructions.md.**

<workspace>/.github/instructions/security-core.instructions.md

<details>
<summary>12 lines</summary>

```
---
applyTo: "**"
---

# Security Core

These rules apply to every change, in every language. They are never optional.

- **No secrets or personal data in logs.** Never log tokens, passwords, keys, fødselsnummer (FNR), name or address.
- **Secrets come from the environment.** Read tokens, passwords and keys from environment variables or the secret store. Never hardcode them.
- **Parameterized queries only.** Never concatenate user input into SQL, shell or any other command. Validate input at trust boundaries.
- **Verify resource ownership.** Check that the caller owns the resource, not only that the caller is authenticated.
- **Validate `azp` for machine-to-machine calls** against `AZURE_APP_PRE_AUTHORIZED_APPS`.
- **TLS 1.2 or newer.** Never set `InsecureSkipVerify: true` or disable certificate validation.

Code-level OWASP Top 10:2025 patterns: the `$security-owasp` skill.
```

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/.github/instructions/testing.instructions.md.**

<workspace>/.github/instructions/testing.instructions.md

<details>
<summary>85 lines</summary>

````
---
name: Testing Standards
description: "Felles testprinsipper for Nav. Språkspesifikke eksempler finnes i egne instruksjoner for Kotlin og TypeScript."
applyTo: "**/*.test.{ts,tsx,kt,kts}"
---

# Testing Standards

Common testing principles for Nav. Language-specific examples can be found in separate instructions for [Kotlin](testing-kotlin.instructions.md) and [TypeScript](testing-typescript.instructions.md).

## Test Coverage

### Coverage Requirements

- **Utilities in `lib/`**: 80%+ coverage required
- **Business logic**: 70%+ coverage required
- **API routes**: Test happy path + error cases
- **Repositories**: Test CRUD operations
- **Event handlers**: Test event processing + publishing

## Test Naming

```kotlin
// ✅ Good - describes behavior
`should create user when valid data provided`
`should throw exception when email is invalid`
`should publish event after successful processing`

// ❌ Bad - not descriptive
`test1`
`createUserTest`
`testValidation`
```

## Test Strategy

Choose test type based on what you're verifying:

| What to test | Test type | Tools |
|---|---|---|
| Pure functions, utils | Unit test | Kotest / Vitest |
| Controller + validation | Slice test | `@WebMvcTest` + MockkBean |
| Repository + SQL | Slice test | `@DataJpaTest` + Testcontainers |
| Full API flow | Integration test | `@SpringBootTest` + Testcontainers |
| User workflows | E2E test | Playwright |
| Accessibility | E2E test | Playwright + axe-core |

### When to use what

- **Unit**: Business logic, data transformations, formatting
- **Slice** (`@WebMvcTest`, `@DataJpaTest`): Faster than full integration, tests one layer
- **Integration** (`@SpringBootTest`): Auth flow, multi-layer, real DB
- **E2E** (Playwright): Critical user journeys, form submission, navigation

## Playwright E2E Tests

```typescript
import { test, expect } from "@playwright/test";

test.describe("Oversikt", () => {
  test("should display vedtak list", async ({ page }) => {
    await page.goto("/oversikt");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByRole("table")).toBeVisible();
  });

  test("should filter by status", async ({ page }) => {
    await page.goto("/oversikt");
    await page.getByRole("combobox", { name: /status/i }).selectOption("aktiv");
    await expect(page.getByRole("row")).toHaveCount(await page.getByRole("row").count());
  });
});
```

### Accessibility in E2E

```typescript
import AxeBuilder from "@axe-core/playwright";

test("should have no a11y violations", async ({ page }) => {
  await page.goto("/oversikt");
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});
```

## Boundaries

### ✅ Always

- Write tests for new code before committing
- Test both success and error cases
- Use descriptive test names
- Clean up test data after each test
- Run full test suite before pushing

### ⚠️ Ask First

- Changing test framework or structure
- Adding complex test fixtures
- Modifying shared test utilities
- Disabling or skipping tests

### 🚫 Never

- Commit failing tests
- Skip tests without good reason
- Test implementation details
- Share mutable state between tests
- Commit without running tests
````

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/.github/instructions/testing-kotlin.instructions.md.**

<workspace>/.github/instructions/testing-kotlin.instructions.md

<details>
<summary>145 lines</summary>

````
---
name: Kotlin Testing
description: "Kotlin-spesifikke testmønstre for Nav: Kotest-matchers, TestRapid, Testcontainers og MockOAuth2Server."
applyTo: "**/*.test.{kt,kts}"
---

# Kotlin Testing (Kotest & JUnit 5)

Kotlin-specific test patterns for Nav: Kotest-matchers, TestRapid, Testcontainers, and MockOAuth2Server.

## Test Structure

```kotlin
import io.kotest.matchers.shouldBe
import io.kotest.matchers.shouldNotBe
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.BeforeAll

class ServiceTest {
    companion object {
        @BeforeAll
        @JvmStatic
        fun setup() {
            // Setup code
        }
    }

    @Test
    fun `should process event correctly`() {
        // Arrange
        val input = createTestInput()

        // Act
        val result = service.process(input)

        // Assert
        result shouldBe expectedResult
        result.status shouldBe "completed"
    }
}
```

## Kotest Matchers

```kotlin
// Equality
result shouldBe expected
result shouldNotBe unexpected

// Null checks
result shouldNotBe null
nullableValue shouldBe null

// Collections
list.size shouldBe 3
list shouldContain item
list shouldContainAll listOf(item1, item2)

// Exceptions
shouldThrow<IllegalArgumentException> {
    service.processInvalid()
}

// Numeric comparisons
value shouldBeGreaterThan 0
value shouldBeLessThanOrEqual 100
```

## Testing Kafka Events (TestRapid)

```kotlin
import no.nav.helse.rapids_rivers.testsupport.TestRapid

class EventHandlerTest {
    private val testRapid = TestRapid()
    private val service = Service(testRapid)

    @Test
    fun `should publish event after processing`() {
        val testMessage = """
            {
                "@event_name": "test_event",
                "required_field": "value"
            }
        """.trimIndent()

        testRapid.sendTestMessage(testMessage)

        testRapid.inspektør.size shouldBe 1
        val published = testRapid.inspektør.message(0)
        published["@event_name"].asText() shouldBe "response_event"
        published["processed"].asBoolean() shouldBe true
    }
}
```

## Testing with Testcontainers

```kotlin
import org.testcontainers.containers.PostgreSQLContainer
import org.testcontainers.junit.jupiter.Container
import org.testcontainers.junit.jupiter.Testcontainers

@Testcontainers
class RepositoryTest {
    companion object {
        @Container
        val postgres = PostgreSQLContainer<Nothing>("postgres:15").apply {
            withDatabaseName("testdb")
        }
    }

    private lateinit var dataSource: HikariDataSource
    private lateinit var repository: Repository

    @BeforeEach
    fun setup() {
        dataSource = HikariDataSource().apply {
            jdbcUrl = postgres.jdbcUrl
            username = postgres.username
            password = postgres.password
        }

        // Run migrations
        Flyway.configure()
            .dataSource(dataSource)
            .load()
            .migrate()

        repository = RepositoryPostgres(dataSource)
    }

    @Test
    fun `should save and retrieve entity`() {
        val entity = Entity(name = "test")
        val id = repository.save(entity)

        val retrieved = repository.findById(id)

        retrieved shouldNotBe null
        retrieved?.name shouldBe "test"
    }
}
```

## Testing Authentication (MockOAuth2Server)

```kotlin
import no.nav.security.mock.oauth2.MockOAuth2Server

class AuthenticationTest {
    private val mockOAuth2Server = MockOAuth2Server()

    @BeforeEach
    fun setup() {
        mockOAuth2Server.start()
    }

    @AfterEach
    fun tearDown() {
        mockOAuth2Server.shutdown()
    }

    @Test
    fun `should authenticate with valid token`() {
        val token = mockOAuth2Server.issueToken(
            issuerId = "azuread",
            subject = "test-user",
            claims = mapOf("preferred_username" to "test@nav.no")
        )

        val response = client.get("/api/protected") {
            bearerAuth(token.serialize())
        }

        response.status shouldBe HttpStatusCode.OK
    }
}
```

## Run Tests

```bash
./gradlew test
```
````

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt

<details>
<summary>19 lines</summary>

```
package no.nav.benchmark

import java.time.Clock
import java.util.concurrent.ConcurrentHashMap

/** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */
class TokenCache(
    private val texas: TexasClient,
    private val clock: Clock,
) {
    private val tokens = ConcurrentHashMap<String, OboToken>()

    suspend fun token(userToken: String, audience: String): String {
        val cached = tokens[audience]
        if (cached != null && cached.isValid(clock.instant())) {
            return cached.accessToken
        }
        val fresh = texas.exchange(userToken, audience)
        tokens[audience] = fresh
        return fresh.accessToken
    }
}
```

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/Token.kt

<details>
<summary>5 lines</summary>

```
package no.nav.benchmark

import java.time.Instant

data class OboToken(val accessToken: String, val expiresAt: Instant) {
    fun isValid(now: Instant): Boolean = now.isBefore(expiresAt)
}
```

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TokenConfig.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TokenConfig.kt

<details>
<summary>8 lines</summary>

```
package no.nav.benchmark

import java.time.Duration

/** Innstillinger for TokenX-veksling. Se docs/adr/0002-tokenx-via-texas.md. */
data class TokenConfig(
    val texasEndpoint: String = "http://texas/api/v1/token/exchange",
    /** Hvor lang tid før utløp et token regnes som brukt opp. */
    val expiryLeeway: Duration = Duration.ofSeconds(30),
)
```

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt.**

<workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt

<details>
<summary>38 lines</summary>

```
[nav-pilot] Some values in this output were masked ([REDACTED:…]). The file on disk is unchanged. Do not write the placeholders back; ask the user if you need a value.

package no.nav.benchmark

import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.server.testing.testApplication
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import kotlin.test.Test
import kotlin.test.assertEquals

class VedtakProxyTest {
    private val json = headersOf(HttpHeaders.ContentType, "application/json")

    @Test
    fun henterVedtakMedVeksletToken() = testApplication {
        val texas = MockEngine {
            respond("""{"access_token":"[REDACTED:secret]","expires_in":3600,"token_type":"Bearer"}""", headers = json)
        }
        val vedtak = MockEngine { request ->
            respond("""{"id":"42","token":"${request.headers[HttpHeaders.Authorization]}"}""", headers = json)
        }
        application { vedtakModule(texas, vedtak, Clock.fixed(Instant.parse("2026-03-04T10:00:00Z"), ZoneOffset.UTC)) }

        val response = client.get("/api/vedtak/42") { bearerAuth("bruker-token") }

        assertEquals(HttpStatusCode.OK, response.status)
        assertEquals("""{"id":"42","token":"******"}""", response.bodyAsText())
    }

    @Test
    fun avviserForespørselUtenToken() = testApplication {
        val engine = MockEngine { error("skal ikke kalles") }
        application { vedtakModule(engine, engine) }

        assertEquals(HttpStatusCode.Unauthorized, client.get("/api/vedtak/42").status)
    }
}
```

</details>


---

<sub>14s</sub>

### `view`

**view the file at <workspace>/docs/adr/0002-tokenx-via-texas.md.**

<workspace>/docs/adr/0002-tokenx-via-texas.md

<details>
<summary>18 lines</summary>

```
# 2. TokenX-veksling via Texas med cache

Dato: 2026-03-04

## Kontekst

`vedtak-proxy` kaller `vedtak-api` og `arkiv-api` på vegne av innlogget bruker.
Hvert kall trenger et TokenX-token for API-ets audience. Texas-sidecaren veksler
brukerens token mot et slikt token. Uten cache går det ett kall til Texas per
forespørsel.

## Beslutning

- Vekslede tokener caches i minnet per pod, felles for alle klientene.
- Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en
  annen bruker eller mot et annet API.
- Tokener fornyes når tiden som er igjen før de utløper, er kortere enn
  `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.
- Texas har en grense for antall vekslinger per sekund. Samtidige forespørsler fra
  samme bruker mot samme API skal derfor dele én veksling.
- Tiden hentes fra en injisert `Clock`, så testene kan styre den.

## Konsekvenser

Cachen vokser med antall aktive brukere. Det er akseptabelt med dagens volum.
```

</details>


---

<sub>20s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md (lines 1-240)

<details>
<summary>193 lines</summary>

````
---
name: Kotlin Backend Patterns
description: "Kotlin-backendmønstre for Nav: Ktor og Rapids & Rivers, Spring Boot, database, auth, testing og feilhåndtering."
applyTo: "**/*.kt"
---

Kotlin backend patterns for Nav: Ktor and Rapids & Rivers, Spring Boot, database access, auth, testing, and error handling.

> **Pick one half.** A file path cannot tell Ktor from Spring Boot, so this file covers both. Decide from the file itself: `RapidApplication`, `routing`, `River`, `embeddedServer` means Ktor; `@RestController`, `@Service`, `@SpringBootApplication` means Spring Boot. Apply the matching half and ignore the other. Ktor is the most widely used framework for new Kotlin backends in Nav, but Spring Boot is actively used by many teams, and teams choose for themselves. For migrating between them, see [$java-to-kotlin](../skills/java-to-kotlin/).

# Kotlin Backend Standards

## Ktor and Rapids & Rivers

### Application Structure

Use the ApplicationBuilder pattern for bootstrapping applications:

```kotlin
class ApplicationBuilder(configuration: Map<String, String>) {
    private val meterRegistry = PrometheusMeterRegistry(PrometheusConfig.DEFAULT)
    private val dataSource = PostgresDataSourceBuilder.dataSource
    private val rapidsConnection: RapidsConnection

    init {
        rapidsConnection = RapidApplication.create(configuration)
        // Register rivers and event handlers
    }

    fun start() {
        rapidsConnection.start()
    }
}
```

### Configuration Pattern

Use sealed classes for environment-specific configuration with compile-time safety:

```kotlin
sealed class ApplicationConfig {
    abstract val database: DatabaseConfig
    abstract val kafka: KafkaConfig
    abstract val http: HttpConfig

    data class Dev(
        override val database: DatabaseConfig,
        override val kafka: KafkaConfig,
        override val http: HttpConfig
    ) : ApplicationConfig()

    data class Prod(...) : ApplicationConfig()
    data class Local(...) : ApplicationConfig()
}

// Usage
val config = when (environment) {
    "prod" -> ApplicationConfig.Prod(...)
    "dev" -> ApplicationConfig.Dev(...)
    else -> ApplicationConfig.Local(...)
}
```

### Database Access (Kotliquery)

Use Kotliquery with HikariCP connection pooling. This is Nav's standard — not JPA/Hibernate.

```kotlin
object PostgresDataSourceBuilder {
    val dataSource by lazy {
        HikariDataSource().apply {
            jdbcUrl = getOrThrow(DB_URL_KEY)
            maximumPoolSize = 5 // Start low in K8s; scale up if needed
            minimumIdle = 1
        }
    }
}

// Repository pattern with interface
class RepositoryPostgres(private val dataSource: DataSource) : Repository {
    override fun save(entity: Entity): Long {
        return using(sessionOf(dataSource)) { session ->
            session.run(
                queryOf(
                    "INSERT INTO table (col1, col2) VALUES (?, ?)",
                    entity.col1, entity.col2
                ).asUpdateAndReturnGeneratedKey
            ) ?: throw Exception("Failed to insert")
        }
    }

    override fun findById(id: Long): Entity? {
        return using(sessionOf(dataSource)) { session ->
            session.run(
                queryOf("SELECT * FROM table WHERE id = ?", id)
                    .map { row ->
                        Entity(
                            id = row.long("id"),
                            col1 = row.string("col1")
                        )
                    }.asSingle
            )
        }
    }
}
```

#### Kotliquery Patterns

```kotlin
// ✅ Batch insert
fun saveAll(entities: List<Entity>) = using(sessionOf(dataSource)) { session ->
    session.batchPreparedNamedStatement(
        "INSERT INTO table (col1, col2) VALUES (:col1, :col2)",
        entities.map { mapOf("col1" to it.col1, "col2" to it.col2) }
    )
}

// ✅ Row mapper as extension function
private fun Row.toEntity() = Entity(
    id = long("id"),
    name = string("name"),
    description = stringOrNull("description"),
    createdAt = localDateTime("created_at"),
)
```

### Transaction Patterns

JDBC connections are thread-bound. `ThreadLocal` values do not propagate to new coroutines.
Never use `launch`, `async`, or other coroutine builders inside a transaction block.

#### Simple single-block transaction

Works when all operations happen in the same place:

```kotlin
fun transferFunds(fromId: Long, toId: Long, amount: BigDecimal) =
    using(sessionOf(dataSource)) { session ->
        session.transaction { tx ->
            tx.run(queryOf("UPDATE accounts SET balance = balance - ? WHERE id = ?", amount, fromId).asUpdate)
            tx.run(queryOf("UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, toId).asUpdate)
        }
    }
```

#### Explicit transaction parameter

Recommended when the transaction spans multiple layers. Type-safe and easy to follow:

```kotlin
class DbTransaction(val session: TransactionalSession)

fun <T> transaction(dataSource: DataSource, block: DbTransaction.() -> T): T =
    sessionOf(dataSource).use { session ->
        session.transaction { tx -> DbTransaction(tx).block() }
    }

// Repository methods take DbTransaction as receiver
fun DbTransaction.saveOrder(order: Order): Long =
    session.run(
        queryOf("INSERT INTO orders (product, amount) VALUES (?, ?)", order.product, order.amount)
            .asUpdateAndReturnGeneratedKey
    ) ?: error("Failed to insert order")

// Usage — everything runs in the same transaction without ThreadLocal
transaction(dataSource) {
    val orderId = saveOrder(order)
    updateInventory(order.productId, -order.quantity)
}
```

#### ThreadLocal-based transaction

Pragmatic for existing layered architectures where many service methods already call repositories.
Repositories automatically reuse the active transaction:

```kotlin
object Database {
    private lateinit var dataSource: DataSource
    private val transactionalSession = ThreadLocal<TransactionalSession?>()

    fun <T> query(block: (Session) -> T): T {
        val tx = transactionalSession.get()
        return if (tx != null) block(tx) else using(sessionOf(dataSource)) { block(it) }
    }

    fun <T> transaction(block: () -> T): T {
        check(transactionalSession.get() == null) { "Nested transactions are not supported" }
        return sessionOf(dataSource).use { session ->
            session.transaction { tx ->
                transactionalSession.set(tx)
                try { block() } finally { transactionalSession.remove() }
            }
        }
    }
}
```

> **⚠️** ThreadLocal does not propagate to new coroutines. This approach only works
> when all code in the transaction block runs on the same thread without suspend calls.

### Dependency Injection (Koin)

For small apps, constructor injection without a framework is simplest — pass dependencies directly.
For larger apps with many services and repositories, Koin keeps the wiring manageable:

```kotlin
// Module definition
fun appModule(config: ApplicationConfig) = module {
    single<DataSource> { Database.dataSource(config.database) }
    single<ResourceRepository> { ResourceRepositoryPostgres(get()) }
    single<ResourceService> { ResourceService(get()) }
    factory<SomeClient> { SomeClient(get<AppConfig>().clientUrl) }
}

// Install in Application
fun Application.main() {
    install(Koin) {
        slf4jLogger()
        modules(appModule(config))
    }
}

// Inject in routes
fun Route.resourceRoutes() {
    val service by inject<ResourceService>()
    get("/api/resources") { call.respond(service.findAll()) }
}
```

Swap the modules for fakes in tests, and call `stopKoin()` afterwards:

```kotlin
@Test
fun `should process user`() {
    startKoin {
        modules(module {
            single<UserRepository> { FakeUserRepository() }
            single<UserService> { UserService(get()) }
````

</details>


---

<sub>20s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt

<details>
<summary>36 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.forms.submitForm
import io.ktor.client.statement.bodyAsText
import io.ktor.http.isSuccess
import io.ktor.http.parameters
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import java.time.Clock

class TexasException(message: String) : RuntimeException(message)

/** Bytter innkommende brukertoken mot et TokenX-token via Texas-sidecaren. */
class TexasClient(
    private val http: HttpClient,
    private val clock: Clock,
    private val endpoint: String = "http://texas/api/v1/token/exchange",
) {
    suspend fun exchange(userToken: String, audience: String): OboToken {
        val response = http.submitForm(
            url = endpoint,
            formParameters = parameters {
                append("identity_provider", "tokenx")
                append("target", audience)
                append("user_token", userToken)
            },
        )
        if (!response.status.isSuccess()) {
            throw TexasException("Texas svarte ${response.status.value}")
        }
        val body = Json.parseToJsonElement(response.bodyAsText()).jsonObject
        val accessToken = body.getValue("access_token").jsonPrimitive.content
        val expiresIn = body.getValue("expires_in").jsonPrimitive.long
        return OboToken(accessToken, clock.instant().plusSeconds(expiresIn))
    }
}
```

</details>


---

<sub>20s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/Application.kt

<details>
<summary>53 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.cio.CIO
import io.ktor.client.statement.HttpResponse
import io.ktor.client.statement.bodyAsText
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.server.application.Application
import io.ktor.server.application.log
import io.ktor.server.engine.embeddedServer
import io.ktor.server.netty.Netty
import io.ktor.server.response.respond
import io.ktor.server.response.respondText
import io.ktor.server.routing.RoutingCall
import io.ktor.server.routing.get
import io.ktor.server.routing.routing
import java.time.Clock

fun Application.vedtakModule(
    texasEngine: HttpClientEngine,
    vedtakEngine: HttpClientEngine,
    clock: Clock = Clock.systemUTC(),
    arkivEngine: HttpClientEngine = vedtakEngine,
    config: TokenConfig = TokenConfig(),
) {
    val texas = TexasClient(HttpClient(texasEngine), clock, config.texasEndpoint)
    val tokens = TokenCache(texas, clock)
    val vedtak = VedtakClient(HttpClient(vedtakEngine), tokens)
    val arkiv = ArkivClient(HttpClient(arkivEngine), tokens)

    routing {
        get("/internal/isalive") { call.respondText("ALIVE") }
        get("/api/vedtak/{id}") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            val id = call.parameters["id"]!!
            call.forward(vedtak.hent(userToken, id), "vedtak-api")
        }
        get("/api/vedtak/{id}/dokumenter") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            call.forward(arkiv.dokumenter(userToken, call.parameters["id"]!!), "arkiv-api")
        }
    }
}

private fun RoutingCall.userToken(): String? =
    request.headers[HttpHeaders.Authorization]?.removePrefix("Bearer ")?.takeIf { it.isNotBlank() }

private suspend fun RoutingCall.forward(response: HttpResponse, upstream: String) {
    application.log.info("{} svarte {} for {}", upstream, response.status.value, request.local.uri)
    respondText(response.bodyAsText(), ContentType.Application.Json, response.status)
}

fun main() {
    embeddedServer(Netty, port = 8080) { vedtakModule(CIO.create(), CIO.create(), arkivEngine = CIO.create()) }
        .start(wait = true)
}
```

</details>


---

<sub>20s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/VedtakClient.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/VedtakClient.kt

<details>
<summary>16 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.HttpResponse

const val VEDTAK_AUDIENCE = "dev-gcp:vedtak:vedtak-api"

class VedtakClient(
    private val http: HttpClient,
    private val tokens: TokenCache,
    private val baseUrl: String = "http://vedtak-api",
) {
    suspend fun hent(userToken: String, id: String): HttpResponse {
        val token = tokens.token(userToken, VEDTAK_AUDIENCE)
        return http.get("$baseUrl/api/vedtak/$id") { bearerAuth(token) }
    }
}
```

</details>


---

<sub>20s</sub>

### `view`

**view the file at <workspace>/build.gradle.kts.**

<workspace>/build.gradle.kts

<details>
<summary>22 lines</summary>

```
plugins {
    kotlin("jvm") version "2.1.20"
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("io.ktor:ktor-server-core-jvm:3.1.3")
    implementation("io.ktor:ktor-server-netty-jvm:3.1.3")
    implementation("io.ktor:ktor-client-core-jvm:3.1.3")
    implementation("io.ktor:ktor-client-cio-jvm:3.1.3")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.8.1")
    testImplementation("io.ktor:ktor-server-test-host-jvm:3.1.3")
    testImplementation("io.ktor:ktor-client-mock-jvm:3.1.3")
    testImplementation("org.jetbrains.kotlin:kotlin-test-junit5:2.1.20")
}

tasks.test {
    useJUnitPlatform()
}

kotlin {
    jvmToolchain(21)
}
```

</details>


---

<sub>20s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/ArkivClient.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/ArkivClient.kt

<details>
<summary>16 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.HttpResponse

const val ARKIV_AUDIENCE = "dev-gcp:teamdokumenthandtering:arkiv-api"

class ArkivClient(
    private val http: HttpClient,
    private val tokens: TokenCache,
    private val baseUrl: String = "http://arkiv-api",
) {
    suspend fun dokumenter(userToken: String, vedtakId: String): HttpResponse {
        val token = tokens.token(userToken, ARKIV_AUDIENCE)
        return http.get("$baseUrl/api/dokumenter?vedtak=$vedtakId") { bearerAuth(token) }
    }
}
```

</details>


---

<sub>1m 3s</sub>

### Copilot

Årsaken er i cache-nøkkelen: den bruker bare audience, slik at samme audience kan treffe en annen brukers token. Samtidig mangler cache-samordning for parallelle fornyelser, og `expiryLeeway` er konfigurert men ikke brukt. Jeg retter alle tre og legger til tester for bruker-isolasjon, fornying før utløp og sammenslåtte samtidige vekslinger.

---

<sub>1m 3s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: src/main/kotlin/no/nav/benchmark/TokenCache.kt\n@@\n import java.time.Clock\n+import java.time.Duration\n+import java.util.Base64\n import java.util.concurrent.ConcurrentHashMap\n+import java.security.MessageDigest\n+import kotlinx.coroutines.CompletableDeferred\n \n /** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */\n class TokenCache(\n     private val texas: TexasClient,\n     private val clock: Clock,\n+    private val expiryLeeway: Duration = Duration.ofSeconds(30),\n ) {\n-    private val tokens = ConcurrentHashMap<String, OboToken>()\n+    private data class CacheKey(val userTokenFingerprint: String, val audience: String)\n+\n+    private val tokens = ConcurrentHashMap<CacheKey, OboToken>()\n+    private val exchanges = ConcurrentHashMap<CacheKey, CompletableDeferred<OboToken>>()\n+\n+    init {\n+        require(!expiryLeeway.isNegative) { \"expiryLeeway cannot be negative\" }\n+    }\n \n     suspend fun token(userToken: String, audience: String): String {\n-        val cached = tokens[audience]\n-        if (cached != null && cached.isValid(clock.instant())) {\n+        val key = CacheKey(userToken.fingerprint(), audience)\n+        val cached = tokens[key]\n+        if (cached != null && cached.isValid(clock.instant(), expiryLeeway)) {\n             return cached.accessToken\n         }\n-        val fresh = texas.exchange(userToken, audience)\n-        tokens[audience] = fresh\n-        return fresh.accessToken\n+\n+        val pendingExchange = CompletableDeferred<OboToken>()\n+        val activeExchange = exchanges.putIfAbsent(key, pendingExchange)\n+        if (activeExchange != null) {\n+            return activeExchange.await().accessToken\n+        }\n+\n+        try {\n+            val fresh = texas.exchange(userToken, audience)\n+            tokens[key] = fresh\n+            pendingExchange.complete(fresh)\n+            return fresh.accessToken\n+        } catch (cause: Throwable) {\n+            pendingExchange.completeExceptionally(cause)\n+            throw cause\n+        } finally {\n+            exchanges.remove(key, pendingExchange)\n+        }\n     }\n+\n+    private fun String.fingerprint(): String =\n+        Base64.getUrlEncoder().withoutPadding().encodeToString(\n+            MessageDigest.getInstance(\"SHA-256\").digest(toByteArray(Charsets.UTF_8)),\n+        )\n }\n*** Update File: src/main/kotlin/no/nav/benchmark/Token.kt\n@@\n import java.time.Instant\n+import java.time.Duration\n \n data class OboToken(val accessToken: String, val expiresAt: Instant) {\n-    fun isValid(now: Instant): Boolean = now.isBefore(expiresAt)\n+    fun isValid(now: Instant, leeway: Duration = Duration.ZERO): Boolean =\n+        expiresAt.isAfter(now.plus(leeway))\n }\n*** Update File: src/main/kotlin/no/nav/benchmark/Application.kt\n@@\n-    val tokens = TokenCache(texas, clock)\n+    val tokens = TokenCache(texas, clock, config.expiryLeeway)\n*** Add File: src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n+package no.nav.benchmark\n+\n+import io.ktor.client.HttpClient\n+import io.ktor.client.engine.mock.MockEngine\n+import io.ktor.client.engine.mock.respond\n+import io.ktor.http.headersOf\n+import kotlinx.coroutines.async\n+import kotlinx.coroutines.awaitAll\n+import kotlinx.coroutines.coroutineScope\n+import kotlinx.coroutines.delay\n+import kotlinx.coroutines.runBlocking\n+import java.time.Clock\n+import java.time.Instant\n+import java.time.ZoneId\n+import java.util.concurrent.atomic.AtomicInteger\n+import kotlin.test.Test\n+import kotlin.test.assertEquals\n+\n+class TokenCacheTest {\n+    private val epoch = Instant.parse(\"2026-03-04T10:00:00Z\")\n+    private val json = headersOf(\"Content-Type\", \"application/json\")\n+\n+    @Test\n+    fun `caches tokens separately for each user and audience`() = runBlocking {\n+        val exchangeCount = AtomicInteger()\n+        val texas = texasClient {\n+            val number = exchangeCount.incrementAndGet()\n+            respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n+\n+        assertEquals(\"token-1\", cache.token(\"user-a\", \"api-a\"))\n+        assertEquals(\"token-2\", cache.token(\"user-b\", \"api-a\"))\n+        assertEquals(\"token-3\", cache.token(\"user-a\", \"api-b\"))\n+        assertEquals(\"token-1\", cache.token(\"user-a\", \"api-a\"))\n+        assertEquals(3, exchangeCount.get())\n+    }\n+\n+    @Test\n+    fun `refreshes token before expiry leeway`() = runBlocking {\n+        val exchangeCount = AtomicInteger()\n+        val clock = MutableClock(epoch)\n+        val texas = texasClient {\n+            val number = exchangeCount.incrementAndGet()\n+            respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        val cache = TokenCache(texas, clock, java.time.Duration.ofSeconds(30))\n+\n+        assertEquals(\"token-1\", cache.token(\"user-a\", \"api-a\"))\n+        clock.advanceSeconds(3570)\n+        assertEquals(\"token-2\", cache.token(\"user-a\", \"api-a\"))\n+        assertEquals(2, exchangeCount.get())\n+    }\n+\n+    @Test\n+    fun `shares one exchange among concurrent requests for same user and audience`() = runBlocking {\n+        val exchangeCount = AtomicInteger()\n+        val texas = texasClient {\n+            val number = exchangeCount.incrementAndGet()\n+            delay(50)\n+            respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n+\n+        val results = coroutineScope {\n+            (1..20).map {\n+                async { cache.token(\"user-a\", \"api-a\") }\n+            }.awaitAll()\n+        }\n+\n+        assertEquals(listOf(\"token-1\"), results.distinct())\n+        assertEquals(1, exchangeCount.get())\n+    }\n+\n+    private fun texasClient(handler: suspend io.ktor.client.engine.mock.MockRequestHandleScope.() -> io.ktor.client.statement.HttpResponseData): TexasClient =\n+        TexasClient(HttpClient(MockEngine(handler)), Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n+\n+    private class MutableClock(private var instant: Instant) : Clock() {\n+        override fun getZone(): ZoneId = ZoneId.of(\"UTC\")\n+\n+        override fun withZone(zone: ZoneId): Clock = this\n+\n+        override fun instant(): Instant = instant\n+\n+        fun advanceSeconds(seconds: Long) {\n+            instant = instant.plusSeconds(seconds)\n+        }\n+    }\n+}\n*** End Patch\n"
```

</details>

```
Added 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
Modified 3 file(s): <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt
```


---

<sub>1m 7s</sub>

### `bash`

**Run targeted Kotlin test suite**

$ cd <workspace> && ./gradlew test --no-daemon

```
/bin/bash: ./gradlew: No such file or directory
<shellId: 1 completed with exit code 127>
```


---

<sub>1m 10s</sub>

### `bash`

**Inspect project files and review patch**

$ cd <workspace> && ls -a && git status --short --branch && git diff --check && git diff -- src/main/kotlin/no/nav/benchmark/TokenCache.kt src/main/kotlin/no/nav/benchmark/Token.kt src/main/kotlin/no/nav/benchmark/Application.kt src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt

<details>
<summary>9 lines</summary>

```
.
..
.github
build.gradle.kts
docs
settings.gradle.kts
src
fatal: unable to get current working directory: Operation not permitted
<shellId: 2 completed with exit code 128>
```

</details>


---

<sub>1m 10s</sub>

### `bash`

**Check whether Gradle is available**

$ gradle --version

<details>
<summary>13 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djansi.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58801 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58801 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

------------------------------------------------------------
Gradle 8.14.5
------------------------------------------------------------

Build time:    2026-05-07 11:03:29 UTC
Revision:      62345becae08b13e793521816d585102fea66398

Kotlin:        2.0.21
Groovy:        3.0.25
Ant:           Apache Ant(TM) version 1.10.15 compiled on August 25 2024
Launcher JVM:  21.0.2 (Oracle Corporation 21.0.2+13-58)
Daemon JVM:    ~/.local/share/mise/installs/java/21.0.2 (no JDK specified, using current Java home)
OS:            Mac OS X 26.7.1 aarch64
<shellId: 3 completed with exit code 0>
```

</details>


---

<sub>1m 22s</sub>

### `bash`

**Compile and run Kotlin tests**

$ cd <workspace> && gradle test --no-daemon

<details>
<summary>26 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djansi.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58801 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58801 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
To honour the JVM settings for this build a single-use Daemon process will be forked. For more on this, please refer to https://docs.gradle.org/8.14.5/userguide/gradle_daemon.html#sec:disabling_the_daemon in the Gradle documentation.
Daemon will be stopped at the end of the build 
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :processResources NO-SOURCE
> Task :processTestResources NO-SOURCE
> Task :compileKotlin
> Task :compileJava NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar

> Task :compileTestKotlin FAILED
e: <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt:75:127 Unresolved reference 'HttpResponseData'.
e: <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt:76:43 Argument type mismatch: actual type is 'SuspendFunction1<MockRequestHandleScope, ERROR CLASS: Symbol not found for io.ktor.client.statement.HttpResponseData>', but 'MockEngineConfig' was expected.

FAILURE: Build failed with an exception.

* What went wrong:
Execution failed for task ':compileTestKotlin'.
> A failure occurred while executing org.jetbrains.kotlin.compilerRunner.GradleCompilerRunnerWithWorkers$GradleKotlinCompilerWorkAction
   > Compilation error. See log for more details

* Try:
> Run with --stacktrace option to get the stack trace.
> Run with --info or --debug option to get more log output.
> Run with --scan to get full insights.
> Get more help at https://help.gradle.org.

BUILD FAILED in 7s
3 actionable tasks: 3 executed
<shellId: 4 completed with exit code 1>
```

</details>


---

<sub>1m 28s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n@@\n     fun `caches tokens separately for each user and audience`() = runBlocking {\n         val exchangeCount = AtomicInteger()\n-        val texas = texasClient {\n+        val texas = TexasClient(HttpClient(MockEngine {\n             val number = exchangeCount.incrementAndGet()\n             respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\", headers = json)\n-        }\n-        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n+        }), Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n+        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n@@\n         val exchangeCount = AtomicInteger()\n         val clock = MutableClock(epoch)\n-        val texas = texasClient {\n+        val texas = TexasClient(HttpClient(MockEngine {\n             val number = exchangeCount.incrementAndGet()\n             respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\", headers = json)\n-        }\n-        val cache = TokenCache(texas, clock, java.time.Duration.ofSeconds(30))\n+        }), clock)\n+        val cache = TokenCache(texas, clock, java.time.Duration.ofSeconds(30))\n@@\n     fun `shares one exchange among concurrent requests for same user and audience`() = runBlocking {\n         val exchangeCount = AtomicInteger()\n-        val texas = texasClient {\n+        val texas = TexasClient(HttpClient(MockEngine {\n             val number = exchangeCount.incrementAndGet()\n             delay(50)\n             respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\", headers = json)\n-        }\n-        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n+        }), Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n+        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n@@\n-    private fun texasClient(handler: suspend io.ktor.client.engine.mock.MockRequestHandleScope.() -> io.ktor.client.statement.HttpResponseData): TexasClient =\n-        TexasClient(HttpClient(MockEngine(handler)), Clock.fixed(epoch, ZoneId.of(\"UTC\")))\n-\n     private class MutableClock(private var instant: Instant) : Clock() {\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
```


---

<sub>1m 37s</sub>

### `bash`

**Re-run Kotlin test suite**

$ cd <workspace> && gradle test --no-daemon

<details>
<summary>18 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djansi.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58801 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58801 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
To honour the JVM settings for this build a single-use Daemon process will be forked. For more on this, please refer to https://docs.gradle.org/8.14.5/userguide/gradle_daemon.html#sec:disabling_the_daemon in the Gradle documentation.
Daemon will be stopped at the end of the build 
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :compileKotlin UP-TO-DATE
> Task :compileJava NO-SOURCE
> Task :processResources NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar UP-TO-DATE
> Task :processTestResources NO-SOURCE
> Task :compileTestKotlin
> Task :compileTestJava NO-SOURCE
> Task :testClasses UP-TO-DATE

> Task :test
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djansi.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58801 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58801 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 7s
4 actionable tasks: 2 executed, 2 up-to-date
<shellId: 5 completed with exit code 0>
```

</details>


---

<sub>1m 51s</sub>

### `skill`

**klarsprak**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "klarsprak"
}
```

</details>

```
Skill "klarsprak" loaded successfully. Follow the instructions in the skill context.
```


---

<sub>1m 54s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt

<details>
<summary>47 lines</summary>

```
package no.nav.benchmark

import java.time.Clock
import java.time.Duration
import java.util.Base64
import java.util.concurrent.ConcurrentHashMap
import java.security.MessageDigest
import kotlinx.coroutines.CompletableDeferred

/** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */
class TokenCache(
    private val texas: TexasClient,
    private val clock: Clock,
    private val expiryLeeway: Duration = Duration.ofSeconds(30),
) {
    private data class CacheKey(val userTokenFingerprint: String, val audience: String)

    private val tokens = ConcurrentHashMap<CacheKey, OboToken>()
    private val exchanges = ConcurrentHashMap<CacheKey, CompletableDeferred<OboToken>>()

    init {
        require(!expiryLeeway.isNegative) { "expiryLeeway cannot be negative" }
    }

    suspend fun token(userToken: String, audience: String): String {
        val key = CacheKey(userToken.fingerprint(), audience)
        val cached = tokens[key]
        if (cached != null && cached.isValid(clock.instant(), expiryLeeway)) {
            return cached.accessToken
        }

        val pendingExchange = CompletableDeferred<OboToken>()
        val activeExchange = exchanges.putIfAbsent(key, pendingExchange)
        if (activeExchange != null) {
            return activeExchange.await().accessToken
        }

        try {
            val fresh = texas.exchange(userToken, audience)
            tokens[key] = fresh
            pendingExchange.complete(fresh)
            return fresh.accessToken
        } catch (cause: Throwable) {
            pendingExchange.completeExceptionally(cause)
            throw cause
        } finally {
            exchanges.remove(key, pendingExchange)
        }
    }

    private fun String.fingerprint(): String =
        Base64.getUrlEncoder().withoutPadding().encodeToString(
            MessageDigest.getInstance("SHA-256").digest(toByteArray(Charsets.UTF_8)),
        )
}
```

</details>


---

<sub>1m 54s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/Token.kt

<details>
<summary>7 lines</summary>

```
package no.nav.benchmark

import java.time.Instant
import java.time.Duration

data class OboToken(val accessToken: String, val expiresAt: Instant) {
    fun isValid(now: Instant, leeway: Duration = Duration.ZERO): Boolean =
        expiresAt.isAfter(now.plus(leeway))
}
```

</details>


---

<sub>1m 54s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/Application.kt (lines 20-50)

<details>
<summary>27 lines</summary>

```
import java.time.Clock

fun Application.vedtakModule(
    texasEngine: HttpClientEngine,
    vedtakEngine: HttpClientEngine,
    clock: Clock = Clock.systemUTC(),
    arkivEngine: HttpClientEngine = vedtakEngine,
    config: TokenConfig = TokenConfig(),
) {
    val texas = TexasClient(HttpClient(texasEngine), clock, config.texasEndpoint)
    val tokens = TokenCache(texas, clock, config.expiryLeeway)
    val vedtak = VedtakClient(HttpClient(vedtakEngine), tokens)
    val arkiv = ArkivClient(HttpClient(arkivEngine), tokens)

    routing {
        get("/internal/isalive") { call.respondText("ALIVE") }
        get("/api/vedtak/{id}") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            val id = call.parameters["id"]!!
            call.forward(vedtak.hent(userToken, id), "vedtak-api")
        }
        get("/api/vedtak/{id}/dokumenter") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            call.forward(arkiv.dokumenter(userToken, call.parameters["id"]!!), "arkiv-api")
        }
    }
}

private fun RoutingCall.userToken(): String? =
    request.headers[HttpHeaders.Authorization]?.removePrefix("Bearer ")?.takeIf { it.isNotBlank() }
```

</details>


---

<sub>1m 54s</sub>

### `view`

**view the file at <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt.**

<workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt

<details>
<summary>74 lines</summary>

```
[nav-pilot] Some values in this output were masked ([REDACTED:…]). The file on disk is unchanged. Do not write the placeholders back; ask the user if you need a value.

package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.headersOf
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import java.util.concurrent.atomic.AtomicInteger
import kotlin.test.Test
import kotlin.test.assertEquals

class TokenCacheTest {
    private val epoch = Instant.parse("2026-03-04T10:00:00Z")
    private val json = headersOf("Content-Type", "application/json")

    @Test
    fun `caches tokens separately for each user and audience`() = runBlocking {
        val exchangeCount = AtomicInteger()
        val texas = TexasClient(HttpClient(MockEngine {
            val number = exchangeCount.incrementAndGet()
            respond("""{"access_token":"[REDACTED:secret]","expires_in":3600}""", headers = json)
        }), Clock.fixed(epoch, ZoneId.of("UTC")))
        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of("UTC")))

        assertEquals("token-1", cache.token("user-a", "api-a"))
        assertEquals("token-2", cache.token("user-b", "api-a"))
        assertEquals("token-3", cache.token("user-a", "api-b"))
        assertEquals("token-1", cache.token("user-a", "api-a"))
        assertEquals(3, exchangeCount.get())
    }

    @Test
    fun `refreshes token before expiry leeway`() = runBlocking {
        val exchangeCount = AtomicInteger()
        val clock = MutableClock(epoch)
        val texas = TexasClient(HttpClient(MockEngine {
            val number = exchangeCount.incrementAndGet()
            respond("""{"access_token":"[REDACTED:secret]","expires_in":3600}""", headers = json)
        }), clock)
        val cache = TokenCache(texas, clock, java.time.Duration.ofSeconds(30))

        assertEquals("token-1", cache.token("user-a", "api-a"))
        clock.advanceSeconds(3570)
        assertEquals("token-2", cache.token("user-a", "api-a"))
        assertEquals(2, exchangeCount.get())
    }

    @Test
    fun `shares one exchange among concurrent requests for same user and audience`() = runBlocking {
        val exchangeCount = AtomicInteger()
        val texas = TexasClient(HttpClient(MockEngine {
            val number = exchangeCount.incrementAndGet()
            delay(50)
            respond("""{"access_token":"[REDACTED:secret]","expires_in":3600}""", headers = json)
        }), Clock.fixed(epoch, ZoneId.of("UTC")))
        val cache = TokenCache(texas, Clock.fixed(epoch, ZoneId.of("UTC")))

        val results = coroutineScope {
            (1..20).map {
                async { cache.token("user-a", "api-a") }
            }.awaitAll()
        }

        assertEquals(listOf("token-1"), results.distinct())
        assertEquals(1, exchangeCount.get())
    }

    private class MutableClock(private var instant: Instant) : Clock() {
        override fun getZone(): ZoneId = ZoneId.of("UTC")

        override fun withZone(zone: ZoneId): Clock = this

        override fun instant(): Instant = instant

        fun advanceSeconds(seconds: Long) {
            instant = instant.plusSeconds(seconds)
        }
    }
}
```

</details>


---

<sub>2m 1s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: src/main/kotlin/no/nav/benchmark/TokenCache.kt\n@@\n-import java.time.Clock\n-import java.time.Duration\n-import java.util.Base64\n-import java.util.concurrent.ConcurrentHashMap\n import java.security.MessageDigest\n+import java.time.Clock\n+import java.time.Duration\n+import java.util.Base64\n+import java.util.concurrent.ConcurrentHashMap\n import kotlinx.coroutines.CompletableDeferred\n*** Update File: src/main/kotlin/no/nav/benchmark/Token.kt\n@@\n-import java.time.Instant\n import java.time.Duration\n+import java.time.Instant\n*** Update File: src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n@@\n import io.ktor.client.engine.mock.respond\n import io.ktor.http.headersOf\n-import kotlinx.coroutines.async\n-import kotlinx.coroutines.awaitAll\n-import kotlinx.coroutines.coroutineScope\n-import kotlinx.coroutines.delay\n-import kotlinx.coroutines.runBlocking\n+import java.time.Duration\n import java.time.Clock\n import java.time.Instant\n import java.time.ZoneId\n import java.util.concurrent.atomic.AtomicInteger\n+import kotlinx.coroutines.async\n+import kotlinx.coroutines.awaitAll\n+import kotlinx.coroutines.coroutineScope\n+import kotlinx.coroutines.delay\n+import kotlinx.coroutines.runBlocking\n@@\n-        val cache = TokenCache(texas, clock, java.time.Duration.ofSeconds(30))\n+        val cache = TokenCache(texas, clock, Duration.ofSeconds(30))\n*** End Patch\n"
```

</details>

```
Modified 3 file(s): <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt, <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
```


---

<sub>2m 4s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n@@\n-import java.time.Duration\n import java.time.Clock\n+import java.time.Duration\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
```


---

<sub>2m 15s</sub>

### `bash`

**Verify token cache fixes with tests**

$ cd <workspace> && gradle test --no-daemon

<details>
<summary>18 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djansi.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58801 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58801 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
To honour the JVM settings for this build a single-use Daemon process will be forked. For more on this, please refer to https://docs.gradle.org/8.14.5/userguide/gradle_daemon.html#sec:disabling_the_daemon in the Gradle documentation.
Daemon will be stopped at the end of the build 
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :processResources NO-SOURCE
> Task :processTestResources NO-SOURCE
> Task :compileKotlin
> Task :compileJava NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar UP-TO-DATE
> Task :compileTestKotlin
> Task :compileTestJava NO-SOURCE
> Task :testClasses UP-TO-DATE

> Task :test
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djansi.tmpdir=~/Library/Caches/cplt/tmp/0d0c636222d621dfbdd94f218bc7179e -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58801 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58801 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 8s
4 actionable tasks: 3 executed, 1 up-to-date
<shellId: 6 completed with exit code 0>
```

</details>


---

<sub>2m 27s</sub>

### Copilot

**Feilen er rettet.** Cachen brukte bare audience som nøkkel, så ulike brukere kunne få samme token. Samtidige forespørsler ved cache-miss startet også hver sin veksling. Cachen brukte ikke `expiryLeeway`, så den kunne sende et token som rakk å utløpe underveis.

Nå skiller cachen på SHA-256-fingeravtrykk av brukertokenet og audience, og deler én pågående veksling per nøkkel. Den fornyer også tokenet innenfor konfigurert leeway. Dette unngår å lagre rå brukertoken i cache-nøkkelen, samtidig som vekslinger for ulike brukere kan gå parallelt. En global lås ville hindret den samtidigheten.

🔴 Rød sone: token-håndtering. Gå gjennom endringene nøye.

**Sjekkliste for rød sone**
- [ ] Kontroller at cache-nøkkelen skiller mellom brukere og API-er uten å lagre rå brukertoken.
- [ ] Kontroller at samtidige forespørsler for samme bruker og API deler én veksling.
- [ ] Kontroller at token fornyes før utløp, etter konfigurert leeway.

Kan du forklare hvorfor en global lås ville begrenset samtidigheten mellom ulike brukere? Still gjerne spørsmål om valgene over.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
