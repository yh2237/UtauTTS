#!/usr/bin/env bash
# 参照の合成で同梱するモデルを、版を固定した配布元から取得してハッシュを確かめる。
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
destination="${1:-${root_dir}/models/english-reference}"
kokoro="https://huggingface.co/hexgrad/Kokoro-82M/resolve/f3ff3571791e39611d31c381e3a41a3af07b4987"
kokoro_zh="https://huggingface.co/hexgrad/Kokoro-82M-v1.1-zh/resolve/01e7505bd6a7a2ac4975463114c3a7650a9f7218"
hubert="https://huggingface.co/facebook/hubert-base-ls960/resolve/af46f65f540dc3ca7aa59f46c6c3d5dbb4374fa8"
wavlm="https://huggingface.co/microsoft/wavlm-base-plus/resolve/4c66d4806a428f2e922ccfa1a962776e232d487b"
misaki="https://raw.githubusercontent.com/hexgrad/misaki/fba1236595f2d2bf21d414ba6e57d25256afada3/misaki/data"

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

fetch_to() {
  local directory="$1" name="$2" url="$3" expected="$4"
  local target="${directory}/${name}"
  if [[ -f "${target}" && "$(sha256 "${target}")" == "${expected}" ]]; then
    return
  fi
  mkdir -p "$(dirname "${target}")"
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

fetch() {
  fetch_to "${destination}" "$@"
}

mkdir -p "${destination}"
fetch kokoro-v1_0.pth "${kokoro}/kokoro-v1_0.pth" 496dba118d1a58f5f3db2efc88dbdc216e0483fc89fe6e47ee1f2c53f18ad1e4
fetch config.json "${kokoro}/config.json" 5abb01e2403b072bf03d04fde160443e209d7a0dad49a423be15196b9b43c17f
fetch af_heart.pt "${kokoro}/voices/af_heart.pt" 0ab5709b8ffab19bfd849cd11d98f75b60af7733253ad0d67b12382a102cb4ff
fetch ef_dora.pt "${kokoro}/voices/ef_dora.pt" d9d69b0f8a2b87a345f269d89639f89dfbd1a6c9da0c498ae36dd34afcf35530
fetch pf_dora.pt "${kokoro}/voices/pf_dora.pt" 07e4ff987c5d5a8c3995efd15cc4f0db7c4c15e881b198d8ab7f67ecf51f5eb7
fetch hubert-base-ls960.safetensors "${hubert}/model.safetensors" ec18ca2453e7cf3de5e4fc5b79e7484b27506ed0ad684282ddd10ecb9492f37f
fetch us_gold.json "${misaki}/us_gold.json" dc414872a49a28ae6c141463d502fd945f3b2fde040484fdc47d00cc4612686f
fetch us_silver.json "${misaki}/us_silver.json" de8f67be911bb6c659187b4a65fd966b6a30e56350e0f790d763210b053ac475
fetch kokoro-zh/kokoro-v1_1-zh.pth "${kokoro_zh}/kokoro-v1_1-zh.pth" b1d8410fa44dfb5c15471fd6c4225ea6b4e9ac7fa03c98e8bea47a9928476e2b
fetch kokoro-zh/config.json "${kokoro_zh}/config.json" bc333efa5ce4ceff433c8c8e5d027a1eca0166001e4e4a62bea2d26ff7a46890
fetch kokoro-zh/zf_001.pt "${kokoro_zh}/voices/zf_001.pt" 9bdc9a87e13e9bb1ea3e7803259c2ecbfebaeeb2ff80b5d0c76df1a464c1c962

# WavLM is distributed as pytorch_model.bin, so download it and convert it to
# safetensors when the converted file is not there yet.
wavlm_dir="${destination}/wavlm"
mkdir -p "${wavlm_dir}"
safetensors="${wavlm_dir}/wavlm-base-plus.safetensors"
fetch_to "${wavlm_dir}" config.json "${wavlm}/config.json" fea6df1c2700a3954fc07e70588aecc9055eeb28db2ff57151a2db0d19180ed4
if [[ ! -f "${safetensors}" ]]; then
  fetch_to "${wavlm_dir}" pytorch_model.bin "${wavlm}/pytorch_model.bin" 3bb273a6ace99408b50cfc81afdbb7ef2de02da2eab0234e18db608ce692fe51
  echo "Converting ${wavlm_dir}/pytorch_model.bin to safetensors"
  python3 -c "import torch; from safetensors.torch import save_file; state=torch.load(r'${wavlm_dir}/pytorch_model.bin', map_location='cpu', weights_only=True); save_file({k: v.contiguous() for k, v in state.items()}, r'${safetensors}')"
fi
