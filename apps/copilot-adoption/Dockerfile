FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY views/ views/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o copilot-adoption .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/copilot-adoption .

USER nonroot:nonroot

ENTRYPOINT ["/app/copilot-adoption"]
