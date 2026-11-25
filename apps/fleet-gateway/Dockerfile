FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

LABEL io.modelcontextprotocol.server.name="io.github.navikt/fleet-gateway"

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY *.go pilot-routes.dev.yaml pilot-routes.prod.yaml ./
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o main .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/pilot-routes.dev.yaml /app/pilot-routes.prod.yaml ./

EXPOSE 8080

CMD ["./main"]
