FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o talos-mcp .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/talos-mcp /usr/local/bin/talos-mcp
ENTRYPOINT ["talos-mcp"]
