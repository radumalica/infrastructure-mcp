#!/usr/bin/env bash

set -euo pipefail

version="${1:-dev}"
output_arg="${2:-dist/skills}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
skills_root="$repo_root/.github/skills"

if [[ ! "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "version must be a filesystem-safe token" >&2
  exit 1
fi
if ! command -v zip >/dev/null 2>&1; then
  echo "zip is required to package agent skills" >&2
  exit 1
fi

if [[ "$output_arg" = /* ]]; then
  output_dir="$output_arg"
else
  output_dir="$repo_root/$output_arg"
fi

skills=(
  infrastructure-mcp-operator
)

mkdir -p "$output_dir"
find "$output_dir" -maxdepth 1 -type f -name '*.zip' -delete

pushd "$skills_root" >/dev/null
for skill in "${skills[@]}"; do
  files=(
    "$skill/SKILL.md"
    "$skill/references/workflow.md"
  )
  if find "$skill" -type l -print -quit | grep -q .; then
    echo "skill packages cannot contain symlinks: $skill" >&2
    exit 1
  fi
  mapfile -t actual_files < <(find "$skill" -type f -printf '%p\n' | sort)
  mapfile -t expected_files < <(printf '%s\n' "${files[@]}" | sort)
  if [[ "${actual_files[*]}" != "${expected_files[*]}" ]]; then
    echo "public skill files do not match the packaging manifest: $skill" >&2
    printf 'expected: %s\n' "${expected_files[*]}" >&2
    printf 'actual: %s\n' "${actual_files[*]}" >&2
    exit 1
  fi
  archive="$output_dir/${skill}-${version}.zip"
  zip -q "$archive" "${files[@]}"

  if unzip -Z1 "$archive" | grep -Ev "^${skill}/" | grep -q .; then
    echo "archive contains an unexpected top-level path: $archive" >&2
    exit 1
  fi
done
popd >/dev/null

printf 'Created public skill archive in %s\n' "$output_dir"
