FROM cgr.dev/chainguard/go:latest-dev AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o copilot-api .

# Final stage
FROM cgr.dev/chainguard/static:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/copilot-api .

EXPOSE 8080

ENTRYPOINT ["./copilot-api"]
