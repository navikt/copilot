# Dependency Pre-flight — Full Reference

Details behind the pre-flight checklist in `SKILL.md`: framework minimums, version skew, and leftover Jackson 2 jars.

## Framework minimums

- **Spring**: Jackson 3 support ships with **Spring Boot 4.0** (baseline: Spring Framework 7.0+, Java 17+). Spring Boot 3.x is Jackson 2 only — there is no Jackson-3-compatible Boot 3.x.
- **Ktor**: the dedicated `ktor-serialization-jackson3` artifact requires **Ktor 3.4.0+**. On an older Ktor version the artifact/import doesn't exist yet.
- Don't just check that "some version exists" — do not migrate Jackson before the framework version that actually carries Jackson 3 support is in place.

## Internal version skew (the classic false trail)

A version catalog entry correctly points at a Jackson-3-compatible artifact (e.g. `ktor-serialization-jackson3`), while a *different*, hardcoded version string for the same library elsewhere in the build (e.g. `val ktorVersion = "3.3.3"`) is still too old for that artifact to exist. This produces an `Unresolved reference` / missing-class error (e.g. `JacksonConverter`, `jacksonObjectMapper`) that looks like a Jackson package-rename mistake but is actually a version mismatch.

**Don't try to locate and grep the catalog file — it may not be local at all.** A Gradle version catalog can be a project-local `gradle/libs.versions.toml`, declared inline in `settings.gradle.kts` (`dependencyResolutionManagement.versionCatalogs`), *or* a published catalog artifact pulled in via `from("no.nav.dagpenger:dp-version-catalog:x.y.z")` — resolved and cached like any other dependency (`~/.gradle/caches/modules-2/files-2.1/...`), with no toml file in the repo to grep at all.

Ask Gradle for the resolved truth instead:

```bash
./gradlew dependencyInsight --dependency ktor-serialization-jackson3 --configuration compileClasspath
```

Per-module if multi-project. It prints every requested version, which requester asked for it, and the final resolved version — regardless of whether the request came from a local toml, a published catalog, or a hardcoded string. More reliable than grepping build files for version-looking strings.

## Stray Jackson 2.x `databind`/`core` on the classpath

A not-yet-upgraded dependency (test library, HTTP client, an internal module, etc.) can transitively pull in `com.fasterxml.jackson.core:jackson-databind`/`jackson-core` alongside the new `tools.jackson.*` jars. Gradle/Maven will happily resolve both at once, so old `com.fasterxml.jackson.databind.*` imports (e.g. `JsonMappingException`, `ObjectMapper`) **keep compiling with no error** — they just silently bind to the leftover 2.x jar instead of failing loudly.

```bash
./gradlew dependencies --configuration compileClasspath | grep jackson
```

(or the Maven `dependency:tree` equivalent). Confirm the only `com.fasterxml.jackson*` group left is `jackson-annotations`. If something else shows up, `dependencyInsight --dependency jackson-databind` shows exactly which dependency pulls it in, so you can upgrade that dependency or `exclude(group = "com.fasterxml.jackson.core")` on it explicitly.

## Ktor content negotiation artifact

Ktor ships a *dedicated* module for Jackson 3 — `io.ktor:ktor-serialization-jackson3` — with the `jackson3 { ... }` DSL under package `io.ktor.serialization.jackson3.*`. The old `ktor-serialization-jackson` artifact (`io.ktor.serialization.jackson.*`) stays on Jackson 2 and is **not** drop-in compatible; swap the artifact and the import together, not just the Jackson dependency. Before/after snippet in [kotlin-cleanup.md](kotlin-cleanup.md).
