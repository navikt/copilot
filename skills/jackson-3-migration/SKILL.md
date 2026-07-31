---
name: jackson-3-migration
description: Migrer Jackson 2.x til Jackson 3.x (tools.jackson) i Kotlin/Java-prosjekter — automatisert OpenRewrite-pass pluss manuell Kotlin-spesifikk opprydding og verifisering
license: MIT
compatibility: Gradle/Maven project using com.fasterxml.jackson (2.x), migrating to Jackson 3.x
metadata:
  domain: backend
  tags: jackson kotlin java migration serialization openrewrite ktor
---

# Jackson 2 → 3 Migration

Systematic migration from Jackson 2.x (`com.fasterxml.jackson`) to Jackson 3.x (`tools.jackson`). Combines an automated OpenRewrite pass for mechanical Java changes with explicit manual steps for Kotlin-specific patterns that OpenRewrite does not cover, followed by build+test verification.

## When to Use

- A Nav service depends on `com.fasterxml.jackson.*` and needs to move to Jackson 3.x (e.g. for Spring Boot 4 compatibility)
- Build fails after a transitive dependency bump pulls in Jackson 3
- Recurring "fix Jackson migration" requests — use this skill instead of re-deriving the rules each time

## Pre-flight Checks

1. Confirm baseline JDK is 17+ (`java -version`, `sourceCompatibility` in Gradle). Jackson 3 does not run on Java 8/11 — stop and flag if not met.
2. Confirm all direct/transitive Jackson-dependent libraries (Spring, Ktor, testing libs) have Jackson 3-compatible versions available before starting, using these known minimums — don't just check "some version exists":
   - **Spring**: Jackson 3 support ships with **Spring Boot 4.0** (baseline: Spring Framework 7.0+, Java 17+). Spring Boot 3.x is Jackson 2 only — there is no Jackson-3-compatible Boot 3.x.
   - **Ktor**: the dedicated `ktor-serialization-jackson3` artifact requires **Ktor 3.4.0+**. On an older Ktor version the artifact/import doesn't exist yet — do not migrate Jackson before bumping Ktor.
   - Do not migrate Jackson before the framework version that actually carries Jackson 3 support is in place.
3. **Check for internal version skew, not just the dependency's own compatibility.** A common false trail: a version catalog entry correctly points at a Jackson-3-compatible artifact (e.g. `ktor-serialization-jackson3`), while a *different*, hardcoded version string for the same library elsewhere in the build (e.g. `val ktorVersion = "3.3.3"`) is still too old for that artifact to exist. This produces an `Unresolved reference` / missing-class error (e.g. `JacksonConverter`, `jacksonObjectMapper`) that looks like a Jackson package-rename mistake but is actually a version mismatch.
   - **Don't try to locate and grep the catalog file — it may not be local at all.** A Gradle version catalog can be a project-local `gradle/libs.versions.toml`, declared inline in `settings.gradle.kts` (`dependencyResolutionManagement.versionCatalogs`), *or* a published catalog artifact pulled in via `from("no.nav.dagpenger:dp-version-catalog:x.y.z")` — resolved and cached like any other dependency (`~/.gradle/caches/modules-2/files-2.1/...`), with no toml file in the repo to grep at all.
   - Ask Gradle for the resolved truth instead: `./gradlew dependencyInsight --dependency ktor-serialization-jackson3 --configuration compileClasspath` (per-module if multi-project) prints every requested version, which requester asked for it, and the final resolved version — regardless of whether the request came from a local toml, a published catalog, or a hardcoded string. This is more reliable than grepping build files for version-looking strings.
