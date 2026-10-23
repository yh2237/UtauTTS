#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release_root="${1:-${root_dir}/release}"
gui_dir="${release_root}/UtauTTS-macos"
server_dir="${release_root}/UtauTTS-Server-macos"
gui_zip="${release_root}/UtauTTS-mac-arm64.zip"
server_zip="${release_root}/UtauTTS-Server-mac-arm64.zip"
bundled_voicebank_sha256='B96D1B21145F22E573AFD9EC8AEAAD0EC9CBAEE581C2623C64ADDEB31DE46B3D'

case "${release_root}" in
  "${root_dir}/release"|"${root_dir}/release"/*) ;;
  *)
    echo "release_root must be under ${root_dir}/release" >&2
    exit 1
    ;;
esac

go_command="${GO_BIN:-go}"
python_command="${PYTHON:-python3}"
cmake_command="${CMAKE:-cmake}"
ninja_command="${NINJA:-ninja}"
qt_root="${QT_ROOT:-}"
mac_arch="${MACOS_ARCH:-$(uname -m)}"

for command_name in "${go_command}" "${python_command}" "${cmake_command}" "${ninja_command}" zip unzip curl shasum install_name_tool; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "required command was not found: ${command_name}" >&2
    exit 1
  fi
done

if [[ -z "${qt_root}" ]] && command -v brew >/dev/null 2>&1; then
  qt_root="$(brew --prefix qt 2>/dev/null || true)"
fi
if [[ -z "${qt_root}" ]]; then
  echo 'QT_ROOT is required and must point to a Qt 6 installation' >&2
  exit 1
fi
qt_root="$(cd "${qt_root}" && pwd)"
qt_config="$(find "${qt_root}" -path '*/lib/cmake/Qt6/Qt6Config.cmake' -print -quit)"
macdeployqt="${QT_MACDEPLOYQT:-${qt_root}/bin/macdeployqt}"
if [[ -z "${qt_config}" || ! -x "${macdeployqt}" ]]; then
  echo "Qt6Config.cmake or macdeployqt was not found below ${qt_root}" >&2
  exit 1
fi

rm -rf "${gui_dir}" "${server_dir}"
rm -f "${gui_zip}" "${server_zip}"
mkdir -p \
  "${gui_dir}/tools" "${gui_dir}/runtime" "${gui_dir}/models" \
  "${server_dir}/runtime" "${server_dir}/models" \
  "${root_dir}/build/native"

export CGO_ENABLED=1
export GOCACHE="${GOCACHE:-${root_dir}/build/go-cache-macos}"
export GOMODCACHE="${GOMODCACHE:-${root_dir}/build/go-mod-cache-macos}"
cd "${root_dir}"

echo '=== Test ==='
"${go_command}" test ./...
"${go_command}" vet ./...

echo '=== Build CLI, updater, and server ==='
"${go_command}" build -trimpath -ldflags="-s -w" -o "${server_dir}/utautts-server" ./cmd/utautts-server
cp "${server_dir}/utautts-server" "${gui_dir}/utautts-server"
"${go_command}" build -trimpath -ldflags="-s -w" -o "${gui_dir}/tools/utautts-cli" ./cmd/utautts-cli
"${go_command}" build -trimpath -ldflags="-s -w" -o "${gui_dir}/tools/utautts-ustx" ./cmd/tools/utautts-ustx
"${go_command}" build -trimpath -ldflags="-s -w" -o "${gui_dir}/tools/utautts-updater" ./cmd/utautts-updater

echo '=== Build native library and Qt app ==='
"${go_command}" build -trimpath -buildmode=c-shared -ldflags="-s -w" \
  -o "${root_dir}/build/native/libutautts_native.dylib" ./cmd/utautts-native
qt_build_dir="${root_dir}/build/qt-macos"
"${cmake_command}" -S "${root_dir}/qt" -B "${qt_build_dir}" \
  -G Ninja -DCMAKE_BUILD_TYPE=Release \
  "-DCMAKE_MAKE_PROGRAM=${ninja_command}" \
  "-DQt6_DIR=${qt_config}" "-DCMAKE_PREFIX_PATH=${qt_root}" \
  "-DCMAKE_OSX_ARCHITECTURES=${mac_arch}" \
  "-DUTAUTTS_NATIVE_DIR=${root_dir}/build/native"
"${cmake_command}" --build "${qt_build_dir}" --config Release
"${cmake_command}" --install "${qt_build_dir}" --prefix "${gui_dir}"

app_path="$(find "${gui_dir}" -type d -name 'utautts.app' -print -quit)"
if [[ -z "${app_path}" || ! -d "${app_path}/Contents/MacOS" ]]; then
  echo "Qt macOS application bundle was not produced under ${gui_dir}" >&2
  exit 1
