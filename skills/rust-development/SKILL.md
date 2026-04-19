---
name: rust-development
description: Idiomatisk Rust-utvikling med cargo, clippy, error handling, async/tokio, unsafe og testing
license: MIT
compatibility: Rust project with Cargo
metadata:
  domain: backend
  tags: rust cargo clippy async tokio
---

# Rust Development Skill

Patterns, templates, and procedures for building high-quality Rust applications and libraries. Based on [Microsoft Pragmatic Rust Guidelines](https://microsoft.github.io/rust-guidelines/) and [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/checklist.html).

## When to Use

- Setting up a new Rust project or workspace
- Configuring `Cargo.toml` with recommended lints and dependencies
- Implementing error handling (library vs application)
- Writing async code with tokio
- Reviewing or writing `unsafe` code
- Optimizing performance with benchmarks
- Structuring a crate for public API quality

## Procedure: New Project Setup

### 1. Initialize

```bash
cargo init <project-name>
cd <project-name>
```

### 2. Configure Cargo.toml

```toml
[package]
name = "my-service"
version = "0.1.0"
edition = "2024"
rust-version = "1.85"

[dependencies]
anyhow = "1"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
tokio = { version = "1", features = ["full"] }
tracing = "0.1"
tracing-subscriber = { version = "0.3", features = ["env-filter", "json"] }

[dev-dependencies]
proptest = "1"

[lints.rust]
missing_debug_implementations = "warn"
redundant_imports = "warn"
trivial_numeric_casts = "warn"
unsafe_op_in_unsafe_fn = "warn"
unused_lifetimes = "warn"

[lints.clippy]
cargo = { level = "warn", priority = -1 }
complexity = { level = "warn", priority = -1 }
correctness = { level = "warn", priority = -1 }
pedantic = { level = "warn", priority = -1 }
perf = { level = "warn", priority = -1 }
style = { level = "warn", priority = -1 }
suspicious = { level = "warn", priority = -1 }
undocumented_unsafe_blocks = "warn"
clone_on_ref_ptr = "warn"
map_err_ignore = "warn"
```

### 3. Configure rustfmt

Create `rustfmt.toml`:

```toml
edition = "2024"
max_width = 100
use_field_init_shorthand = true
```

### 4. Set up CI checks

Minimum verification pipeline:

```bash
cargo fmt --check
cargo clippy -- -W clippy::pedantic
cargo test
cargo doc --no-deps
cargo audit
```

## Procedure: Error Handling

### Library Errors (thiserror)

Use struct-based errors with backtrace support per Microsoft guidelines:

```rust
use std::backtrace::Backtrace;

#[derive(Debug)]
pub struct ParseError {
    kind: ParseErrorKind,
    backtrace: Backtrace,
}

#[derive(Debug, thiserror::Error)]
enum ParseErrorKind {
    #[error("invalid header: {0}")]
    InvalidHeader(String),
    #[error("missing field: {field}")]
    MissingField { field: &'static str },
    #[error(transparent)]
    Io(#[from] std::io::Error),
}

impl std::fmt::Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        self.kind.fmt(f)
    }
}

impl std::error::Error for ParseError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        self.kind.source()
    }
}
```

### Application Errors (anyhow)

```rust
use anyhow::{Context, Result};

fn load_config(path: &str) -> Result<Config> {
    let content = std::fs::read_to_string(path)
        .context("failed to read config file")?;
    let config: Config = serde_json::from_str(&content)
        .context("failed to parse config")?;
    Ok(config)
}
```

**Rule**: Libraries expose structured errors. Applications use `anyhow::Result`.

## Procedure: Async Application (tokio + axum)

### Minimal HTTP service

```rust
use axum::{Router, routing::get};
use tokio::net::TcpListener;
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env())
        .json()
        .init();

    let app = Router::new()
        .route("/isalive", get(|| async { "Alive" }))
        .route("/isready", get(|| async { "Ready" }));

    let listener = TcpListener::bind("0.0.0.0:8080").await?;
    tracing::info!("listening on {}", listener.local_addr()?);
    axum::serve(listener, app).await?;
    Ok(())
}
```

### Graceful shutdown

```rust
use tokio::signal;

async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c().await.expect("failed to listen for ctrl+c");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install SIGTERM handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = ctrl_c => {},
        () = terminate => {},
    }
}

// Usage:
axum::serve(listener, app)
    .with_graceful_shutdown(shutdown_signal())
    .await?;
```

### Shared state

```rust
use std::sync::Arc;
use tokio::sync::RwLock;

#[derive(Clone)]
struct AppState {
    db: sqlx::PgPool,
    cache: Arc<RwLock<HashMap<String, CachedItem>>>,
}

let state = AppState {
    db: sqlx::PgPool::connect(&database_url).await?,
    cache: Arc::new(RwLock::new(HashMap::new())),
};

let app = Router::new()
    .route("/items", get(list_items))
    .with_state(state);
```

## Procedure: Unsafe Code Review

When writing or reviewing `unsafe`:

1. **Justify** — is it novel abstractions, performance (with benchmarks), or FFI?
2. **Document** — every `unsafe` block has a `// SAFETY:` comment
3. **Minimize** — smallest possible `unsafe` scope
4. **Test** — run `cargo +nightly miri test` for UB detection
5. **Verify** — no aliasing violations, no data races, all invariants upheld

```rust
// ✅ Correct — documented and minimal
// SAFETY: We verified the pointer is non-null and properly aligned
// in the constructor. The data outlives this reference because
// it is held by the owning Arc<T>.
unsafe { &*self.ptr }

// ❌ Wrong — undocumented, overly broad
unsafe {
    // lots of code here making it impossible to audit
}
```

## Procedure: Performance Optimization

### 1. Use mimalloc as global allocator

```rust
use mimalloc::MiMalloc;

#[global_allocator]
static GLOBAL: MiMalloc = MiMalloc;
```

### 2. Benchmark with criterion

```rust
// benches/parse.rs
use criterion::{criterion_group, criterion_main, Criterion};

fn bench_parsing(c: &mut Criterion) {
    let input = include_str!("../fixtures/large_input.json");
    c.bench_function("parse_json", |b| {
        b.iter(|| parse(criterion::black_box(input)));
    });
}

criterion_group!(benches, bench_parsing);
criterion_main!(benches);
```

Enable debug symbols for flamegraphs:

```toml
[profile.bench]
debug = 1
```

### 3. Avoid common performance pitfalls

```rust
// ✅ Pre-allocate with capacity
let mut buffer = Vec::with_capacity(estimated_size);

// ✅ Use write! instead of format! on hot paths
use std::fmt::Write;
let mut output = String::new();
write!(output, "count: {count}")?;

// ✅ Cow for conditional ownership
fn process(input: &str) -> Cow<'_, str> {
    if needs_transform(input) {
        Cow::Owned(transform(input))
    } else {
        Cow::Borrowed(input)
    }
}
```

## Procedure: Structured Logging

```rust
use tracing::{info, warn, instrument};

#[instrument(skip(db), fields(user_id = %user_id))]
async fn process_request(user_id: &str, db: &PgPool) -> Result<()> {
    info!(event = "request.processing.started");

    let result = db.fetch_one(query).await?;

    info!(
        event = "request.processing.completed",
        rows_affected = result.rows_affected(),
    );

    Ok(())
}
```

Naming: `<component>.<operation>.<state>` — e.g., `db.query.completed`, `http.request.failed`.

## Procedure: Testing Patterns

### Unit tests (in-module)

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_valid_input() {
        let result = parse("valid input");
        assert_eq!(result.unwrap(), expected);
    }
}
```

### Table-driven tests

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn conversion_cases() {
        let cases = [
            ("input1", "expected1"),
            ("input2", "expected2"),
            ("input3", "expected3"),
        ];

        for (input, expected) in cases {
            assert_eq!(convert(input), expected, "failed for input: {input}");
        }
    }
}
```

