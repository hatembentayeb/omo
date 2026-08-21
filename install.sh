#!/usr/bin/env bash
set -euo pipefail

# One-line installer for omo:
#   curl -fsSL https://raw.githubusercontent.com/hatembentayeb/omo/main/install.sh | bash
#
# Termux (Android):
#   pkg install curl tar man
#   curl -fsSL https://raw.githubusercontent.com/hatembentayeb/omo/main/install.sh | bash
# Installs into $PREFIX/bin and $PREFIX/share/man/man1 (no sudo).

REPO="hatembentayeb/omo"
OMO_HOME="${HOME}/.omo"

is_termux() {
  [ -n "${TERMUX_VERSION:-}" ] && return 0
  [ -n "${TERMUX_PREFIX:-}" ] && return 0
  case "${PREFIX:-}" in
    *com.termux*) return 0 ;;
  esac
  [ -d /data/data/com.termux/files/usr ] && return 0
  return 1
}

if is_termux; then
  TERMUX=1
  PREFIX="${PREFIX:-${TERMUX_PREFIX:-/data/data/com.termux/files/usr}}"
  DEFAULT_BIN="${PREFIX}/bin"
  DEFAULT_MAN="${PREFIX}/share/man/man1"
else
  TERMUX=0
  DEFAULT_BIN="/usr/local/bin"
  DEFAULT_MAN="/usr/local/share/man/man1"
fi

INSTALL_DIR="${OMO_INSTALL_DIR:-$DEFAULT_BIN}"
MANDIR="${OMO_MANDIR:-$DEFAULT_MAN}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)            ARCH="amd64" ;;
  aarch64|arm64)     ARCH="arm64" ;;
  armv7l|armv8l|arm) ARCH="arm" ;;
  i686|i386)         ARCH="386" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Termux reports Linux; static linux binaries (CGO_ENABLED=0) run there.
if [ "$TERMUX" = 1 ]; then
  OS="linux"
fi

case "$ARCH" in
  amd64|arm64) ;;
  *)
    echo "No prebuilt omo for ${OS}/${ARCH}."
    if [ "$TERMUX" = 1 ]; then
      echo "Termux needs a 64-bit device (aarch64 or x86_64)."
      echo "Or build from source: pkg install golang git make && git clone https://github.com/${REPO}.git && cd omo && make build PREFIX=\"\$PREFIX\""
    fi
    exit 1
    ;;
esac

if [ "$TERMUX" = 1 ]; then
  echo "Detecting platform: ${OS}/${ARCH} (Termux)"
else
  echo "Detecting platform: ${OS}/${ARCH}"
fi

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

# dest_dir mode src name — sudo only when the dest is not writable (never on Termux).
install_file() {
  local dest_dir="$1" mode="$2" src="$3" name="$4"
  if [ ! -d "$dest_dir" ]; then
    if mkdir -p "$dest_dir" 2>/dev/null; then
      :
    elif [ "$TERMUX" = 1 ]; then
      echo "Cannot create ${dest_dir}"
      return 1
    else
      sudo mkdir -p "$dest_dir"
    fi
  fi
  if [ -w "$dest_dir" ]; then
    if command -v install >/dev/null 2>&1; then
      install -m "$mode" "$src" "${dest_dir}/${name}"
    else
      cp "$src" "${dest_dir}/${name}"
      chmod "$mode" "${dest_dir}/${name}"
    fi
  elif [ "$TERMUX" = 1 ]; then
    echo "Cannot write ${dest_dir} (Termux has no sudo). Set OMO_INSTALL_DIR to a writable PATH directory."
    return 1
  else
    echo "Installing ${name} to ${dest_dir} (sudo)..."
    sudo install -m "$mode" "$src" "${dest_dir}/${name}"
  fi
}

install_file "$INSTALL_DIR" 755 "$BINARY" omo

MAN_SRC=""
for candidate in "${TMPDIR}/omo.1" "${TMPDIR}/man/omo.1"; do
  if [ -f "$candidate" ]; then
    MAN_SRC="$candidate"
    break
  fi
done
if [ -z "$MAN_SRC" ] && [ "$OS" != "windows" ]; then
  MAN_SRC="${TMPDIR}/omo.1"
  if ! curl -fsSL "https://raw.githubusercontent.com/${REPO}/${TAG}/man/omo.1" -o "$MAN_SRC"; then
    curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/man/omo.1" -o "$MAN_SRC" || MAN_SRC=""
  fi
  if [ -n "$MAN_SRC" ] && [ ! -s "$MAN_SRC" ]; then
    MAN_SRC=""
  fi
fi
if [ -n "$MAN_SRC" ] && [ "$OS" != "windows" ]; then
  if install_file "$MANDIR" 644 "$MAN_SRC" omo.1; then
    echo "man omo → ${MANDIR}/omo.1"
  else
    echo "warning: could not install man page to ${MANDIR}"
  fi
fi

mkdir -p "${OMO_HOME}/plugins" "${OMO_HOME}/logs"

hash -r 2>/dev/null || true

echo ""
echo "omo ${TAG} installed to ${INSTALL_DIR}/omo"
if command -v omo >/dev/null 2>&1; then
  echo "on PATH: $(command -v omo)"
else
  echo "not on PATH — add ${INSTALL_DIR} to PATH"
fi
echo ""
echo "Get started:"
echo "  omo                          Launch the TUI"
echo "  man omo                      Manual"
echo "  -> Package Manager (p)       Install plugins"
echo "  -> Press S to sync index"
echo "  -> Press A to install all"
echo ""
