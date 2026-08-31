#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=linux-environment.sh
source "${root_dir}/tools/linux-environment.sh"
utautts_load_linux_env "${root_dir}"
utautts_configure_qt_environment

python_command="$(utautts_resolve_python "${root_dir}" || true)"
release_root="${1:-${root_dir}/release}"
gui_zip="${release_root}/UtauTTS-linux-x64.zip"
server_zip="${release_root}/UtauTTS-Server-linux-x64.zip"
temporary_root="$(mktemp -d -t utautts-linux-release-test-XXXXXX)"
server_pid=""

cleanup() {
  if [ -n "${server_pid}" ] && kill -0 "${server_pid}" 2>/dev/null; then
    kill "${server_pid}" 2>/dev/null || true
    wait "${server_pid}" 2>/dev/null || true
  fi
  rm -rf "${temporary_root}"
}
trap cleanup EXIT

fail() {
  echo "$*" >&2
  exit 1
}

has_linux_audio_service() {
  local runtime_directory="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
  if command -v pactl >/dev/null 2>&1; then
    if timeout 5 pactl info >/dev/null 2>&1; then
      return 0
    fi
  fi
  if command -v pw-cli >/dev/null 2>&1; then
    if timeout 5 pw-cli info 0 >/dev/null 2>&1; then
      return 0
    fi
  fi
  if ! command -v pactl >/dev/null 2>&1 && ! command -v pw-cli >/dev/null 2>&1 && \
     { [ -S "${runtime_directory}/pipewire-0" ] || [ -S "${runtime_directory}/pulse/native" ]; }; then
    return 0
  fi
  return 1
}

for command_name in curl fc-list timeout unzip; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "${command_name} is required"
done
[ -n "${python_command}" ] || fail 'python3 is required'
japanese_fonts="$(fc-list :lang=ja 2>/dev/null)"
[ -n "${japanese_fonts}" ] || fail 'a Japanese font is required; install fonts-noto-cjk'
for archive in "${gui_zip}" "${server_zip}"; do
  [ -f "${archive}" ] || fail "release archive is missing: ${archive}"
  unzip -tq "${archive}" >/dev/null
done

gui_root="${temporary_root}/gui"
server_root="${temporary_root}/server"
mkdir -p "${gui_root}" "${server_root}"
unzip -q "${gui_zip}" -d "${gui_root}"
unzip -q "${server_zip}" -d "${server_root}"

for required in \
  "${gui_root}/utautts" \
  "${gui_root}/libutautts_native.so" \
  "${gui_root}/tools/utautts-cli" \
  "${gui_root}/tools/utautts-ustx" \
  "${gui_root}/tools/utautts-updater" \
  "${gui_root}/runtime/utautts-openjtalk-features" \
  "${gui_root}/runtime/utautts-worldline-bridge" \
  "${gui_root}/runtime/libworldline.so" \
  "${server_root}/utautts-server" \
  "${server_root}/manual-pitch.md" \
  "${gui_root}/LICENSE" \
  "${gui_root}/THIRD_PARTY_NOTICES.txt" \
  "${gui_root}/docs/README.md" \
  "${gui_root}/docs/installation.md" \
  "${gui_root}/docs/building.md" \
  "${gui_root}/docs/technical-design.md" \
  "${server_root}/licenses/README.txt" \
  "${gui_root}/licenses/PROSODY-MODELS.txt" \
  "${server_root}/models/README.md" \
  "${gui_root}/runtime/licenses/PYTHON_LICENSE.txt" \
  "${server_root}/runtime/licenses/PYINSTALLER_COPYING.txt" \
  "${server_root}/runtime/utautts-worldline-bridge"; do
  [ -f "${required}" ] || fail "required package file is missing: ${required}"
done
for removed_runtime in libcoreclr.so libhostpolicy.so utautts-worldline-bridge.runtimeconfig.json; do
  [ ! -e "${server_root}/runtime/${removed_runtime}" ] \
    || fail "server package still contains an obsolete worldline runtime file: ${removed_runtime}"
