#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=linux-environment.sh
source "${root_dir}/tools/linux-environment.sh"
utautts_load_linux_env "${root_dir}"

go_command="$(utautts_resolve_go "${root_dir}" || true)"
[ -n "${go_command}" ] || { echo 'go is required; run tools/setup-linux.sh' >&2; exit 1; }

export GOCACHE="${GOCACHE:-${root_dir}/build/go-cache}"
export GOMODCACHE="${GOMODCACHE:-${root_dir}/build/go-mod-cache}"
cd "${root_dir}"

"${go_command}" test ./...
"${go_command}" vet ./...
