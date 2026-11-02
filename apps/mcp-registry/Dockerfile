FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

LABEL io.modelcontextprotocol.server.name="io.github.navikt/mcp-registry"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY migrations ./migrations
COPY allowlist.json ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o main .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/allowlist.json .

EXPOSE 8080

CMD ["./main"]