#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release_root="${1:-${root_dir}/release}"
gui_dir="${release_root}/UtauTTS-linux"
server_dir="${release_root}/UtauTTS-Server-linux"
gui_zip="${release_root}/UtauTTS-linux-x64.zip"
server_zip="${release_root}/UtauTTS-Server-linux-x64.zip"

case "${release_root}" in
  "${root_dir}"/release)
    : ;;
  "${root_dir}"/release/*)
    : ;;
  *)
    echo "release_root must be under ${root_dir}/release" >&2
    exit 1
    ;;
esac
rm -rf "${gui_dir}" "${server_dir}"

for directory in /usr/local/go/bin; do
  if [ -d "${directory}" ]; then
    export PATH="${directory}:${PATH}"
  fi
done

export CGO_ENABLED=1

if [ -f "${root_dir}/.env" ]; then
  set -a
  source "${root_dir}/.env"
  set +a
fi

windows_user="${WINDOWS_USERNAME:-}"
if [ -z "${windows_user}" ]; then
  for user_dir in /mnt/c/Users/*/go; do
    if [ -d "${user_dir}/pkg/mod" ]; then
      windows_user="$(basename "$(dirname "${user_dir}")")"
      break
    fi
  done
fi
if [ -n "${windows_user}" ] && [ -z "${GOMODCACHE:-}" ] && \
   [ -d "/mnt/c/Users/${windows_user}/go/pkg/mod" ]; then
  export GOMODCACHE="/mnt/c/Users/${windows_user}/go/pkg/mod"
fi
export GOCACHE="${GOCACHE:-${root_dir}/build/linux-go-cache}"

for command_name in go cmake python3 curl zip; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "${command_name} is required" >&2
    exit 1
  fi
done

mkdir -p "${gui_dir}/tools" "${gui_dir}/runtime" "${gui_dir}/models" "${gui_dir}/plugins" \
  "${server_dir}/runtime" "${server_dir}/models" "${server_dir}/plugins" \
  "${root_dir}/build/native" "${root_dir}/build/qt-linux"
cd "${root_dir}"

echo '=== Test ==='
go test ./...
go vet ./...

echo '=== Build server ==='
go build -trimpath -o "${server_dir}/utautts-server" ./cmd/utautts-server

echo '=== Build CLI ==='
go build -trimpath -o "${gui_dir}/tools/utautts-cli" ./cmd/utautts-cli
go build -trimpath -o "${gui_dir}/tools/utautts-updater" ./cmd/utautts-updater

echo '=== Build native library and Qt GUI ==='
go build -trimpath -buildmode=c-shared -o "${root_dir}/build/native/libutautts_native.so" ./cmd/utautts-native
cmake -S "${root_dir}/qt" -B "${root_dir}/build/qt-linux" -DCMAKE_BUILD_TYPE=Release -G Ninja
cmake --build "${root_dir}/build/qt-linux" --config Release
cp "${root_dir}/build/qt-linux/app/utautts" "${gui_dir}/utautts"
cp "${root_dir}/build/native/libutautts_native.so" "${gui_dir}/libutautts_native.so"

echo '=== Build Open JTalk frontend helper ==='
PYTHON="${PYTHON:-/opt/utautts-py/bin/python}" bash "${root_dir}/tools/build-openjtalk-feature-bridge.sh"
for runtime_dir in "${gui_dir}/runtime" "${server_dir}/runtime"; do
  cp "${root_dir}/tools/openjtalk-feature-bridge/bin/utautts-openjtalk-features" "${runtime_dir}/"
  cp -R "${root_dir}/.tmp-openjtalk-linux/pyopenjtalk/open_jtalk_dic_utf_8-1.11" "${runtime_dir}/"
done

echo '=== Build native worldline bridge and install Linux worldline library ==='
staging_dir="${root_dir}/.tmp-worldline-linux"
rm -rf "${staging_dir}"
mkdir -p "${staging_dir}"
CGO_ENABLED=1 go build -trimpath \
  -o "${staging_dir}/utautts-worldline-bridge" \
  ./cmd/utautts-worldline-bridge
worldline_sha256="EEAE80212191C84EF2A1EBCD33567F47D9700F8E74136578944DBBEEE209136C"
worldline_source="${root_dir}/assets/worldline/linux-x64/libworldline.so"
if ! command -v sha256sum >/dev/null 2>&1; then
  echo "sha256sum is required" >&2
  exit 1