fi
mkdir -p "${app_path}/Contents/Frameworks"
native_library="${app_path}/Contents/Frameworks/libutautts_native.dylib"
cp "${root_dir}/build/native/libutautts_native.dylib" "${native_library}"
install_name_tool -id '@rpath/libutautts_native.dylib' "${native_library}"
install_name_tool -change "${root_dir}/build/native/libutautts_native.dylib" \
  '@rpath/libutautts_native.dylib' "${app_path}/Contents/MacOS/utautts" || true
install_name_tool -change 'libutautts_native.dylib' \
  '@rpath/libutautts_native.dylib' "${app_path}/Contents/MacOS/utautts" || true
"${macdeployqt}" "${app_path}" "-qmldir=${root_dir}/qt/qml"

find "${app_path}" -type f \( -iname '*ffmpeg*' -o -iname 'libavcodec*' \
  -o -iname 'libavformat*' -o -iname 'libavutil*' -o -iname 'libswresample*' \
  -o -iname 'libswscale*' \) -delete
find "${app_path}" -type d -path '*/Resources/translations' -prune -exec rm -rf {} +
controls_dir="${app_path}/Contents/Resources/qml/QtQuick/Controls"
for style_name in FluentWinUI3 Imagine Material Universal Windows; do
  rm -rf "${controls_dir}/${style_name}"
done
dialog_style_dir="${app_path}/Contents/Resources/qml/QtQuick/Dialogs/quickimpl/qml"
for style_name in +Imagine +Material +Universal; do
  rm -rf "${dialog_style_dir}/${style_name}"
done
find "${app_path}" -type f \( -iname 'QtSvg' -o -iname 'Qt6Svg' \
  -o -iname 'QtSvgWidgets' -o -iname 'qsvgicon*' -o -iname 'qsvg.*' \
  -o -iname 'libqsvg*' \) -delete
find "${app_path}" -type d \( -iname 'QtSvg.framework' -o -iname 'QtSvgWidgets.framework' \) \
  -prune -exec rm -rf {} +
find "${app_path}" -type f \( -iname '*FluentWinUI3*' -o -iname '*Imagine*' \
  -o -iname '*Material*' -o -iname '*Universal*' -o -iname '*WindowsStyleImpl*' \) -delete

echo '=== Build UtauTTS native engines ==='
staging_dir="${root_dir}/.tmp-native-macos"
mkdir -p "${staging_dir}"
bash "${root_dir}/tools/build-native-engines.sh" "${staging_dir}"
cp -R "${staging_dir}/." "${gui_dir}/runtime/"
cp -R "${staging_dir}/." "${server_dir}/runtime/"

echo '=== Models ==='
for reference_file in kokoro-v1_0.pth config.json af_heart.pt ef_dora.pt pf_dora.pt jf_alpha.pt us_gold.json us_silver.json; do
  if [[ ! -f "${root_dir}/models/reference/${reference_file}" ]]; then
    echo "Reference model is missing: models/reference/${reference_file}. Run tools/fetch-reference-models.sh." >&2
    exit 1
  fi
done
for wavlm_file in wavlm/wavlm-base-plus.safetensors wavlm/config.json; do
  if [[ ! -f "${root_dir}/models/reference/${wavlm_file}" ]]; then
    echo "WavLM model is missing: models/reference/${wavlm_file}. Run tools/fetch-reference-models.sh." >&2
    exit 1
  fi
done
for chinese_file in kokoro-zh/kokoro-v1_1-zh.pth kokoro-zh/config.json kokoro-zh/zf_001.pt; do
  if [[ ! -f "${root_dir}/models/reference/${chinese_file}" ]]; then
    echo "Chinese reference model is missing: models/reference/${chinese_file}. Run tools/fetch-reference-models.sh." >&2
    exit 1
  fi
done
cp -R "${root_dir}/models/." "${gui_dir}/models/"
cp -R "${root_dir}/models/." "${server_dir}/models/"
for package_dir in "${gui_dir}" "${server_dir}"; do
  (cd "${root_dir}" && "${go_command}" run ./cmd/tools/copy-model-license-notices \
    --models "${package_dir}/models" \
    --embedded-models "${root_dir}/internal/speechtiming" \
    --repository-root "${root_dir}" \
    --package-root "${package_dir}")
done

