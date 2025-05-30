# Multi-stage Dockerfile for GlobeCo Portfolio Accounting Service
# Stage 1: Build environment
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

# Docker automatic platform arguments
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG BUILDPLATFORM

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod and sum files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies with better caching
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy source code
COPY . .

# Build with cross-compilation and caching
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o bin/server ./cmd/server

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o bin/cli ./cmd/cli

# Stage 2: Production image
FROM scratch AS production

# Import CA certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Create non-root user structure
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy binaries from builder
COPY --from=builder /app/bin/server /usr/local/bin/server
COPY --from=builder /app/bin/cli /usr/local/bin/cli

# Copy configuration template
COPY --from=builder /app/config.yaml.example /etc/globeco/config.yaml

# Set up directory structure
USER nobody:nobody

# Expose port
EXPOSE 8087

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/server", "--health-check"] || exit 1

# Default command
CMD ["/usr/local/bin/server"]

# Stage 3: Development image with debugging tools
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS development

# Install development tools
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    curl \
    netcat-openbsd \
    postgresql-client \
    redis \
    bash \
    vim \
    htop

# Install Air for hot reloading
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/cosmtrek/air@latest

# Install delve for debugging
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@latest

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Create non-root user
RUN addgroup -g 1001 appuser && \
    adduser -D -s /bin/bash -u 1001 -G appuser appuser

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser

# Expose ports (app + delve)
EXPOSE 8087 2345

# Default command for development
CMD ["air", "-c", ".air.toml"]

# Stage 4: Testing image
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS testing

# Install test dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata

# Set working directory
WORKDIR /app

# Copy source code
COPY . .

# Download dependencies and run tests with caching
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && \
    go test -v ./...

# Stage 5: CLI-only image
FROM alpine:3.19 AS cli

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 appuser && \
    adduser -D -s /bin/sh -u 1001 -G appuser appuser

# Copy CLI binary from builder
COPY --from=builder /app/bin/cli /usr/local/bin/cli

# Copy configuration template
COPY --from=builder /app/config.yaml.example /etc/globeco/config.yaml

USER appuser

# Default command
ENTRYPOINT ["/usr/local/bin/cli"] 