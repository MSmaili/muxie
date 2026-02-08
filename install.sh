#!/usr/bin/env bash
#
# Secure installer for hetki with checksum verification
# Usage: curl -fsSL https://raw.githubusercontent.com/MSmaili/hetki/main/install.sh | bash
#

set -euo pipefail

#######################################
# Constants
#######################################

readonly SCRIPT_NAME="$(basename "$0")"
readonly REPO="MSmaili/hetki"
readonly DEFAULT_INSTALL_DIR="${HOME}/.local/bin"

#######################################
# Global Variables
#######################################

VERSION="${HETKI_VERSION:-latest}"
INSTALL_DIR="${HETKI_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
SKIP_CHECKSUM="${HETKI_SKIP_CHECKSUM:-false}"
OS=""
ARCH=""

#######################################
# Logging Functions
#######################################

info() {
    printf '\033[0;32m==>\033[0m %s\n' "$1"
}

warn() {
    printf '\033[1;33mWarning:\033[0m %s\n' "$1" >&2
}

error() {
    printf '\033[0;31mError:\033[0m %s\n' "$1" >&2
    exit "${2:-1}"
}

#######################################
# Cleanup
#######################################

declare -a TEMP_FILES=()

cleanup() {
    local temp
    for temp in "${TEMP_FILES[@]}"; do
        if [[ -e "$temp" ]]; then
            rm -rf "$temp"
        fi
    done
}

trap cleanup EXIT

#######################################
# Platform Detection
#######################################

detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            error "Unsupported OS: $os"
            ;;
    esac

    case "$arch" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64 | arm64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            ;;
    esac

    info "Detected platform: $OS/$ARCH"
}

#######################################
# Installation Directory Setup
#######################################

ensure_install_dir() {
    if [[ ! -d "$INSTALL_DIR" ]]; then
        info "Creating installation directory: $INSTALL_DIR"
        if ! mkdir -p "$INSTALL_DIR"; then
            error "Failed to create $INSTALL_DIR. Check permissions."
        fi
    fi

    if [[ ! -w "$INSTALL_DIR" ]]; then
        error "Cannot write to $INSTALL_DIR. Try: HETKI_INSTALL_DIR=~/.local/bin bash install.sh"
    fi
}

check_path() {
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

show_path_instructions() {
    local shell_rc

    case "$SHELL" in
        */zsh)
            shell_rc="~/.zshrc"
            ;;
        */bash)
            shell_rc="~/.bashrc"
            ;;
        *)
            shell_rc="your shell configuration file"
            ;;
    esac

    warn "$INSTALL_DIR is not in your PATH"
    printf '\nAdd this to %s:\n' "$shell_rc"
    printf '  \033[0;34mexport PATH="%s:$PATH"\033[0m\n\n' "$INSTALL_DIR"
    printf 'Then reload your shell:\n'
    printf '  \033[0;34msource %s\033[0m\n\n' "$shell_rc"
}

#######################################
# Dependency Checks
#######################################

has_go() {
    command -v go >/dev/null 2>&1
}

has_sha256sum() {
    command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1
}

check_tmux() {
    if ! command -v tmux >/dev/null 2>&1; then
        warn "tmux is not installed. hetki works best with tmux or zellij."
        printf '\nInstall tmux:\n'
        printf '  macOS:  brew install tmux\n'
        printf '  Ubuntu: sudo apt install tmux\n'
        printf 'Or install zellij:\n'
        printf '  cargo install zellij\n\n'
    fi
}

#######################################
# Checksum Verification
#######################################

# Download checksums file from release
# Arguments:
#   Version tag (or "latest")
#   Destination path
# Returns:
#   0 if successful, 1 otherwise
download_checksums() {
    local version="$1"
    local dest="$2"
    local checksum_url

    if [[ "$version" == "latest" ]]; then
        checksum_url="https://github.com/${REPO}/releases/latest/download/checksums.txt"
    else
        checksum_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
    fi

    curl -fsSL "$checksum_url" -o "$dest" 2>/dev/null
}

# Verify binary checksum
# Arguments:
#   Binary file path
#   Checksums file path
#   Binary name (e.g., hetki-linux-amd64)
# Returns:
#   0 if verified, 1 if failed
verify_checksum() {
    local binary="$1"
    local checksums="$2"
    local binary_name="$3"
    local expected_hash actual_hash

    # Extract expected hash from checksums file
    expected_hash="$(grep "${binary_name}$" "$checksums" 2>/dev/null | awk '{print $1}')"

    if [[ -z "$expected_hash" ]]; then
        error "Could not find checksum for ${binary_name} in checksums.txt"
    fi

    # Calculate actual hash
    if command -v sha256sum >/dev/null 2>&1; then
        actual_hash="$(sha256sum "$binary" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        actual_hash="$(shasum -a 256 "$binary" | awk '{print $1}')"
    else
        warn "Neither sha256sum nor shasum found, skipping checksum verification"
        return 1
    fi

    if [[ "$actual_hash" != "$expected_hash" ]]; then
        error "Checksum verification failed!\n  Expected: ${expected_hash}\n  Actual:   ${actual_hash}"
    fi

    info "Checksum verified: ${actual_hash}"
    return 0
}

