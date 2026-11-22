# Build context is the repository root, so the seed files can be copied in.
FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY apps/fleet-control/go.mod apps/fleet-control/go.sum ./
RUN go mod download

COPY apps/fleet-control/*.go ./
COPY apps/fleet-control/migrations ./migrations

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o main .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/main .
# Seed input, read once into an empty database (#2334). Removed with the files in phase 6.
COPY apps/fleet-gateway/pilot-routes.dev.yaml apps/fleet-gateway/pilot-routes.prod.yaml apps/mcp-registry/allowlist.json ./seed/

EXPOSE 8080

CMD ["./main"]
