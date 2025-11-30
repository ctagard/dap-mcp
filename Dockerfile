# Runtime image - uses pre-built binary from goreleaser
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

# Copy the pre-built binary from goreleaser
COPY dap-mcp /usr/local/bin/dap-mcp

# Set environment variables
ENV PATH="/usr/local/bin:${PATH}"

# Default command
ENTRYPOINT ["/usr/local/bin/dap-mcp"]
