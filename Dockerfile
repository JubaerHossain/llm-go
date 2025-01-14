# Stage 1: Build the application
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates
RUN apk update && apk add --no-cache git ca-certificates tzdata

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=$(git describe --tags --always)" \
    -o llm-api

# Stage 2: Create a minimal runtime image
FROM alpine:latest

# Install essential tools
RUN apk --no-cache add ca-certificates tzdata curl

# Set working directory
WORKDIR /app

# Create config directory
RUN mkdir -p /app/config

# Copy the binary
COPY --from=builder /app/llm-api /app/

# Optional: If you have any configuration files, uncomment and adjust
# COPY --from=builder /app/config/ /app/config/

# Set environment variables with secure defaults
ENV APP_ENV=production \
    GIN_MODE=release \
    OLLAMA_MODEL=llama3 \
    OLLAMA_HOST=ollama \
    API_PORT=8080 \
    MAX_REQUESTS=50 \
    RATE_LIMIT_PERIOD=60s \
    LOG_LEVEL=info

# Expose application port
EXPOSE 8080

# Create a non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s \
  CMD curl -f http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/llm-api"]