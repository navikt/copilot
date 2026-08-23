# Kotlin Cleanup — Full Reference

Detailed before/after snippets for Kotlin-specific Jackson 3 patterns beyond `jackson-module-kotlin` and the æ/ø/å gotcha covered in `SKILL.md`.

## Finding every `jsonNode.map { }` call site (Jackson 3.1+ shadowing)

Background and the fix itself are in `SKILL.md`; this is the search procedure. Do this on every Kotlin module, before declaring the migration done — the compiler catches most, but not all, of these.

### 1. Source search

Only files that actually see a Jackson 3 `JsonNode` matter, so scope the search first:

```bash
# files using Jackson 3 that contain any .map call
grep -rl "^import tools\.jackson" --include=*.kt . \
  | xargs grep -lE "\.map\s*[{(]" > /tmp/candidates.txt

# receivers that are almost certainly JsonNode
xargs grep -nE '(\.path\([^()]*\)|\.get\([^()]*\)|\["[^"]*"\]|readTree\([^()]*\))\s*\.map\s*[({]|\.map\(JsonNode::' < /tmp/candidates.txt
```

Also check `JsonNode`-typed locals and parameters, which the receiver patterns above miss: for each candidate file, list the names declared as `: JsonNode` and grep for `<name>.map`. `JsonMessage`-style accessors (`packet["@behov"]`, `message["x"]`) return `JsonNode` too, even when the file never imports `JsonNode` itself — include them.

Ignore hits already written as `.values().map` (the fix), `.toList().map` (an equivalent older workaround — leave it, or normalise to `values()`), or where an intervening `filter`/`filterNot`/`flatMap` has already produced a `List`.

### 2. Bytecode check (catches the silently-compiling ones)

Source review can't tell a shadowed call from a deliberate one, and the silent cases produce no compiler output at all. After a build, look for actual invocations of the member — in migrated Kotlin code there should normally be **zero**:

```bash
find . -path "*build/classes/kotlin*" -name "*.class" > /tmp/classes.txt
xargs -P 8 -n 200 javap -p -c < /tmp/classes.txt 2>/dev/null \
  | grep -E 'tools/jackson/databind/(node/)?[A-Za-z]*Node\.map:'
```

**Don't narrow this to `JsonNode.map`.** The call site records the *static* type of the receiver, so a receiver declared `ArrayNode` or `ObjectNode` compiles to `.../node/ArrayNode.map:` and a `JsonNode`-only grep reports a false clean. The pattern above covers all of them; the distinctive descriptor is `(Ljava/util/function/Function;)Ljava/lang/Object;` if you want to confirm a hit is really this method.

Every hit is either a shadowed `Iterable.map` that changed meaning, or an intentional single-node transform — inspect each. Sanity-check the pipeline first by grepping for a method you know is used (e.g. `Node\.path:`); an empty result from a stale or missing build directory is not evidence of a clean codebase, so confirm the build is up to date.

### 3. Regression test

The silent cases are behavioral, not structural — add or keep a test asserting that mapping over an array node yields *all* elements (`assertEquals(listOf("a", "b"), node.path("arr").values().map(JsonNode::asString))`). A test that only checks "doesn't throw" passes happily with the shadowed member.

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

// if the Kotlin module is needed too, prefer jacksonMapperBuilder() over
// JsonMapper.builder().addModule(kotlinModule()) — same builder, kotlin module pre-added:
import tools.jackson.module.kotlin.jacksonMapperBuilder
val mapper = jacksonMapperBuilder()
    .disable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
    .build()
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
