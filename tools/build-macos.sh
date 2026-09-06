#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release_root="${1:-${root_dir}/release}"
gui_dir="${release_root}/UtauTTS-macos"
server_dir="${release_root}/UtauTTS-Server-macos"
gui_zip="${release_root}/UtauTTS-macos-arm64.zip"
server_zip="${release_root}/UtauTTS-Server-macos-arm64.zip"

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

for command_name in "${go_command}" "${python_command}" "${cmake_command}" "${ninja_command}" zip unzip curl install_name_tool; do
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
"${go_command}" build -trimpath -o "${server_dir}/utautts-server" ./cmd/utautts-server
"${go_command}" build -trimpath -o "${gui_dir}/tools/utautts-cli" ./cmd/utautts-cli
"${go_command}" build -trimpath -o "${gui_dir}/tools/utautts-ustx" ./cmd/tools/utautts-ustx
"${go_command}" build -trimpath -o "${gui_dir}/tools/utautts-updater" ./cmd/utautts-updater

echo '=== Build native library and Qt app ==='
"${go_command}" build -trimpath -buildmode=c-shared \
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
"${go_command}" build -trimpath -o "${staging_dir}/utautts-worldline-bridge" ./cmd/utautts-worldline-bridge
bash "${root_dir}/tools/build-world-engine-macos.sh" "${staging_dir}"
cp -R "${staging_dir}/." "${gui_dir}/runtime/"
cp -R "${staging_dir}/." "${server_dir}/runtime/"
cp "${openjtalk_helper}" "${gui_dir}/runtime/"
cp "${openjtalk_helper}" "${server_dir}/runtime/"
cp -R "${openjtalk_dictionary}" "${gui_dir}/runtime/"
cp -R "${openjtalk_dictionary}" "${server_dir}/runtime/"

echo '=== Models and renderer manifests ==='
cp -R "${root_dir}/models/." "${gui_dir}/models/"
cp -R "${root_dir}/models/." "${server_dir}/models/"
cp -R "${root_dir}/renderer/." "${gui_dir}/renderer/"
cp -R "${root_dir}/renderer/." "${server_dir}/renderer/"
for package_dir in "${gui_dir}" "${server_dir}"; do
  rm -rf \
    "${package_dir}/renderer/utautts-world-phrase-cuda" \
    "${package_dir}/renderer/openutau-worldline-r-faithful" \
    "${package_dir}/renderer/diffsinger"
done

echo '=== Voicebanks ==='
mkdir -p "${gui_dir}/voice" "${server_dir}/voice"
if compgen -G "${root_dir}/voice/*.zip" >/dev/null; then
  SRC_DIR="${root_dir}/voice" OUT_DIR="${gui_dir}/voice" "${python_command}" - <<'PY'
import glob
import os
import zipfile

src_dir = os.environ["SRC_DIR"]
out_dir = os.environ["OUT_DIR"]
for archive in sorted(glob.glob(os.path.join(src_dir, "*.zip"))):
    with zipfile.ZipFile(archive, metadata_encoding="cp932") as zf:
        zf.extractall(out_dir)
    print(f"extracted {os.path.basename(archive)}")
PY
fi
echo 'Place each UTAU voicebank in its own folder here.' > "${server_dir}/voice/PUT_VOICEBANKS_HERE.txt"

echo '=== Docs and legal files ==='
cp -R "${root_dir}/docs" "${gui_dir}/docs"
cp "${root_dir}/LICENSE" "${gui_dir}/LICENSE"
cp "${root_dir}/THIRD_PARTY_NOTICES.txt" "${gui_dir}/THIRD_PARTY_NOTICES.txt"
cp "${root_dir}/README.md" "${gui_dir}/README.md"
cp "${root_dir}/docs/server.md" "${server_dir}/README.md"
cp "${root_dir}/docs/manual-pitch.md" "${server_dir}/manual-pitch.md"
cp "${root_dir}/LICENSE" "${server_dir}/LICENSE"
cp "${root_dir}/THIRD_PARTY_NOTICES.txt" "${server_dir}/THIRD_PARTY_NOTICES.txt"

for package_dir in "${gui_dir}" "${server_dir}"; do
  license_root="${package_dir}/licenses"
  mkdir -p "${license_root}/Go" "${license_root}/OpenJTalk" "${license_root}/Worldline" "${license_root}/WORLD"
  cp -R "${root_dir}/licenses/." "${license_root}/"
  cp "${root_dir}/third_party/world/LICENSE.txt" "${license_root}/WORLD/WORLD-LICENSE.txt"
  cp "${root_dir}/third_party/world/OOURA-NOTICE.txt" "${license_root}/WORLD/OOURA-NOTICE.txt"
  cp "${root_dir}/third_party/world/MACRODEFINITIONS-LICENSE.txt" "${license_root}/WORLD/MACRODEFINITIONS-LICENSE.txt"
  cp "${root_dir}/licenses/JSUT-DATA-AND-LABELS.txt" "${license_root}/"
  cp "${root_dir}/licenses/PROSODY-MODELS.txt" "${license_root}/"
  cp "${root_dir}/licenses/openjtalk/"*.txt "${license_root}/OpenJTalk/"
  cp "${root_dir}/licenses/worldline/"*.txt "${license_root}/Worldline/"
  cp "${root_dir}/licenses/APACHE-2.0.txt" "${license_root}/APACHE-2.0.txt"

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
  if [[ -n "${python_license}" ]]; then
    cp "${python_license}" "${license_root}/PYTHON_LICENSE.txt"
  fi
  pyinstaller_license="$(find "${root_dir}/.tmp-pyinstaller-macos" -type f \
    \( -path '*/pyinstaller-*.dist-info/licenses/COPYING.txt' -o \
       -path '*/pyinstaller-*.dist-info/COPYING.txt' \) -print -quit)"
  if [[ -n "${pyinstaller_license}" ]]; then
    cp "${pyinstaller_license}" "${license_root}/PYINSTALLER_COPYING.txt"
  fi
  {
    echo 'This directory contains license and notice files copied from the exact'
    echo 'SDK/package/toolchain versions used to assemble this release.'
    echo ''
    echo 'The project-wide summary is ../THIRD_PARTY_NOTICES.txt.'
  } > "${license_root}/README.txt"
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
