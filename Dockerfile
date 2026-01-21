# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o watchdog ./cmd/watchdog

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata iputils && \
    adduser -D -u 1000 watchdog

WORKDIR /app

COPY --from=builder /build/watchdog .
COPY vms.yaml .

USER watchdog

CMD ["./watchdog"]
