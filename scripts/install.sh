#!/bin/bash
set -euo pipefail

# nav-pilot installer — installs the nav-pilot CLI.
#
# On macOS: uses Homebrew (brew install navikt/tap/nav-pilot navikt/tap/cplt) when available.
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

# ─── Homebrew install (macOS) ────────────────────────────────────────────────
#
# Homebrew is only used on macOS. Homebrew on Linux exists, but navikt/tap/cplt
# ships macOS-only bottles, so `brew install` aborts the whole install with
# "formula requires at least a URL". Linux falls through to the download path
# below, where cplt's own installer provides a Linux binary.

if [[ "$OS" == "darwin" && "$NO_BREW" == false && -z "$VERSION" && -z "$INSTALL_DIR" ]] && command -v brew &>/dev/null; then
  echo "→ Installing via Homebrew..."
  brew install navikt/tap/nav-pilot navikt/tap/cplt
  echo ""
  INSTALLED_VERSION=$(nav-pilot version 2>/dev/null || echo "unknown")
  echo "✓ nav-pilot is ready! (${INSTALLED_VERSION})"
  echo ""
  echo "Get started: run nav-pilot in a git repo to start the setup wizard."
  echo ""
  echo "Upgrade later with: brew upgrade navikt/tap/nav-pilot"
  exit 0
fi

# ─── Security notice ────────────────────────────────────────────────────────
#
# WARNING: Piping curl to bash (curl | bash) means the script itself is not
# verified before execution. If the GitHub CDN or repository is compromised,
# this script could be replaced before the binary checks run.
#
# For the strongest supply chain security, install via Homebrew:
#   brew install navikt/tap/nav-pilot navikt/tap/cplt
#
# On Linux/CI, download and inspect the script before running:
#   curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh -o install.sh
#   cat install.sh  # Inspect before running!
#   bash install.sh

# ─── Resolve version ─────────────────────────────────────────────────────────

