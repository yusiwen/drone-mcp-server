# Multi-stage Docker build for drone-mcp-server
# Build stage
FROM golang:1.25.1-alpine AS builder

# Build arguments
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_DATE=unknown

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# -ldflags="-s -w" strips debug symbols to reduce binary size
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
        -X main.buildVersion=${BUILD_VERSION} \
        -X main.buildCommit=${BUILD_COMMIT} \
        -X main.buildDate=${BUILD_DATE}" \
    -o drone-mcp-server .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=appuser:appgroup /app/drone-mcp-server .

# Copy configuration files if any
# COPY --from=builder --chown=appuser:appgroup /app/config.yaml ./config.yaml

# Switch to non-root user
USER appuser

# Expose port (if using SSE mode)
EXPOSE 8080

# Set environment variables
ENV DRONE_SERVER=""
ENV DRONE_TOKEN=""
ENV MCP_AUTH_TOKEN=""

# Health check (adjust based on your application)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Entrypoint
ENTRYPOINT ["./drone-mcp-server"]

# Default command (stdio mode)
CMD []

# To run in SSE mode with custom parameters:
# docker run -e DRONE_SERVER=... -e DRONE_TOKEN=... -p 8080:8080 drone-mcp-server --sse --host 0.0.0.0