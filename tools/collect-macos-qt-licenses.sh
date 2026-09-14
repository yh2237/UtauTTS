#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
package_root="${1:?package root is required}"
app_path="${2:?application bundle path is required}"
qt_root="${3:?Qt root is required}"
audit_root="${4:-${root_dir}/build/license-audit/Qt/macos}"
license_root="${package_root}/licenses/Qt"

copy_required() {
  local source="$1"
  local destination="$2"
  if [[ ! -f "${source}" ]]; then
    echo "required Qt license file was not found: ${source}" >&2
    exit 1
  fi
  mkdir -p "$(dirname "${destination}")"
  cp "${source}" "${destination}"
}

mkdir -p "${license_root}"
rm -f "${license_root}/LGPL-2.1.txt" \
  "${license_root}/FFmpeg-SOURCE-AND-LICENSE.txt" \
  "${license_root}/FFmpeg-SOURCE-OFFER.txt" \
  "${license_root}/FFmpeg-OPTIONAL.txt"
rm -rf "${license_root}/sbom"
qt_config="$(find "${qt_root}" -path '*/lib/cmake/Qt6/Qt6Config.cmake' -type f -print -quit)"
if [[ -z "${qt_config}" ]]; then
  echo "Qt6Config.cmake was not found below ${qt_root}" >&2
  exit 1
fi

qt_major="$(sed -nE 's/.*QT_VERSION_MAJOR[[:space:]]+([0-9]+).*/\1/p' "${qt_config}" | head -n 1)"
qt_minor="$(sed -nE 's/.*QT_VERSION_MINOR[[:space:]]+([0-9]+).*/\1/p' "${qt_config}" | head -n 1)"
qt_patch="$(sed -nE 's/.*QT_VERSION_PATCH[[:space:]]+([0-9]+).*/\1/p' "${qt_config}" | head -n 1)"
if [[ -z "${qt_major}" || -z "${qt_minor}" || -z "${qt_patch}" ]]; then
  echo "Qt version could not be read from ${qt_config}" >&2
  exit 1
fi
qt_version="${qt_major}.${qt_minor}.${qt_patch}"
qt_doc_series="${qt_major}.${qt_minor}"

qt_sdk_parent="$(cd "${qt_root}/../.." && pwd)"
sbom_source_root=""
for candidate in "${qt_root}/sbom" "${qt_sdk_parent}/sbom"; do
  if [[ -d "${candidate}" ]]; then
    sbom_source_root="${candidate}"
    break
  fi
done
[[ -n "${sbom_source_root}" ]] || {
  echo "Qt SBOM directory was not found below ${qt_root}" >&2
  exit 1
}
sbom_names=(
  "qtbase-${qt_version}.spdx.json"
  "qtdeclarative-${qt_version}.spdx.json"
  "qtmultimedia-${qt_version}.spdx.json"
)
rm -rf "${audit_root}"
mkdir -p "${audit_root}"
{
  echo "Qt SBOM files for Qt ${qt_version}"
  echo '================================='
  echo
  echo 'Raw SPDX JSON files are kept in the build audit directory and are not included in the release package.'
  echo 'Audit directory: build/license-audit/Qt/macos'
  echo
  for sbom_name in "${sbom_names[@]}"; do
    sbom_source="${sbom_source_root}/${sbom_name}"
    copy_required "${sbom_source}" "${audit_root}/${sbom_name}"
    echo "${sbom_name}"
    echo "  SHA-256: $(shasum -a 256 "${sbom_source}" | awk '{print $1}')"
  done
} > "${license_root}/Qt-SBOM-MANIFEST.txt"
ffmpeg_files=()
while IFS= read -r candidate; do
  [[ -n "${candidate}" ]] && ffmpeg_files+=("${candidate}")
done < <(find "${app_path}" -type f \( -iname '*ffmpeg*' \
  -o -iname '*avcodec*.dylib' -o -iname '*avformat*.dylib' \
  -o -iname '*avutil*.dylib' -o -iname '*swresample*.dylib' \
  -o -iname '*swscale*.dylib' \) -print | sort)
if [[ "${#ffmpeg_files[@]}" -gt 0 ]]; then
  echo 'FFmpeg files must not be bundled in the macOS app' >&2
  printf '  %s\n' "${ffmpeg_files[@]}" >&2
  exit 1
fi
{
  echo
  echo 'Qt Multimedia FFmpeg status:'
  echo 'FFmpeg is not bundled. The Qt Multimedia SBOM may list optional FFmpeg support from the Qt SDK.'
  echo 'FFmpeg package files bundled: false'
} >> "${license_root}/Qt-SBOM-MANIFEST.txt"

