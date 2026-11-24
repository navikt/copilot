FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG APP_VERSION=dev

# Built from the repo root so the replace directive can reach lib/mcpkit.
WORKDIR /src
COPY lib/mcpkit/ lib/mcpkit/
COPY apps/mcp-onboarding/go.mod apps/mcp-onboarding/go.sum apps/mcp-onboarding/
WORKDIR /src/apps/mcp-onboarding
RUN go mod download

COPY apps/mcp-onboarding/*.go ./
COPY apps/mcp-onboarding/internal/ ./internal/
COPY apps/mcp-onboarding/cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION}" -o mcp-onboarding .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /src/apps/mcp-onboarding/mcp-onboarding .

EXPOSE 8080

ENTRYPOINT ["./mcp-onboarding"]