echo '=== Voicebanks ==='
mkdir -p "${gui_dir}/voice" "${server_dir}/voice"
voice_archives=()
for archive in "${root_dir}/voice"/*.zip; do
  [[ -f "${archive}" ]] && voice_archives+=("${archive}")
done
if [[ "${#voice_archives[@]}" -eq 1 ]]; then
  voice_hash="$(shasum -a 256 "${voice_archives[0]}" | awk '{print toupper($1)}')"
  [[ "${voice_hash}" == "${bundled_voicebank_sha256}" ]] || {
    echo "Bundled voicebank hash mismatch: expected ${bundled_voicebank_sha256}, got ${voice_hash}" >&2
    exit 1
  }
  OUT_DIR="${gui_dir}/voice" ARCHIVE="${voice_archives[0]}" "${python_command}" - <<'PY'
import os
import zipfile

out_dir = os.environ["OUT_DIR"]
archive = os.environ["ARCHIVE"]
with zipfile.ZipFile(archive, metadata_encoding="cp932") as zf:
    zf.extractall(out_dir)
print(f"extracted {os.path.basename(archive)}")
PY
else
  echo "Expected exactly one bundled voicebank archive, found ${#voice_archives[@]}" >&2
  exit 1
fi
echo 'Place each UTAU voicebank in its own folder here.' > "${server_dir}/voice/PUT_VOICEBANKS_HERE.txt"

echo '=== Docs and legal files ==='
cp -R "${root_dir}/docs" "${gui_dir}/docs"
cp "${root_dir}/LICENSE" "${gui_dir}/LICENSE"
cp "${root_dir}/LICENSE-SCOPE.md" "${gui_dir}/LICENSE-SCOPE.md"
cp "${root_dir}/THIRD_PARTY_NOTICES.txt" "${gui_dir}/THIRD_PARTY_NOTICES.txt"
cp "${root_dir}/THIRD_PARTY_NOTICES-MACOS-GUI.txt" "${gui_dir}/THIRD_PARTY_NOTICES-MACOS-GUI.txt"
cp "${root_dir}/README.md" "${gui_dir}/README.md"
cp "${root_dir}/docs/server.md" "${server_dir}/README.md"
cp "${root_dir}/LICENSE" "${server_dir}/LICENSE"
cp "${root_dir}/LICENSE-SCOPE.md" "${server_dir}/LICENSE-SCOPE.md"
cp "${root_dir}/THIRD_PARTY_NOTICES.txt" "${server_dir}/THIRD_PARTY_NOTICES.txt"

for package_dir in "${gui_dir}" "${server_dir}"; do
  license_root="${package_dir}/licenses"
  mkdir -p "${license_root}/Go" "${license_root}/OpenJTalk" "${license_root}/WORLD"
  bash "${root_dir}/tools/collect-go-licenses.sh" "${package_dir}" "${go_command}"
  cp "${root_dir}/third_party/world/LICENSE.txt" "${license_root}/WORLD/WORLD-LICENSE.txt"
  cp "${root_dir}/third_party/world/OOURA-NOTICE.txt" "${license_root}/WORLD/OOURA-NOTICE.txt"
  cp "${root_dir}/third_party/world/MACRODEFINITIONS-LICENSE.txt" "${license_root}/WORLD/MACRODEFINITIONS-LICENSE.txt"
  cp "${root_dir}/licenses/MATERIAL-SYMBOLS.txt" "${license_root}/MATERIAL-SYMBOLS.txt"
  cp "${root_dir}/licenses/LINE-SEED-JP.txt" "${license_root}/LINE-SEED-JP.txt"
  rm -f "${license_root}/OpenJTalk/DICTIONARY_COPYING.txt"
  cp "${root_dir}/licenses/openjtalk/"*.txt "${license_root}/OpenJTalk/"
  dict_copying="${package_dir}/runtime/open_jtalk_dic_utf_8-1.11/COPYING"
  [[ -f "${dict_copying}" ]] || {
    echo "Open JTalk dictionary license was not found: ${dict_copying}" >&2
    exit 1
  }

  if [[ "${package_dir}" == "${gui_dir}" ]]; then
    qt_audit_root="${root_dir}/build/license-audit/Qt/macos"
    bash "${root_dir}/tools/collect-macos-qt-licenses.sh" "${package_dir}" "${app_path}" "${qt_root}" "${qt_audit_root}"
    (cd "${root_dir}" && "${go_command}" run ./cmd/tools/verify-qt-sbom \
      --package-root "${package_dir}" --sbom-root "${qt_audit_root}")
  fi
done

chmod +x "${app_path}/Contents/MacOS/utautts" \
  "${gui_dir}/utautts-server" "${gui_dir}/tools/"* "${gui_dir}/runtime/"* \
  "${server_dir}/utautts-server" "${server_dir}/runtime/"*

echo '=== Package ==='
(cd "${gui_dir}" && zip -qyr "${gui_zip}" .)
(cd "${server_dir}" && zip -qyr "${server_zip}" .)

echo '=== Release smoke test ==='
PYTHON="${python_command}" bash "${root_dir}/tools/test-macos-package.sh" "${release_root}"

echo 'Built macOS packages:'
echo "  ${gui_zip}"
echo "  ${server_zip}"
