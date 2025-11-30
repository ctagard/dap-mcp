# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /dap-mcp ./cmd/dap-mcp

# Runtime stage
FROM alpine:3.19

# Install debug adapters and their dependencies
RUN apk add --no-cache \
    # Common utilities
    ca-certificates \
    tzdata \
    # Go debugger (Delve)
    go \
    # Python and debugpy
    python3 \
    py3-pip \
    # Node.js
    nodejs \
    npm

# Install Delve
RUN go install github.com/go-delve/delve/cmd/dlv@latest && \
    mv /root/go/bin/dlv /usr/local/bin/

# Install debugpy
RUN pip3 install --break-system-packages debugpy

# Create non-root user
RUN adduser -D -u 1000 dap-mcp
USER dap-mcp
WORKDIR /home/dap-mcp

# Copy the binary from builder
COPY --from=builder /dap-mcp /usr/local/bin/dap-mcp

# Set environment variables
ENV PATH="/usr/local/bin:${PATH}"

# Default command
ENTRYPOINT ["/usr/local/bin/dap-mcp"]