done
for executable in \
  "${gui_root}/utautts" \
  "${gui_root}/tools/utautts-cli" \
  "${gui_root}/tools/utautts-ustx" \
  "${gui_root}/tools/utautts-updater" \
  "${gui_root}/runtime/utautts-openjtalk-features" \
  "${gui_root}/runtime/utautts-worldline-bridge" \
  "${server_root}/utautts-server"; do
  [ -x "${executable}" ] || fail "executable permission is missing: ${executable}"
done
for development_tool in \
  connection-benchmark connection-compare connection-dataset connection-eval \
  connection-lattice connection-train listening-score listening-test oto-inspect \
  prosody-dataset prosody-train utautts-server; do
  [ ! -e "${gui_root}/tools/${development_tool}" ] \
    || fail "GUI release package contains a development-only tool: ${development_tool}"
done

if ldd "${gui_root}/utautts" | grep -q 'not found'; then
  ldd "${gui_root}/utautts" >&2
  fail 'GUI has unresolved shared-library dependencies'
fi

voicebank="$(find "${gui_root}/voice" -mindepth 1 -maxdepth 1 -type d -print -quit)"
[ -n "${voicebank}" ] || fail 'GUI package contains no bundled voicebank'
work_dir="${temporary_root}/work"
mkdir -p "${work_dir}"

if [ "${UTAUTTS_SKIP_GUI_SELF_TEST:-0}" = "1" ]; then
  echo 'Skipping packaged Linux GUI self-test (UTAUTTS_SKIP_GUI_SELF_TEST=1)'
elif has_linux_audio_service || [ "${UTAUTTS_REQUIRE_GUI_SELF_TEST:-0}" = "1" ]; then
  (
    cd "${gui_root}"
    QT_QPA_PLATFORM=offscreen QT_QUICK_BACKEND=software \
      timeout 120 ./utautts --self-test >"${work_dir}/gui.stdout.log" 2>"${work_dir}/gui.stderr.log"
  ) || {
    tail -100 "${work_dir}/gui.stderr.log" >&2 || true
    fail 'packaged Linux GUI self-test failed'
  }
else
  echo 'Skipping packaged Linux GUI self-test: no PipeWire/PulseAudio session was detected'
fi

smoke_text='こんにちは'
"${gui_root}/tools/utautts-cli" --renderer waveform --voicebank "${voicebank}" \
  --text "${smoke_text}" --out "${work_dir}/waveform.wav"
"${gui_root}/tools/utautts-cli" --voicebank "${voicebank}" --text "${smoke_text}" \
  --prosody frame-intonation-v8 --renderer openutau-classic-worldline-faithful \
  --apply-pitch --intonation-strength 1 --out "${work_dir}/production.wav"
"${gui_root}/tools/utautts-cli" --voicebank "${voicebank}" --text "${smoke_text}" \
  --prosody frame-intonation-v8 --renderer openutau-worldline-r-faithful \
  --apply-pitch --intonation-strength 1 --out "${work_dir}/worldline-r.wav"
for wav in "${work_dir}/waveform.wav" "${work_dir}/production.wav" "${work_dir}/worldline-r.wav"; do
  [ "$(stat -c %s "${wav}")" -gt 44 ] || fail "synthesis output is empty: ${wav}"
done
if "${gui_root}/tools/utautts-cli" --renderer waveform --voicebank "${voicebank}" \
    --text "${smoke_text}" --mora-ms NaN --out "${work_dir}/nan.wav" \
    >"${work_dir}/nan.stdout.log" 2>"${work_dir}/nan.stderr.log"; then
  fail 'packaged CLI accepted NaN input'
fi
if grep -qi panic "${work_dir}/nan.stderr.log"; then
  fail 'packaged CLI panicked on NaN input'
fi

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
  kill -0 "${server_pid}" 2>/dev/null || {
    cat "${work_dir}/server.stderr.log" >&2
    fail 'packaged Linux server exited during startup'
  }
  sleep 0.1