fi
actual_worldline_hash="$(sha256sum "${worldline_source}" | awk '{print $1}')"
if [ "${actual_worldline_hash^^}" != "${worldline_sha256}" ]; then
  echo "libworldline.so SHA-256 mismatch: ${actual_worldline_hash}" >&2
  exit 1
fi
for runtime_dir in "${gui_dir}/runtime" "${server_dir}/runtime"; do
  cp -R "${staging_dir}/." "${runtime_dir}/"
  cp "${worldline_source}" "${runtime_dir}/libworldline.so"
done

echo '=== Python and PyInstaller licenses ==='
python_license="$(find /usr/lib -maxdepth 3 -name 'LICENSE.txt' -path '*python3*' 2>/dev/null | head -1 || true)"
pyinstaller_license="$(find "${root_dir}/.tmp-pyinstaller-linux" -path '*/pyinstaller-*.dist-info/licenses/COPYING.txt' -print -quit)"
if [ -z "${python_license}" ] || [ -z "${pyinstaller_license}" ]; then
  echo 'Python or PyInstaller license was not found' >&2
  exit 1
fi
for runtime_dir in "${gui_dir}/runtime" "${server_dir}/runtime"; do
  license_dir="${runtime_dir}/licenses"
  mkdir -p "${license_dir}"
  cp "${python_license}" "${license_dir}/PYTHON_LICENSE.txt"
  cp "${pyinstaller_license}" "${license_dir}/PYINSTALLER_COPYING.txt"
done

echo '=== Go licenses ==='
for package_dir in "${gui_dir}" "${server_dir}"; do
  license_dir="${package_dir}/licenses/Go"
  mkdir -p "${license_dir}"
  cp "$(go env GOROOT)/LICENSE" "${license_dir}/GO-LICENSE.txt"
  for module in golang.org/x/text github.com/ikawaha/kagome/v2 github.com/ikawaha/kagome-dict github.com/ikawaha/kagome-dict/ipa; do
    module_info="$(go list -m -f '{{.Dir}}|{{.Version}}' "${module}")"
    module_dir="${module_info%%|*}"
    module_version="${module_info#*|}"
    module_name="${module//\//_}"
    module_name="${module_name//./_}"
    cp "${module_dir}/LICENSE" "${license_dir}/${module_name}-${module_version}-LICENSE.txt"
    if [[ -f "${module_dir}/NOTICE.txt" ]]; then
      cp "${module_dir}/NOTICE.txt" "${license_dir}/${module_name}-${module_version}-NOTICE.txt"
    fi
  done
done

echo '=== OpenJTalk, worldline, and dataset licenses ==='
for package_dir in "${gui_dir}" "${server_dir}"; do
  license_root="${package_dir}/licenses"
  mkdir -p "${license_root}/OpenJTalk" "${license_root}/Worldline"
  cp "${root_dir}/licenses/openjtalk/"*.txt "${license_root}/OpenJTalk/"
  cp "${root_dir}/licenses/worldline/"*.txt "${license_root}/Worldline/"
  cp "${root_dir}/licenses/JSUT-DATA-AND-LABELS.txt" "${license_root}/"
  cp "${root_dir}/licenses/PROSODY-MODELS.txt" "${license_root}/"
  dict_copying="${package_dir}/runtime/open_jtalk_dic_utf_8-1.11/COPYING"
  if [[ -f "${dict_copying}" ]]; then
    cp "${dict_copying}" "${license_root}/OpenJTalk/DICTIONARY_COPYING.txt"
  fi
done

echo '=== License manifest ==='
for package_dir in "${gui_dir}" "${server_dir}"; do
  license_root="${package_dir}/licenses"
  manifest="${license_root}/README.txt"
  {
    echo 'This directory contains license and notice files copied from the exact'
    echo 'SDK/package/toolchain versions used to assemble this release.'
    echo ''
    echo 'The project-wide summary is ../THIRD_PARTY_NOTICES.txt.'
  } > "${manifest}"
  find "${license_root}" -type f | sort | sed "s#${package_dir}/##" >> "${manifest}"
done

