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
  "${gui_dir}/tools" "${gui_dir}/runtime" "${gui_dir}/models" "${gui_dir}/renderer" \
  "${server_dir}/runtime" "${server_dir}/models" "${server_dir}/renderer" \
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

# Remove optional Qt Multimedia, translation, and style files from the package.
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

echo '=== Build Open JTalk frontend helper ==='
PYTHON="${python_command}" bash "${root_dir}/tools/build-openjtalk-feature-bridge-macos.sh"
openjtalk_helper="${root_dir}/tools/openjtalk-feature-bridge/bin/utautts-openjtalk-features"
openjtalk_dictionary="${root_dir}/.tmp-openjtalk-macos/pyopenjtalk/open_jtalk_dic_utf_8-1.11"
if [[ ! -x "${openjtalk_helper}" || ! -d "${openjtalk_dictionary}" ]]; then
  echo 'Open JTalk helper build did not produce the helper or dictionary' >&2
  exit 1
fi

echo '=== Build UtauTTS WORLD bridge and engine ==='
staging_dir="${root_dir}/.tmp-worldline-macos"
rm -rf "${staging_dir}"
mkdir -p "${staging_dir}"
"${go_command}" build -trimpath -ldflags="-s -w" -o "${staging_dir}/utautts-worldline-bridge" ./cmd/utautts-worldline-bridge
bash "${root_dir}/tools/build-world-engine-macos.sh" "${staging_dir}"
cp -R "${staging_dir}/." "${gui_dir}/runtime/"
cp -R "${staging_dir}/." "${server_dir}/runtime/"
cp "${openjtalk_helper}" "${gui_dir}/runtime/"
cp "${openjtalk_helper}" "${server_dir}/runtime/"
cp -R "${openjtalk_dictionary}" "${gui_dir}/runtime/"
cp -R "${openjtalk_dictionary}" "${server_dir}/runtime/"

echo '=== Models and renderers ==='
cp -R "${root_dir}/models/." "${gui_dir}/models/"
cp -R "${root_dir}/models/." "${server_dir}/models/"
cp -R "${root_dir}/renderer/." "${gui_dir}/renderer/"
cp -R "${root_dir}/renderer/." "${server_dir}/renderer/"
for package_dir in "${gui_dir}" "${server_dir}"; do
  rm -rf \
    "${package_dir}/renderer/diffsinger"
  "${python_command}" "${root_dir}/tools/copy-model-license-notices.py" \
    --models "${package_dir}/models" \
    --repository-root "${root_dir}" \
    --package-root "${package_dir}"
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
cp "${root_dir}/docs/manual-pitch.md" "${server_dir}/manual-pitch.md"
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
  cp "${root_dir}/licenses/JSUT-DATA-AND-LABELS.txt" "${license_root}/"
  cp "${root_dir}/licenses/PROSODY-MODELS.txt" "${license_root}/"
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
    "${python_command}" "${root_dir}/tools/verify-qt-sbom.py" \
      --package-root "${package_dir}" --sbom-root "${qt_audit_root}"
  fi

  python_license="$("${python_command}" - <<'PY'
import os
import sysconfig

roots = [
    sysconfig.get_path("stdlib"),
    sysconfig.get_config_var("prefix"),
    sysconfig.get_config_var("base"),
]
for root in roots:
    if not root:
        continue
    for name in ("LICENSE.txt", "LICENSE", "License.txt", "LICENSE.md"):
        candidate = os.path.join(root, name)
        if os.path.isfile(candidate):
            print(candidate)
            raise SystemExit
PY
  )"
  runtime_license_root="${package_dir}/runtime/licenses"
  mkdir -p "${runtime_license_root}"
  if [[ -n "${python_license}" ]]; then
    cp "${python_license}" "${runtime_license_root}/PYTHON_LICENSE.txt"
  fi
  pyinstaller_license="$(find "${root_dir}/.tmp-pyinstaller-macos" -type f \
    \( -path '*/pyinstaller-*.dist-info/licenses/COPYING.txt' -o \
       -path '*/pyinstaller-*.dist-info/COPYING.txt' \) -print -quit)"
  if [[ -n "${pyinstaller_license}" ]]; then
    cp "${pyinstaller_license}" "${runtime_license_root}/PYINSTALLER_COPYING.txt"
  fi
  [[ -n "${python_license}" && -n "${pyinstaller_license}" ]] || {
    echo 'Python or PyInstaller license was not found' >&2
    exit 1
  }
  PYTHONPATH="${root_dir}/.tmp-pyinstaller-macos" "${python_command}" \
    "${root_dir}/tools/collect-pyinstaller-runtime-licenses.py" \
    --archive "${package_dir}/runtime/utautts-openjtalk-features" \
    --output-dir "${runtime_license_root}" \
    --python-license "${python_license}" \
    --pyinstaller-license "${pyinstaller_license}"
done

chmod +x "${app_path}/Contents/MacOS/utautts" \
  "${gui_dir}/tools/"* "${gui_dir}/runtime/"* \
  "${server_dir}/utautts-server" "${server_dir}/runtime/"*

echo '=== Package ==='
(cd "${gui_dir}" && zip -qyr "${gui_zip}" .)
(cd "${server_dir}" && zip -qyr "${server_zip}" .)

echo '=== Release smoke test ==='
PYTHON="${python_command}" bash "${root_dir}/tools/test-macos-package.sh" "${release_root}"

echo 'Built macOS packages:'
echo "  ${gui_zip}"
echo "  ${server_zip}"
