---
name: ktor-scaffold
description: Scaffold eit nytt Ktor-prosjekt med Kotliquery, Flyway, Koin og Nais-konfigurasjon
license: MIT
compatibility: Kotlin with Ktor on Nais
metadata:
  domain: backend
  tags: kotlin ktor scaffold nais kotliquery
---

# Ktor Project Scaffold

Scaffold a new Kotlin/Ktor service following Nav's standard patterns. This skill composes patterns from existing Nav skills and instructions.

## Related Skills

Use these skills for deeper guidance on specific areas:

| Skill | When |
|-------|------|
| `kotlin-app-config` | Sealed class configuration pattern |
| `flyway-migration` | Database migration scripts |
| `tokenx-auth` | Service-to-service authentication |
| `observability-setup` | Prometheus metrics and health endpoints |
| `api-design` | REST API conventions and error handling |

## Step 1: Project Structure

Generate this directory layout:

```
my-service/
├── build.gradle.kts
├── settings.gradle.kts
├── gradle/
│   └── libs.versions.toml
├── src/
│   ├── main/
│   │   ├── kotlin/no/nav/myservice/
│   │   │   ├── Application.kt
│   │   │   ├── Config.kt
│   │   │   ├── Routes.kt
│   │   │   ├── Database.kt
│   │   │   └── domain/
│   │   └── resources/
│   │       ├── db/migration/
│   │       │   └── V1__initial_schema.sql
│   │       └── logback.xml
│   └── test/
│       └── kotlin/no/nav/myservice/
│           └── ApplicationTest.kt
├── .nais/
│   ├── app.yaml
│   └── app-dev.yaml
└── Dockerfile
```

## Step 2: Gradle Configuration

### libs.versions.toml

```toml
[versions]
kotlin = "2.3.20"
ktor = "3.4.2"
kotliquery = "1.9.1"
hikaricp = "7.0.2"
flyway = "12.2.0"
postgresql = "42.7.10"
koin = "4.2.0"
logback = "1.5.32"
logstash = "9.0"
micrometer = "1.16.4"
kotest = "6.1.10"
mockk = "1.14.9"
testcontainers = "2.0.4"
mockOAuth2 = "3.0.1"

[libraries]
ktor-server-core = { module = "io.ktor:ktor-server-core", version.ref = "ktor" }
ktor-server-netty = { module = "io.ktor:ktor-server-netty", version.ref = "ktor" }
ktor-server-content-negotiation = { module = "io.ktor:ktor-server-content-negotiation", version.ref = "ktor" }
ktor-serialization-jackson = { module = "io.ktor:ktor-serialization-jackson", version.ref = "ktor" }
ktor-server-auth = { module = "io.ktor:ktor-server-auth", version.ref = "ktor" }
ktor-server-auth-jwt = { module = "io.ktor:ktor-server-auth-jwt", version.ref = "ktor" }
ktor-server-metrics-micrometer = { module = "io.ktor:ktor-server-metrics-micrometer", version.ref = "ktor" }
ktor-server-test-host = { module = "io.ktor:ktor-server-test-host", version.ref = "ktor" }

kotliquery = { module = "com.github.seratch:kotliquery", version.ref = "kotliquery" }
hikaricp = { module = "com.zaxxer:HikariCP", version.ref = "hikaricp" }
flyway-core = { module = "org.flywaydb:flyway-core", version.ref = "flyway" }
flyway-postgres = { module = "org.flywaydb:flyway-database-postgresql", version.ref = "flyway" }
postgresql = { module = "org.postgresql:postgresql", version.ref = "postgresql" }

koin-ktor = { module = "io.insert-koin:koin-ktor", version.ref = "koin" }
koin-logger-slf4j = { module = "io.insert-koin:koin-logger-slf4j", version.ref = "koin" }

logback-classic = { module = "ch.qos.logback:logback-classic", version.ref = "logback" }
logstash-encoder = { module = "net.logstash.logback:logstash-logback-encoder", version.ref = "logstash" }
micrometer-prometheus = { module = "io.micrometer:micrometer-registry-prometheus", version.ref = "micrometer" }

kotest-runner = { module = "io.kotest:kotest-runner-junit5", version.ref = "kotest" }
kotest-assertions = { module = "io.kotest:kotest-assertions-core", version.ref = "kotest" }
mockk = { module = "io.mockk:mockk", version.ref = "mockk" }
testcontainers-postgres = { module = "org.testcontainers:postgresql", version.ref = "testcontainers" }
mock-oauth2-server = { module = "no.nav.security:mock-oauth2-server", version.ref = "mockOAuth2" }
```

### build.gradle.kts

