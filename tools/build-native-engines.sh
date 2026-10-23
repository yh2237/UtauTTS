#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-${root_dir}/runtime}"
cmake_command="${CMAKE:-cmake}"
ninja_command="${NINJA:-ninja}"
extension="so"
if [[ "$(uname -s)" == "Darwin" ]]; then
  extension="dylib"
fi
build_dir="${root_dir}/build/native-$(uname -s | tr '[:upper:]' '[:lower:]')"

"${cmake_command}" -S "${root_dir}/native" -B "${build_dir}" \
  -G Ninja -DCMAKE_BUILD_TYPE=Release "-DCMAKE_MAKE_PROGRAM=${ninja_command}"
"${cmake_command}" --build "${build_dir}" --config Release
mkdir -p "${output_dir}"
cp "${build_dir}/world-engine/utautts-world-engine.${extension}" "${output_dir}/"
cp "${build_dir}/openjtalk-engine/utautts-openjtalk-engine.${extension}" "${output_dir}/"

dictionary="open_jtalk_dic_utf_8-1.11"
if [[ ! -f "${output_dir}/${dictionary}/sys.dic" ]]; then
  archive="${build_dir}/${dictionary}.tar.gz"
  curl -fsSL -o "${archive}" "https://github.com/r9y9/open_jtalk/releases/download/v1.11.1/${dictionary}.tar.gz"
  if command -v sha256sum >/dev/null; then
    hash="$(sha256sum "${archive}" | cut -d' ' -f1)"
  else
    hash="$(shasum -a 256 "${archive}" | cut -d' ' -f1)"
  fi
  if [[ "${hash}" != "fe6ba0e43542cef98339abdffd903e062008ea170b04e7e2a35da805902f382a" ]]; then
    echo "Open JTalk dictionary checksum mismatch: ${hash}" >&2
    exit 1
  fi
  tar -xzf "${archive}" -C "${output_dir}"
fi
