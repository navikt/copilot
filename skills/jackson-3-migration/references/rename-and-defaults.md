# Rename Tables and Changed Defaults — Full Reference

Full lookup tables for `Step 3: General Manual Cleanup` and `Step 4: Verification` in `SKILL.md`. Consult this file when a compile error or test failure doesn't match one of the inline examples.

## Fully-qualified 3.x package paths (verified — don't guess, don't unzip jars)

**General rule: most `com.fasterxml.jackson.databind.*` types land in the `tools.jackson.databind` *root* package in 3.x, not in a `.exc`/`.core`-style subpackage**, even though a few 2.x types were historically split across subpackages. Guessing a subpackage by analogy with 2.x layout is a common wrong turn — confirm with IDE autocomplete / LSP `workspaceSymbol` search instead of inspecting jar contents.

Verified against Jackson 3.1.2 / jackson-module-kotlin 3.2.1 / Ktor 3.5.1:

| Class (2.x name → 3.x name) | Fully-qualified 3.x path |
|---|---|
| `JsonMappingException` → `DatabindException` | `tools.jackson.databind.DatabindException` (root package, **not** `.exc`) |
| `DeserializationFeature` | `tools.jackson.databind.DeserializationFeature` (root package) |
| `SerializationFeature` | `tools.jackson.databind.SerializationFeature` (root package) |
| `ObjectMapper` (mutable) → `JsonMapper` | `tools.jackson.databind.json.JsonMapper` |
| `jacksonObjectMapper()` (Kotlin module) | `tools.jackson.module.kotlin.jacksonObjectMapper` |
| `jacksonMapperBuilder()` (Kotlin module) | `tools.jackson.module.kotlin.jacksonMapperBuilder` |
| Ktor `JacksonConverter` | `io.ktor.serialization.jackson3.JacksonConverter` (artifact `io.ktor:ktor-serialization-jackson3`) |

The rename tables below give class-name mappings only, not full package paths, for every renamed type — treat the general rule above as the default assumption for anything not listed here, and verify with `workspaceSymbol`/autocomplete rather than trial-and-error imports.

## Exceptions and core type renames

| Area | Before (2.x) | After (3.x) |
|------|--------------|-------------|
| Exceptions | `catch (e: JsonProcessingException)` | `catch (e: JacksonException)` (now unchecked `RuntimeException`) |
| Exceptions | `JsonMappingException` | `DatabindException` — `tools.jackson.databind.DatabindException` (root package, see above) |
| Exceptions | `JsonEOFException` | `UnexpectedEndOfInputException` |
| Format mappers | `ObjectMapper(YAMLFactory())` | `YAMLMapper()` / `YAMLMapper.builder()` |
| Streaming features | `JsonParser.Feature` | `StreamReadFeature` / `JsonReadFeature` |
| Streaming features | `JsonGenerator.Feature` | `StreamWriteFeature` / `JsonWriteFeature` |
| Dependencies | `jackson-datatype-jsr310`, `jackson-datatype-jdk8`, `jackson-module-parameter-names` | remove — built into `jackson-databind` |
| Dependencies | version per module | use `tools.jackson:jackson-bom` platform, drop explicit versions |

