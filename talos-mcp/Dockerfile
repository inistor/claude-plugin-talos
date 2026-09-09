FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o talos-mcp .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/talos-mcp /usr/local/bin/talos-mcp

# Default destination for written artifacts (etcd snapshots). The container is
# ephemeral, so this must be bind-mounted from the host to survive; the
# talos_etcd_snapshot handler refuses to write to an unmounted path.
RUN mkdir -p /out
ENTRYPOINT ["talos-mcp"]