done
curl -fsS "${base_url}/api/health" >/dev/null || fail 'packaged Linux server health check timed out'
curl -fsS "${base_url}/" | grep -q 'UtauTTS Server Console' || fail 'server console is unavailable'
curl -fsS "${base_url}/api/voicebanks" >"${work_dir}/voicebanks.json"
curl -fsS "${base_url}/api/models" >"${work_dir}/models.json"
curl -fsS "${base_url}/api/renderers" >"${work_dir}/renderers.json"

VOICEBANK_JSON="${work_dir}/voicebanks.json" REQUEST_DIR="${work_dir}" "${python_command}" - <<'PY'
import json
import os
from pathlib import Path

root = Path(os.environ["REQUEST_DIR"])
with open(os.environ["VOICEBANK_JSON"], encoding="utf-8") as stream:
    voices = json.load(stream).get("voicebanks", [])
if not voices:
    raise SystemExit("server returned no voicebank")
voice = voices[0]["id"]
(root / "analyze.json").write_text(json.dumps({"text": "こんにちは"}, ensure_ascii=False), encoding="utf-8")
(root / "synthesize.json").write_text(json.dumps({
    "text": "こんにちは", "voicebank_id": voice, "renderer": "waveform", "mora_duration_ms": 120
}, ensure_ascii=False), encoding="utf-8")
(root / "faithful.json").write_text(json.dumps({
    "text": "こんにちは", "voicebank_id": voice,
    "model_id": "frame-intonation-v8",
    "renderer": "openutau-classic-worldline-faithful",
    "intonation_strength": 1, "apply_pitch": True,
}, ensure_ascii=False), encoding="utf-8")
(root / "batch.json").write_text(json.dumps({"items": [
    {"name": "first.wav", "request": {"text": "こんにちは", "voicebank_id": voice, "renderer": "waveform"}},
    {"name": "second.wav", "request": {"kana": "あ", "voicebank_id": voice, "renderer": "waveform"}}
]}, ensure_ascii=False), encoding="utf-8")
PY

curl -fsS -H 'Content-Type: application/json; charset=utf-8' --data-binary @"${work_dir}/analyze.json" \
  "${base_url}/api/analyze" >"${work_dir}/analysis.json"
ANALYSIS_JSON="${work_dir}/analysis.json" MODELS_JSON="${work_dir}/models.json" \
RENDERERS_JSON="${work_dir}/renderers.json" "${python_command}" - <<'PY'
import json
import os

def load(name):
    with open(os.environ[name], encoding="utf-8") as stream:
        return json.load(stream)

analysis = load("ANALYSIS_JSON")
if not analysis.get("reading") or not analysis.get("morae"):
    raise SystemExit("server analysis returned no reading")
if not load("MODELS_JSON").get("models"):
    raise SystemExit("server returned no model")
if not load("RENDERERS_JSON").get("renderers"):
    raise SystemExit("server returned no renderer")
PY
curl -fsS -H 'Content-Type: application/json; charset=utf-8' --data-binary @"${work_dir}/synthesize.json" \
  "${base_url}/api/synthesize/audio" >"${work_dir}/server.wav"
[ "$(stat -c %s "${work_dir}/server.wav")" -gt 44 ] || fail 'server synthesis output is empty'
curl -fsS -H 'Content-Type: application/json; charset=utf-8' --data-binary @"${work_dir}/faithful.json" \
  "${base_url}/api/synthesize/audio" >"${work_dir}/server-faithful.wav"
[ "$(stat -c %s "${work_dir}/server-faithful.wav")" -gt 44 ] || fail 'server faithful synthesis output is empty'
curl -fsS -H 'Content-Type: application/json; charset=utf-8' --data-binary @"${work_dir}/batch.json" \
  "${base_url}/api/synthesize/batch" >"${work_dir}/batch.zip"
unzip -tq "${work_dir}/batch.zip" >/dev/null
curl -fsS -X POST -H 'Content-Type: application/json' --data '{}' \
  "${base_url}/api/voicebanks/reload" >"${work_dir}/reload.json"

kill "${server_pid}"
wait "${server_pid}" || true
server_pid=""
echo 'Linux release package smoke test passed'
