---
applyTo: "**/*.kt"
---

Spring Boot-mønstre for Nav-backends: controller, service, repository, validering og feilhåndtering.

> Ktor er det mest brukte rammeverket for nye Kotlin-backends i Nav, men Spring Boot brukes aktivt av mange team. Teamene velger selv. For migrering mellom rammeverkene, se [$java-to-kotlin](../skills/java-to-kotlin/).

> Spring Boot patterns for Nav backends. Apply when the file uses Spring (`@RestController`, `@Service`, Spring Data etc.) — for Ktor/Rapids & Rivers apps, see `kotlin-ktor.instructions.md` instead.

# Spring Boot Framework Patterns

## Controller Layer

```kotlin
@RestController
@RequestMapping("/api")
class ResourceController(
    private val service: ResourceService
) {
    @GetMapping("/resources/{id}")
    fun getResource(@PathVariable id: UUID): ResponseEntity<ResourceDTO> {
        val resource = service.findById(id)
        return ResponseEntity.ok(resource)
    }

    @PostMapping("/resources")
    fun createResource(@RequestBody @Valid request: CreateResourceRequest): ResponseEntity<ResourceDTO> {
        val created = service.create(request)
        return ResponseEntity.status(HttpStatus.CREATED).body(created)
    }
}
```

## Service Layer

```kotlin
@Service
class ResourceService(
    private val repository: ResourceRepository
) {
    @Transactional
    fun create(request: CreateResourceRequest): ResourceDTO {
        val entity = request.toEntity()
        return repository.save(entity).toDTO()
    }
}
```

## Database Access

Check existing repository implementations in the codebase — patterns vary:

```kotlin
// Option A: CrudRepository interface
@Repository
interface ResourceRepository : CrudRepository<ResourceEntity, UUID> {
    fun findByIdent(ident: String): List<ResourceEntity>

    @Query("SELECT * FROM resource WHERE status = :status")
    fun findByStatus(status: String): List<ResourceEntity>
}

// Option B: NamedParameterJdbcTemplate (raw SQL)
@Repository
class JdbcResourceRepository(
    private val namedParameterJdbcTemplate: NamedParameterJdbcTemplate
) {
    fun findById(id: UUID): ResourceEntity? {
        val sql = "SELECT * FROM resource WHERE id = :id"
        return namedParameterJdbcTemplate.query(sql, mapOf("id" to id)) { rs, _ ->
            ResourceEntity(id = rs.getObject("id", UUID::class.java))
        }.firstOrNull()
    }
}
```

## Auth (token-validation-spring)

```kotlin
@ProtectedWithClaims(issuer = "azuread")
@RestController
class ProtectedController {
    @GetMapping("/api/protected")
    fun protectedEndpoint(): ResponseEntity<Any> {
        // Token validation is handled automatically by the filter
        return ResponseEntity.ok(mapOf("status" to "ok"))
    }
}
```

## Configuration

Use `application.yml` / `application-{profile}.yml` for Spring configuration:

```yaml
spring:
  datasource:
    url: jdbc:postgresql://${DB_HOST}:${DB_PORT}/${DB_DATABASE}
    username: ${DB_USERNAME}
    password: ${DB_PASSWORD}
  flyway:
    enabled: true
```

## Structured Logging

```kotlin
// Check existing log statements in the repo to match the established pattern
// SLF4J placeholder format (always available)
logger.info("Processing event: eventId={}", eventId)

// If logstash-logback-encoder is on the classpath:
// logger.info("Processing event {}", kv("event_id", eventId))

// Spring request-scoped MDC via filter
MDC.put("x_request_id", request.getHeader("X-Request-ID"))
```

## Error Handling (ProblemDetail)

```kotlin
@RestControllerAdvice
class ErrorHandler {
    @ExceptionHandler(ResourceNotFoundException::class)
    fun handleNotFound(ex: ResourceNotFoundException): ProblemDetail =
        ProblemDetail.forStatusAndDetail(HttpStatus.NOT_FOUND, ex.message ?: "Ressurs ikke funnet")

    @ExceptionHandler(MethodArgumentNotValidException::class)
    fun handleValidation(ex: MethodArgumentNotValidException): ProblemDetail =
        ProblemDetail.forStatusAndDetail(HttpStatus.BAD_REQUEST, "Validering feilet").apply {
            setProperty("feil", ex.bindingResult.fieldErrors.map {
                mapOf("felt" to it.field, "melding" to it.defaultMessage)
            })
        }
}
```