4. Target Jackson **3.1+** (LTS). Jackson 3.0.x is a transitional, non-LTS release — avoid pinning to it if 3.1+ is available.
5. **Check for a stray Jackson 2.x `databind`/`core` on the classpath before declaring the migration done.** A not-yet-upgraded dependency (test library, HTTP client, an internal module, etc.) can transitively pull in `com.fasterxml.jackson.core:jackson-databind`/`jackson-core` alongside the new `tools.jackson.*` jars. Gradle/Maven will happily resolve both at once, so old `com.fasterxml.jackson.databind.*` imports (e.g. `JsonMappingException`, `ObjectMapper`) **keep compiling with no error** — they just silently bind to the leftover 2.x jar instead of failing loudly. Run `./gradlew dependencies --configuration compileClasspath | grep jackson` (or the Maven `dependency:tree` equivalent) after migrating and confirm the only `com.fasterxml.jackson*` group left is `jackson-annotations`. If something else shows up, `dependencyInsight --dependency jackson-databind` (same command as above) shows exactly which dependency is pulling it in, so you can upgrade that dependency or `exclude(group = "com.fasterxml.jackson.core")` on it explicitly.
6. **Ktor bonus:** if the service uses Ktor's content negotiation, it ships a *dedicated* module for Jackson 3 — `io.ktor:ktor-serialization-jackson3` — with the `jackson3 { ... }` DSL under package `io.ktor.serialization.jackson3.*`. The old `ktor-serialization-jackson` artifact (`io.ktor.serialization.jackson.*`) stays on Jackson 2 and is **not** drop-in compatible; swap the artifact and the import together, not just the Jackson dependency.

## Package/Group-ID Exception (apply before any blanket rename)

`com.fasterxml.jackson` → `tools.jackson` is **not** a universal find-replace:

- `jackson-annotations` (group-id, and `com.fasterxml.jackson.annotation.*` package) **stays on the old name** — it is still versioned as a 2.x artifact (e.g. `jackson-annotations:2.20`) and used as-is by Jackson 3.
- **Exception to the exception:** databind-level annotations like `@JsonSerialize`/`@JsonDeserialize`, and format-specific annotations (e.g. XML ones), **do** move to `tools.jackson.databind.annotation` / the corresponding new package.
- It is correct and expected for a fully migrated Jackson 3 codebase to still import `com.fasterxml.jackson.annotation.*` (for `@JsonProperty`, `@JsonIgnore`, etc.) alongside `tools.jackson.*` imports elsewhere. Do not "fix" these as if they were missed renames.

**Prefer semantic tools over text search-and-replace for this step.** Before renaming, use LSP/code-intelligence tools (`findReferences`, `goToDefinition`) or an IDE's MCP server (e.g. `search_symbol`, `rename_refactoring`) to enumerate every real usage of a Jackson type — a blind `grep`+`sed` pass cannot tell `com.fasterxml.jackson.annotation.*` (stays) apart from `com.fasterxml.jackson.databind.*` (moves), and will happily "rename" a string literal or comment that only looks like a package path. After renaming, re-run `findReferences`/symbol search to confirm no stale `com.fasterxml.jackson.*` symbol remains outside the annotation exception.

**Don't guess a 3.x subpackage by analogy with 2.x layout, and don't unzip jars to check.** Most `com.fasterxml.jackson.databind.*` types land in the `tools.jackson.databind` *root* package in 3.x (e.g. `DatabindException`, not `tools.jackson.databind.exc.DatabindException`) — see [references/rename-and-defaults.md](references/rename-and-defaults.md) for a verified list of fully-qualified paths. If a class isn't in that list, verify the package with LSP `workspaceSymbol` search or IDE autocomplete before trying an import.

## Step 1: Automated Pass (OpenRewrite)

Use the official recipe to handle mechanical Java renames before touching anything by hand.

**Check the current state before assuming a clean 2.x baseline.** If the codebase already has a partial, hand-rolled migration attempt (mixed `com.fasterxml`/`tools.jackson` imports, ad-hoc renames), running the recipe may not apply cleanly or may not be the fastest path. In that case, run `./gradlew compileKotlin` (or the Java equivalent) directly first to see the actual current compile errors, then work from those rather than assuming this playbook's recipe-first order fits as-is.

```kotlin
// build.gradle.kts — add temporarily if not already present
plugins {
    id("org.openrewrite.rewrite") version "<latest>"
}
dependencies {
    rewrite("org.openrewrite.recipe:rewrite-jackson:<latest>")
}
```

```bash
./gradlew rewriteRun --recipe=org.openrewrite.java.jackson.UpgradeJackson_2_3
```

For Maven projects, use the `rewrite-maven-plugin` equivalent instead:

```bash
mvn org.openrewrite.maven:rewrite-maven-plugin:run \
  -Drewrite.activeRecipes=org.openrewrite.java.jackson.UpgradeJackson_2_3
```