lgpl_path=""
for search_root in "${qt_root}" "${qt_sdk_parent}/Tools"; do
  if [[ ! -d "${search_root}" ]]; then
    continue
  fi
  while IFS= read -r candidate; do
    lgpl_path="${candidate}"
    break 2
  done < <(find "${search_root}" -type f \( -iname 'LGPLv3.txt' -o -iname 'LGPL-3.0.txt' \) -print 2>/dev/null)
done
if [[ -z "${lgpl_path}" ]]; then
  lgpl_path="${root_dir}/licenses/Qt/LGPL-3.0.txt"
fi
copy_required "${lgpl_path}" "${license_root}/LGPL-3.0.txt"

cat > "${license_root}/Qt-SOURCE-OFFER.txt" <<EOF
Qt source offer
===============

This package contains dynamically linked Qt ${qt_version} libraries.
For at least three years after this package was distributed, UtauTTS will
provide the complete corresponding source for the LGPL-covered Qt modules in
a machine-readable archive at no charge other than the reasonable cost of
performing the source distribution.

Source requests:
https://github.com/yh2237/UtauTTS/issues/new?title=Qt%20source%20request

Include the UtauTTS release version and Qt version ${qt_version} in a request.
Identify the Qt modules concerned. The repository and its build scripts provide
the corresponding application source and relinking information.

Qt source locations:
https://download.qt.io/official_releases/qt/${qt_major}.${qt_minor}/${qt_version}/submodules/
https://code.qt.io/cgit/qt/qt5.git/tag/?h=v${qt_version}

The deployed modules include Qt Core, Qt GUI, Qt QML, Qt Quick, Qt Quick
Controls, Qt Multimedia, and Qt Concurrent. This offer covers the exact Qt
version used by this package, not an arbitrary later version.
EOF

cat > "${license_root}/Qt-RELINK-INSTRUCTIONS.txt" <<EOF
Qt replacement and relinking information
==========================================

The GUI links dynamically to the Qt frameworks distributed inside
UtauTTS.app/Contents/Frameworks. An end user may replace those frameworks with
compatible modified LGPL-covered Qt builds, subject to Qt's license terms and
ABI compatibility.

To rebuild the application against a modified Qt build:

1. Obtain the UtauTTS source for the same release.
2. Set QT_ROOT to a Qt kit containing lib/cmake/Qt6/Qt6Config.cmake.
3. Set MACOS_ARCH=arm64 and run tools/build-macos.sh on an Apple Silicon Mac.
4. Replace the frameworks inside the resulting app bundle with compatible
   modified Qt frameworks and verify their install names and rpaths.

The corresponding Qt source offer, LGPLv3 text, and third-party attribution
information are included beside this file. The source request procedure is in
Qt-SOURCE-OFFER.txt.
EOF

cat > "${license_root}/Qt-THIRD-PARTY-ATTRIBUTIONS.txt" <<EOF
Qt ${qt_version} third-party attributions
======================================

Qt modules contain third-party components with their own copyright and
license terms. Raw SPDX SBOM files used for this package are kept in the build
audit directory, with SHA-256 values recorded in Qt-SBOM-MANIFEST.txt. The
authoritative attribution list for this Qt version is:
https://doc.qt.io/qt-${qt_doc_series}/licenses-used-in-qt.html

Qt Multimedia attribution and licensing information:
https://doc.qt.io/qt-${qt_doc_series}/qtmultimedia-attribution-ffmpeg.html

Qt Multimedia uses a native backend in this package. An optional external
FFmpeg backend is described in FFmpeg-OPTIONAL.txt.
EOF

cat > "${license_root}/FFmpeg-OPTIONAL.txt" <<EOF
Optional FFmpeg runtime
=======================

UtauTTS does not bundle FFmpeg. Qt Multimedia uses its native backend when
available. If an external Qt Multimedia FFmpeg backend is needed, set the path
in Settings or one of these environment variables before the first launch:

UTAUTTS_FFMPEG_PATH
FFMPEG_PATH
FFMPEG_DIR
FFMPEG_ROOT

The path should point to a directory containing the Qt Multimedia FFmpeg plugin
and its codec libraries. A standalone ffmpeg command-line executable is not a
replacement for that plugin. See the Qt Multimedia and FFmpeg documentation:
https://doc.qt.io/qt-6.8/qtmultimedia-index.html
https://ffmpeg.org/legal.html
EOF
