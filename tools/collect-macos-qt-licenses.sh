#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
package_root="${1:?package root is required}"
app_path="${2:?application bundle path is required}"
qt_root="${3:?Qt root is required}"
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

lgpl_path=""
qt_sdk_parent="$(cd "${qt_root}/../.." && pwd)"
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
Identify whether the request concerns Qt itself, Qt Multimedia's FFmpeg
deployment, or both. The repository and its build scripts provide the
corresponding application source and relinking information.

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
license terms. The authoritative attribution list for this Qt version is:
https://doc.qt.io/qt-${qt_doc_series}/licenses-used-in-qt.html

Qt Multimedia documentation and licensing information:
https://doc.qt.io/qt-${qt_doc_series}/qtmultimedia-index.html

Qt Multimedia may deploy FFmpeg components. The exact files detected in this
application bundle, together with their SHA-256 values and source guidance,
are recorded in FFmpeg-SOURCE-AND-LICENSE.txt.
EOF

ffmpeg_files=()
while IFS= read -r candidate; do
  [[ -n "${candidate}" ]] && ffmpeg_files+=("${candidate}")
done < <(find "${app_path}" -type f \( -iname '*ffmpeg*' -o -iname 'libavcodec*.dylib' -o -iname 'libavformat*.dylib' -o -iname 'libavutil*.dylib' -o -iname 'libswresample*.dylib' -o -iname 'libswscale*.dylib' \) -print | sort)

{
  echo 'FFmpeg as deployed by Qt Multimedia'
  echo '==================================='
  echo
  echo "This notice was generated for Qt ${qt_version}."
  echo
  if [[ "${#ffmpeg_files[@]}" -eq 0 ]]; then
    echo 'No FFmpeg-named files were detected in the deployed app bundle.'
    echo 'Review the Qt Multimedia backend before distributing a build that'
    echo 'uses a differently named FFmpeg binary.'
  else
    echo 'The following FFmpeg-related files were detected:'
    echo
    for candidate in "${ffmpeg_files[@]}"; do
      relative_path="${candidate#${package_root}/}"
      sha256="$(shasum -a 256 "${candidate}" | awk '{print $1}')"
      echo "${relative_path}"
      echo "  SHA-256: ${sha256}"
    done
  fi
  echo
  echo 'Qt Multimedia documentation and source guidance:'
  echo "https://doc.qt.io/qt-${qt_doc_series}/qtmultimedia-index.html"
  echo 'https://ffmpeg.org/legal.html'
  echo
  echo 'The corresponding Qt and FFmpeg source request procedure is identified'
  echo 'in Qt-SOURCE-OFFER.txt. The source must correspond to the exact files'
  echo 'listed above; a generic FFmpeg source tree is not a substitute for the'
  echo 'matching source.'
} > "${license_root}/FFmpeg-SOURCE-AND-LICENSE.txt"
