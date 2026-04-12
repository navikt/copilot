#!/bin/bash
set -euo pipefail

# nav-pilot installer — downloads the latest release binary from GitHub.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash
#   curl -fsSL ... | bash -s -- --version nav-pilot/2026.04.12-abc1234
#   curl -fsSL ... | bash -s -- --dir /usr/local/bin

REPO="navikt/copilot"
BINARY="nav-pilot"
VERSION=""
INSTALL_DIR=""

# ─── Parse arguments ─────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version|-v)
      if [[ $# -lt 2 ]]; then echo "Error: --version requires a value"; exit 1; fi
      VERSION="$2"; shift 2 ;;
    --dir|-d)
      if [[ $# -lt 2 ]]; then echo "Error: --dir requires a value"; exit 1; fi
      INSTALL_DIR="$2"; shift 2 ;;
    --help|-h)
      echo "Usage: install.sh [--version <tag>] [--dir <path>]"
      echo ""
      echo "  --version  Install a specific version (default: latest release)"
      echo "  --dir      Install directory (default: auto-detect from PATH)"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ─── Detect platform ─────────────────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin) ;;
  linux)  ;;
  *)      echo "Error: Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64)        ARCH="amd64" ;;
  *)             echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
esac

ASSET="${BINARY}-${OS}-${ARCH}"

# ─── Resolve version ─────────────────────────────────────────────────────────

if [[ -z "$VERSION" ]]; then
  echo "→ Fetching latest release..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  if [[ -z "$VERSION" ]]; then
    echo "Error: Could not determine latest version. Use --version to specify."
    exit 1
  fi
fi

echo "→ Installing nav-pilot ${VERSION} (${OS}/${ARCH})"

# ─── Find install directory ──────────────────────────────────────────────────

find_install_dir() {
  if [[ -n "$INSTALL_DIR" ]]; then
    return
  fi

  # Prefer directories already on PATH, in order of preference
  for dir in "$HOME/.local/bin" "$HOME/bin" "/usr/local/bin"; do
    if echo "$PATH" | tr ':' '\n' | grep -qx "$dir"; then
      if [[ -w "$dir" ]] || [[ ! -d "$dir" && -w "$(dirname "$dir")" ]]; then
        INSTALL_DIR="$dir"
        return
      fi
    fi
  done

  # Default to ~/.local/bin
  INSTALL_DIR="$HOME/.local/bin"
}

find_install_dir
mkdir -p "$INSTALL_DIR"

# ─── Download binary ─────────────────────────────────────────────────────────

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/SHA256SUMS"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "→ Downloading ${ASSET}..."
if ! curl -fsSL -o "${TMP_DIR}/${ASSET}" "$DOWNLOAD_URL"; then
  echo "Error: Failed to download ${DOWNLOAD_URL}"
  echo "Check that version ${VERSION} exists: https://github.com/${REPO}/releases"
  exit 1
fi

# ─── Verify checksum ─────────────────────────────────────────────────────────

echo "→ Verifying checksum..."
if curl -fsSL -o "${TMP_DIR}/SHA256SUMS" "$CHECKSUM_URL" 2>/dev/null; then
  EXPECTED=$(grep -F "  ${ASSET}" "${TMP_DIR}/SHA256SUMS" | awk '{print $1}')
  if [[ -z "$EXPECTED" ]]; then
    echo "Error: No checksum entry found for ${ASSET}"
    exit 1
  fi
  if [[ -n "$EXPECTED" ]]; then
    if command -v sha256sum &>/dev/null; then
      ACTUAL=$(sha256sum "${TMP_DIR}/${ASSET}" | awk '{print $1}')
    elif command -v shasum &>/dev/null; then
      ACTUAL=$(shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')
    else
      echo "  ⚠ Neither sha256sum nor shasum found (skipping verification)"
      ACTUAL=""
    fi
    if [[ -n "$ACTUAL" && "$EXPECTED" != "$ACTUAL" ]]; then
      echo "Error: Checksum mismatch!"
      echo "  Expected: ${EXPECTED}"
      echo "  Got:      ${ACTUAL}"
      exit 1
    fi
    echo "  ✓ Checksum verified"
  fi
else
  echo "  ⚠ Could not download checksums (skipping verification)"
fi

# ─── Install ─────────────────────────────────────────────────────────────────

chmod +x "${TMP_DIR}/${ASSET}"
mv "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BINARY}"

echo "  ✓ Installed to ${INSTALL_DIR}/${BINARY}"

# ─── Verify ──────────────────────────────────────────────────────────────────

if ! command -v "$BINARY" &>/dev/null; then
  echo ""
  echo "⚠ ${INSTALL_DIR} is not in your PATH."
  echo ""
  SHELL_NAME=$(basename "$SHELL")
  case "$SHELL_NAME" in
    zsh)  RC_FILE="$HOME/.zshrc" ;;
    bash) RC_FILE="$HOME/.bashrc" ;;
    fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
    *)    RC_FILE="your shell config" ;;
  esac
  echo "Add it with:"
  if [[ "$SHELL_NAME" == "fish" ]]; then
    echo "  fish_add_path ${INSTALL_DIR}"
  else
    echo "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ${RC_FILE}"
  fi
  echo ""
  echo "Then restart your terminal or run:"
  echo "  source ${RC_FILE}"
else
  INSTALLED_VERSION=$("${INSTALL_DIR}/${BINARY}" version 2>/dev/null || echo "unknown")
  echo ""
  echo "✓ nav-pilot is ready! (${INSTALLED_VERSION})"
fi

echo ""
echo "Get started:"
echo "  nav-pilot list                    # See available collections"
echo "  nav-pilot install kotlin-backend  # Install a collection"
echo "  nav-pilot install --dry-run fullstack  # Preview first"
