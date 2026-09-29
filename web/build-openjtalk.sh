#!/usr/bin/env bash
# Build Open JTalk (third_party/open_jtalk) to WebAssembly and link the C ABI wrapper.
# Requires: activated Emscripten SDK, cmake and ninja on PATH.
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jtalk="${root_dir}/third_party/open_jtalk/src"
build_dir="${root_dir}/build/openjtalk-wasm"
out_dir="${script_dir}/dist/openjtalk"
dict_src="${OPENJTALK_DICT:-${root_dir}/.tmp-openjtalk/pyopenjtalk/open_jtalk_dic_utf_8-1.11}"
mkdir -p "${out_dir}"

# 1) cmake: build the wasm static library and generate config.h.
if [ ! -f "${build_dir}/build.ninja" ]; then
  emcmake cmake -G Ninja -S "${jtalk}" -B "${build_dir}" \
    -DBUILD_SHARED_LIBS=OFF -DBUILD_PROGRAMS=OFF \
    -DCMAKE_BUILD_TYPE=Release -DCMAKE_POLICY_VERSION_MINIMUM=3.5
fi
cmake --build "${build_dir}" --target openjtalk
library="${build_dir}/libopenjtalk.a"

# 2) Compile the wrapper and link it into JS + wasm.
include_dirs=(jpcommon mecab/src mecab2njd njd njd2jpcommon njd_set_accent_phrase \
  njd_set_accent_type njd_set_digit njd_set_long_vowel njd_set_pronunciation \
  njd_set_unvoiced_vowel text2mecab)
includes=()
for dir in "${include_dirs[@]}"; do includes+=("-I${jtalk}/${dir}"); done

exported='_UtauTTSOpenJTalkInit,_UtauTTSOpenJTalkRun,_UtauTTSOpenJTalkError,_malloc,_free'

em++ -O3 -std=c++17 \
  "${root_dir}/native/openjtalk-engine/openjtalk_engine.cpp" \
  "${library}" \
  "${includes[@]}" \
  -DHAVE_CONFIG_H -DDIC_VERSION=102 -DCHARSET_UTF_8 -DMECAB_UTF8_USE_ONLY -DMECAB_CHARSET=utf-8 \
  -fexceptions \
  -sMODULARIZE=1 \
  -sEXPORT_NAME=createUtauTTSOpenJTalk \
  -sENVIRONMENT=node,web \
  -sFORCE_FILESYSTEM=1 \
  -sALLOW_MEMORY_GROWTH=1 \
  -sSTACK_SIZE=8MB \
  -sEXPORTED_RUNTIME_METHODS=ccall,cwrap,FS,HEAPF64,HEAPU8,UTF8ToString,stringToUTF8,lengthBytesUTF8 \
  -sEXPORTED_FUNCTIONS="${exported}" \
  -o "${out_dir}/utautts-openjtalk.js"

# 3) Stage the dictionary for the browser and write a manifest listing its files.
dict_out="${out_dir}/dict"
if [ ! -d "${dict_src}" ]; then
  echo "dictionary not found, skipping staging: ${dict_src}" >&2
else
  mkdir -p "${dict_out}"
  cp -f "${dict_src}"/* "${dict_out}/" 2>/dev/null || true
  python3 - "${dict_out}" "${out_dir}/dict-manifest.json" <<'PY'
import json, os, sys
dict_out, manifest = sys.argv[1], sys.argv[2]
files = sorted(name for name in os.listdir(dict_out) if os.path.isfile(os.path.join(dict_out, name)))
with open(manifest, "w", encoding="utf-8") as handle:
    json.dump({"files": files}, handle, ensure_ascii=False)
PY
  echo "Staged dictionary"
fi

echo "Built ${out_dir}"
