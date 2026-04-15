---
name: ktor-endpoint
description: Generer ein Ktor-rute med autentisering, validering og feilhåndtering
---

# Ktor Endpoint

Generate a Ktor route handler following Nav patterns.

## Input

Describe the endpoint: HTTP method, path, request/response shape, authentication requirements, and database operations.

## Output

Generate a Ktor route with:

1. **Route definition** with authentication
2. **Request validation** with error responses
3. **Database interaction** via Kotliquery
4. **Error handling** with proper HTTP status codes
5. **Structured logging**

## Patterns

### Authenticated Endpoint

```kotlin
fun Route.resourceRoutes(repository: ResourceRepository) {
    authenticate("azureAd") {
        route("/api/v1/resources") {
            get {
                val resources = repository.findAll()
                call.respond(HttpStatusCode.OK, resources)
            }

            get("/{id}") {
                val id = call.parameters["id"]?.toLongOrNull()
                    ?: return@get call.respond(HttpStatusCode.BadRequest, "Invalid id")

                val resource = repository.findById(id)
                    ?: return@get call.respond(HttpStatusCode.NotFound, "Resource not found")

                call.respond(HttpStatusCode.OK, resource)
            }

            post {
                val request = call.receive<CreateResourceRequest>()

                val errors = request.validate()
                if (errors.isNotEmpty()) {
                    return@post call.respond(HttpStatusCode.BadRequest, ValidationError(errors))
                }

                val created = repository.save(request.toEntity())
                call.respond(HttpStatusCode.Created, created)
            }

            delete("/{id}") {
                val id = call.parameters["id"]?.toLongOrNull()
                    ?: return@delete call.respond(HttpStatusCode.BadRequest, "Invalid id")

                val deleted = repository.deleteById(id)
                if (!deleted) {
                    return@delete call.respond(HttpStatusCode.NotFound, "Resource not found")
                }

                call.respond(HttpStatusCode.NoContent)
            }
        }
    }
}
```

### Request Validation

```kotlin
@Serializable
data class CreateResourceRequest(
    val name: String,
    val description: String? = null,
) {
    fun validate(): List<String> = buildList {
        if (name.isBlank()) add("name cannot be blank")
        if (name.length > 255) add("name must be 255 characters or less")
    }

    fun toEntity() = Resource(name = name, description = description)
}

@Serializable
data class ValidationError(val errors: List<String>)
```

### Repository (Kotliquery)

```kotlin
class ResourceRepository(private val dataSource: DataSource) {
    fun findAll(): List<Resource> = using(sessionOf(dataSource)) { session ->
        session.run(
            queryOf("SELECT * FROM resources ORDER BY created_at DESC")
                .map { row -> row.toResource() }
                .asList
        )
    }

    fun findById(id: Long): Resource? = using(sessionOf(dataSource)) { session ->
        session.run(
            queryOf("SELECT * FROM resources WHERE id = ?", id)
                .map { row -> row.toResource() }
                .asSingle
        )
    }

    fun save(resource: Resource): Resource = using(sessionOf(dataSource)) { session ->
        val id = session.run(
            queryOf(
                "INSERT INTO resources (name, description) VALUES (?, ?) RETURNING id",
                resource.name, resource.description
            ).asUpdateAndReturnGeneratedKey
        ) ?: throw IllegalStateException("Failed to insert resource")

        resource.copy(id = id)
    }

    fun deleteById(id: Long): Boolean = using(sessionOf(dataSource)) { session ->
        session.run(
            queryOf("DELETE FROM resources WHERE id = ?", id).asUpdate
        ) > 0
    }

    private fun Row.toResource() = Resource(
        id = long("id"),
        name = string("name"),
        description = stringOrNull("description"),
        createdAt = localDateTime("created_at"),
    )
}
```

### Wiring in Application

```kotlin
fun Application.configureRouting(repository: ResourceRepository) {
    routing {
        resourceRoutes(repository)

        get("/isalive") { call.respondText("Alive") }
        get("/isready") { call.respondText("Ready") }
    }
}
```

## Forstå koden

After generating the endpoint, explain:

1. **Arkitektoniske valg** — Why this layering (route → repository)? What are the alternatives, and why is this preferred in Ktor?
2. **Feilhåndtering** — Why explicit status codes instead of exceptions? What happens if the database is down?
3. **Sikkerhet** — Why `authenticate("azureAd")` wraps the entire route block. What would happen without it?
4. **Tradeoffs** — What does this pattern give up (e.g., no service layer) and when would you need more structure?

🔴 **Rød sone**: Request validation logic and error handling are areas worth understanding deeply — don't just copy the pattern, think about what your specific endpoint needs to validate.

Still gjerne spørsmål om valgene over.
