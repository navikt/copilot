FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

LABEL io.modelcontextprotocol.server.name="io.github.navikt/mcp-kodeverk"

# Built from the repo root so the replace directive can reach lib/mcpkit.
WORKDIR /src
COPY lib/mcpkit/ lib/mcpkit/
COPY apps/mcp-kodeverk/go.mod apps/mcp-kodeverk/go.sum* apps/mcp-kodeverk/
WORKDIR /src/apps/mcp-kodeverk
RUN go mod download

COPY apps/mcp-kodeverk/*.go ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o main .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /src/apps/mcp-kodeverk/main .

EXPOSE 8080

CMD ["./main"]
