#!/usr/bin/env bash
# WORLD (WORLD C++ + native/world-engine) を WebAssembly へビルドする。
# 事前に Emscripten SDK を activate しておくこと。
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
out_dir="${script_dir}/dist/world"
mkdir -p "${out_dir}"

emcc="${EMXX:-em++}"
world_src="${root_dir}/third_party/world/src"
exported='_UtauTTSWorldF0,_UtauTTSWorldAnalysisShape,_UtauTTSWorldAnalyze,_UtauTTSWorldSynthesize,_UtauTTSWorldAnalyzeFlat,_UtauTTSWorldSynthesizeFlat,_malloc,_free'

"${emcc}" -O3 -msimd128 -std=c++17 "-I${world_src}" \
  "${root_dir}/native/world-engine/world_engine.cpp" \
  "${world_src}/cheaptrick.cpp" \
  "${world_src}/common.cpp" \
  "${world_src}/d4c.cpp" \
  "${world_src}/fft.cpp" \
  "${world_src}/harvest.cpp" \
  "${world_src}/matlabfunctions.cpp" \
  "${world_src}/synthesis.cpp" \
  -fexceptions \
  -sMODULARIZE=1 \
  -sEXPORT_NAME=createUtauTTSWorld \
  -sENVIRONMENT=web,worker,node \
  -sALLOW_MEMORY_GROWTH=1 \
  -sSTACK_SIZE=16MB \
  -sEXPORTED_RUNTIME_METHODS=ccall,cwrap,UTF8ToString,HEAPF64,HEAP32,HEAPU8 \
  -sEXPORTED_FUNCTIONS="${exported}" \
  -o "${out_dir}/utautts-world.js"

echo "Built ${out_dir}"
