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
2. Confirm the frameworks that actually carry Jackson 3 support are in place *first*: **Spring Boot 4.0+** (Boot 3.x is Jackson 2 only) and **Ktor 3.4.0+** (required for `ktor-serialization-jackson3`).
3. Check for internal version skew — a hardcoded version string elsewhere in the build can contradict the version catalog, and the catalog may not be a local file you can grep. Ask Gradle: `./gradlew dependencyInsight --dependency <artifact> --configuration compileClasspath`.
4. Target Jackson **3.1+** (LTS). Jackson 3.0.x is a transitional, non-LTS release — avoid pinning to it if 3.1+ is available.
5. After migrating, confirm no stray Jackson 2.x `databind`/`core` remains on the classpath (`./gradlew dependencies --configuration compileClasspath | grep jackson`) — a leftover 2.x jar lets old `com.fasterxml.jackson.databind.*` imports keep compiling with no error.
6. If the service uses Ktor content negotiation, swap `ktor-serialization-jackson` for `ktor-serialization-jackson3` (package `io.ktor.serialization.jackson3.*`, `jackson3 { ... }` DSL) — the old artifact stays on Jackson 2 and is not drop-in compatible.

Failure modes, exact commands and reasoning for steps 2, 3, 5 and 6: [references/dependency-preflight.md](references/dependency-preflight.md).

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

### `JsonNode.map()` shadows Kotlin's `Iterable.map` (Jackson 3.1+)