echo '=== Models and plugins ==='
for package_dir in "${gui_dir}" "${server_dir}"; do
  cp -R "${root_dir}/models/." "${package_dir}/models/"
  cp -R "${root_dir}/plugins/renderers/." "${package_dir}/plugins/renderers/"
  rm -rf "${package_dir}/plugins/renderers/openutau-classic-faithful-gpu"
  cat > "${package_dir}/plugins/renderers/openutau-classic-faithful/plugin.json" <<'EOF'
{
  "manifest_version": 1, "kind": "renderer", "id": "openutau-classic-worldline-faithful",
  "display_name": "OpenUTAU Classic faithful", "description": "OpenUTAU Classicのphone timing、Worldline resampling、5点envelopeを再現します。",
  "backend": "openutau-classic-worldline-faithful", "version": "1", "acceleration": "cpu", "default_priority": 50,
  "capabilities": { "frame_pitch": true },
  "assets": {
    "worldline": "../../../runtime/libworldline.so",
    "worldline_bridge": "../../../runtime/utautts-worldline-bridge"
  }
}
EOF
  cat > "${package_dir}/plugins/renderers/openutau-worldline-r-faithful/plugin.json" <<'EOF'
{
  "manifest_version": 1, "kind": "renderer", "id": "openutau-worldline-r-faithful",
  "display_name": "OpenUTAU WORLDLINE-R faithful", "description": "OpenUTAU 0.1.565のPhraseSynthでフレーズ全体をWORLD合成します。",
  "backend": "openutau-worldline-r-faithful", "version": "1", "acceleration": "cpu", "default_priority": 200,
  "capabilities": { "frame_pitch": true },
  "assets": {
    "worldline": "../../../runtime/libworldline.so",
    "worldline_bridge": "../../../runtime/utautts-worldline-bridge"
  }
}
EOF
done

echo '=== Voicebanks ==='
mkdir -p "${gui_dir}/voice"
if compgen -G "${root_dir}/voice/*.zip" > /dev/null; then
  SRC_DIR="${root_dir}/voice" OUT_DIR="${gui_dir}/voice" python3 - <<'PYEOF'
import glob
import os
import zipfile

src_dir = os.environ["SRC_DIR"]
out_dir = os.environ["OUT_DIR"]
for archive in sorted(glob.glob(os.path.join(src_dir, "*.zip"))):
    with zipfile.ZipFile(archive, metadata_encoding="cp932") as zf:
        zf.extractall(out_dir)
    print(f"extracted {os.path.basename(archive)}")
PYEOF
fi
mkdir -p "${server_dir}/voice"
echo 'Place each UTAU voicebank in its own folder here.' > "${server_dir}/voice/PUT_VOICEBANKS_HERE.txt"

echo '=== Docs and legal ==='
cp -R "${root_dir}/docs" "${gui_dir}/docs"
cp "${root_dir}/LICENSE" "${gui_dir}/LICENSE"
cp "${root_dir}/THIRD_PARTY_NOTICES.txt" "${gui_dir}/THIRD_PARTY_NOTICES.txt"
cp "${root_dir}/README.md" "${gui_dir}/README.md"
cp "${root_dir}/docs/server.md" "${server_dir}/README.md"
cp "${root_dir}/docs/manual-pitch.md" "${server_dir}/manual-pitch.md"
cp "${root_dir}/LICENSE" "${server_dir}/LICENSE"
cp "${root_dir}/THIRD_PARTY_NOTICES.txt" "${server_dir}/THIRD_PARTY_NOTICES.txt"

chmod +x "${gui_dir}/utautts" "${gui_dir}/libutautts_native.so" \
  "${gui_dir}/tools/"* "${gui_dir}/runtime/utautts-openjtalk-features" \
  "${gui_dir}/runtime/utautts-worldline-bridge" \
  "${server_dir}/utautts-server" "${server_dir}/runtime/utautts-openjtalk-features" \
  "${server_dir}/runtime/utautts-worldline-bridge"

echo '=== Package ==='
rm -f "${gui_zip}" "${server_zip}"
(cd "${gui_dir}" && zip -qr "${gui_zip}" .)
(cd "${server_dir}" && zip -qr "${server_zip}" .)

echo '=== Release smoke test ==='
bash "${root_dir}/tools/test-linux-package.sh" "${release_root}"

echo "Built Linux packages:"
echo "  ${gui_zip}"
echo "  ${server_zip}"
echo 'GUI:'
ls "${gui_dir}"
echo 'Server:'
ls "${server_dir}"
