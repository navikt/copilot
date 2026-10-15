FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG APP_VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION}" -o mcp-onboarding .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/mcp-onboarding .

EXPOSE 8080

ENTRYPOINT ["./mcp-onboarding"]
