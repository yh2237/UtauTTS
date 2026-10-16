#!/usr/bin/env bash
# 英語の合成で同梱するモデルを、版を固定した配布元から取得してハッシュを確かめる。
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
destination="${1:-${root_dir}/models/english-reference}"
kokoro="https://huggingface.co/hexgrad/Kokoro-82M/resolve/f3ff3571791e39611d31c381e3a41a3af07b4987"
hubert="https://huggingface.co/facebook/hubert-base-ls960/resolve/af46f65f540dc3ca7aa59f46c6c3d5dbb4374fa8"
misaki="https://raw.githubusercontent.com/hexgrad/misaki/fba1236595f2d2bf21d414ba6e57d25256afada3/misaki/data"

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

fetch() {
  local name="$1" url="$2" expected="$3"
  local target="${destination}/${name}"
  if [[ -f "${target}" && "$(sha256 "${target}")" == "${expected}" ]]; then
    return
  fi
  echo "Downloading ${url}"
  curl -fL --retry 3 -o "${target}.download" "${url}"
  local actual
  actual="$(sha256 "${target}.download")"
  if [[ "${actual}" != "${expected}" ]]; then
    rm -f "${target}.download"
    echo "SHA256 mismatch for ${name}: ${actual}" >&2
    exit 1
  fi
  mv -f "${target}.download" "${target}"
}

mkdir -p "${destination}"
fetch kokoro-v1_0.pth "${kokoro}/kokoro-v1_0.pth" 496dba118d1a58f5f3db2efc88dbdc216e0483fc89fe6e47ee1f2c53f18ad1e4
fetch config.json "${kokoro}/config.json" 5abb01e2403b072bf03d04fde160443e209d7a0dad49a423be15196b9b43c17f
fetch af_heart.pt "${kokoro}/voices/af_heart.pt" 0ab5709b8ffab19bfd849cd11d98f75b60af7733253ad0d67b12382a102cb4ff
fetch hubert-base-ls960.safetensors "${hubert}/model.safetensors" ec18ca2453e7cf3de5e4fc5b79e7484b27506ed0ad684282ddd10ecb9492f37f
fetch us_gold.json "${misaki}/us_gold.json" dc414872a49a28ae6c141463d502fd945f3b2fde040484fdc47d00cc4612686f
fetch us_silver.json "${misaki}/us_silver.json" de8f67be911bb6c659187b4a65fd966b6a30e56350e0f790d763210b053ac475
