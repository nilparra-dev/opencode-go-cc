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

# Get latest release
LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"
echo "Fetching latest release info..."
DOWNLOAD_URL=$(curl -s "$LATEST_URL" | grep "browser_download_url.*${PLATFORM}" | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Could not find release binary for platform: $PLATFORM"
    echo "You may need to build from source."
    exit 1
fi

# Download
echo "Downloading ${BINARY} for ${PLATFORM}..."
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${BINARY_NAME}"

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
