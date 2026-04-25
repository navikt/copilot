---
applyTo: "**/*.go"
---

# Go on NAIS

Go-mønstre for Nav-tjenester på Nais-plattformen. Gjelder Go HTTP-tjenester, CLI-verktøy og plattformkomponenter.

> These patterns apply when building Go services for the NAIS platform. For non-NAIS Go code (scripts, libraries), use standard Go conventions.

## Standard Stack

```
Go 1.22+ + standard library (net/http)
+ pgx (database) + sqlc (type-safe SQL) + goose (migrations)
+ slog (structured logging) + Prometheus client (metrics)
+ viper (config, optional) + OpenTelemetry (emerging)
```

## Project Structure

```
├── cmd/
│   └── myservice/
│       └── main.go           # Entry point
├── internal/                  # Private application code
│   ├── api/                  # HTTP handlers
│   ├── database/             # pgx queries, sqlc generated code
│   ├── config/               # Configuration loading
│   └── [domain]/             # Business logic
├── pkg/                      # Reusable packages (if needed)
├── sql/
│   ├── migrations/           # goose SQL migrations
│   └── queries/              # sqlc query definitions
├── .nais/
│   ├── app.yaml              # NAIS manifest
│   ├── app-dev.yaml          # Dev overrides
│   └── app-prod.yaml         # Prod overrides
├── Dockerfile
├── sqlc.yaml
├── go.mod
└── Makefile
```

## HTTP Handlers (Standard Library)

```go
func main() {
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    mux := http.NewServeMux()

    // Application routes
    mux.HandleFunc("GET /api/v1/resources", handleListResources(db, log))
    mux.HandleFunc("GET /api/v1/resources/{id}", handleGetResource(db, log))
    mux.HandleFunc("POST /api/v1/resources", handleCreateResource(db, log))

    // NAIS health endpoints
    mux.HandleFunc("GET /isalive", func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    mux.HandleFunc("GET /isready", func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    mux.HandleFunc("GET /metrics", promhttp.Handler().ServeHTTP)

    server := &http.Server{
        Addr:              ":8080",
        Handler:           mux,
        ReadHeaderTimeout: 10 * time.Second,
    }

    log.Info("starting server", "port", 8080)
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Error("server failed", "error", err)
        os.Exit(1)
    }
}
```

### Handler Pattern

```go
// ✅ Handler as closure — accepts dependencies, returns http.HandlerFunc
func handleGetResource(db *database.Queries, log *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")

        resource, err := db.GetResource(r.Context(), id)
        if err != nil {
            if errors.Is(err, pgx.ErrNoRows) {
                http.Error(w, "not found", http.StatusNotFound)
                return
            }
            log.Error("database error", "error", err, "id", id)
            http.Error(w, "internal error", http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resource)
    }
}
```

## When Using Chi/Gin

For complex APIs with middleware chains, Chi or Gin are acceptable:

```go
// Chi — lightweight router
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)
r.Route("/api/v1", func(r chi.Router) {
    r.Get("/resources", handleList)
    r.Post("/resources", handleCreate)
})
```

## Database Access (pgx + sqlc)

### sqlc Configuration

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "sql/queries/"
    schema: "sql/migrations/"
    gen:
      go:
        package: "database"
        out: "internal/database"
        sql_package: "pgx/v5"
        emit_json_tags: true
```

### SQL Queries

```sql
-- sql/queries/resources.sql

-- name: GetResource :one
SELECT * FROM resources WHERE id = $1;

-- name: ListResources :many
SELECT * FROM resources ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CreateResource :one
INSERT INTO resources (name, description) VALUES ($1, $2) RETURNING *;

-- name: DeleteResource :exec
DELETE FROM resources WHERE id = $1;
```

### Connection Setup

```go
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, fmt.Errorf("parsing database config: %w", err)
    }

    config.MaxConns = 25
    config.MinConns = 2

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("creating connection pool: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("pinging database: %w", err)
    }

    return pool, nil
}
```

## Migrations (goose)

```go
import "github.com/pressly/goose/v3"

