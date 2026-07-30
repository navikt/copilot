# Kotlin Cleanup — Full Reference

Detailed before/after snippets for Kotlin-specific Jackson 3 patterns beyond `jackson-module-kotlin` and the æ/ø/å gotcha covered in `SKILL.md`.

## Mutable `ObjectMapper` construction (very common in Kotlin)

```kotlin
// before — post-construction mutation, silently ignored/broken in 3.x
val mapper = ObjectMapper().apply {
    registerModule(JavaTimeModule())
    disable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
}

// after — builder pattern, everything set at construction time
val mapper = JsonMapper.builder()
    .disable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
    .build()
// java.time support (JavaTimeModule) is now built into jackson-databind — no explicit registration needed
```

## `setSerializationInclusion` / `serializationInclusion` (removed, not just renamed)

A very common config call that silently breaks under immutability — official replacement is `changeDefaultPropertyInclusion` (call twice, once per aspect, per the FasterXML migration guide):

```kotlin
// before
val mapper = ObjectMapper().apply {
    setSerializationInclusion(JsonInclude.Include.NON_NULL)
}

// after
val mapper = JsonMapper.builder()
    .changeDefaultPropertyInclusion { it.withValueInclusion(JsonInclude.Include.NON_NULL) }
    .changeDefaultPropertyInclusion { it.withContentInclusion(JsonInclude.Include.NON_NULL) }
    .build()
```

## Date format / time zone configuration

```kotlin
// before
mapper.setDateFormat(SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ssZ"))
mapper.setTimeZone(TimeZone.getDefault())

// after — note: builder default time zone is UTC, NOT the JVM default, if omitted
val mapper = JsonMapper.builder()
    .defaultDateFormat(SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ssZ"))
    .defaultTimeZone(TimeZone.getDefault())
    .build()
```

## Visibility configuration (e.g. field-only detection)

```kotlin
// before
mapper.disable(MapperFeature.AUTO_DETECT_FIELDS)

// after
val mapper = JsonMapper.builder()
    .changeDefaultVisibility { it.withFieldVisibility(JsonAutoDetect.Visibility.NONE) }
    .build()
```

## Polymorphic/default typing (`activateDefaultTypingAsProperty`)

These are **two different methods** — don't conflate them:

- `ObjectMapper.enableDefaultTyping()` (the blanket, validator-less variant): **removed entirely, no replacement.** It was dropped for security reasons (arbitrary type instantiation). If code calls this, the fix is to redesign using explicit, validated polymorphic typing below — not to find a drop-in replacement.
- `activateDefaultTypingAsProperty(...)` (the targeted, validator-based variant): **still exists**, but requires a real `PolymorphicTypeValidator` and builder-based construction, since `LaissezFaireSubTypeValidator` is no longer public:

```kotlin
// before
mapper.activateDefaultTypingAsProperty(
    LaissezFaireSubTypeValidator.instance,
    ObjectMapper.DefaultTyping.NON_CONCRETE_AND_ARRAYS,
    "@class"
)

// after
val typeValidator = BasicPolymorphicTypeValidator.builder()
    .allowIfSubType("no.nav.")
    .build()

val mapper = JsonMapper.builder()
    .activateDefaultTypingAsProperty(typeValidator, DefaultTyping.NON_CONCRETE_AND_ARRAYS, "@class")
    .build()
// DefaultTyping moved from ObjectMapper.DefaultTyping to tools.jackson.databind.DefaultTyping
```

## `JsonFactory`/`TokenStreamFactory` builder (also immutable)

Same immutability applies to the streaming factory, not just `ObjectMapper` — easy to miss since it's configured less often:

```kotlin
// before
val factory = JsonFactory()
factory.disable(JsonParser.Feature.AUTO_CLOSE_SOURCE)

// after
val factory = JsonFactory.builder()
    .disable(StreamReadFeature.AUTO_CLOSE_SOURCE)
    .build()
val mapper = JsonMapper.builder(factory).build()
```

If 2.x performance characteristics matter, also consider setting the recycler pool explicitly (3.0 defaults to a deque-based pool, differing from 2.x's `threadLocalPool()`):

```kotlin
val factory = JsonFactory.builder()
    .recyclerPool(JsonRecyclerPools.threadLocalPool())
    .build()
```

## `@JsonView` default configuration

`objectMapper.setConfig(...)` no longer works (immutable). Per-request `ObjectReader.withView()` / `ObjectWriter.withView()` are unchanged. For a mapper-level default view, `MapperBuilder.defaultSerializationView()`/`defaultDeserializationView()` are only available from **Jackson 3.1** — if pinned to 3.0.x, fall back to per-request views.

## Ktor content negotiation

If the service uses Ktor's `ContentNegotiation` plugin, swap the Jackson module together with the artifact — it is Ktor-specific, not just a Jackson dependency bump:

```kotlin
// before — Jackson 2, old artifact
implementation("io.ktor:ktor-serialization-jackson:$ktorVersion")
```
```kotlin
import io.ktor.serialization.jackson.*
install(ContentNegotiation) { jackson { /* ObjectMapper config */ } }
```

```kotlin
// after — Jackson 3, dedicated artifact + package
implementation("io.ktor:ktor-serialization-jackson3:$ktorVersion")
```
```kotlin
import io.ktor.serialization.jackson3.*
install(ContentNegotiation) { jackson3 { /* JsonMapper.Builder config */ } }
```

The `jackson3 { ... }` DSL configures a `JsonMapper.Builder`, not a mutable `ObjectMapper` — apply the same builder-based config patterns from this reference inside the block.