- Review the diff — OpenRewrite handles Java import/package renames (`com.fasterxml.jackson.*` → `tools.jackson.*`, respecting the `jackson-annotations` exception above), some deprecated-API replacements, and Maven/Gradle group-id/dependency-coordinate updates for Java sources.
- **It does not reliably rewrite Kotlin source files** — treat its output as the Java-side baseline, not the finish line.
- Run a build immediately after the recipe (before doing manual cleanup) — this surfaces removed/renamed APIs the recipe didn't catch as compile errors early, rather than mixing them with behavioral cleanup later.
- Remove the OpenRewrite plugin/dependency again once the recipe has run, unless you want to keep it for future recipes.

## Step 2: Kotlin-Specific Cleanup (manual — OpenRewrite does not cover this)

Nav services are predominantly Kotlin. These changes must be applied by hand. See [references/kotlin-cleanup.md](references/kotlin-cleanup.md) for the full set of before/after snippets (mutable-`ObjectMapper` patterns, date/timezone config, visibility config, polymorphic typing, `JsonFactory` builder, `@JsonView`).

### `jackson-module-kotlin`

```kotlin
// build.gradle.kts — before
implementation("com.fasterxml.jackson.module:jackson-module-kotlin:2.x")

// after
implementation("tools.jackson.module:jackson-module-kotlin:3.x")
```

```kotlin
// before
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.registerKotlinModule
val mapper = ObjectMapper().registerKotlinModule()

// after — package changes, and ObjectMapper is now immutable/builder-based
import tools.jackson.databind.json.JsonMapper
import tools.jackson.module.kotlin.jacksonObjectMapper
val mapper = jacksonObjectMapper()

// need further config? prefer jacksonMapperBuilder() over manually chaining
// JsonMapper.builder().addModule(kotlinModule()) — same result, one call:
import tools.jackson.module.kotlin.jacksonMapperBuilder
val mapper = jacksonMapperBuilder()
    .disable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
    .build()
```

Search for `ObjectMapper()` followed by `.apply`, `.registerModule`, `.configure`, `.enable`, `.disable`, `.setXxx` calls outside a builder chain — these are the highest-risk pattern in Kotlin codebases and fail silently rather than with a compile error (full before/after in the reference file).

### Nav-specific gotcha: fields/properties starting with æ, ø, å

**Extremely sneaky** — not a compile error, not a migration-specific bug, but very likely to surface (or resurface) exactly when an `ObjectMapper`/`JsonMapper` gets rebuilt during this migration. Jackson's default accessor-naming validator treats the first character of a derived property name strictly (roughly: must be an ASCII letter), so Kotlin properties/Java fields whose *first character* is `æ`, `ø`, or `å` (very common in Norwegian domain models, e.g. `årsak`, `øknad`) get silently rejected as valid getter/setter targets — the property is dropped from (de)serialization with no exception, no warning.

Fix by relaxing first-character acceptance on the accessor naming strategy:

```kotlin
val mapper = JsonMapper.builder()
    .accessorNaming(
        DefaultAccessorNamingStrategy.Provider()
            .withFirstCharAcceptance(true, true)
    )
    .build()
```

- First `true` = allow a lower-case first character; second `true` = allow a non-(ASCII-)letter first character — Jackson's validator treats `æ`/`ø`/`å` as "non-letter" for this check even though they are letters.
- This is **not new in Jackson 3** — the same fix applies to `ObjectMapper.setAccessorNaming(...)` in 2.x. Check for it explicitly during migration since it's an easy thing to forget to port when rebuilding config as a builder chain, and it fails silently rather than breaking the build.
- Add a regression test asserting a field starting with æ/ø/å actually round-trips through the mapper — this bug class does not show up via compile errors or typical "does it crash" tests.

## Step 3: General Manual Cleanup

Exceptions are now unchecked (`JsonProcessingException` → `JacksonException` extends `RuntimeException`), several class/method renames apply to `jackson-databind` and the streaming API, and a number of defaults changed silently (dates as ISO-8601 strings instead of epoch millis, alphabetical property sorting, enums via `toString()`, etc.). These do not surface as compile errors — they change runtime behavior and typically show up as test failures. See [references/rename-and-defaults.md](references/rename-and-defaults.md) for the full rename tables and the complete list of changed defaults.

```kotlin
// jackson-bom — recommended to avoid version-skew across modules
dependencies {
    implementation(platform("tools.jackson:jackson-bom:3.1.0"))
    implementation("tools.jackson.core:jackson-databind")
    implementation("tools.jackson.module:jackson-module-kotlin")
}
```

