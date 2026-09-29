#!/usr/bin/env bash
# Qt wasm アプリをビルドし、web/dist と renderer をまとめた配布ディレクトリを作る。
# 事前に emsdk を有効化し、web/dist のGo wasm・Open JTalk・WORLD資産を用意しておくこと。
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
build_dir="${root_dir}/build/qt-wasm"
release_dir="${root_dir}/build/web-release"

: "${QT_WASM_DIR:?set QT_WASM_DIR to the Qt wasm kit (e.g. .../wasm_singlethread)}"
: "${QT_HOST_PATH:?set QT_HOST_PATH to the desktop Qt kit}"

"${QT_WASM_DIR}/bin/qt-cmake" -S "${root_dir}/qt" -B "${build_dir}" \
  -DCMAKE_BUILD_TYPE=Release \
  -DQT_HOST_PATH="${QT_HOST_PATH}"
cmake --build "${build_dir}"

rm -rf "${release_dir}"
mkdir -p "${release_dir}/web" "${release_dir}/renderer/utautts-world-phrase"
cp "${build_dir}/index.html" "${build_dir}/engine-loader.js" "${build_dir}/engine-worker.js" \
   "${build_dir}/utautts.js" "${build_dir}/utautts.wasm" "${build_dir}/qtloader.js" \
   "${release_dir}/"
cp -r "${root_dir}/web/dist" "${release_dir}/web/dist"
cp "${root_dir}/renderer/utautts-world-phrase/renderer.json" \
   "${release_dir}/renderer/utautts-world-phrase/renderer.json"

echo "Assembled ${release_dir}"
