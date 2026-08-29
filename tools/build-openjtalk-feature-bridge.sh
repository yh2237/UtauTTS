#!/usr/bin/env bash
set -euo pipefail

# Linux向けutautts-openjtalk-featuresをPyInstallerで単一ファイル化する。
#
# PYTHON: pipを利用できるPython（既定: リポジトリ/.venv/bin/python または python3）

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=linux-environment.sh
source "${root_dir}/tools/linux-environment.sh"
utautts_load_linux_env "${root_dir}"

python_bin="$(utautts_resolve_python "${root_dir}" || true)"
if [ -z "${python_bin}" ]; then
  echo 'python3 is required; run tools/setup-linux.sh or set PYTHON for a custom interpreter' >&2
  exit 1
fi

package_root="${root_dir}/.tmp-openjtalk-linux"
pyinstaller_root="${root_dir}/.tmp-pyinstaller-linux"
if [ ! -d "${package_root}/pyopenjtalk" ]; then
  "${python_bin}" -m pip install --target "${package_root}" 'pyopenjtalk==0.4.1'
fi

dictionary_path="${package_root}/pyopenjtalk/open_jtalk_dic_utf_8-1.11"
if [ ! -d "${dictionary_path}" ]; then
  if [ -d "${root_dir}/.tmp-openjtalk/pyopenjtalk/open_jtalk_dic_utf_8-1.11" ]; then
    mkdir -p "$(dirname "${dictionary_path}")"
    cp -R "${root_dir}/.tmp-openjtalk/pyopenjtalk/open_jtalk_dic_utf_8-1.11" "${dictionary_path}"
  else
    PYTHONPATH="${package_root}" "${python_bin}" -c "import pyopenjtalk; pyopenjtalk.run_frontend('テスト')"
  fi
fi

if [ ! -d "${pyinstaller_root}/PyInstaller" ]; then
  "${python_bin}" -m pip install --target "${pyinstaller_root}" 'pyinstaller==6.16.0'
fi

extension="$(find "${package_root}/pyopenjtalk" -maxdepth 1 -name 'openjtalk.cpython-*-x86_64-linux-gnu.so' | head -1)"
if [ -z "${extension}" ]; then
  echo "no Linux pyopenjtalk extension (openjtalk.cpython-*-x86_64-linux-gnu.so) in ${package_root}/pyopenjtalk" >&2
  exit 1
fi

input_path="${root_dir}/.tmp-openjtalk-bridge-input"
work_path="${root_dir}/.tmp-openjtalk-bridge-build"
spec_path="${root_dir}/.tmp-openjtalk-bridge-spec"
dist_path="${root_dir}/tools/openjtalk-feature-bridge/bin"
rm -rf "${input_path}" "${work_path}" "${spec_path}" "${dist_path}"
mkdir -p "${input_path}" "${work_path}" "${spec_path}" "${dist_path}"
cp "${extension}" "${input_path}/openjtalk.so"

PYTHONPATH="${pyinstaller_root}" "${python_bin}" -m PyInstaller --noconfirm --clean --onefile \
  --name utautts-openjtalk-features \
  --paths "${input_path}" \
  --hidden-import openjtalk \
  --distpath "${dist_path}" \
  --workpath "${work_path}" \
  --specpath "${spec_path}" \
  "${root_dir}/tools/openjtalk-feature-bridge.py"

helper_path="${dist_path}/utautts-openjtalk-features"
"${python_bin}" "${root_dir}/tools/verify-openjtalk-feature-bridge.py" \
  --helper "${helper_path}" --dictionary "${dictionary_path}" \
  --corpus "${root_dir}/out/prosody/openjtalk-accent-features-v1.json"
ls -la "${helper_path}"
