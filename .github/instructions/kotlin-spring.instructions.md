---
applyTo: "**/*.kt"
---

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

## Testing

- Use `@SpringBootTest` for integration tests
- Use Testcontainers for integration tests with real databases
- Use MockOAuth2Server for auth testing
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

### ⚠️ Ask First
- Introducing new Spring modules or starters
- Changing transaction isolation levels

### 🚫 Never
- Use field injection (`@Autowired` on fields)
- Mix Spring Data JPA and JDBC in the same repository layer
- Put business logic in controllers
