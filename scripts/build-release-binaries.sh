#!/usr/bin/env bash
# Cross-compiles the server for the supported platforms and writes
# <name>_<version>_<os>_<arch>.tar.gz archives plus a SHA256SUMS file.
#
# Usage: scripts/build-release-binaries.sh <version> [output-dir]

set -euo pipefail

version="${1:?usage: build-release-binaries.sh <version> [output-dir]}"
output_arg="${2:-dist/binaries}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ ! "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "version must be a filesystem-safe token" >&2
  exit 1
fi

if [[ "$output_arg" = /* ]]; then
  output_dir="$output_arg"
else
  output_dir="$repo_root/$output_arg"
fi

name="infrastructure-mcp"
platforms=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64)

# The output directory is deleted below, so refuse the checkout itself and any
# of its ancestors (for example ".", "..", or "/").
resolved_output="$(mkdir -p "$output_dir" && cd "$output_dir" && pwd -P)"
resolved_root="$(cd "$repo_root" && pwd -P)"
if [[ "$resolved_root/" == "$resolved_output/"* ]]; then
  echo "output directory must not be the repository root or one of its parents: $output_dir" >&2
  exit 1
fi

rm -rf "$output_dir"
mkdir -p "$output_dir"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

cd "$repo_root"
for platform in "${platforms[@]}"; do
  goos="${platform%/*}"
  goarch="${platform#*/}"
  dir="$stage/${name}_${version}_${goos}_${goarch}"
  mkdir -p "$dir"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="-s -w" -o "$dir/$name" ./cmd/server
  cp LICENSE README.md "$dir/"
  cp configs/inventory.example.yaml "$dir/"
  tar -C "$stage" -czf "$output_dir/${name}_${version}_${goos}_${goarch}.tar.gz" \
    "${name}_${version}_${goos}_${goarch}"
done

if command -v sha256sum >/dev/null 2>&1; then
  checksum=(sha256sum)
else
  checksum=(shasum -a 256)
fi
(cd "$output_dir" && "${checksum[@]}" ./*.tar.gz | sed 's# \./# #' > SHA256SUMS)
printf 'Created release binaries in %s\n' "$output_dir"
