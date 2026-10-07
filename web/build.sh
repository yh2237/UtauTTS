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
