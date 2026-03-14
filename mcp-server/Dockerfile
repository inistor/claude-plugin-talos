FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY *.go ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o talos-mcp .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/talos-mcp /usr/local/bin/talos-mcp
ENTRYPOINT ["talos-mcp"]