**The most widespread Kotlin breakage in this migration** — and one OpenRewrite does not touch. Jackson 3.1 added a member method on `JsonNode` ([jackson-databind#5579](https://github.com/FasterXML/jackson-databind/issues/5579)):

```java
public <R> R map(Function<? super JsonNode, ? extends R> mapper) { return mapper.apply(this); }
```

`JsonNode` still implements `Iterable<JsonNode>`, and **Kotlin resolves members before extensions**, so existing `jsonNode.map { ... }` stops calling `kotlin.collections.map` (iterate children → `List<R>`) and instead calls the new member (apply the lambda once to the node itself → a single `R`). Method references (`node.map(JsonNode::asString)`) are SAM-converted onto the member just the same.

Fix by going through `values()` — the same children view `JsonNode.iterator()` uses, so no copy and identical semantics to the old behavior:

```kotlin
// before (Jackson 2, or Jackson 3.0.x): List<String>. On 3.1+: one String.
val behov = packet["@behov"].map { it.asString() }

// after
val behov = packet["@behov"].values().map { it.asString() }

// null-safe chains need the conversion inside the chain
val orgnumre = godkjenning["orgnummere"]?.values()?.map(JsonNode::asString).orEmpty()
```

- **Usually, but not always, a compile error.** It typically surfaces as an `Unresolved reference` / type mismatch on the *next* call in the chain (`.containsAll(...)`, `.sorted()`). It compiles silently — and ships a behavior change — when the single result happens to fit: e.g. the lambda returns `String` and the chain continues with `CharSequence` extensions (`first()`, `take()`, `filter { }`), or the result is only interpolated into a string or passed as `Any`.
- **Only `map` is affected.** `filter`, `filterNot`, `flatMap`, `mapNotNull`, `any`, `none`, `first` have no `JsonNode` member and still resolve to the Kotlin extensions — including on a `List` produced by an earlier `filter`. `forEach` resolves to `java.lang.Iterable.forEach(Consumer)`, which behaves identically.
- The `asIterable()` workaround suggested in the issue thread does **not** exist as a Kotlin stdlib extension on `Iterable`.
- A project pinned to Jackson **3.0.x** won't see this at all; it appears the moment the version is bumped to 3.1+.

How to find every call site — including the silently-compiling ones — is in [references/kotlin-cleanup.md](references/kotlin-cleanup.md).

### Nav-specific gotcha: fields/properties starting with æ, ø, å

**Extremely sneaky, and silent.** Jackson's default accessor-naming validator treats `æ`/`ø`/`å` as a non-letter first character, so Kotlin properties/Java fields starting with one (common in Norwegian domain models: `årsak`, `ønsket`) are rejected as getter/setter targets and dropped from (de)serialization with no exception and no warning. Not new in Jackson 3 — but very likely to resurface exactly when an `ObjectMapper`/`JsonMapper` config gets rebuilt as a builder chain.

```kotlin
val mapper = JsonMapper.builder()
    .accessorNaming(
        DefaultAccessorNamingStrategy.Provider()
            .withFirstCharAcceptance(true, true) // lower-case ok, non-ASCII-letter ok
    )
    .build()
```

Add a regression test asserting that a field starting with æ/ø/å round-trips — this bug class never shows up as a compile error or in a "does it crash" test.

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
3. Check for `JsonNode.map` shadowing (Jackson 3.1+): source-search every `jsonNode.map { }` call site, then confirm with `javap -p -c` over `build/classes/kotlin` that no call to `…databind/*Node.map:` remains — match every `*Node` type, not just `JsonNode`, since the call site records the receiver's static type. A compiling build does *not* prove this one is clean. Procedure in [references/kotlin-cleanup.md](references/kotlin-cleanup.md).
4. Grep for any remaining `com.fasterxml.jackson` imports outside `jackson-annotations` usage — these indicate incomplete migration.
5. Re-run the dependency-tree check from Pre-flight step 5 — grepping your own source is not enough, since a stray transitive `com.fasterxml.jackson.core:jackson-databind` can let old imports keep compiling without ever showing up in a source-level grep.
6. If default-setting changes break existing behavior intentionally relied upon, consider `JsonMapper.builderWithJackson2Defaults()` as a stepping stone rather than reintroducing legacy settings ad hoc.

For symptom → likely cause → fix lookups (e.g. "dates serialize as strings now", "property order changed"), see [references/rename-and-defaults.md](references/rename-and-defaults.md).

## Related

| Resource | Use For |
|----------|---------|
| `java-to-kotlin` skill | Broader Java→Kotlin conversion if migrating both at once |
| `kotlin-app-config` skill | Sealed class config pattern, useful when rebuilding `ObjectMapper` setup as a builder |
| OpenRewrite recipe `org.openrewrite.java.jackson.UpgradeJackson_2_3` | Automated mechanical Java-side migration |
| [Official migration guide](https://github.com/FasterXML/jackson/blob/main/jackson3/MIGRATING_TO_JACKSON_3.md) | Authoritative source — consult for anything not covered here |
| [jackson-databind#5579](https://github.com/FasterXML/jackson-databind/issues/5579) | The `JsonNode.map()` addition (3.1) and the Kotlin shadowing discussion in its comments |

## Boundaries

### ✅ Always

- Verify Java 17+ baseline before starting
- Verify framework minimums (Ktor 3.4.0+, Spring Boot 4.0+) and check for hardcoded-vs-catalog version skew with `./gradlew dependencyInsight --dependency <lib>`
- Run the OpenRewrite recipe first, then do Kotlin-specific cleanup by hand
- Search explicitly for post-construction `ObjectMapper` mutation (`.apply { ... }` patterns) — the #1 silent-failure risk
- Rewrite every `jsonNode.map { ... }` / `jsonNode.map(JsonNode::asX)` as `.values().map { ... }` on Jackson 3.1+ — the new `JsonNode.map()` member shadows Kotlin's `Iterable.map` — and verify with a `javap` bytecode grep, since some cases compile silently
- Run a dependency-tree check (`./gradlew dependencies | grep jackson`) after migrating — old imports keep compiling with no error if a stray Jackson 2.x jar is still on the classpath
- Use LSP/symbol tools (`findReferences`, rename refactoring) to verify Jackson usages before and after renaming, not blind text search-and-replace
- Run full build + test suite after migration, not just compile
- Target Jackson 3.1+ (LTS), not 3.0.x
- Verify `accessorNaming` handles æ/ø/å-prefixed properties when rebuilding any `ObjectMapper`/`JsonMapper` config — add a round-trip test, don't just trust that it "still works"
- Swap `ktor-serialization-jackson` for `ktor-serialization-jackson3` (and its `io.ktor.serialization.jackson3.*` import) together with the Jackson 3 bump, if the project uses Ktor content negotiation

### ⚠️ Ask First

- Removing the OpenRewrite plugin vs. keeping it in the build for future use
- Whether to adopt `jackson-bom` if the project doesn't already pin Jackson versions centrally
- Reintroducing Jackson 2.x default behavior via `JsonMapper.builderWithJackson2Defaults()` instead of adapting to new defaults

### 🚫 Never

- Assume OpenRewrite fully handles Kotlin source files — it doesn't
- Assume a green build means the Kotlin side is done — `jsonNode.map { }` can bind to the new `JsonNode.map()` member and compile cleanly while returning a single value instead of a list
- Blanket-rewrite `com.fasterxml.jackson.annotation.*` to `tools.jackson` — that package intentionally stays on the old name; only databind/format-specific annotations move
- Leave `com.fasterxml.jackson` imports referring to non-annotation packages (`databind`, `core`, `datatype`, etc.) after migration is declared done — those must all move to `tools.jackson`
- Migrate Jackson before confirming all dependent libraries (Spring, Ktor, etc.) support Jackson 3
- Treat `enableDefaultTyping()` as something to find a replacement for — it's removed by design for security; redesign around validated `activateDefaultTypingAsProperty(...)` instead
