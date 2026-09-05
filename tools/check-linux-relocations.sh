#!/usr/bin/env bash
set -euo pipefail

# システムQtを読み込まず、互換性のないELFを早期に拒否する。
binary="${1:?usage: check-linux-relocations.sh ELF-executable}"
relocations="$(LC_ALL=C readelf --relocs --wide "${binary}")"
dynamic="$(LC_ALL=C readelf --dynamic --wide "${binary}")"
if grep -Eq 'R_[[:alnum:]_]+_COPY[[:space:]]' <<<"${relocations}"; then
  echo "COPY relocations are forbidden in ${binary}; compile with -fPIC" >&2
  exit 1
fi
if grep -q TEXTREL <<<"${dynamic}"; then
  echo "Text relocations are forbidden in ${binary}" >&2
  exit 1
fi
echo "ELF relocation check passed: ${binary}"