### Async tests (tokio)

```rust
#[tokio::test]
async fn fetches_user() {
    let pool = setup_test_db().await;
    let user = get_user(&pool, "test-id").await.unwrap();
    assert_eq!(user.name, "Test User");
}
```

### Property-based tests (proptest)

```rust
use proptest::prelude::*;

proptest! {
    #[test]
    fn roundtrip_serialization(input in "\\PC*") {
        let serialized = serde_json::to_string(&input).unwrap();
        let deserialized: String = serde_json::from_str(&serialized).unwrap();
        prop_assert_eq!(input, deserialized);
    }
}
```

## API Design Checklist

Based on [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/checklist.html):

- [ ] Eagerly implement `Debug`, `Clone`, `Default`, `Eq`, `Hash` where applicable
- [ ] Public enums and structs use `#[non_exhaustive]`
- [ ] Functions accept borrowed params (`&str`, `impl AsRef<Path>`) not owned
- [ ] Getters: `fn name(&self)` not `fn get_name(&self)`
- [ ] Conversions: `as_` (cheap ref), `to_` (expensive), `into_` (consuming)
- [ ] Constructors: `new` (default), `with_*` (configured), `from_*` (conversion)
- [ ] Builder pattern for 4+ optional parameters
- [ ] Newtype wrappers for domain types (`struct UserId(u64)`)
- [ ] No `Arc`/`Box` in public signatures unless necessary
- [ ] Iterators implement `Iterator`, `DoubleEndedIterator`, `ExactSizeIterator` where possible

## Documentation Standards

```rust
/// Short one-sentence summary. (imperative mood)
///
/// Detailed explanation if needed. Include examples for
/// non-trivial public items.
///
/// # Errors
///
/// Returns `ParseError` if the input is not valid UTF-8.
///
/// # Panics
///
/// Panics if `index` is out of bounds.
///
/// # Examples
///
/// ```
/// use my_crate::parse;
/// let result = parse("hello")?;
/// assert_eq!(result.len(), 5);
/// # Ok::<(), my_crate::ParseError>(())
/// ```
pub fn parse(input: &str) -> Result<Parsed, ParseError> {
    // ...
}
```

Required doc sections for public items: summary, `# Errors`, `# Panics`, `# Examples`.
Module docs: `//!` at the top of `lib.rs` and significant modules.

## References

- [Microsoft Pragmatic Rust Guidelines](https://microsoft.github.io/rust-guidelines/)
- [Rust API Guidelines Checklist](https://rust-lang.github.io/api-guidelines/checklist.html)
- [Rust Design Patterns](https://rust-unofficial.github.io/patterns/)
- [The Cargo Book](https://doc.rust-lang.org/cargo/)
