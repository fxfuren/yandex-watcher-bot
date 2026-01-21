# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o watchdog ./cmd/watchdog

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/watchdog .

# Copy example config (will be overridden by volume mount)
COPY vms.yaml.example ./vms.yaml.example 2>/dev/null || :

# Run as non-root user
RUN addgroup -g 1000 watchdog && \
    adduser -D -u 1000 -G watchdog watchdog && \
    chown -R watchdog:watchdog /app

USER watchdog

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pgrep watchdog || exit 1

CMD ["./watchdog"]
