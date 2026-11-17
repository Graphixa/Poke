#!/bin/sh
#
# poke Installer Script
# Installs poke (+ pokedex alias) on Linux and macOS
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --dry-run
#   POKE_VERSION=1.0.0 curl -fsSL .../install.sh | sh
#
# Environment:
#   POKE_VERSION          Version or latest (default: latest)
#   POKE_INSTALL_DIR      Install directory (default: ~/.local/bin)
#   POKE_NONINTERACTIVE=1 Skip "Continue?" (non-interactive install)
#   POKE_DRY_RUN=1        Print plan only; no download or install (same as --dry-run)
#   CI                       When non-empty, prompt is skipped (common on CI runners)
#   NO_COLOR=1               Disable ANSI colors
#

set -e

DRY_RUN=false
for _arg in "$@"; do
    case "$_arg" in
        --dry-run|--dryrun)
            DRY_RUN=true
            ;;
        --help|-h)
            cat <<'EOF'
poke install.sh

  curl -fsSL https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.sh | sh
  curl -fsSL .../install.sh | sh -s -- --dry-run

Flags:
  --dry-run       Print URLs and paths only; no download or install
  --help, -h      This message

Environment:
  POKE_VERSION, POKE_INSTALL_DIR, POKE_NONINTERACTIVE=1, POKE_DRY_RUN=1
  NO_COLOR=1
EOF
            exit 0
            ;;
    esac
done
case "${POKE_DRY_RUN:-}" in 1|true|TRUE|yes|YES) DRY_RUN=true ;; esac

# Colors (respect NO_COLOR). printf gives real ESC; '\033' in quotes is literal.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    ESC=$(printf '\033')
    RED="${ESC}[0;31m"
    GREEN="${ESC}[0;32m"
    YELLOW="${ESC}[1;33m"
    BLUE="${ESC}[94m"
    NC="${ESC}[0m"
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

REPO="Graphixa/Poke"
REPO_URL="https://github.com/${REPO}"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
    echo "${RED}Error: Unsupported operating system: $OS${NC}" >&2
    echo "This installer supports Linux and macOS only. On Windows use install.ps1 or winget." >&2
    exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "${RED}Error: Unsupported architecture: $ARCH${NC}" >&2
        exit 1
        ;;
esac

VERSION="${POKE_VERSION:-latest}"
DISPLAY_VERSION="$VERSION"
if [ "$VERSION" != "latest" ]; then
    DISPLAY_VERSION=$(echo "$VERSION" | sed 's/^v//')
fi

if [ "$VERSION" = "latest" ]; then
    BASE_URL="${REPO_URL}/releases/latest/download"
else
    VERSION=$(echo "$VERSION" | sed 's/^v//')
    BASE_URL="${REPO_URL}/releases/download/v${VERSION}"
fi

# GoReleaser archives: poke_<version>_<os>_<arch>.tar.gz (see .goreleaser.yaml)
if [ "$VERSION" = "latest" ]; then
    ARCHIVE_NAME=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep -oE "poke_[^\"[:space:]]+_${OS}_${ARCH}\\.tar\\.gz" | head -n1)
    if [ -z "$ARCHIVE_NAME" ]; then
        echo "${RED}Error: Could not find release archive for ${OS}/${ARCH}${NC}" >&2
        exit 1
    fi
else
    ARCHIVE_NAME="poke_${VERSION}_${OS}_${ARCH}.tar.gz"
fi

DOWNLOAD_URL="${BASE_URL}/${ARCHIVE_NAME}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"
INSTALL_DIR="${POKE_INSTALL_DIR:-${HOME}/.local/bin}"
INSTALLED_BIN="${INSTALL_DIR}/poke"
INSTALLED_ALIAS="${INSTALL_DIR}/pokedex"

echo "${BLUE}OS:${NC} ${OS} ${BLUE}|${NC} ${BLUE}ARCH:${NC} ${ARCH}"
echo ""
echo "This will install poke ${DISPLAY_VERSION} to ${INSTALLED_BIN}"
echo "  (and alias pokedex at ${INSTALLED_ALIAS})"
echo ""

# Existing install notice before prompt
if [ -f "$INSTALLED_BIN" ]; then
    CURRENT_VERSION=$("$INSTALLED_BIN" version 2>/dev/null | head -n1 || echo "unknown")
    echo "${YELLOW}poke is already installed at: $INSTALLED_BIN${NC}"
    echo "${YELLOW}Current version: ${CURRENT_VERSION}${NC}"
    echo "${YELLOW}This will be overwritten.${NC}"
    echo ""
fi

if [ "$DRY_RUN" = true ]; then
    echo "${BLUE}[dry-run] No download or install will be performed.${NC}"
    echo "${BLUE}[dry-run] Download:${NC} ${DOWNLOAD_URL}"
    echo "${BLUE}[dry-run] Checksums:${NC} ${CHECKSUMS_URL}"
    echo "${BLUE}[dry-run] Install to:${NC} ${INSTALLED_BIN} + ${INSTALLED_ALIAS}"
    exit 0
fi

# Continue prompt: only when interactive; CI / non-TTY / NONINTERACTIVE skip
SHOULD_PROMPT=true
if [ "${POKE_NONINTERACTIVE:-0}" = "1" ]; then
    SHOULD_PROMPT=false
elif [ ! -t 0 ] || [ ! -t 1 ]; then
    SHOULD_PROMPT=false
elif [ -n "${CI:-}" ]; then
    SHOULD_PROMPT=false
fi

if [ "$SHOULD_PROMPT" = true ]; then
    printf "Continue? [y/N] "
    read -r REPLY || REPLY=n
    case "$REPLY" in
        [yY]|[yY][eE][sS]) ;;
        *)
            echo "${YELLOW}Cancelled.${NC}"
            exit 0
            ;;
    esac
    echo ""