#######################################
# Installation Methods
#######################################

download_file() {
    local url="$1"
    local dest="$2"

    if ! curl -fsSL "$url" -o "$dest" 2>/dev/null; then
        return 1
    fi

    return 0
}

install_from_source() {
    local temp_dir

    info "Installing from source..."

    if ! has_go; then
        error "Go is not installed. Please install Go or use a pre-built binary."
    fi

    temp_dir="$(mktemp -d)"
    TEMP_FILES+=("$temp_dir")

    info "Cloning repository..."
    if ! git clone --depth 1 "https://github.com/${REPO}.git" "$temp_dir" 2>/dev/null; then
        error "Failed to clone repository"
    fi

    (
        cd "$temp_dir"
        info "Building hetki..."
        if ! go build -o hetki .; then
            error "Build failed"
        fi

        info "Installing to $INSTALL_DIR..."
        if ! mv hetki "${INSTALL_DIR}/hetki"; then
            error "Failed to install"
        fi
        chmod +x "${INSTALL_DIR}/hetki"
    )
}

install_from_release() {
    local download_url checksum_url
    local temp_file temp_checksums
    local target_path="${INSTALL_DIR}/hetki"
    local binary_name="hetki-${OS}-${ARCH}"
    local checksum_verified=false

    info "Downloading hetki $VERSION for $OS/$ARCH..."

    if [[ "$VERSION" == "latest" ]]; then
        download_url="https://github.com/${REPO}/releases/latest/download/${binary_name}"
        checksum_url="https://github.com/${REPO}/releases/latest/download/checksums.txt"
    else
        download_url="https://github.com/${REPO}/releases/download/${VERSION}/${binary_name}"
        checksum_url="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
    fi

    temp_file="$(mktemp)"
    TEMP_FILES+=("$temp_file")

    if ! download_file "$download_url" "$temp_file"; then
        warn "No pre-built binary found, installing from source..."
        install_from_source
        return 0
    fi

    # Sanity check: Go binaries are typically 5-15MB; reject tiny files
    # (catches HTML error pages, truncated downloads, etc.)
    local file_size
    file_size="$(wc -c < "$temp_file")"
    if [[ "$file_size" -lt 1048576 ]]; then
        error "Downloaded file is too small (${file_size} bytes). Expected a Go binary (>1MB)."
    fi

    # Verify checksum if not skipped
    if [[ "$SKIP_CHECKSUM" != "true" ]]; then
        if has_sha256sum; then
            temp_checksums="$(mktemp)"
            TEMP_FILES+=("$temp_checksums")

            info "Downloading checksums..."
            if download_file "$checksum_url" "$temp_checksums"; then
                if verify_checksum "$temp_file" "$temp_checksums" "$binary_name"; then
                    checksum_verified=true
                fi
            else
                error "Could not download checksums file. Set HETKI_SKIP_CHECKSUM=true to bypass."
            fi
        else
            warn "Checksum tools not available (sha256sum/shasum), skipping verification"
        fi
    else
        warn "Skipping checksum verification (HETKI_SKIP_CHECKSUM=true)"
    fi

    info "Installing to $INSTALL_DIR..."

    if ! mv "$temp_file" "$target_path"; then
        error "Failed to install"
    fi
    chmod +x "$target_path"

    if [[ "$checksum_verified" == "true" ]]; then
        info "Installation successful with verified checksum"
    else
        info "Installation successful (checksum not verified)"
    fi
}

#######################################
# Verification
#######################################

verify_installation() {
    local hetki_path="${INSTALL_DIR}/hetki"

    if [[ ! -x "$hetki_path" ]]; then
        error "Installation failed - hetki not found at $hetki_path"
    fi

    info "Successfully installed hetki to $hetki_path"
    printf '\n'
    "$hetki_path" --version 2>/dev/null || true
    printf '\n'
}

show_post_install_info() {
    printf 'Get started:\n'
    printf '  hetki start <workspace>    # Start a workspace\n'
    printf '  hetki save                 # Save current session\n'
    printf '  hetki list sessions        # List sessions\n'
    printf '\n'
    printf 'For more info: hetki --help\n'
}

#######################################
# Main
#######################################

main() {
    detect_platform
    ensure_install_dir
    check_tmux

    # Check if forced to install from source
    if [[ -n "${HETKI_FROM_SOURCE:-}" ]]; then
        install_from_source
    else
        install_from_release
    fi

    verify_installation
    show_post_install_info

    if ! check_path; then
        show_path_instructions
    fi
}

main "$@"
