FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

LABEL io.modelcontextprotocol.server.name="io.github.navikt/mcp-team-catalog"

# Built from the repo root so the replace directive can reach lib/mcpkit.
WORKDIR /src
COPY lib/mcpkit/ lib/mcpkit/
COPY apps/mcp-team-catalog/go.mod apps/mcp-team-catalog/go.sum* apps/mcp-team-catalog/
WORKDIR /src/apps/mcp-team-catalog
RUN go mod download

COPY apps/mcp-team-catalog/*.go ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o main .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /src/apps/mcp-team-catalog/main .

EXPOSE 8080

CMD ["./main"]
