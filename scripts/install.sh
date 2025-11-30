#!/bin/bash
# DAP-MCP Installation Script
# Usage: curl -sSL https://raw.githubusercontent.com/ctagard/dap-mcp/main/scripts/install.sh | bash

set -e

REPO="ctagard/dap-mcp"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="dap-mcp"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $ARCH in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case $OS in
        linux|darwin)
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            log_error "Unsupported OS: $OS"
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    log_info "Detected platform: $PLATFORM"
}

# Get the latest version from GitHub
get_latest_version() {
    VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        log_error "Failed to get latest version"
        exit 1
    fi
    log_info "Latest version: v$VERSION"
}

# Download and install the binary
install_binary() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY_NAME}_${VERSION}_${PLATFORM}.tar.gz"

    log_info "Downloading from: $DOWNLOAD_URL"

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/dap-mcp.tar.gz"

    tar -xzf "$TMP_DIR/dap-mcp.tar.gz" -C "$TMP_DIR"

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        log_warn "Need sudo to install to $INSTALL_DIR"
        sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi

    log_info "Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"
}

# Verify installation
verify_installation() {
    if command -v $BINARY_NAME &> /dev/null; then
        log_info "Installation successful!"
        echo ""
        $BINARY_NAME --version 2>/dev/null || $BINARY_NAME --help | head -1
    else
        log_warn "$BINARY_NAME installed but not in PATH"
        log_info "Add $INSTALL_DIR to your PATH or run: $INSTALL_DIR/$BINARY_NAME"
    fi
}

# Print next steps
print_next_steps() {
    echo ""
    log_info "Next steps:"
    echo ""
    echo "1. Install debug adapters for languages you want to debug:"
    echo "   Go:     go install github.com/go-delve/delve/cmd/dlv@latest"
    echo "   Python: pip install debugpy"
    echo ""
    echo "2. Configure your AI client (Claude Code example):"
    echo "   Add to ~/.claude.json:"
    echo '   {'
    echo '     "mcpServers": {'
    echo '       "dap-mcp": {'
    echo "         \"command\": \"$INSTALL_DIR/$BINARY_NAME\","
    echo '         "args": ["--mode", "full"]'
    echo '       }'
    echo '     }'
    echo '   }'
    echo ""
    echo "3. See full documentation: https://github.com/${REPO}"
}

main() {
    echo ""
    echo "======================================"
    echo "  DAP-MCP Installer"
    echo "======================================"
    echo ""

    detect_platform
    get_latest_version
    install_binary
    verify_installation
    print_next_steps
}

main "$@"