func runMigrations(ctx context.Context, db *sql.DB) error {
    goose.SetBaseFS(embedMigrations)
    return goose.UpContext(ctx, db, "sql/migrations")
}
```

## Structured Logging (slog)

```go
// ✅ JSON handler for production (NAIS log collection)
log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// ✅ Contextual logging
log.Info("processing request",
    "method", r.Method,
    "path", r.URL.Path,
    "request_id", r.Header.Get("X-Request-ID"),
)

// ✅ Error logging with context
log.Error("failed to fetch resource",
    "error", err,
    "resource_id", id,
)

// ❌ Don't use fmt.Printf or log.Println in production
```

## Error Handling

```go
// ✅ Wrap errors with context
if err != nil {
    return fmt.Errorf("fetching resource %s: %w", id, err)
}

// ✅ Sentinel errors for business logic
var ErrNotFound = errors.New("resource not found")
var ErrForbidden = errors.New("access denied")

// ✅ Check specific errors with errors.Is
if errors.Is(err, pgx.ErrNoRows) {
    return ErrNotFound
}

// ✅ Custom error types for rich error info
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation: %s — %s", e.Field, e.Message)
}

// ✅ Use errors.As for type assertion
var valErr ValidationError
if errors.As(err, &valErr) {
    http.Error(w, valErr.Message, http.StatusBadRequest)
    return
}

// ❌ Don't use %v for wrapping — loses error chain
return fmt.Errorf("error: %v", err)
```

## Naming Conventions

```go
// ✅ MixedCaps for identifiers; acronyms ALL CAPS
type HTTPClient struct { ... }    // not HttpClient
type URLValidator struct { ... }  // not UrlValidator

// ✅ Package names: lowercase, single word, no underscores
package database   // not package db_utils
package handlers   // not package httpHandlers

// ✅ Short receiver names (1-2 chars)
func (s *Service) Process(ctx context.Context) error { ... }
func (db *Database) Query(ctx context.Context) error { ... }

// ❌ Avoid generic package names
package util    // what utilities?
package common  // be specific
```

## Interface Patterns

```go
// ✅ Accept interfaces, return structs
type Store interface {
    GetResource(ctx context.Context, id string) (*Resource, error)
    SaveResource(ctx context.Context, r *Resource) error
}

// Constructor returns concrete struct
func NewService(store Store, log *slog.Logger) *Service {
    return &Service{store: store, log: log}
}

// ✅ Small interfaces — one or two methods
type Validator interface {
    Validate(ctx context.Context) error
}

// ✅ Interface-based test doubles
type mockStore struct {
    getResourceFn func(ctx context.Context, id string) (*Resource, error)
}

func (m *mockStore) GetResource(ctx context.Context, id string) (*Resource, error) {
    return m.getResourceFn(ctx, id)
}
```

## Concurrency

### errgroup for Parallel Work

```go
import "golang.org/x/sync/errgroup"

func LoadDashboard(ctx context.Context, id string) (*Dashboard, error) {
    g, ctx := errgroup.WithContext(ctx)

    var user *User
    var stats *Stats

    g.Go(func() error {
        var err error
        user, err = db.GetUser(ctx, id)
        return err
    })

    g.Go(func() error {
        var err error
        stats, err = db.GetStats(ctx, id)
        return err
    })

    if err := g.Wait(); err != nil {
        return nil, fmt.Errorf("loading dashboard: %w", err)
    }

    return &Dashboard{User: user, Stats: stats}, nil
}
```

### Graceful Shutdown

> **NAIS pod lifecycle:** When Kubernetes terminates a pod, the load balancer and pod are notified simultaneously. NAIS injects a `sleep 5` preStop hook before your app receives SIGTERM, so by the time SIGTERM arrives, the load balancer has already had 5+ seconds to drain traffic. Your app does **not** need to manipulate readiness probes during shutdown — just finish in-flight requests and exit. The default `terminationGracePeriodSeconds` (30s) gives you 25 seconds after SIGTERM.

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    server := &http.Server{Addr: ":8080", Handler: mux}

    go func() {
        if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Error("server failed", "error", err)
        }
    }()

    <-ctx.Done()
    log.Info("shutting down")

    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        log.Error("shutdown failed", "error", err)
    }
}
```

