---
name: spring-boot-scaffold
description: Scaffold et nytt Spring Boot Kotlin-prosjekt med Nais-konfigurasjon, Flyway og standard Nav-mønstre
---

# Spring Boot Kotlin Project Scaffold

Scaffold a new Spring Boot project with Nav standards: Nais manifest, Flyway migrations, Azure AD, health endpoints, and Docker.

## Workflow

1. Create project structure (see layout below)
2. Configure `build.gradle.kts` with Nav dependencies
3. Create `Application.kt` and `application.yml`
4. Set up Nais manifest (`.nais/nais.yaml`)
5. Write `Dockerfile` with multi-stage build
6. Add controller, service, and repository layers
7. Write integration tests with MockOAuth2Server + Testcontainers
8. Add `docker-compose.yml` for local development

## Project Structure

```
my-app/
├── .nais/
│   ├── nais.yaml
│   └── nais-dev.yaml
├── .github/
│   └── workflows/
│       └── build-deploy.yml
├── src/
│   ├── main/
│   │   ├── kotlin/no/nav/myapp/
│   │   │   ├── Application.kt
│   │   │   ├── config/
│   │   │   │   └── SecurityConfig.kt
│   │   │   ├── controller/
│   │   │   │   └── ResourceController.kt
│   │   │   ├── service/
│   │   │   │   └── ResourceService.kt
│   │   │   ├── repository/
│   │   │   │   └── ResourceRepository.kt
│   │   │   └── model/
│   │   │       ├── Resource.kt
│   │   │       └── ResourceDTO.kt
│   │   └── resources/
│   │       ├── application.yml
│   │       ├── application-dev.yml
│   │       └── db/migration/
│   │           └── V1__initial_schema.sql
│   └── test/
│       └── kotlin/no/nav/myapp/
│           ├── controller/
│           │   └── ResourceControllerTest.kt
│           └── repository/
│               └── ResourceRepositoryTest.kt
├── build.gradle.kts
├── Dockerfile
└── settings.gradle.kts
```

## build.gradle.kts

```kotlin
plugins {
    id("org.springframework.boot") version "3.4.1"
    id("io.spring.dependency-management") version "1.1.7"
    kotlin("jvm") version "2.1.0"
    kotlin("plugin.spring") version "2.1.0"
}

group = "no.nav"
version = "0.0.1-SNAPSHOT"

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

repositories {
    mavenCentral()
    maven("https://github-package-registry-mirror.gc.nav.no/cached/maven-release")
}

dependencies {
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.boot:spring-boot-starter-jdbc")
    implementation("com.fasterxml.jackson.module:jackson-module-kotlin")
    implementation("org.flywaydb:flyway-core")
    implementation("org.flywaydb:flyway-database-postgresql")
    implementation("org.postgresql:postgresql")
    implementation("io.micrometer:micrometer-registry-prometheus")
    implementation("net.logstash.logback:logstash-logback-encoder:8.0")

    // Nav token-validation
    implementation("no.nav.security:token-validation-spring:5.0.13")

    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testImplementation("no.nav.security:mock-oauth2-server:2.1.10")
    testImplementation("org.testcontainers:postgresql:1.20.4")
    testImplementation("org.testcontainers:junit-jupiter:1.20.4")
    testImplementation("io.mockk:mockk:1.13.14")
}

kotlin {
    compilerOptions {
        freeCompilerArgs.addAll("-Xjsr305=strict")
    }
}

tasks.withType<Test> {
    useJUnitPlatform()
}
```

## Application.kt

```kotlin
package no.nav.myapp

import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication

@SpringBootApplication
class Application

fun main(args: Array<String>) {
    runApplication<Application>(*args)
}
```

## application.yml

```yaml
server:
  port: 8080

spring:
  application:
    name: my-app
  datasource:
    url: jdbc:postgresql://${DB_HOST:localhost}:${DB_PORT:5432}/${DB_DATABASE:myapp}
    username: ${DB_USERNAME:postgres}
    password: ${DB_PASSWORD:postgres}
  flyway:
    enabled: true

management:
  endpoints:
    web:
      exposure:
        include: health,prometheus
      base-path: /internal
  endpoint:
    health:
      show-details: always
  metrics:
    tags:
      application: ${spring.application.name}

no.nav.security.jwt:
  issuer:
    azuread:
      discoveryurl: ${AZURE_APP_WELL_KNOWN_URL}
      accepted-audience: ${AZURE_APP_CLIENT_ID}
```

## Nais Manifest (.nais/nais.yaml)

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: mitt-team
  labels:
    team: mitt-team
