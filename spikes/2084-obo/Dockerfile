FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o main .
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/main /main
CMD ["/main"]
