---
applyTo: "**/Dockerfile"
---

# Dockerfile Standards

Standarder for Dockerfile i Nav: Chainguard base images, multi-stage builds og sikkerhetspraksis.

Reference: [Chainguard base images — sikkerhet.nav.no](https://sikkerhet.nav.no/docs/verktoy/chainguard-dockerimages)

## Base Images — Chainguard

Nav pays for [Chainguard base images](https://sikkerhet.nav.no/docs/verktoy/chainguard-dockerimages) with minimal vulnerabilities. Use these instead of Google distroless or full OS images.

### Nav's private registry (JVM, Node, Python)

```
europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/<image>:<tag>
```

Available images: `jdk`, `jre`, `node`, `python`, `airflow-core`.

### Free Chainguard images (Go, nginx)

```
cgr.dev/chainguard/<image>:<tag>
```

For Go and nginx, good free alternatives exist in Chainguard's public registry.

### Tags and updates

- Use major version (e.g. `openjdk-21`, `22-slim`) — **Chainguard does not backport** to minor/patch
- Recommendation: don't pin SHA. Set up a workflow to rebuild regularly instead
- Use [digestabot](https://github.com/navikt/digestabot) if you want to pin SHA and get automatic PRs

```dockerfile
# ✅ Chainguard fra Navs registry
FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/jre:openjdk-21
FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/node:22-slim

# ✅ Free Chainguard for Go and nginx
FROM cgr.dev/chainguard/go:latest
FROM cgr.dev/chainguard/static:latest
FROM cgr.dev/chainguard/nginx:latest

# ⚠️ Google distroless works, but Chainguard is preferred at Nav
FROM gcr.io/distroless/java21-debian12:nonroot
FROM gcr.io/distroless/static-debian12:nonroot

# ❌ Avoid full OS images
FROM ubuntu:22.04
FROM openjdk:21
```

## Multi-Stage Builds

All Nav apps must use multi-stage builds for minimal image size.

### JVM applications (build outside Dockerfile)

```dockerfile
FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/jre:openjdk-21
ENV TZ="Europe/Oslo"
COPY target/app.jar app.jar
CMD ["-jar","app.jar"]
```

### JVM with build in Dockerfile (Kotlin/Java)

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

### Spring Boot

```dockerfile
FROM gradle:8-jdk21 AS build
WORKDIR /app
COPY . .
RUN gradle bootJar --no-daemon

FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/jre:openjdk-21
WORKDIR /app
COPY --from=build /app/build/libs/*.jar app.jar
ENTRYPOINT ["java", "-jar", "app.jar"]
```

### Go

```dockerfile
FROM cgr.dev/chainguard/go:latest AS builder
ENV GOOS=linux
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -a -installsuffix cgo -o /bin/app .

FROM cgr.dev/chainguard/static:latest
WORKDIR /app
COPY --from=builder /bin/app /app/app
ENTRYPOINT ["/app/app"]
```

### Node.js

```dockerfile
FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/node:22-slim
ENV NODE_ENV=production
ENV NPM_CONFIG_CACHE=/tmp
WORKDIR /app
COPY dist dist/
COPY server server/
EXPOSE 8080
CMD ["server/dist/index.js"]
```

### Node.js with build in Dockerfile

```dockerfile
FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/node:22-dev AS builder
WORKDIR /app
COPY . /app
RUN npm ci
RUN npm run build

FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/node:22-slim
WORKDIR /app
COPY --from=builder /app /app
CMD ["build/server.js"]
```

### Python with build in Dockerfile

```dockerfile
FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/python:3.12-dev AS builder
WORKDIR /app
RUN python3 -m venv venv
ENV PATH=/app/venv/bin:$PATH
COPY requirements.txt requirements.txt
RUN pip install -r requirements.txt

FROM europe-north1-docker.pkg.dev/cgr-nav/pull-through/nav.no/python:3.12 AS runner
WORKDIR /app
COPY src/ .
COPY --from=builder /app/venv /app/venv
ENV PATH="/app/venv/bin:$PATH"
ENTRYPOINT ["python", "main.py"]
```

### Nginx

```dockerfile
FROM cgr.dev/chainguard/node:latest-dev AS build
USER root
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM cgr.dev/chainguard/nginx AS production
COPY --from=build /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
```

## Security

```dockerfile
# ✅ Chainguard images run as non-root by default
# No USER instruction needed for Chainguard

# ✅ For other base images — run as non-root
USER nonroot                          # distroless
USER 1001                            # numerisk UID
RUN adduser --system --uid 1001 app  # Alpine

# ✅ Minimal COPY — never COPY entire context into final stage
COPY --from=build /app/build/libs/app.jar .

# ❌ Wrong — copies secrets, test files, .git
COPY . .
```

## .dockerignore

Always create a `.dockerignore`:

```
.git
.github
node_modules
.next
build
target
*.md
docker-compose*.yml
.env*
```

## Layer Caching

```dockerfile
# ✅ Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# ❌ Wrong — invalidates cache on any file change
COPY . .
RUN go mod download && go build
```

## CI — Chainguard Authentication

Use `nais/docker-build-push` in GitHub Actions — it handles authentication to Nav's Chainguard registry automatically:

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write
    steps:
      - uses: actions/checkout@v6
      - uses: nais/docker-build-push@v0
        id: docker-push
        with:
          team: <myteam>
```

## Boundaries

### ✅ Always

- Chainguard base images fra Navs registry (JVM/Node/Python) eller `cgr.dev` (Go/nginx)
- Multi-stage builds
- `.dockerignore`-fil
- Copy dependencies separately for layer caching
- `nais/docker-build-push` for CI

### ⚠️ Ask First

- Custom base images
- `--privileged` or extra Linux capabilities
- Mounting secrets in build
- Google distroless instead of Chainguard

### 🚫 Never

- `COPY . .` in final stage
- Root user in production
- Secrets in Dockerfile (`ENV SECRET=...`, `ARG PASSWORD=...`)
- `latest` tag on Nav registry images (use specific major version)
- Full OS images (`ubuntu`, `debian`, `openjdk`)
