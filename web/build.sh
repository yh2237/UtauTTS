#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dist="${script_dir}/dist"

mkdir -p "${dist}/models"

export CGO_ENABLED=0
export GOOS=js
export GOARCH=wasm

( cd "${root_dir}" && go build -trimpath -ldflags '-s -w' -o "${dist}/utautts.wasm" ./cmd/utautts-wasm )

goroot="$(go env GOROOT)"
cp "${goroot}/lib/wasm/wasm_exec.js" "${dist}/wasm_exec.js"
cp "${script_dir}/fs-shim.js" "${dist}/fs-shim.js"
cp "${script_dir}/openjtalk-bridge.js" "${dist}/openjtalk-bridge.js"
cp "${script_dir}/world-bridge.js" "${dist}/world-bridge.js"
cp "${script_dir}/reference-inference.js" "${dist}/reference-inference.js"

# 参照照合の推論に使うONNX Runtime Web（MIT）。版とハッシュを固定して取る。
ort_version=1.23.2
ort_sha256=0b8707d7efab9a2bea63564d66cad79e897cfd0bd627e5ae000488ffc1c45c7d
if [ ! -f "${dist}/ort/ort.wasm.min.js" ] || [ "$(cat "${dist}/ort/version" 2>/dev/null)" != "${ort_version}" ]; then
    ort_tgz="$(mktemp)"
    curl -fsSL -o "${ort_tgz}" "https://registry.npmjs.org/onnxruntime-web/-/onnxruntime-web-${ort_version}.tgz"
    echo "${ort_sha256}  ${ort_tgz}" | sha256sum -c -
    rm -rf "${dist}/ort"
    mkdir -p "${dist}/ort"
    tar -xzf "${ort_tgz}" -C "${dist}/ort" --strip-components=2 package/dist/ort.wasm.min.js package/dist/ort-wasm-simd-threaded.mjs package/dist/ort-wasm-simd-threaded.wasm
    echo "${ort_version}" > "${dist}/ort/version"
    rm -f "${ort_tgz}"
fi
# 参照照合のモデル（tools/web-reference/export.pyで作る）は大きいので、置いてあるときだけ使う。
# UTAUTTS_WEB_REFERENCE を指定すると、その場所から写す。
if [ -n "${UTAUTTS_WEB_REFERENCE:-}" ]; then
    rm -rf "${dist}/reference"
    cp -R "${UTAUTTS_WEB_REFERENCE}" "${dist}/reference"
fi
# 同梱から外したモデルが前回のビルドから残らないよう、先に消す。
rm -f "${dist}/models/"*.json
cp "${root_dir}/models/"*.json "${dist}/models/"
{
    printf '{"models":['
    first=1
    for model in "${root_dir}/models/"*.json; do
        name="$(basename "${model}")"
        if [ "${first}" -eq 1 ]; then first=0; else printf ','; fi
        printf '"%s"' "${name}"
    done
    printf ']}'
} > "${dist}/models/manifest.json"

voice_zip="$(ls "${root_dir}/voice/"*.zip 2>/dev/null | head -n1 || true)"
if [ -n "${voice_zip}" ]; then
    voice_stage="${root_dir}/out/web-voice-$$"
    ( cd "${root_dir}" && GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" go run ./cmd/tools/build-voice "${voice_zip}" "out/web-voice-$$" )
    mkdir -p "${dist}/voice"
    cp -R "${voice_stage}/." "${dist}/voice/"
    rm -rf -- "${voice_stage}"
fi

echo "Built ${dist}"
echo "Build the browser UI next with qt/wasm/build.sh"