## Step 4: Verification

1. `./gradlew build` (or project's equivalent) — compile errors will surface most renamed classes/methods immediately.
2. Run the full test suite — immutable `ObjectMapper` misconfiguration and default-setting changes (e.g. `FAIL_ON_TRAILING_TOKENS` now on by default) typically show up as test failures, not compile errors.
3. Grep for any remaining `com.fasterxml.jackson` imports outside `jackson-annotations` usage — these indicate incomplete migration.
4. Re-run the dependency-tree check from Pre-flight step 4 — grepping your own source is not enough, since a stray transitive `com.fasterxml.jackson.core:jackson-databind` can let old imports keep compiling without ever showing up in a source-level grep.
5. If default-setting changes break existing behavior intentionally relied upon, consider `JsonMapper.builderWithJackson2Defaults()` as a stepping stone rather than reintroducing legacy settings ad hoc.

For symptom → likely cause → fix lookups (e.g. "dates serialize as strings now", "property order changed"), see [references/rename-and-defaults.md](references/rename-and-defaults.md).

## Related

| Resource | Use For |
|----------|---------|
| `java-to-kotlin` skill | Broader Java→Kotlin conversion if migrating both at once |
| `kotlin-app-config` skill | Sealed class config pattern, useful when rebuilding `ObjectMapper` setup as a builder |
| OpenRewrite recipe `org.openrewrite.java.jackson.UpgradeJackson_2_3` | Automated mechanical Java-side migration |
| [Official migration guide](https://github.com/FasterXML/jackson/blob/main/jackson3/MIGRATING_TO_JACKSON_3.md) | Authoritative source — consult for anything not covered here |

## Boundaries

### ✅ Always

- Verify Java 17+ baseline before starting
- Verify framework version minimums before migrating (Ktor 3.4.0+ for `ktor-serialization-jackson3`, Spring Boot 4.0+/Framework 7.0+ for Jackson 3) and use `./gradlew dependencyInsight --dependency <lib>` to check for hardcoded-vs-catalog version skew — don't assume the catalog is a local toml file to grep
- Run the OpenRewrite recipe first, then do Kotlin-specific cleanup by hand
- Search explicitly for post-construction `ObjectMapper` mutation (`.apply { ... }` patterns) — the #1 silent-failure risk
- Run a dependency-tree check (`./gradlew dependencies | grep jackson`) after migrating to confirm no `com.fasterxml.jackson.core:jackson-databind`/`jackson-core` remains transitively — old imports keep compiling with no error if a stray 2.x jar is still on the classpath
- Use LSP/symbol tools (`findReferences`, rename refactoring) to verify Jackson usages before and after renaming, not blind text search-and-replace
- Run full build + test suite after migration, not just compile
- Target Jackson 3.1+ (LTS), not 3.0.x
- Verify `accessorNaming` handles æ/ø/å-prefixed properties when rebuilding any `ObjectMapper`/`JsonMapper` config — add a round-trip test, don't just trust that it "still works"
- Swap `ktor-serialization-jackson` for `ktor-serialization-jackson3` (and the `io.ktor.serialization.jackson3.*` import) together with the Jackson 3 bump, if the project uses Ktor content negotiation

### ⚠️ Ask First

- Removing the OpenRewrite plugin vs. keeping it in the build for future use
- Whether to adopt `jackson-bom` if the project doesn't already pin Jackson versions centrally
- Reintroducing Jackson 2.x default behavior via `JsonMapper.builderWithJackson2Defaults()` instead of adapting to new defaults

### 🚫 Never

- Assume OpenRewrite fully handles Kotlin source files — it doesn't
- Blanket-rewrite `com.fasterxml.jackson.annotation.*` to `tools.jackson` — that package intentionally stays on the old name; only databind/format-specific annotations move
- Leave `com.fasterxml.jackson` imports referring to non-annotation packages (`databind`, `core`, `datatype`, etc.) after migration is declared done — those must all move to `tools.jackson`
- Migrate Jackson before confirming all dependent libraries (Spring, Ktor, etc.) support Jackson 3
- Treat `enableDefaultTyping()` as something to find a replacement for — it's removed by design for security; redesign around validated `activateDefaultTypingAsProperty(...)` instead
