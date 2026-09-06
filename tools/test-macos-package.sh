#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release_root="${1:-${root_dir}/release}"
gui_zip="${release_root}/UtauTTS-macos-arm64.zip"
server_zip="${release_root}/UtauTTS-Server-macos-arm64.zip"
python_command="${PYTHON:-python3}"
temporary_root="$(mktemp -d -t utautts-macos-release-test.XXXXXX)"
server_pid=""

cleanup() {
  if [[ -n "${server_pid}" ]] && kill -0 "${server_pid}" >/dev/null 2>&1; then
    kill "${server_pid}" >/dev/null 2>&1 || true
    wait "${server_pid}" >/dev/null 2>&1 || true
  fi
  rm -rf "${temporary_root}"
}
trap cleanup EXIT

fail() {
  echo "$*" >&2
  exit 1
}

file_size() {
  wc -c < "$1" | tr -d ' '
}

for command_name in unzip curl "${python_command}"; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "${command_name} is required"
done
for archive in "${gui_zip}" "${server_zip}"; do
  [[ -f "${archive}" ]] || fail "release archive is missing: ${archive}"
  unzip -tq "${archive}" >/dev/null
done

gui_root="${temporary_root}/gui"
server_root="${temporary_root}/server"
mkdir -p "${gui_root}" "${server_root}"
(cd "${gui_root}" && unzip -q "${root_dir}/${gui_zip#${root_dir}/}")
(cd "${server_root}" && unzip -q "${root_dir}/${server_zip#${root_dir}/}")

app="${gui_root}/UtauTTS.app"
cli="${gui_root}/tools/utautts-cli"
for required in \
  "${app}/Contents/MacOS/utautts" \
  "${app}/Contents/Frameworks/libutautts_native.dylib" \
  "${cli}" \
  "${gui_root}/tools/utautts-ustx" \
  "${gui_root}/tools/utautts-updater" \
  "${gui_root}/runtime/utautts-openjtalk-features" \
  "${gui_root}/runtime/utautts-worldline-bridge" \
  "${gui_root}/runtime/utautts-world-engine.dylib" \
  "${server_root}/utautts-server" \
  "${server_root}/runtime/utautts-world-engine.dylib" \
  "${gui_root}/models/frame-intonation-v8.json" \
  "${gui_root}/renderer/waveform/renderer.json" \
  "${gui_root}/renderer/utautts-world-phrase/renderer.json"; do
  [[ -f "${required}" ]] || fail "required package file is missing: ${required}"
done
for removed in \
  "${gui_root}/renderer/openutau-worldline-r-faithful" \
  "${gui_root}/renderer/diffsinger" \
  "${gui_root}/renderer/utautts-world-phrase-cuda"; do
  [[ ! -e "${removed}" ]] || fail "initial macOS package contains an excluded renderer: ${removed}"
done
for executable in \
  "${app}/Contents/MacOS/utautts" "${cli}" \
  "${gui_root}/tools/utautts-ustx" "${gui_root}/tools/utautts-updater" \
  "${gui_root}/runtime/utautts-openjtalk-features" \
  "${gui_root}/runtime/utautts-worldline-bridge" "${server_root}/utautts-server"; do
  [[ -x "${executable}" ]] || fail "executable permission is missing: ${executable}"
done

voicebank="$(find "${gui_root}/voice" -mindepth 1 -maxdepth 1 -type d -print -quit)"
[[ -n "${voicebank}" ]] || fail 'GUI package contains no bundled voicebank'
work_dir="${temporary_root}/work"
mkdir -p "${work_dir}"

app_binary="${app}/Contents/MacOS/utautts"
(
  cd "${gui_root}"
  QT_QPA_PLATFORM=cocoa QT_QUICK_BACKEND=software \
    "${app_binary}" --self-test >"${work_dir}/gui.stdout.log" 2>"${work_dir}/gui.stderr.log"
) || {
  tail -100 "${work_dir}/gui.stderr.log" >&2 || true
  fail 'packaged macOS GUI self-test failed'
}

smoke_text='こんにちは'
"${cli}" --renderer waveform --voicebank "${voicebank}" \
  --text "${smoke_text}" --out "${work_dir}/waveform.wav"
"${cli}" --voicebank "${voicebank}" --text "${smoke_text}" \
  --prosody frame-intonation-v8 --renderer utautts-world-phrase \
  --apply-pitch --intonation-strength 1 --out "${work_dir}/utautts-world.wav"
for wav in "${work_dir}/waveform.wav" "${work_dir}/utautts-world.wav"; do
  [[ "$(file_size "${wav}")" -gt 44 ]] || fail "synthesis output is empty: ${wav}"
done

port="$("${python_command}" -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')"
(
  cd "${server_root}"
  exec ./utautts-server --host 127.0.0.1 --port "${port}" \
    --voice-dir "${gui_root}/voice" --renderer waveform
) >"${work_dir}/server.stdout.log" 2>"${work_dir}/server.stderr.log" &
server_pid=$!
base_url="http://127.0.0.1:${port}"
for _ in $(seq 1 100); do
  if curl -fsS "${base_url}/api/health" >"${work_dir}/health.json" 2>/dev/null; then
    break
  fi
  if ! kill -0 "${server_pid}" >/dev/null 2>&1; then
    cat "${work_dir}/server.stderr.log" >&2
    fail 'packaged macOS server exited during startup'
  fi
  sleep 0.1
done
curl -fsS "${base_url}/api/health" >/dev/null || fail 'packaged macOS server health check timed out'
curl -fsS "${base_url}/api/renderers" >"${work_dir}/renderers.json"
"${python_command}" - "${work_dir}/renderers.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    renderers = json.load(stream).get("renderers", [])
ids = {item.get("id") for item in renderers}
for required in ("waveform", "utautts-world-phrase"):
    if required not in ids:
        raise SystemExit(f"server did not expose renderer: {required}")
PY

kill "${server_pid}"
wait "${server_pid}" || true
server_pid=""
echo 'macOS release package smoke test passed'
