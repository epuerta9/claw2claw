#!/bin/bash
#===============================================================================
#
#          FILE: install.sh
#
#         USAGE: curl -sSL https://raw.githubusercontent.com/epuerta9/claw2claw/main/install.sh | bash
#                  OR
#                wget -qO- https://raw.githubusercontent.com/epuerta9/claw2claw/main/install.sh | bash
#
#   DESCRIPTION: claw2claw Installer Script
#
#                Installs claw2claw CLI for secure AI-to-AI context sharing.
#                Downloads pre-built binary or builds from source.
#
#       OPTIONS: -p, --prefix <path>   Install prefix (default: /usr/local/bin)
#                -v, --version <ver>   Specific version (default: latest)
#                --source               Build from source instead of binary
#
#===============================================================================
set -e

#-------------------------------------------------------------------------------
# CONFIGURATION
#-------------------------------------------------------------------------------
REPO="epuerta9/claw2claw"
BINARY_NAME="claw"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"
BUILD_FROM_SOURCE="${BUILD_FROM_SOURCE:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

#-------------------------------------------------------------------------------
# FUNCTIONS
#-------------------------------------------------------------------------------

print_banner() {
    echo -e "${BLUE}"
    cat << 'EOF'
        _                 ____       _
   ___| | __ ___      __|___ \  ___| | __ ___      __
  / __| |/ _` \ \ /\ / /  __) |/ __| |/ _` \ \ /\ / /
 | (__| | (_| |\ V  V /  / __/| (__| | (_| |\ V  V /
  \___|_|\__,_| \_/\_/  |_____|\_____|_|\__,_| \_/\_/

EOF
    echo -e "${NC}"
    echo "Secure AI-to-AI Context Sharing"
    echo "================================"
    echo ""
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

detect_os() {
    OS="$(uname -s)"
    case "${OS}" in
        Linux*)     OS="linux";;
        Darwin*)    OS="darwin";;
        MINGW*|MSYS*|CYGWIN*) OS="windows";;
        *)          error "Unsupported OS: ${OS}";;
    esac
    echo "${OS}"
}

detect_arch() {
    ARCH="$(uname -m)"
    case "${ARCH}" in
        x86_64|amd64)   ARCH="amd64";;
        aarch64|arm64)  ARCH="arm64";;
        armv7l)         ARCH="arm";;
        i386|i686)      ARCH="386";;
        *)              error "Unsupported architecture: ${ARCH}";;
    esac
    echo "${ARCH}"
}

check_command() {
    command -v "$1" >/dev/null 2>&1
}

get_latest_version() {
    if check_command curl; then
        curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    elif check_command wget; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one."
    fi
}

install_from_source() {
    info "Building from source..."

    if ! check_command go; then
        error "Go is not installed. Please install Go 1.22+ or use binary install."
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    info "Found Go ${GO_VERSION}"

    TEMP_DIR=$(mktemp -d)
    trap "rm -rf ${TEMP_DIR}" EXIT

    info "Cloning repository..."
    git clone --depth 1 "https://github.com/${REPO}.git" "${TEMP_DIR}/claw2claw"

    cd "${TEMP_DIR}/claw2claw"

    info "Building claw2claw..."
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${BINARY_NAME}" ./cmd/claw

    info "Installing to ${INSTALL_DIR}..."
    if [ -w "${INSTALL_DIR}" ]; then
        mv "${BINARY_NAME}" "${INSTALL_DIR}/"
    else
        sudo mv "${BINARY_NAME}" "${INSTALL_DIR}/"
    fi

    success "Built and installed from source!"
}

install_from_binary() {
    local os="$1"
    local arch="$2"
    local version="$3"

    # Remove 'v' prefix if present for URL
    local version_num="${version#v}"

    local filename="claw2claw_${version_num}_${os}_${arch}.tar.gz"
    local url="https://github.com/${REPO}/releases/download/${version}/${filename}"

    info "Downloading ${filename}..."

    TEMP_DIR=$(mktemp -d)
    trap "rm -rf ${TEMP_DIR}" EXIT

    if check_command curl; then
        curl -sSL "${url}" -o "${TEMP_DIR}/${filename}" || {
            warn "Binary not found, falling back to source install..."
            install_from_source
            return
        }
    elif check_command wget; then
        wget -q "${url}" -O "${TEMP_DIR}/${filename}" || {
            warn "Binary not found, falling back to source install..."
            install_from_source
            return
        }
    fi

    info "Extracting..."
    tar -xzf "${TEMP_DIR}/${filename}" -C "${TEMP_DIR}"

    info "Installing to ${INSTALL_DIR}..."
    if [ -w "${INSTALL_DIR}" ]; then
        mv "${TEMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/"
    else
        sudo mv "${TEMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/"
    fi

    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    success "Binary installed!"
}

install_claude_skill() {
    local skill_dir="${HOME}/.claude/skills/claw2claw"

    if [ -d "${HOME}/.claude" ]; then
        info "Installing Claude Code skill..."
        mkdir -p "${skill_dir}"

        if check_command curl; then
            curl -sSL "https://raw.githubusercontent.com/${REPO}/main/.claude/skills/claw2claw/SKILL.md" -o "${skill_dir}/SKILL.md"
        elif check_command wget; then
            wget -q "https://raw.githubusercontent.com/${REPO}/main/.claude/skills/claw2claw/SKILL.md" -O "${skill_dir}/SKILL.md"
        fi

        success "Claude Code skill installed!"
    fi
}

print_success() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  claw2claw installed successfully!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "Quick start:"
    echo "  claw send <file>              # Share a file"
    echo "  claw receive <code>           # Receive a file"
    echo "  claw read <file>              # Read safely"
    echo ""
    echo "Documentation: https://github.com/${REPO}"
    echo "Web Dashboard: https://claw2claw.cloudshipai.com"
    echo ""
}

#-------------------------------------------------------------------------------
# PARSE ARGUMENTS
#-------------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--prefix)
            INSTALL_DIR="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        --source)
            BUILD_FROM_SOURCE="true"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -p, --prefix <path>   Install directory (default: /usr/local/bin)"
            echo "  -v, --version <ver>   Version to install (default: latest)"
            echo "  --source              Build from source instead of binary"
            echo "  -h, --help            Show this help"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

#-------------------------------------------------------------------------------
# MAIN
#-------------------------------------------------------------------------------
print_banner

OS=$(detect_os)
ARCH=$(detect_arch)

info "Detected: ${OS}/${ARCH}"

if [ "${VERSION}" = "latest" ]; then
    info "Fetching latest version..."
    VERSION=$(get_latest_version)
    if [ -z "${VERSION}" ]; then
        warn "Could not determine latest version, building from source..."
        BUILD_FROM_SOURCE="true"
    else
        info "Latest version: ${VERSION}"
    fi
fi

if [ "${BUILD_FROM_SOURCE}" = "true" ]; then
    install_from_source
else
    install_from_binary "${OS}" "${ARCH}" "${VERSION}"
fi

install_claude_skill

print_success
