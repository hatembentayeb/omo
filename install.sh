#!/usr/bin/env bash
set -euo pipefail

# One-line installer for omo:
#   curl -fsSL https://raw.githubusercontent.com/hatembentayeb/omo/main/install.sh | bash

REPO="hatembentayeb/omo"
INSTALL_DIR="${OMO_INSTALL_DIR:-/usr/local/bin}"
OMO_HOME="${HOME}/.omo"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Detecting platform: ${OS}/${ARCH}"

TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "Failed to fetch latest release tag"
  exit 1
fi
echo "Latest release: ${TAG}"

ASSET="omo-${TAG}-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
  ASSET="${ASSET}.exe"
fi
TARBALL="${ASSET}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${TARBALL}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${TARBALL}..."
curl -fsSL -o "${TMPDIR}/${TARBALL}" "$URL"

echo "Extracting..."
tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

BINARY="${TMPDIR}/${ASSET}"
if [ ! -f "$BINARY" ]; then
  echo "Error: expected binary ${ASSET} not found in archive"
  exit 1
fi

chmod +x "$BINARY"

if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "${INSTALL_DIR}/omo"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "$BINARY" "${INSTALL_DIR}/omo"
fi

mkdir -p "${OMO_HOME}/plugins" "${OMO_HOME}/logs"

echo ""
echo "omo ${TAG} installed to ${INSTALL_DIR}/omo"
echo ""
echo "Get started:"
echo "  omo                          Launch the TUI"
echo "  -> Package Manager (p)       Install plugins"
echo "  -> Press S to sync index"
echo "  -> Press A to install all"
echo ""
