#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-${root_dir}/runtime}"
build_dir="${root_dir}/build/world-engine-macos"
cmake_command="${CMAKE:-cmake}"
ninja_command="${NINJA:-ninja}"

"${cmake_command}" -S "${root_dir}/native/world-engine" -B "${build_dir}" \
  -G Ninja -DCMAKE_BUILD_TYPE=Release "-DCMAKE_MAKE_PROGRAM=${ninja_command}"
"${cmake_command}" --build "${build_dir}" --config Release

engine="${build_dir}/utautts-world-engine.dylib"
if [[ ! -f "${engine}" ]]; then
  echo "macOS WORLD engine was not produced: ${engine}" >&2
  exit 1
fi

mkdir -p "${output_dir}"
cp "${engine}" "${output_dir}/"