## Configuration Properties

```kotlin
@ConfigurationProperties(prefix = "app")
data class AppProperties(
    val externalApiUrl: String,
    val maxRetries: Int = 3,
    val featureFlags: FeatureFlags = FeatureFlags(),
) {
    data class FeatureFlags(
        val nyFunksjon: Boolean = false,
    )
}

// Enable in Application.kt
@SpringBootApplication
@EnableConfigurationProperties(AppProperties::class)
class Application
```

## Testing

### Full integration test

- Use `@SpringBootTest` for integration tests
- Use Testcontainers for integration tests with real databases
- Use MockOAuth2Server for auth testing

### Test Slices (faster, isolated tests)

```kotlin
// Controller-only test — no database, no service
@WebMvcTest(ResourceController::class)
class ResourceControllerSliceTest {
    @Autowired lateinit var mockMvc: MockMvc
    @MockkBean lateinit var service: ResourceService

    @Test
    fun `should return 200`() {
        every { service.findAll() } returns listOf(testResource())
        mockMvc.get("/api/resources") {
            header("Authorization", "Bearer ${token()}")
        }.andExpect { status { isOk() } }
    }
}

// Repository-only test — real database, no controllers
@DataJpaTest
@Testcontainers
class ResourceRepositorySliceTest {
    @Autowired lateinit var repository: ResourceRepository

    companion object {
        @Container val postgres = PostgreSQLContainer("postgres:15")

        @DynamicPropertySource @JvmStatic
        fun configure(registry: DynamicPropertyRegistry) {
            registry.add("spring.datasource.url") { postgres.jdbcUrl }
            registry.add("spring.datasource.username") { postgres.username }
            registry.add("spring.datasource.password") { postgres.password }
        }
    }

    @Test
    fun `should save and find`() {
        val saved = repository.save(ResourceEntity(name = "test"))
        val found = repository.findById(saved.id!!)
        found shouldNotBe null
    }
}
```
- Use `@MockkBean` for mocking Spring beans (requires `com.ninja-squad:springmockk` — verify it is in `build.gradle.kts` before using)

```kotlin
@SpringBootTest
class ResourceServiceTest {
    @MockkBean
    private lateinit var repository: ResourceRepository

    @Autowired
    private lateinit var service: ResourceService

    @Test
    fun `should create resource`() {
        every { repository.save(any()) } returns testEntity
        val result = service.create(request)
        result.id shouldBe testEntity.id
    }
}
```

## Boundaries

### ✅ Always
- Use constructor injection (not field injection)
- Annotate transactional boundaries explicitly
- Follow existing repository pattern in the codebase — don't mix styles
- Preserve existing code structure when making targeted fixes — don't rename, restructure, or refactor working code beyond the task at hand

### ⚠️ Ask First
- Introducing new Spring modules or starters
- Changing transaction isolation levels

### 🚫 Never
- Use field injection (`@Autowired` on fields)
- Mix Spring Data JPA and JDBC in the same repository layer
- Put business logic in controllers

## Related

| Type | Name | When to use |
|------|------|-------------|
| Instruction | [kotlin-ktor](kotlin-ktor.instructions.md) | Ktor patterns for Nav backends |
| Skill | [$java-to-kotlin](../skills/java-to-kotlin/) | Migrating Spring Boot code to Kotlin/Ktor |
| Skill | [$ktor-scaffold](../skills/ktor-scaffold/) | Scaffolding a new Ktor project to replace a Spring service |
| Skill | [$spring-boot-scaffold](../skills/spring-boot-scaffold/) | Scaffolding a new Spring Boot Kotlin project |
| Skill | [$flyway-migration](../skills/flyway-migration/) | Database migration patterns |
| Agent | @auth-agent | Authentication setup (TokenX, Azure AD) |
