#!/usr/bin/env bash
# UtauTTS wasm向けビルド。GoのwasmとJS資産、既定の抑揚モデルを web/dist へ集める。
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dist="${script_dir}/dist"
model_name="frame-intonation-tcn-v9.1-t.json"

mkdir -p "${dist}/models"

export CGO_ENABLED=0
export GOOS=js
export GOARCH=wasm

( cd "${root_dir}" && go build -trimpath -ldflags '-s -w' -o "${dist}/utautts.wasm" ./cmd/utautts-wasm )

goroot="$(go env GOROOT)"
cp "${goroot}/lib/wasm/wasm_exec.js" "${dist}/wasm_exec.js"
cp "${script_dir}/index.html" "${dist}/index.html"
cp "${script_dir}/main.js" "${dist}/main.js"
cp "${script_dir}/fs-shim.js" "${dist}/fs-shim.js"
cp "${script_dir}/openjtalk-bridge.js" "${dist}/openjtalk-bridge.js"
cp "${root_dir}/models/${model_name}" "${dist}/models/${model_name}"

echo "Built ${dist}"
echo "Serve it with:  python -m http.server --directory \"${dist}\""