if [[ -z "$VERSION" ]]; then
  echo "→ Fetching latest nav-pilot release..."
  # Filter by nav-pilot/ tag prefix to avoid picking up unrelated releases (e.g. skills)
  set +o pipefail
  # Authenticate when a token is available. Anonymous api.github.com allows 60
  # requests per hour per IP, and a CI runner shares that IP with everyone else
  # on the same host — so this returned 403 and the installer reported "could
  # not determine latest version", which reads like a broken release rather
  # than a rate limit. It blocked every pull request in this repository.
  #
  # Both spellings are accepted: the workflow sets GITHUB_TOKEN, the script's
  # own error message tells people to set GH_TOKEN, and gh sets GH_TOKEN. All
  # three were in play and none of them reached this call.
  API_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
  # The header goes in on stdin, not in the argument list. Anything passed as an
  # argument shows up in `ps` for every user on the machine, and this script is
  # run on shared runners.
  auth_config() {
    [[ -n "$API_TOKEN" ]] && printf 'header = "Authorization: Bearer %s"\n' "$API_TOKEN"
    return 0
  }
  RELEASES=$(auth_config | curl -fsSL -K - "https://api.github.com/repos/${REPO}/releases?per_page=100")
  VERSION=$(printf '%s' "$RELEASES" \
    | tr ',' '\n' \
    | awk -v asset="$ASSET" '
        # Per release object: tag_name comes first, draft, prerelease and
        # assets later. Skip drafts (a token with write access lists them
        # first, and their assets 404) and prereleases, and releases without
        # this platform'\''s binary: a release run that died mid-upload
        # published one (#2057). Escaped quotes in body text do not match.
        function emit() {
          if (done || tag !~ /^nav-pilot\// || skip) return
          if (!has) { if (missing == "") missing = tag; return }
          if (missing != "") print "→ " missing " has no " asset " yet, so installing " tag > "/dev/stderr"
          print tag; done = 1; exit
        }
        /(^|[^\\])"tag_name": *"/ { emit(); tag = $0; sub(/.*"tag_name": *"/, "", tag); sub(/".*/, "", tag); skip = 0; has = 0 }
        /(^|[^\\])"(draft|prerelease)": *true/ { skip = 1 }
        $0 ~ "(^|[^\\\\])\"name\": *\"" asset "\"" { has = 1 }
        END { emit() }')
  set -o pipefail
  if [[ -n "$RELEASES" && -z "$VERSION" ]]; then
    echo "Error: No recent nav-pilot release has ${ASSET}, the binary for ${OS}/${ARCH}."
    echo "  Try again later, or pass --version with a release that has it."
    exit 1
  fi
  if [[ -z "$VERSION" ]]; then
    echo "Error: Could not determine latest version."
    if [[ -z "$API_TOKEN" ]]; then
      echo "  No GH_TOKEN or GITHUB_TOKEN was set, so the request was anonymous."
      echo "  Anonymous api.github.com is limited to 60 requests per hour per IP,"
      echo "  and a shared network or CI runner reaches that quickly."
    fi
    echo "  Pass --version to skip the lookup entirely."
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
    # A plain string match: `echo | grep -q` loses the match to SIGPIPE under
    # pipefail (#1318), and ${PATH//:/...} is cubic in macOS bash 3.2, so a
    # long PATH takes minutes (#2019).
    if [[ ":${PATH}:" == *":${dir}:"* ]]; then
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
  VERIFY_ERR="${TMP_DIR}/verify_err.txt"
  if gh attestation verify "${TMP_DIR}/${ASSET}" --repo "${REPO}" >/dev/null 2>"$VERIFY_ERR"; then
    echo "  ✓ Provenance verified (SLSA)"
  else
    ERR_MSG=$(cat "$VERIFY_ERR")
    if grep -Eiq "auth|token|login|credential|sign in" <<<"$ERR_MSG"; then
      echo ""
      echo "  ⚠ WARNING: GitHub CLI (gh) is not authenticated — skipping provenance verification!"
      echo "  Error from gh: $(echo "$ERR_MSG" | tr '\n' ' ')"
      echo "  To enable provenance verification:"
      echo "    - In GitHub Actions: pass env: GH_TOKEN: \${{ secrets.GITHUB_TOKEN }}"
      echo "    - Locally: set GH_TOKEN, or run 'gh auth login' outside a cplt sandbox"
      echo "      ('gh auth login' is blocked by cplt's gh guard, on by default since cplt#335)."
      echo ""
    else
      echo "Error: Provenance verification failed!"
      echo "  The binary was not produced by the official GitHub Actions workflow."
      echo "  This may indicate supply chain tampering. Do not proceed."
      echo "  Report this to the nav-pilot team: https://github.com/${REPO}/issues"
      if [[ -n "$ERR_MSG" ]]; then
        echo "  Details: $ERR_MSG"
      fi
      exit 1
    fi
  fi
else
  echo ""
  echo "  ⚠ WARNING: GitHub CLI (gh) not found — skipping provenance verification!"
  echo "  This means the binary's build origin cannot be confirmed."
  echo "  Install gh for full supply chain security: https://cli.github.com"
  echo "  Or use Homebrew, which verifies the download: brew install navikt/tap/nav-pilot navikt/tap/cplt"
  echo ""
fi

# ─── Install ─────────────────────────────────────────────────────────────────

chmod +x "${TMP_DIR}/${ASSET}"
mv "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BINARY}"

echo "  ✓ Installed nav-pilot to ${INSTALL_DIR}/${BINARY}"

# ─── Install Dependencies (cplt) ─────────────────────────────────────────────

echo ""
echo "→ Installing cplt (sandbox)..."
# NOTE: cplt is installed via its own install script without provenance
# verification. The script is fetched from navikt/cplt and is not controlled by
# nav-pilot's release pipeline.
if curl -fsSL https://raw.githubusercontent.com/navikt/cplt/main/install.sh | bash; then
  echo "  ✓ Installed cplt"
else
  echo "  ⚠ Failed to install cplt"
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
echo "Get started: run nav-pilot in a git repo to start the setup wizard."
