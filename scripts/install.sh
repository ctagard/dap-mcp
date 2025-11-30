#!/bin/bash
# DAP-MCP Installation Script
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/ctagard/dap-mcp/main/scripts/install.sh | bash
#
# Options (via environment variables):
#   INSTALL_DIR     - Installation directory (default: /usr/local/bin)
#   INSTALL_VERSION - Specific version to install (default: latest)
#   INSTALL_METHOD  - Force method: binary, brew, deb, rpm (default: auto-detect)
#
# Examples:
#   # Install latest to default location
#   curl -sSL https://raw.githubusercontent.com/ctagard/dap-mcp/main/scripts/install.sh | bash
#
#   # Install specific version
#   curl -sSL ... | INSTALL_VERSION=0.1.1 bash
#
#   # Install to custom directory
#   curl -sSL ... | INSTALL_DIR=$HOME/.local/bin bash
#
#   # Force binary install (skip package manager)
#   curl -sSL ... | INSTALL_METHOD=binary bash

set -e

REPO="ctagard/dap-mcp"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="dap-mcp"
GITHUB_URL="https://github.com/${REPO}"
RAW_URL="https://raw.githubusercontent.com/${REPO}/main"

# Colors for output (disabled if not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    NC=''
fi

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Check for required commands
check_requirements() {
    local missing=""

    if ! command -v curl &> /dev/null; then
        missing="$missing curl"
    fi

    if ! command -v tar &> /dev/null; then
        missing="$missing tar"
    fi

    if [ -n "$missing" ]; then
        log_error "Missing required commands:$missing"
        log_info "Please install them and try again"
        exit 1
    fi
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armv6l)
            log_error "ARM 32-bit is not supported. Please use a 64-bit system."
            exit 1
            ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case $OS in
        linux)
            # Detect Linux distribution
            if [ -f /etc/os-release ]; then
                . /etc/os-release
                DISTRO="$ID"
                DISTRO_FAMILY="$ID_LIKE"
            elif [ -f /etc/debian_version ]; then
                DISTRO="debian"
            elif [ -f /etc/redhat-release ]; then
                DISTRO="rhel"
            else
                DISTRO="unknown"
            fi
            ;;
        darwin)
            DISTRO="macos"
            # Check for Homebrew
            if command -v brew &> /dev/null; then
                HAS_BREW=true
            fi
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            DISTRO="windows"
            log_warn "Windows detected via Git Bash/MSYS"
            log_info "For native Windows, download from: ${GITHUB_URL}/releases"
            ;;
        *)
            log_error "Unsupported OS: $OS"
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    log_info "Detected: $OS ($ARCH)"

    if [ "$DISTRO" != "unknown" ] && [ "$DISTRO" != "$OS" ]; then
        log_info "Distribution: $DISTRO"
    fi
}

# Get the latest version from GitHub
get_latest_version() {
    if [ -n "$INSTALL_VERSION" ]; then
        VERSION="$INSTALL_VERSION"
        log_info "Using specified version: v$VERSION"
    else
        log_step "Fetching latest version..."
        VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            log_error "Failed to get latest version from GitHub API"
            log_info "You can specify a version manually: INSTALL_VERSION=0.1.1 bash install.sh"
            exit 1
        fi
        log_info "Latest version: v$VERSION"
    fi
}