Common anti-patterns:
- ❌ Setting readiness to `false` on SIGTERM — unnecessary on NAIS, the load balancer already stopped routing
- ❌ `terminationGracePeriodSeconds: 5` — too short, gives 0 seconds after SIGTERM
- ✅ `server.Shutdown()` drains in-flight requests — this is all you need

### Context Propagation

```go
// ✅ Always pass context through call chains
func handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // inherit from request
    resource, err := service.Get(ctx, id)
    // ...
}

// ✅ Use context for cancellation in goroutines
go func() {
    select {
    case <-ctx.Done():
        return
    case result := <-ch:
        process(result)
    }
}()
```

## Configuration

```go
// ✅ Simple: environment variables directly
type Config struct {
    Port        int
    DatabaseURL string
    LogLevel    string
}

func LoadConfig() Config {
    return Config{
        Port:        getEnvInt("PORT", 8080),
        DatabaseURL: mustGetEnv("DATABASE_URL"),
        LogLevel:    getEnv("LOG_LEVEL", "INFO"),
    }
}

func mustGetEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        panic(fmt.Sprintf("required env var %s not set", key))
    }
    return v
}
```

## Observability

```go
import "github.com/prometheus/client_golang/prometheus"
import "github.com/prometheus/client_golang/prometheus/promauto"

var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )
)
```

## Docker (Chainguard)

```dockerfile
FROM cgr.dev/chainguard/go:latest AS builder
ENV GOOS=linux CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/app ./cmd/myservice

FROM cgr.dev/chainguard/static:latest
WORKDIR /app
COPY --from=builder /bin/app /app/app
ENTRYPOINT ["/app/app"]
```

## Testing

```go
// ✅ Table-driven tests
func TestHandleGetResource(t *testing.T) {
    tests := []struct {
        name       string
        id         string
        wantStatus int
    }{
        {"valid id", "abc-123", http.StatusOK},
        {"not found", "nonexistent", http.StatusNotFound},
        {"empty id", "", http.StatusBadRequest},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("GET", "/api/v1/resources/"+tt.id, nil)
            rec := httptest.NewRecorder()
            handler := handleGetResource(testDB, slog.Default())
            handler.ServeHTTP(rec, req)

            if rec.Code != tt.wantStatus {
                t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
            }
        })
    }
}

// ✅ Integration tests with testcontainers
func TestDatabase(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    ctx := context.Background()
    container, err := postgres.Run(ctx, "postgres:17",
        postgres.WithDatabase("testdb"),
    )
    if err != nil {
        t.Fatal(err)
    }
    defer container.Terminate(ctx)

    connStr, _ := container.ConnectionString(ctx, "sslmode=disable")
    pool, err := pgxpool.New(ctx, connStr)
    if err != nil {
        t.Fatal(err)
    }

    // Run migrations and test queries
}
```

## Boundaries

### ✅ Always
- Use `slog` with JSON handler for structured logging
- Wrap errors with `fmt.Errorf("context: %w", err)`
- Table-driven tests
- Health endpoints (`/isalive`, `/isready`, `/metrics`)
- Chainguard base images for Docker
- Preserve existing code structure when making targeted fixes — don't rename, restructure, or refactor working code beyond the task at hand

### ⚠️ Ask First
- Adding web frameworks (Chi, Gin) — stdlib preferred
- Adding ORM (GORM) — pgx+sqlc preferred
- Non-PostgreSQL databases

### 🚫 Never
- `fmt.Println` or `log.Println` in production code
- GORM for new projects (use pgx+sqlc)
- Full OS base images (Ubuntu, Alpine) in Docker
- Storing secrets in code or config files

## Related

| Resource | Use For |
|----------|---------|
| `@nais-agent` | Nais manifest, GCP resources, accessPolicy |
| `@observability-agent` | Prometheus metrics, Grafana dashboards |
| `security-owasp` instruction | OWASP Top 10:2025 patterns for Go |
| `@security-champion-agent` | Threat modeling and security architecture |