fi

mkdir -p "$INSTALL_DIR"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

LOCAL_ARCHIVE="${TMP_DIR}/${ARCHIVE_NAME}"
EXTRACT_DIR="${TMP_DIR}/extract"
LOCAL_BIN="${EXTRACT_DIR}/poke"
LOCAL_ALIAS="${EXTRACT_DIR}/pokedex"

echo "${BLUE}Downloading poke...${NC}"
if ! curl -fsSL "$DOWNLOAD_URL" -o "$LOCAL_ARCHIVE"; then
    echo "${RED}Error: Failed to download poke${NC}" >&2
    if [ "${DISPLAY_VERSION}" != "latest" ]; then
        echo "${YELLOW}Version v${DISPLAY_VERSION} may not exist. Check available versions at:${NC}"
        echo "${BLUE}${REPO_URL}/releases${NC}"
    fi
    exit 1
fi

echo "${BLUE}Downloading checksums...${NC}"
if ! curl -fsSL "$CHECKSUMS_URL" -o "${TMP_DIR}/checksums.txt"; then
    echo "${RED}Error: Failed to download checksums.txt${NC}" >&2
    exit 1
fi

EXPECTED=""
EXPECTED=$(tr -d '\r' < "${TMP_DIR}/checksums.txt" | grep -E "[[:space:]]${ARCHIVE_NAME}\$" 2>/dev/null | head -n1 | awk '{print $1}' || true)
if [ -z "$EXPECTED" ]; then
    echo "${RED}Error: No checksum line for ${ARCHIVE_NAME} in checksums.txt${NC}" >&2
    exit 1
fi

ACTUAL=""
if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "$LOCAL_ARCHIVE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL=$(shasum -a 256 "$LOCAL_ARCHIVE" | awk '{print $1}')
else
    ACTUAL=$(openssl dgst -sha256 "$LOCAL_ARCHIVE" | awk '{print $NF}')
fi

EXL=$(printf '%s' "$EXPECTED" | tr '[:upper:]' '[:lower:]')
ACL=$(printf '%s' "$ACTUAL" | tr '[:upper:]' '[:lower:]')
if [ "$EXL" != "$ACL" ]; then
    echo "${RED}Error: Checksum mismatch for ${ARCHIVE_NAME}${NC}" >&2
    exit 1
fi

echo "${GREEN}✓ Checksum verified${NC}"

mkdir -p "$EXTRACT_DIR"
if ! tar -xzf "$LOCAL_ARCHIVE" -C "$EXTRACT_DIR"; then
    echo "${RED}Error: Failed to extract ${ARCHIVE_NAME}${NC}" >&2
    exit 1
fi
if [ ! -f "$LOCAL_BIN" ]; then
    echo "${RED}Error: poke binary not found in archive${NC}" >&2
    exit 1
fi
if [ ! -f "$LOCAL_ALIAS" ]; then
    echo "${RED}Error: pokedex binary not found in archive${NC}" >&2
    exit 1
fi

chmod +x "$LOCAL_BIN" "$LOCAL_ALIAS"

if ! "$LOCAL_BIN" version >/dev/null 2>&1; then
    echo "${RED}Error: Downloaded binary appears to be invalid${NC}" >&2
    exit 1
fi

echo "${BLUE}Installing to ${INSTALL_DIR}...${NC}"
mv "$LOCAL_BIN" "$INSTALLED_BIN"
mv "$LOCAL_ALIAS" "$INSTALLED_ALIAS"

INSTALLED_VERSION=$("$INSTALLED_BIN" version 2>/dev/null | head -n1 || echo "poke")
echo ""
echo "${GREEN}✓ poke installed successfully!${NC}"
echo ""
echo "  Location: ${INSTALLED_BIN}"
echo "  Alias:    ${INSTALLED_ALIAS}"
echo "  Version:  ${INSTALLED_VERSION}"
echo ""

case ":$PATH:" in
    *:"${INSTALL_DIR}":*)
        echo "${GREEN}✓ ${INSTALL_DIR} is already in your PATH${NC}"
        ;;
    *)
        echo "${YELLOW}⚠ ${INSTALL_DIR} is not in your PATH${NC}"
        echo ""
        echo "To use poke, add this to your shell profile:"
        if [ -n "$ZSH_VERSION" ]; then
            echo "  ${BLUE}echo 'export PATH=\"${INSTALL_DIR}:\${PATH}\"' >> ~/.zshrc${NC}"
            echo "  ${BLUE}source ~/.zshrc${NC}"
        elif [ -n "$BASH_VERSION" ]; then
            echo "  ${BLUE}echo 'export PATH=\"${INSTALL_DIR}:\${PATH}\"' >> ~/.bashrc${NC}"
            echo "  ${BLUE}source ~/.bashrc${NC}"
        elif [ -n "$FISH_VERSION" ]; then
            echo "  ${BLUE}set -U fish_user_paths ${INSTALL_DIR} \$fish_user_paths${NC}"
            echo "  ${BLUE}exec fish${NC}"
        else
            echo "  ${BLUE}export PATH=\"${INSTALL_DIR}:\${PATH}\"${NC}"
        fi
        echo ""
        echo "Or run poke directly:"
        echo "  ${BLUE}${INSTALLED_BIN} --help${NC}"
        ;;
esac

echo ""
echo "${GREEN}You can now use 'poke' (or 'pokedex'):${NC}"
echo "  ${BLUE}poke --help${NC}"
echo "  ${BLUE}poke refresh${NC}"
echo "  ${BLUE}poke update${NC}"
echo ""