# Check if already installed
check_existing() {
    if command -v $BINARY_NAME &> /dev/null; then
        EXISTING_VERSION=$($BINARY_NAME --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
        if [ "$EXISTING_VERSION" = "$VERSION" ]; then
            log_info "$BINARY_NAME v$VERSION is already installed"
            echo ""
            read -p "Reinstall? [y/N] " -n 1 -r REPLY </dev/tty || REPLY="y"
            echo ""
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                log_info "Installation cancelled"
                exit 0
            fi
        else
            log_info "Upgrading $BINARY_NAME: v$EXISTING_VERSION -> v$VERSION"
        fi
    fi
}

# Install via Homebrew (macOS/Linux)
install_brew() {
    log_step "Installing via Homebrew..."

    # Add tap if needed
    if ! brew tap | grep -q "ctagard/tap"; then
        brew tap ctagard/tap
    fi

    brew install ctagard/tap/dap-mcp

    log_info "Installed via Homebrew"
}

# Install via .deb package (Debian/Ubuntu)
install_deb() {
    log_step "Installing via .deb package..."

    DOWNLOAD_URL="${GITHUB_URL}/releases/download/v${VERSION}/${BINARY_NAME}_${VERSION}_linux_${ARCH}.deb"
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    log_info "Downloading: $DOWNLOAD_URL"
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/dap-mcp.deb"; then
        log_error "Failed to download .deb package"
        return 1
    fi

    log_info "Installing package..."
    if [ -w /usr/bin ]; then
        dpkg -i "$TMP_DIR/dap-mcp.deb"
    else
        sudo dpkg -i "$TMP_DIR/dap-mcp.deb"
    fi

    log_info "Installed via .deb package"
}

# Install via .rpm package (Fedora/RHEL/CentOS)
install_rpm() {
    log_step "Installing via .rpm package..."

    DOWNLOAD_URL="${GITHUB_URL}/releases/download/v${VERSION}/${BINARY_NAME}_${VERSION}_linux_${ARCH}.rpm"
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    log_info "Downloading: $DOWNLOAD_URL"
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/dap-mcp.rpm"; then
        log_error "Failed to download .rpm package"
        return 1
    fi

    log_info "Installing package..."
    if command -v dnf &> /dev/null; then
        if [ -w /usr/bin ]; then
            dnf install -y "$TMP_DIR/dap-mcp.rpm"
        else
            sudo dnf install -y "$TMP_DIR/dap-mcp.rpm"
        fi
    else
        if [ -w /usr/bin ]; then
            rpm -i "$TMP_DIR/dap-mcp.rpm"
        else
            sudo rpm -i "$TMP_DIR/dap-mcp.rpm"
        fi
    fi

    log_info "Installed via .rpm package"
}

# Install binary directly
install_binary() {
    log_step "Installing binary..."

    if [ "$OS" = "windows" ]; then
        EXT=".zip"
        DOWNLOAD_URL="${GITHUB_URL}/releases/download/v${VERSION}/${BINARY_NAME}_${VERSION}_${PLATFORM}.zip"
    else
        EXT=".tar.gz"
        DOWNLOAD_URL="${GITHUB_URL}/releases/download/v${VERSION}/${BINARY_NAME}_${VERSION}_${PLATFORM}.tar.gz"
    fi

    log_info "Downloading: $DOWNLOAD_URL"

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/dap-mcp$EXT"; then
        log_error "Failed to download binary"
        log_info "Check if version v$VERSION exists: ${GITHUB_URL}/releases"
        exit 1
    fi

    if [ "$EXT" = ".zip" ]; then
        unzip -q "$TMP_DIR/dap-mcp.zip" -d "$TMP_DIR"
    else
        tar -xzf "$TMP_DIR/dap-mcp.tar.gz" -C "$TMP_DIR"
    fi

    # Create install directory if needed
    if [ ! -d "$INSTALL_DIR" ]; then
        log_info "Creating directory: $INSTALL_DIR"
        if [ -w "$(dirname "$INSTALL_DIR")" ]; then
            mkdir -p "$INSTALL_DIR"
        else
            sudo mkdir -p "$INSTALL_DIR"
        fi
    fi

    # Install the binary
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        log_warn "Need sudo to install to $INSTALL_DIR"
        sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi

    log_info "Installed to: $INSTALL_DIR/$BINARY_NAME"
}

# Choose and run installation method
run_install() {
    local method="${INSTALL_METHOD:-auto}"

    if [ "$method" = "auto" ]; then
        # Auto-detect best method
        case "$DISTRO" in
            macos)
                if [ "$HAS_BREW" = true ]; then
                    method="brew"
                else
                    method="binary"
                fi
                ;;
            debian|ubuntu|pop|linuxmint|elementary)
                method="deb"
                ;;
            fedora|rhel|centos|rocky|alma)
                method="rpm"
                ;;
            *)
                # Check DISTRO_FAMILY for derived distros
                case "$DISTRO_FAMILY" in
                    *debian*|*ubuntu*)
                        method="deb"
                        ;;
                    *rhel*|*fedora*)
                        method="rpm"
                        ;;
                    *)
                        method="binary"
                        ;;
                esac
                ;;
        esac
    fi

    log_info "Installation method: $method"
    echo ""

    case "$method" in
        brew)
            install_brew
            ;;
        deb)
            if ! install_deb; then
                log_warn "Falling back to binary install"
                install_binary
            fi
            ;;
        rpm)
            if ! install_rpm; then
                log_warn "Falling back to binary install"
                install_binary
            fi
            ;;
        binary|*)
            install_binary
            ;;
    esac
}

