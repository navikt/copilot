---
name: api-design
description: REST API-designmønstre, versjonering, feilhåndtering (RFC 7807) og OpenAPI-konvensjoner for Nav-tjenester
license: MIT
compatibility: Go or Kotlin backend on Nais
metadata:
  domain: backend
  tags: api rest design openapi error-handling
---

# API Design Skill

REST API design for Nav services. Covers naming conventions, error handling with ProblemDetail, versioning, pagination, and OpenAPI spec.

## URL Conventions

```
# ✅ Correct
GET    /api/vedtak                    # List
GET    /api/vedtak/{id}               # Get by ID
POST   /api/vedtak                    # Create
PUT    /api/vedtak/{id}               # Full update
PATCH  /api/vedtak/{id}               # Partial update
DELETE /api/vedtak/{id}               # Delete

# ✅ Sub-resources
GET    /api/vedtak/{id}/aktiviteter   # List child resources
POST   /api/vedtak/{id}/aktiviteter   # Create child resource

# ✅ Actions (verb as sub-resource)
POST   /api/vedtak/{id}/godkjenn      # State transition

# ❌ Wrong
GET    /api/getVedtak                 # Verb in URL
GET    /api/vedtak/hentAlle           # Verb in URL
POST   /api/createVedtak              # Verb in URL
GET    /api/Vedtak                    # PascalCase
```

## Error Handling (RFC 7807 / ProblemDetail)

```kotlin
// Spring Boot 3+ — built-in ProblemDetail support

@RestControllerAdvice
class ErrorHandler {
    @ExceptionHandler(ResourceNotFoundException::class)
    fun handleNotFound(ex: ResourceNotFoundException): ProblemDetail =
        ProblemDetail.forStatusAndDetail(HttpStatus.NOT_FOUND, ex.message ?: "Resource not found").apply {
            title = "Resource not found"
            setProperty("resourceType", ex.resourceType)
            setProperty("resourceId", ex.resourceId)
        }

    @ExceptionHandler(MethodArgumentNotValidException::class)
    fun handleValidation(ex: MethodArgumentNotValidException): ProblemDetail =
        ProblemDetail.forStatusAndDetail(HttpStatus.BAD_REQUEST, "Validation failed").apply {
            title = "Invalid request"
            setProperty("errors", ex.bindingResult.fieldErrors.map {
                mapOf("field" to it.field, "message" to it.defaultMessage)
            })
        }
}
```

For Ktor, use the `StatusPages` plugin:

```kotlin
install(StatusPages) {
    exception<ResourceNotFoundException> { call, cause ->
        call.respond(HttpStatusCode.NotFound, ProblemResponse(
            title = "Resource not found",
            status = 404,
            detail = cause.message ?: "Resource not found",
            instance = call.request.uri,
        ))
    }
    exception<ValidationException> { call, cause ->
        call.respond(HttpStatusCode.BadRequest, ProblemResponse(
            title = "Invalid request",
            status = 400,
            detail = "Validation failed",
            errors = cause.errors,
        ))
    }
}
```

Response format (RFC 7807):

```json
{
  "type": "about:blank",
  "title": "Resource not found",
  "status": 404,
  "detail": "Vedtak with id 123 does not exist",
  "instance": "/api/vedtak/123",
  "resourceType": "vedtak",
  "resourceId": "123"
}
```

## Pagination

Use offset-based pagination with consistent parameter names:

```kotlin
@GetMapping("/api/vedtak")
fun list(
    @RequestParam(defaultValue = "0") page: Int,
    @RequestParam(defaultValue = "20") size: Int,
    @RequestParam(defaultValue = "opprettetDato") sort: String,
    @RequestParam(defaultValue = "desc") order: String,
): Page<VedtakDTO> {
    require(size in 1..100) { "size must be between 1 and 100" }
    val pageable = PageRequest.of(page, size, Sort.by(Sort.Direction.fromString(order), sort))
    return vedtakService.findAll(pageable)
}
```

Response:

```json
{
  "content": [...],
  "page": {
    "size": 20,
    "number": 0,
    "totalElements": 142,
    "totalPages": 8
  }
}
```

## Input Validation

```kotlin
data class CreateVedtakRequest(
    @field:NotBlank(message = "Title is required")
    val tittel: String,

    @field:Size(min = 11, max = 11, message = "FNR must be 11 digits")
    @field:Pattern(regexp = "\\d{11}", message = "FNR must consist of digits")
    val fnr: String,

    @field:Positive(message = "Amount must be positive")
    val belop: BigDecimal,

    @field:NotNull(message = "Start date is required")
    val fom: LocalDate,
)
```

## HTTP Status Codes

| Code | Usage |
|---|---|
| `200 OK` | Successful GET, PUT, PATCH |
| `201 Created` | Successful POST (new resource) |
| `204 No Content` | Successful DELETE |
| `400 Bad Request` | Invalid input / validation failed |
| `401 Unauthorized` | Missing or invalid token |
| `403 Forbidden` | Valid token, but no access |
| `404 Not Found` | Resource does not exist |
| `409 Conflict` | Duplicate / state conflict |
| `422 Unprocessable Entity` | Semantic error (valid format, wrong content) |
| `500 Internal Server Error` | Unexpected server error |

## OpenAPI / Swagger

```kotlin
// Spring Boot + springdoc-openapi
@Operation(
    summary = "Hent vedtak",
    description = "Henter vedtak basert på ID",
    responses = [
        ApiResponse(responseCode = "200", description = "Vedtak funnet"),
        ApiResponse(responseCode = "404", description = "Vedtak ikke funnet"),
    ]
)
@GetMapping("/{id}")
fun getById(@PathVariable id: UUID): ResponseEntity<VedtakDTO>
```

## Versioning

Use URL-based versioning when breaking changes are necessary:

```kotlin
// v1 — original
@RestController
@RequestMapping("/api/v1/vedtak")
class VedtakV1Controller

// v2 — ny kontrakt
@RestController
@RequestMapping("/api/v2/vedtak")
class VedtakV2Controller
```

Alternatively, avoid versioning by:
- Only adding new fields (never removing)
- Making new fields optional
- Deprecating fields with `@Deprecated` before removal

## Rules

- **Use nouns** in URLs, not verbs
- **Use kebab-case** for multi-word URL segments: `/api/vedtak-perioder`
- **Use camelCase** for JSON fields: `opprettetDato`, `brukerId`
- **Always return ProblemDetail** on errors (not plain text)
- **Validate input** at controller level with `@Valid`
- **Never log PII** in request/response — log correlation ID
- **Set `Content-Type: application/json`** on all responses
