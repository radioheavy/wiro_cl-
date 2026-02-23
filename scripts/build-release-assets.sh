#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-${ROOT_DIR}/dist}"

mkdir -p "${OUT_DIR}"

build_one() {
  local goos="$1"
  local asset_os="$2"
  local goarch="$3"
  local arch_label="$4"
  local ext="$5"

  local out="${OUT_DIR}/wiro-${asset_os}-${arch_label}${ext}"
  echo "Building ${out}"
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    go build -trimpath -o "${out}" "${ROOT_DIR}/cmd/wiro"

  if [[ "${ext}" != ".exe" ]]; then
    chmod +x "${out}"
  fi
}

build_one darwin darwin amd64 x64 ""
build_one darwin darwin arm64 arm64 ""
build_one linux linux amd64 x64 ""
build_one linux linux arm64 arm64 ""
build_one windows win32 amd64 x64 ".exe"
build_one windows win32 arm64 arm64 ".exe"

echo "Release assets generated under ${OUT_DIR}"