# Verify installation
verify_installation() {
    echo ""

    # Check if in PATH
    if command -v $BINARY_NAME &> /dev/null; then
        INSTALLED_VERSION=$($BINARY_NAME --version 2>/dev/null || echo "installed")
        log_info "Installation successful!"
        echo ""
        echo -e "  ${BOLD}$INSTALLED_VERSION${NC}"
    else
        log_warn "$BINARY_NAME installed but not in PATH"
        echo ""

        # Suggest PATH fix
        SHELL_NAME=$(basename "$SHELL")
        case "$SHELL_NAME" in
            zsh)
                RC_FILE="~/.zshrc"
                ;;
            bash)
                if [ "$OS" = "darwin" ]; then
                    RC_FILE="~/.bash_profile"
                else
                    RC_FILE="~/.bashrc"
                fi
                ;;
            fish)
                RC_FILE="~/.config/fish/config.fish"
                ;;
            *)
                RC_FILE="your shell config"
                ;;
        esac

        log_info "Add to $RC_FILE:"
        echo ""
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
        echo ""
        log_info "Or run directly: $INSTALL_DIR/$BINARY_NAME"
    fi
}

# Print next steps
print_next_steps() {
    echo ""
    echo -e "${BOLD}========================================${NC}"
    echo -e "${BOLD}  Next Steps${NC}"
    echo -e "${BOLD}========================================${NC}"
    echo ""

    echo -e "${BOLD}1. Install debug adapters:${NC}"
    echo ""
    echo "   # Go"
    echo "   go install github.com/go-delve/delve/cmd/dlv@latest"
    echo ""
    echo "   # Python"
    echo "   pip install debugpy"
    echo ""
    echo "   # C/C++/Rust (macOS - usually pre-installed)"
    echo "   xcode-select --install"
    echo ""
    echo "   # C/C++/Rust (Linux)"
    echo "   sudo apt install lldb  # or: sudo dnf install lldb"
    echo ""

    echo -e "${BOLD}2. Configure Claude Code:${NC}"
    echo ""
    echo "   Add to ~/.claude.json:"
    echo ""
    echo '   {'
    echo '     "mcpServers": {'
    echo '       "dap-mcp": {'
    echo "         \"command\": \"$BINARY_NAME\","
    echo '         "args": ["--mode", "full"]'
    echo '       }'
    echo '     }'
    echo '   }'
    echo ""

    echo -e "${BOLD}3. Documentation:${NC}"
    echo ""
    echo "   ${GITHUB_URL}"
    echo ""
}

# Main
main() {
    echo ""
    echo -e "${BOLD}======================================${NC}"
    echo -e "${BOLD}  DAP-MCP Installer${NC}"
    echo -e "${BOLD}======================================${NC}"
    echo ""

    check_requirements
    detect_platform
    get_latest_version
    check_existing
    echo ""
    run_install
    verify_installation
    print_next_steps
}

main "$@"