```kotlin
plugins {
    kotlin("jvm") version "2.1.10"
    id("com.gradleup.shadow") version "9.0.0-beta6"
}

application {
    mainClass.set("no.nav.myservice.ApplicationKt")
}

dependencies {
    implementation(libs.ktor.server.core)
    implementation(libs.ktor.server.netty)
    implementation(libs.ktor.server.content.negotiation)
    implementation(libs.ktor.serialization.jackson)
    implementation(libs.ktor.server.auth)
    implementation(libs.ktor.server.auth.jwt)
    implementation(libs.ktor.server.metrics.micrometer)

    implementation(libs.kotliquery)
    implementation(libs.hikaricp)
    implementation(libs.flyway.core)
    implementation(libs.flyway.postgres)
    runtimeOnly(libs.postgresql)

    implementation(libs.koin.ktor)
    implementation(libs.koin.logger.slf4j)

    implementation(libs.logback.classic)
    implementation(libs.logstash.encoder)
    implementation(libs.micrometer.prometheus)

    testImplementation(libs.ktor.server.test.host)
    testImplementation(libs.kotest.runner)
    testImplementation(libs.kotest.assertions)
    testImplementation(libs.mockk)
    testImplementation(libs.testcontainers.postgres)
    testImplementation(libs.mock.oauth2.server)
}

tasks.test {
    useJUnitPlatform()
}
```

## Step 3: Application.kt

```kotlin
package no.nav.myservice

import io.ktor.server.engine.*
import io.ktor.server.netty.*
import org.koin.ktor.plugin.Koin

fun main() {
    val config = Config.from(System.getenv())

    Database.migrate(config.database)

    embeddedServer(Netty, port = 8080) {
        install(Koin) {
            modules(appModule(config))
        }
        configureRouting()
        configureSerialization()
        configureMonitoring()
    }.start(wait = true)
}
```

## Step 4: Config.kt

Use the sealed class pattern from `kotlin-app-config` skill:

```kotlin
package no.nav.myservice

sealed class Config(
    val database: DatabaseConfig,
) {
    data class Dev(private val env: Map<String, String>) : Config(
        database = DatabaseConfig(
            url = env.getValue("DATABASE_URL"),
            username = env.getValue("DATABASE_USERNAME"),
            password = env.getValue("DATABASE_PASSWORD"),
        ),
    )

    data class Prod(private val env: Map<String, String>) : Config(
        database = DatabaseConfig(
            url = env.getValue("DATABASE_URL"),
            username = env.getValue("DATABASE_USERNAME"),
            password = env.getValue("DATABASE_PASSWORD"),
        ),
    )

    data object Local : Config(
        database = DatabaseConfig(
            url = "jdbc:postgresql://localhost:5432/myservice",
            username = "postgres",
            password = "postgres",
        ),
    )

    companion object {
        fun from(env: Map<String, String>): Config = when (env["NAIS_CLUSTER_NAME"]) {
            "dev-gcp" -> Dev(env)
            "prod-gcp" -> Prod(env)
            else -> Local
        }
    }
}

data class DatabaseConfig(val url: String, val username: String = "", val password: String = "")
```

## Step 5: Database.kt

```kotlin
package no.nav.myservice

import com.zaxxer.hikari.HikariDataSource
import kotliquery.queryOf
import kotliquery.sessionOf
import kotliquery.using
import org.flywaydb.core.Flyway
import javax.sql.DataSource

object Database {
    fun dataSource(config: DatabaseConfig): HikariDataSource = HikariDataSource().apply {
        jdbcUrl = config.url
        username = config.username
        password = config.password
        maximumPoolSize = 10
        minimumIdle = 2
    }

    fun migrate(config: DatabaseConfig) {
        Flyway.configure()
            .dataSource(config.url, config.username, config.password)
            .load()
            .migrate()
    }
}
```

## Step 6: Dockerfile

```dockerfile
FROM gradle:8-jdk21 AS build
WORKDIR /app
COPY build.gradle.kts settings.gradle.kts ./
COPY gradle ./gradle
RUN gradle dependencies --no-daemon
COPY src ./src
RUN gradle shadowJar --no-daemon

FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/jre:openjdk-21
WORKDIR /app
COPY --from=build /app/build/libs/*-all.jar app.jar
CMD ["-jar", "app.jar"]
```

## Step 7: NAIS Manifest

Generate using the `nais-manifest` prompt, ensuring:
- PostgreSQL via `gcp.sqlInstances`
- Prometheus enabled at `/metrics`
- Health endpoints at `/isalive` and `/isready`

## Verification

After scaffolding, run:

```bash
./gradlew build
./gradlew test
```