spec:
  image: {{ image }}
  port: 8080
  liveness:
    path: /internal/health/liveness
  readiness:
    path: /internal/health/readiness
  prometheus:
    enabled: true
    path: /internal/prometheus
  replicas:
    min: 2
    max: 4
  resources:
    limits:
      memory: 512Mi
    requests:
      cpu: 50m
      memory: 256Mi
  gcp:
    sqlInstances:
      - type: POSTGRES_15
        databases:
          - name: myapp
  azure:
    application:
      enabled: true
  accessPolicy:
    inbound:
      rules:
        - application: frontend-app
    outbound:
      rules: []
```

## Dockerfile

```dockerfile
FROM gradle:8-jdk21 AS build
WORKDIR /app
COPY build.gradle.kts settings.gradle.kts ./
COPY gradle ./gradle
RUN gradle dependencies --no-daemon
COPY src ./src
RUN gradle bootJar --no-daemon

FROM gcr.io/distroless/java21-debian12:nonroot
WORKDIR /app
COPY --from=build /app/build/libs/*.jar app.jar
ENTRYPOINT ["java", "-jar", "app.jar"]
```

## Controller

```kotlin
package no.nav.myapp.controller

import no.nav.myapp.service.ResourceService
import no.nav.security.token.support.core.api.ProtectedWithClaims
import org.springframework.http.HttpStatus
import org.springframework.http.ProblemDetail
import org.springframework.http.ResponseEntity
import org.springframework.web.bind.annotation.*
import java.util.UUID

@RestController
@RequestMapping("/api/resources")
@ProtectedWithClaims(issuer = "azuread")
class ResourceController(
    private val service: ResourceService,
) {
    @GetMapping("/{id}")
    fun getById(@PathVariable id: UUID): ResponseEntity<ResourceDTO> =
        service.findById(id)
            ?.let { ResponseEntity.ok(it) }
            ?: ResponseEntity.of(
                ProblemDetail.forStatusAndDetail(HttpStatus.NOT_FOUND, "Resource $id not found")
            ).build()

    @PostMapping
    fun create(@RequestBody @Valid request: CreateResourceRequest): ResponseEntity<ResourceDTO> {
        val created = service.create(request)
        return ResponseEntity.status(HttpStatus.CREATED).body(created)
    }
}
```

## Test with MockOAuth2Server + Testcontainers

```kotlin
package no.nav.myapp.controller

import no.nav.security.mock.oauth2.MockOAuth2Server
import org.junit.jupiter.api.*
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.test.context.DynamicPropertyRegistry
import org.springframework.test.context.DynamicPropertySource
import org.springframework.test.web.servlet.MockMvc
import org.springframework.test.web.servlet.get
import org.testcontainers.containers.PostgreSQLContainer
import org.testcontainers.junit.jupiter.Container
import org.testcontainers.junit.jupiter.Testcontainers

@Testcontainers
@SpringBootTest
@AutoConfigureMockMvc
class ResourceControllerTest {
    @Autowired
    lateinit var mockMvc: MockMvc

    companion object {
        @Container
        val postgres = PostgreSQLContainer("postgres:15")

        val mockOAuth2Server = MockOAuth2Server()

        @BeforeAll
        @JvmStatic
        fun setup() {
            mockOAuth2Server.start()
        }

        @AfterAll
        @JvmStatic
        fun tearDown() {
            mockOAuth2Server.shutdown()
        }

        @DynamicPropertySource
        @JvmStatic
        fun configureProperties(registry: DynamicPropertyRegistry) {
            registry.add("spring.datasource.url") { postgres.jdbcUrl }
            registry.add("spring.datasource.username") { postgres.username }
            registry.add("spring.datasource.password") { postgres.password }
            registry.add("no.nav.security.jwt.issuer.azuread.discoveryurl") {
                mockOAuth2Server.wellKnownUrl("azuread").toString()
            }
            registry.add("no.nav.security.jwt.issuer.azuread.accepted-audience") { "test-aud" }
        }
    }

    private fun token() = mockOAuth2Server.issueToken(
        issuerId = "azuread",
        audience = "test-aud",
        claims = mapOf("preferred_username" to "test@nav.no"),
    ).serialize()

    @Test
    fun `should return 401 without token`() {
        mockMvc.get("/api/resources/123").andExpect { status { isUnauthorized() } }
    }

    @Test
    fun `should return resources with valid token`() {
        mockMvc.get("/api/resources") {
            header("Authorization", "Bearer ${token()}")
        }.andExpect {
            status { isOk() }
        }
    }
}
```

## docker-compose.yml (local development)

```yaml
services:
  postgres:
    image: postgres:15
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
```
