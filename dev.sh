#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "${root_dir}/tools/linux-environment.sh"
utautts_load_linux_env "${root_dir}"

go_command="$(utautts_resolve_go "${root_dir}" || true)"
[ -n "${go_command}" ] || { echo 'go is required; run tools/setup-linux.sh' >&2; exit 1; }

voice_dir="${UTAUTTS_VOICE_DIR:-${root_dir}/sample}"
cd "${root_dir}"
echo '=== UtauTTS Dev Server ==='
echo
echo 'Open http://127.0.0.1:8080'
echo

exec "${go_command}" run ./cmd/utautts-server --voice-dir "${voice_dir}" "$@"
