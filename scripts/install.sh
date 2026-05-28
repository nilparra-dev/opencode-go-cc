#!/bin/bash
set -e

# occb installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/nilparra-dev/opencode-go-cc/main/scripts/install.sh | bash

REPO="nilparra-dev/opencode-go-cc"
BINARY="occb"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    amd64) ARCH="amd64" ;;
    arm64) ARCH="arm64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

PLATFORM="${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    BINARY_NAME="${BINARY}.exe"
else
    BINARY_NAME="${BINARY}"
fi

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Get latest release or fall back to Go source
LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"
echo "Fetching latest release info..."
DOWNLOAD_URL=$(curl -fsSL "$LATEST_URL" 2>/dev/null | grep "browser_download_url.*${PLATFORM}" | cut -d '"' -f 4 || true)

if [ -n "$DOWNLOAD_URL" ]; then
    echo "Downloading ${BINARY} for ${PLATFORM}..."
    curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${BINARY_NAME}"
else
    echo "No published release found for ${PLATFORM}. Falling back to 'go install'."
    if ! command -v go >/dev/null 2>&1; then
        echo "Go is required for fallback installation but was not found in PATH."
        exit 1
    fi

    GOBIN="$TMP_DIR" go install github.com/nilparra-dev/opencode-go-cc/cmd/occb@latest
fi

# Determine install directory
if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

# Install
echo "Installing to ${INSTALL_DIR}/${BINARY}..."
chmod +x "${TMP_DIR}/${BINARY_NAME}"
mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY}"

# Check if in PATH
if ! command -v "$BINARY" &> /dev/null; then
    echo ""
    echo "WARNING: ${INSTALL_DIR} is not in your PATH."
    echo "Add the following to your shell profile:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

echo ""
echo "${BINARY} installed successfully!"
echo "Run '${BINARY} init' to get started."
echo "Run '${BINARY} update' later to install the latest release."
