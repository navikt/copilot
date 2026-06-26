#!/bin/bash
set -euo pipefail

# nav-pilot installer — installs the nav-pilot CLI.
#
# On macOS: uses Homebrew (brew install navikt/tap/nav-pilot) when available.
# On Linux / CI: downloads the latest release binary from GitHub.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash
#   curl -fsSL ... | bash -s -- --version nav-pilot/2026.04.12-abc1234
#   curl -fsSL ... | bash -s -- --dir /usr/local/bin
#   curl -fsSL ... | bash -s -- --no-brew

REPO="navikt/copilot"
BINARY="nav-pilot"
VERSION=""
INSTALL_DIR=""
NO_BREW=false

# ─── Parse arguments ─────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version|-v)
      if [[ $# -lt 2 ]]; then echo "Error: --version requires a value"; exit 1; fi
      VERSION="$2"; shift 2 ;;
    --dir|-d)
      if [[ $# -lt 2 ]]; then echo "Error: --dir requires a value"; exit 1; fi
      INSTALL_DIR="$2"; shift 2 ;;
    --no-brew)
      NO_BREW=true; shift ;;
    --help|-h)
      echo "Usage: install.sh [--version <tag>] [--dir <path>] [--no-brew]"
      echo ""
      echo "  --version  Install a specific version (default: latest release)"
      echo "  --dir      Install directory (default: auto-detect from PATH)"
      echo "  --no-brew  Skip Homebrew even if available (use direct download)"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ─── Homebrew install (macOS) ────────────────────────────────────────────────

if [[ "$NO_BREW" == false && -z "$VERSION" && -z "$INSTALL_DIR" ]] && command -v brew &>/dev/null; then
  echo "→ Installing via Homebrew..."
  brew install navikt/tap/nav-pilot navikt/tap/cplt rtk
  echo ""
  INSTALLED_VERSION=$(nav-pilot version 2>/dev/null || echo "unknown")
  echo "✓ nav-pilot is ready! (${INSTALLED_VERSION})"
  echo ""
  echo "Get started:"
  echo "  nav-pilot list                    # See available collections"
  echo "  nav-pilot install kotlin-backend  # Install a collection"
  echo "  nav-pilot install --dry-run fullstack  # Preview first"
  echo ""
  echo "Upgrade later with: brew upgrade nav-pilot"
  exit 0
fi

# ─── Security notice ────────────────────────────────────────────────────────
#
# WARNING: Piping curl to bash (curl | bash) means the script itself is not
# verified before execution. If the GitHub CDN or repository is compromised,
# this script could be replaced before the binary checks run.
#
# For the strongest supply chain security, install via Homebrew:
#   brew install navikt/tap/nav-pilot
#
# On Linux/CI, download and inspect the script before running:
#   curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh -o install.sh
#   cat install.sh  # Inspect before running!
#   bash install.sh

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
  echo "→ Fetching latest nav-pilot release..."
  # Filter by nav-pilot/ tag prefix to avoid picking up unrelated releases (e.g. skills)
  set +o pipefail
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=100" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' \
    | grep '^nav-pilot/' \
    | head -1)
  set -o pipefail
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
#
# NOTE: SHA256SUMS is co-located with the binary on GitHub Releases.
# This protects against accidental corruption and network-level tampering,
# but NOT against a compromised GitHub release. SLSA attestation below
# provides the stronger guarantee.

echo "→ Verifying checksum..."
if ! curl -fsSL -o "${TMP_DIR}/SHA256SUMS" "$CHECKSUM_URL" 2>/dev/null; then
  echo "Error: Could not download SHA256SUMS from ${CHECKSUM_URL}"
  exit 1
fi

EXPECTED=$(grep -F "  ${ASSET}" "${TMP_DIR}/SHA256SUMS" | awk '{print $1}')
if [[ -z "$EXPECTED" ]]; then
  echo "Error: No checksum entry found for ${ASSET} in SHA256SUMS"
  exit 1
fi

if command -v sha256sum &>/dev/null; then
  ACTUAL=$(sha256sum "${TMP_DIR}/${ASSET}" | awk '{print $1}')
elif command -v shasum &>/dev/null; then
  ACTUAL=$(shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')
else
  echo "Error: Neither sha256sum nor shasum found. Cannot verify binary integrity."
  echo "Install coreutils (Linux) or use Homebrew on macOS."
  exit 1
fi

if [[ "$EXPECTED" != "$ACTUAL" ]]; then
  echo "Error: Checksum mismatch!"
  echo "  Expected: ${EXPECTED}"
  echo "  Got:      ${ACTUAL}"
  exit 1
fi
echo "  ✓ Checksum verified"

# ─── Verify provenance (SLSA) ────────────────────────────────────────────────
#
# This is the strongest check: it verifies the binary was produced by our
# GitHub Actions workflow and has not been tampered with since.
# Requires 'gh' (GitHub CLI). Install: https://cli.github.com

echo "→ Verifying build provenance (GitHub Artifact Attestations)..."
if command -v gh &>/dev/null; then
  if gh attestation verify "${TMP_DIR}/${ASSET}" --repo "${REPO}" >/dev/null 2>&1; then
    echo "  ✓ Provenance verified (SLSA)"
  else
    echo "Error: Provenance verification failed!"
    echo "  The binary was not produced by the official GitHub Actions workflow."
    echo "  This may indicate supply chain tampering. Do not proceed."
    echo "  Report this to the nav-pilot team: https://github.com/${REPO}/issues"
    exit 1
  fi
else
  echo ""
  echo "  ⚠ WARNING: GitHub CLI (gh) not found — skipping provenance verification!"
  echo "  This means the binary's build origin cannot be confirmed."
  echo "  Install gh for full supply chain security: https://cli.github.com"
  echo "  Or install via Homebrew for a verified install: brew install navikt/tap/nav-pilot"
  echo ""
fi

# ─── Install ─────────────────────────────────────────────────────────────────

chmod +x "${TMP_DIR}/${ASSET}"
mv "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BINARY}"

echo "  ✓ Installed nav-pilot to ${INSTALL_DIR}/${BINARY}"

# ─── Install Dependencies (cplt, rtk) ────────────────────────────────────────

echo ""
echo "→ Installing cplt (sandbox)..."
# NOTE: cplt and rtk are installed via their own install scripts without
# provenance verification. These scripts are fetched from external repos
# and are not controlled by nav-pilot's release pipeline.
if curl -fsSL https://raw.githubusercontent.com/navikt/cplt/main/install.sh | bash; then
  echo "  ✓ Installed cplt"
else
  echo "  ⚠ Failed to install cplt"
fi

echo ""
echo "→ Installing rtk (token optimizer)..."
if curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh; then
  echo "  ✓ Installed rtk"
else
  echo "  ⚠ Failed to install rtk"
fi

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
