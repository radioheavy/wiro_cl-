#!/usr/bin/env bash
set -euo pipefail

REPO="${WIRO_CLI_REPO:-radioheavy/wiro_cl-}"
INSTALL_DIR="${WIRO_CLI_INSTALL_DIR:-/usr/local/bin}"
VERSION="${WIRO_CLI_VERSION:-}"

OS_UNAME="$(uname -s)"
ARCH_UNAME="$(uname -m)"

case "${OS_UNAME}" in
  Darwin) ASSET_OS="darwin" ;;
  Linux) ASSET_OS="linux" ;;
  *)
    echo "Unsupported OS: ${OS_UNAME}" >&2
    echo "Use npm install for this platform: npm i -g @toolbridge/wiro-cli" >&2
    exit 1
    ;;
esac

case "${ARCH_UNAME}" in
  x86_64|amd64) ASSET_ARCH="x64" ;;
  arm64|aarch64) ASSET_ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: ${ARCH_UNAME}" >&2
    exit 1
    ;;
esac

if [[ -z "${VERSION}" ]]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\([^"]*\)".*/\1/p' | head -n1)"
fi

if [[ -z "${VERSION}" ]]; then
  echo "Could not resolve latest release version." >&2
  echo "Set WIRO_CLI_VERSION manually, e.g. WIRO_CLI_VERSION=0.1.0" >&2
  exit 1
fi

ASSET="wiro-${ASSET_OS}-${ASSET_ARCH}"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}"
TMP_FILE="$(mktemp)"
trap 'rm -f "${TMP_FILE}"' EXIT

echo "Downloading ${URL}"
curl -fL "${URL}" -o "${TMP_FILE}"
chmod +x "${TMP_FILE}"

TARGET="${INSTALL_DIR}/wiro"
if [[ -w "${INSTALL_DIR}" ]]; then
  install -m 755 "${TMP_FILE}" "${TARGET}"
else
  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 755 "${TMP_FILE}" "${TARGET}"
  else
    echo "No write access to ${INSTALL_DIR} and sudo not found." >&2
    exit 1
  fi
fi

echo "Installed wiro v${VERSION} to ${TARGET}"
echo "Run: wiro"
