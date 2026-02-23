#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/prepare-manual-release.sh <version>

Example:
  ./scripts/prepare-manual-release.sh 0.1.2

What it does:
  1) Runs go test ./...
  2) Builds release binaries under dist/release-v<version>/
  3) Writes SHA256SUMS for release assets
  4) Syncs npm/package.json version to <version>

Then it prints next manual commands for:
  - git commit/tag
  - GitHub release upload
  - npm publish
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

VERSION="${1:-}"
if [[ -z "${VERSION}" ]]; then
  usage
  exit 1
fi

if ! [[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid version: ${VERSION}" >&2
  echo "expected semver like: 0.1.2" >&2
  exit 1
fi

OUT_DIR="${ROOT_DIR}/dist/release-v${VERSION}"
ASSETS=(
  "wiro-darwin-x64"
  "wiro-darwin-arm64"
  "wiro-linux-x64"
  "wiro-linux-arm64"
  "wiro-win32-x64.exe"
  "wiro-win32-arm64.exe"
)

echo "==> Running tests"
(
  cd "${ROOT_DIR}"
  go test ./...
)

echo "==> Building release assets in ${OUT_DIR}"
rm -rf "${OUT_DIR}"
"${ROOT_DIR}/scripts/build-release-assets.sh" "${OUT_DIR}"

echo "==> Verifying assets"
for asset in "${ASSETS[@]}"; do
  if [[ ! -f "${OUT_DIR}/${asset}" ]]; then
    echo "missing release asset: ${OUT_DIR}/${asset}" >&2
    exit 1
  fi
done

echo "==> Writing checksums"
(
  cd "${OUT_DIR}"
  shasum -a 256 "${ASSETS[@]}" > SHA256SUMS
)

echo "==> Syncing npm package version to ${VERSION}"
(
  cd "${ROOT_DIR}/npm"
  npm version --no-git-tag-version "${VERSION}" >/dev/null
)

cat <<EOF

Prepared release files:
  ${OUT_DIR}

Next manual steps:
1) Commit changes:
   git add -A
   git commit -m "release: v${VERSION}"

2) Create and push tag:
   git tag v${VERSION}
   git push origin main --tags

3) Create or update GitHub release (manual asset upload):
   gh release create v${VERSION} \\
     "${OUT_DIR}/wiro-darwin-x64" \\
     "${OUT_DIR}/wiro-darwin-arm64" \\
     "${OUT_DIR}/wiro-linux-x64" \\
     "${OUT_DIR}/wiro-linux-arm64" \\
     "${OUT_DIR}/wiro-win32-x64.exe" \\
     "${OUT_DIR}/wiro-win32-arm64.exe" \\
     "${OUT_DIR}/SHA256SUMS" \\
     --title "v${VERSION}" --generate-notes

   If release already exists:
   gh release upload v${VERSION} "${OUT_DIR}"/* --clobber

4) Publish npm package manually:
   cd npm
   npm publish
EOF
