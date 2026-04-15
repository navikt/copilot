---
name: spring-boot-endpoint
description: Scaffold et Spring Boot REST-endepunkt med Controller, Service, Repository, Test og Nais-konfig
---

# Spring Boot Endpoint

Create a complete REST endpoint for a Spring Boot application with Nav standards.

## Questions

Ask these questions before scaffolding:

1. What is the resource name? (e.g. `vedtak`, `søknad`, `bruker`)
2. What fields does the resource have?
3. Does it need auth (Azure AD)?
4. Does it need a database (PostgreSQL)?
5. Which operations? (CRUD, or specific ones?)

## Create these files

### 1. Controller

```kotlin
@RestController
@RequestMapping("/api/{{ressurs}}")
@ProtectedWithClaims(issuer = "azuread")
class {{Ressurs}}Controller(
    private val service: {{Ressurs}}Service,
) {
    @GetMapping
    fun list(): ResponseEntity<List<{{Ressurs}}DTO>> =
        ResponseEntity.ok(service.findAll())

    @GetMapping("/{id}")
    fun getById(@PathVariable id: UUID): ResponseEntity<{{Ressurs}}DTO> =
        service.findById(id)
            ?.let { ResponseEntity.ok(it) }
            ?: ResponseEntity.notFound().build()

    @PostMapping
    fun create(@RequestBody @Valid request: Create{{Ressurs}}Request): ResponseEntity<{{Ressurs}}DTO> {
        val created = service.create(request)
        return ResponseEntity.status(HttpStatus.CREATED).body(created)
    }
}
```

### 2. Service

```kotlin
@Service
class {{Ressurs}}Service(
    private val repository: {{Ressurs}}Repository,
) {
    fun findAll(): List<{{Ressurs}}DTO> =
        repository.findAll().map { it.toDTO() }

    fun findById(id: UUID): {{Ressurs}}DTO? =
        repository.findById(id)?.toDTO()

    @Transactional
    fun create(request: Create{{Ressurs}}Request): {{Ressurs}}DTO {
        val entity = request.toEntity()
        return repository.save(entity).toDTO()
    }
}
```

### 3. Repository

```kotlin
@Repository
class {{Ressurs}}Repository(
    private val jdbcTemplate: NamedParameterJdbcTemplate,
) {
    fun findAll(): List<{{Ressurs}}> =
        jdbcTemplate.query("SELECT * FROM {{tabell}} ORDER BY created_at DESC", rowMapper)

    fun findById(id: UUID): {{Ressurs}}? =
        jdbcTemplate.query(
            "SELECT * FROM {{tabell}} WHERE id = :id",
            mapOf("id" to id),
            rowMapper,
        ).firstOrNull()

    fun save(entity: {{Ressurs}}): {{Ressurs}} {
        jdbcTemplate.update(
            "INSERT INTO {{tabell}} (id, ...) VALUES (:id, ...)",
            mapOf("id" to entity.id, ...),
        )
        return entity
    }

    private val rowMapper = RowMapper<{{Ressurs}}> { rs, _ ->
        {{Ressurs}}(
            id = rs.getObject("id", UUID::class.java),
            // ... map fields
        )
    }
}
```

### 4. Flyway Migration

```sql
-- V{{neste_versjon}}__create_{{tabell}}_table.sql
CREATE TABLE {{tabell}} (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- ... fields
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_{{tabell}}_created_at ON {{tabell}}(created_at);
```

### 5. Test

```kotlin
@Testcontainers
@SpringBootTest
@AutoConfigureMockMvc
class {{Ressurs}}ControllerTest {
    @Autowired lateinit var mockMvc: MockMvc
    @Autowired lateinit var objectMapper: ObjectMapper

    companion object {
        @Container val postgres = PostgreSQLContainer("postgres:15")
        val mockOAuth2Server = MockOAuth2Server()

        @BeforeAll @JvmStatic fun setup() { mockOAuth2Server.start() }
        @AfterAll @JvmStatic fun tearDown() { mockOAuth2Server.shutdown() }

        @DynamicPropertySource @JvmStatic
        fun configure(registry: DynamicPropertyRegistry) {
            registry.add("spring.datasource.url") { postgres.jdbcUrl }
            registry.add("spring.datasource.username") { postgres.username }
            registry.add("spring.datasource.password") { postgres.password }
            registry.add("no.nav.security.jwt.issuer.azuread.discoveryurl") {
                mockOAuth2Server.wellKnownUrl("azuread").toString()
            }
            registry.add("no.nav.security.jwt.issuer.azuread.accepted-audience") { "test" }
        }
    }

    private fun token() = mockOAuth2Server.issueToken("azuread", audience = "test").serialize()

    @Test
    fun `should return 401 without token`() {
        mockMvc.get("/api/{{ressurs}}").andExpect { status { isUnauthorized() } }
    }

    @Test
    fun `should create and get resource`() {
        val request = Create{{Ressurs}}Request(/* ... */)

        mockMvc.post("/api/{{ressurs}}") {
            header("Authorization", "Bearer ${token()}")
            contentType = MediaType.APPLICATION_JSON
            content = objectMapper.writeValueAsString(request)
        }.andExpect { status { isCreated() } }
    }
}
```

## Etter scaffolding

- Kjør `./gradlew test` for å verifisere
- Legg til endepunktet i `accessPolicy.inbound.rules` i Nais-manifestet hvis det skal kalles av andre apper

## Forstå koden

After generating the endpoint, explain:

1. **Lagdeling** — Why Controller → Service → Repository and not just Controller → Repository? What does the Service layer give you for testing and transaction management?
2. **@Transactional** — Why it's on the Service layer, not the Controller. What happens if two database operations in a single request need to be atomic?
3. **@Valid og validering** — How Spring Boot validation annotations work under the hood. Why server-side validation even when the frontend also validates?
4. **Teststrategien** — Why `@SpringBootTest` with Testcontainers instead of mocking the database. When would a `@WebMvcTest` slice test be more appropriate?

🔴 **Rød sone**: Transaction boundaries and error handling are where most production bugs hide — understand `@Transactional` propagation before adding complex business logic.

Still gjerne spørsmål om valgene over.