Since exceptions are now unchecked, also review `throws`/`@Throws` declarations and callers that assumed a checked exception (e.g. wrapping in a broader `try/catch` "just in case" that's no longer required, or Java interop that needs `@Throws` to keep declaring it for Java callers).

## Changed defaults (silent behavior changes — no compile error)

| Area | Before (2.x) | After (3.x) |
|------|--------------|-------------|
| `WRITE_DATES_AS_TIMESTAMPS` on `SerializationFeature`, defaults `true` | moved to `DateTimeFeature.WRITE_DATES_AS_TIMESTAMPS`, now defaults to `false` — dates serialize as ISO-8601 strings, not epoch millis, unless explicitly enabled |
| `FAIL_ON_UNKNOWN_PROPERTIES` | **unchanged — still enabled by default in 3.0.** (Verified against primary source; do not assume this loosened.) |
| `FAIL_ON_NULL_FOR_PRIMITIVES` disabled | now enabled by default — may start failing `@JsonCreator` constructors with missing primitive (`int`, etc.) values |
| `ALLOW_FINAL_FIELDS_AS_MUTATORS` enabled | now disabled — Jackson no longer force-writes `final` fields via reflection; genuinely immutable classes may stop deserializing correctly |
| `DEFAULT_VIEW_INCLUSION` enabled | now disabled — properties without an explicit `@JsonView` are excluded when a view is active; significant for `@JsonView` users |
| `USE_GETTERS_AS_SETTERS` enabled | now disabled — getters returning mutable `Collection`/`Map` are no longer used as implicit setters; may expose previously-masked missing setters |
| `FIX_FIELD_NAME_UPPER_CASE_PREFIX` disabled | now enabled — affects property-name detection for fields like `iPhone`; matters most for Lombok-style fields |
| `SORT_CREATOR_PROPERTIES_BY_DECLARATION_ORDER` (disabled) | feature removed — 3.0 behavior is equivalent to it being enabled (only relevant if `SORT_CREATOR_PROPERTIES_FIRST` is on, which it is by default in both versions) |
| `USE_STD_BEAN_NAMING` | removed — 3.0 always behaves as if it were enabled |
| POJO property serialization order followed declaration order | `MapperFeature.SORT_PROPERTIES_ALPHABETICALLY` now enabled by default — property order changes unless `@JsonPropertyOrder` is used; can break brittle string-equality tests on JSON output |
| `READ_ENUMS_USING_TO_STRING` / `WRITE_ENUMS_USING_TO_STRING` defaulted `false` | now default to `true` (moved to new `EnumFeature` enum) — enums (de)serialize via `toString()` instead of `name()` unless overridden |
| `MapperFeature.AUTO_DETECT_CREATORS` (and related `AUTO_DETECT_*`) | removed — use `JsonMapper.builder().changeDefaultVisibility { ... }` instead |

## Removed APIs and classes

| Removed | Replacement |
|---------|-------------|
| `ObjectMapper.copy()`, `enableDefaultTyping()` | no direct replacement — reconstruct via builder / avoid default typing entirely |
| `setSerializationInclusion(Include.NON_NULL)` | `.builder().changeDefaultPropertyInclusion { it.withValueInclusion(...) }` (see `SKILL.md`) |
| `ObjectMapper.canSerialize()` / `canDeserialize()` | removed entirely, no replacement |
| `LaissezFaireSubTypeValidator` (no longer public) | `BasicPolymorphicTypeValidator.builder()...build()` |
| `MappingJsonFactory`, `ObjectCodec` | `ObjectCodec` split into `ObjectReadContext` / `ObjectWriteContext` |

## Core class/method renames (jackson-databind and streaming API)

| 2.x | 3.x |
|-----|-----|
| `JsonDeserializer` | `ValueDeserializer` |
| `JsonSerializer` | `ValueSerializer` |
| `BeanDeserializerModifier` / `BeanSerializerModifier` | `ValueDeserializerModifier` / `ValueSerializerModifier` |
| `SerializerProvider` | `SerializationContext` |
| `Module` (jackson-databind) | `JacksonModule` |
| `TextNode` | `StringNode` |
| `JsonStreamContext` | `TokenStreamContext` |
| `JsonLocation` | `TokenStreamLocation` |
| `JsonParseException` / `JsonGenerationException` | `StreamReadException` / `StreamWriteException` |
| `JsonToken.FIELD_NAME` | `JsonToken.PROPERTY_NAME` |
| `ObjectMapper.getRegisteredModuleIds()` | `ObjectMapper.registeredModules()` |
| `ContextualDeserializer`/`ContextualSerializer`/`ResolvableDeserializer`/`ResolvableSerializer` | removed — methods folded into `ValueDeserializer`/`ValueSerializer` |

Run a repo-wide symbol search (see `SKILL.md` on LSP/MCP-based refactoring) for these class names in Kotlin/Java source — renamed types compile-fail loudly, but custom (de)serializers extending the old base classes are easy to miss if they're in a less-obvious package.

## Streaming API method renames (`JsonParser`/`JsonGenerator`)

Custom serializers/deserializers and any low-level streaming code (not just POJO mapping) are the most likely place these are missed, since they compile-fail one call at a time rather than surfacing as a single obvious diff:

| 2.x | 3.x |
|-----|-----|
| `JsonGenerator.getCodec()` | `objectWriteContext()` |
| `JsonGenerator.getCurrentValue()` / `.setCurrentValue()` | `currentValue()` / `assignCurrentValue()` |
| `JsonGenerator.writeObject()` | `writePOJO()` |
| `JsonParser.getCodec()` | `objectReadContext()` |
| `JsonParser.getCurrentLocation()` | `currentLocation()` |
| `JsonParser.getTokenLocation()` | `currentTokenLocation()` |
| `JsonParser.getCurrentValue()` / `.setCurrentValue()` | `currentValue()` / `assignCurrentValue()` |
| `JsonParser.getText()`, `getTextCharacters()`, etc. | `getString()`, `getStringCharacters()`, etc. (all `xxxText` → `xxxString`) |
| Any `xxxField`-named method on either class | renamed to `xxxProperty` |

## `DateTimeFeature` and `EnumFeature` — full moved-member list

Both are new enums in 3.x; these values moved off `DeserializationFeature`/`SerializationFeature`. If code does `.enable(DeserializationFeature.XXX)` or `.enable(SerializationFeature.XXX)` for anything below, it will fail to compile until switched to the new enum:

| Moved to `DateTimeFeature` | Moved to `EnumFeature` |
|---|---|
| `ADJUST_DATES_TO_CONTEXT_TIME_ZONE` (from `DeserializationFeature`) | `FAIL_ON_NUMBERS_FOR_ENUMS` (from `DeserializationFeature`) |
| `READ_DATE_TIMESTAMPS_AS_NANOSECONDS` (from `DeserializationFeature`) | `READ_ENUMS_USING_TO_STRING` (from `DeserializationFeature`, default now `true`) |
| `WRITE_DATES_AS_TIMESTAMPS` (from `SerializationFeature`, default now `false`) | `READ_UNKNOWN_ENUM_VALUES_AS_NULL` (from `DeserializationFeature`) |
| `WRITE_DATE_KEYS_AS_TIMESTAMPS` (from `SerializationFeature`) | `READ_UNKNOWN_ENUM_VALUES_USING_DEFAULT_VALUE` (from `DeserializationFeature`) |
| `WRITE_DATE_TIMESTAMPS_AS_NANOSECONDS` (from `SerializationFeature`) | `WRITE_ENUMS_USING_TO_STRING` (from `SerializationFeature`, default now `true`) |
| `WRITE_DATES_WITH_ZONE_ID` (from `SerializationFeature`) | `WRITE_ENUMS_USING_INDEX` (from `SerializationFeature`) |
| `WRITE_DATES_WITH_CONTEXT_TIME_ZONE` (from `SerializationFeature`) | `WRITE_ENUM_KEYS_USING_INDEX` (from `SerializationFeature`) |
| `WRITE_DURATIONS_AS_TIMESTAMPS` (from `SerializationFeature`) | |

```kotlin
// example
val mapper = JsonMapper.builder()
    .enable(DateTimeFeature.WRITE_DATES_WITH_ZONE_ID)
    .build()
```

## Diagnose Table (symptom → likely cause → fix)

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `cannot find symbol: class ObjectMapper` after import change | Import still points to `com.fasterxml.jackson.databind` | Update import to `tools.jackson.databind` |
| Config silently has no effect (e.g. unknown properties still fail) | Mutating `ObjectMapper` after construction | Move config into `.builder()` chain |
| `unreported exception JsonProcessingException; must be caught or declared` disappears, but `catch` blocks become unreachable/unused | Exceptions are now unchecked | Remove unnecessary catch/declare, or catch `JacksonException` if still needed |
| `NoSuchMethodError` / `ClassCastException` mixing Jackson 2 and 3 Kotlin module | `jackson-module-kotlin` still on old `com.fasterxml.jackson.module` group-id | Bump to `tools.jackson.module:jackson-module-kotlin:3.x` |
| Tests fail on trailing content in input that used to parse fine | `FAIL_ON_TRAILING_TOKENS` now enabled by default | Disable explicitly via builder if inputs are trusted, otherwise fix the input |
| Dates serialize as `"2026-07-30T13:50:00"` strings instead of epoch millis | `DateTimeFeature.WRITE_DATES_AS_TIMESTAMPS` now defaults to `false` (moved off `SerializationFeature`, was `true` in 2.x) | If numeric timestamps are still required (e.g. for API back-compat), explicitly `.enable(DateTimeFeature.WRITE_DATES_AS_TIMESTAMPS)` on the builder |
| JSON property order in output changed, breaking a string-equality test | `MapperFeature.SORT_PROPERTIES_ALPHABETICALLY` now enabled by default | Add `@JsonPropertyOrder`, disable the feature via builder, or (preferably) fix the test to not assert on raw string order |
| Deserialization behaves the same as before on unknown fields | `FAIL_ON_UNKNOWN_PROPERTIES` is unchanged (still enabled by default) — this is expected, not a bug | No action needed; don't "fix" this thinking it's a migration gap |
| Enum values (de)serialize differently (e.g. via `toString()` instead of name) | `READ_ENUMS_USING_TO_STRING`/`WRITE_ENUMS_USING_TO_STRING` now default to `true` | Disable explicitly via builder to restore 2.x `name()`-based behavior |
| Build fails needing Java 17 | Baseline raised from Java 8 | Bump JDK baseline before migrating Jackson |
| A field/property starting with æ, ø, or å silently disappears from JSON output/input, no exception thrown | Default accessor-naming validator rejects non-ASCII-letter first characters | Configure `.accessorNaming(DefaultAccessorNamingStrategy.Provider().withFirstCharAcceptance(true, true))` on the builder — applies to 2.x too, not Jackson-3-specific, but easy to lose when rebuilding config |
| Ktor JSON (de)serialization stops compiling/working after the Jackson bump | Still on `ktor-serialization-jackson` (Jackson 2 only) | Swap to `ktor-serialization-jackson3` and `import io.ktor.serialization.jackson3.*`, use the `jackson3 { ... }` DSL |
| `Unresolved reference` for something Jackson-shaped (e.g. `JacksonConverter`, `jacksonObjectMapper`, `jackson3`) even though the artifact/import looks correct | Framework version too old for that artifact, or internal version skew — e.g. a version catalog alias correctly names `ktor-serialization-jackson3`, but a hardcoded `val ktorVersion = "..."` elsewhere in the same build file is below the minimum that artifact requires (Ktor 3.4.0+ for `ktor-serialization-jackson3`; Spring Boot 4.0+/Framework 7.0+ for Jackson 3) | Don't hunt for the catalog file — it may be a published artifact with no local toml to grep. Run `./gradlew dependencyInsight --dependency <lib> --configuration compileClasspath` to see every requester and the actual resolved version, then align the outlier |
| Old `com.fasterxml.jackson.databind.*` imports (e.g. `JsonMappingException`, `ObjectMapper`) still compile fine after migration is claimed done, with no visible error | A not-yet-upgraded transitive dependency (test lib, HTTP client, internal module — not necessarily `jackson-module-kotlin` itself) still resolves `com.fasterxml.jackson.core:jackson-databind`/`jackson-core` onto the classpath alongside the new `tools.jackson.*` jars | Run `./gradlew dependencies \| grep jackson` (or Maven `dependency:tree`); identify the culprit and upgrade it, or `exclude(group = "com.fasterxml.jackson.core")` on it |
