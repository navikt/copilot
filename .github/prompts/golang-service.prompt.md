---
name: golang-service
description: Scaffold ein Go HTTP-teneste med NAIS-mønster, pgx, sqlc og slog
---

# Go NAIS Service

Scaffold a Go HTTP service for the NAIS platform.

## Input

Describe what the service does, what resources it manages, and any external integrations.

## Output

Generate a Go service with:

1. **`cmd/<service>/main.go`** — Entry point with HTTP server, health endpoints, graceful shutdown
2. **`internal/api/handlers.go`** — HTTP handlers as closures accepting dependencies
3. **`internal/database/`** — pgx connection pool setup
4. **`sql/queries/*.sql`** — sqlc query definitions
5. **`sql/migrations/001_initial.sql`** — goose migration
6. **`sqlc.yaml`** — sqlc configuration
7. **`.nais/app.yaml`** — NAIS manifest with PostgreSQL
8. **`Dockerfile`** — Multi-stage with Chainguard base images

## Patterns

### main.go

```go
package main

import (
    "context"
    "errors"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    cfg := loadConfig()

    pool, err := newPool(context.Background(), cfg.DatabaseURL)
    if err != nil {
        log.Error("connecting to database", "error", err)
        os.Exit(1)
    }
    defer pool.Close()

    mux := http.NewServeMux()

    // Health
    mux.HandleFunc("GET /isalive", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
    mux.HandleFunc("GET /isready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
    mux.Handle("GET /metrics", promhttp.Handler())

    // Application routes
    // mux.HandleFunc("GET /api/v1/...", handleList(pool, log))

    srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    go func() {
        log.Info("starting server", "port", cfg.Port)
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Error("server error", "error", err)
            os.Exit(1)
        }
    }()

    <-ctx.Done()
    log.Info("shutting down")
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    srv.Shutdown(shutdownCtx)
}
```

### NAIS Manifest

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: {{service-name}}
  namespace: {{team}}
  labels:
    team: {{team}}
spec:
  image: {{image}}
  port: 8080
  liveness:
    path: /isalive
  readiness:
    path: /isready
  prometheus:
    enabled: true
    path: /metrics
  gcp:
    sqlInstances:
      - type: POSTGRES_17
        databases:
          - name: {{service-name}}
  accessPolicy:
    outbound:
      external: []
```

### Dockerfile

```dockerfile
FROM cgr.dev/chainguard/go:latest AS builder
ENV GOOS=linux CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/app ./cmd/{{service-name}}

FROM cgr.dev/chainguard/static:latest
COPY --from=builder /bin/app /app/app
ENTRYPOINT ["/app/app"]
```

## Forstå koden

After generating the service, explain:

1. **Handler-som-closure** — Why handlers are functions returning `http.HandlerFunc` instead of methods on a struct. What does this pattern give you for testing and dependency injection?
2. **Graceful shutdown** — Why `signal.NotifyContext` + `server.Shutdown()` instead of just `os.Exit`. What happens to in-flight requests during a Kubernetes rolling update without graceful shutdown?
3. **ReadHeaderTimeout** — Why it's set and what attack it prevents (slowloris). Why not set `ReadTimeout` or `WriteTimeout` as well?
4. **CGO_ENABLED=0** — Why this is needed for the Chainguard static base image. What would happen without it?

🔴 **Rød sone**: Graceful shutdown and error handling are critical for production reliability — understand the shutdown sequence before deploying to Nais.

Still gjerne spørsmål om valgene over.
