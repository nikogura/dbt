# Build stage - pin to host architecture for native compilation.
# Go cross-compiles natively via GOOS/GOARCH — no QEMU needed.
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the reposerver binary - static, no CGO.
# TARGETOS and TARGETARCH are set automatically by buildx --platform,
# or can be passed explicitly via --build-arg for non-buildx tools (e.g. Kaniko).
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /bin/reposerver ./cmd/reposerver

# Final stage - distroless
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary from builder
COPY --from=builder /bin/reposerver /reposerver

# Expose the default port
EXPOSE 9999

# Run as non-root user (distroless nonroot user is uid 65532)
USER nonroot:nonroot

# Default command with flags for container environment
# Address 0.0.0.0 is required for Docker networking
# Config file and server root should be mounted at runtime
ENTRYPOINT ["/reposerver"]
CMD ["-a", "0.0.0.0", "-p", "9999", "-r", "/var/dbt"]
