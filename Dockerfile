# Build stage for Delve (needs Go 1.23+)
FROM golang:1.23-alpine AS delve-builder
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Runtime image - uses pre-built binary from goreleaser
FROM alpine:3.19

# Install debug adapters and their dependencies
RUN apk add --no-cache \
    # Common utilities
    ca-certificates \
    tzdata \
    # Python and debugpy
    python3 \
    py3-pip \
    # Node.js
    nodejs \
    npm

# Copy Delve from builder stage
COPY --from=delve-builder /go/bin/dlv /usr/local/bin/dlv

# Install debugpy
RUN pip3 install --break-system-packages debugpy

# Create non-root user
RUN adduser -D -u 1000 dap-mcp
USER dap-mcp
WORKDIR /home/dap-mcp

# Copy the pre-built binary from goreleaser
COPY dap-mcp /usr/local/bin/dap-mcp

# Set environment variables
ENV PATH="/usr/local/bin:${PATH}"

# Default command
ENTRYPOINT ["/usr/local/bin/dap-mcp"]
