#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
package_dir="${1:?package directory is required}"
go_command="${2:-go}"

cd "${root_dir}"
exec "${go_command}" run ./cmd/tools/collect-go-licenses \
  --package-dir "${package_dir}" --root "${root_dir}" --go "${go_command}"
