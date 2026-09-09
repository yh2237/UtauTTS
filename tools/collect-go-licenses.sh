#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
package_dir="${1:?package directory is required}"
go_command="${2:-go}"
license_dir="${package_dir}/licenses/Go"

copy_required() {
  local source="$1"
  local destination="$2"
  if [[ ! -f "${source}" ]]; then
    echo "required Go license file was not found: ${source}" >&2
    exit 1
  fi
  mkdir -p "$(dirname "${destination}")"
  cp "${source}" "${destination}"
}

mkdir -p "${license_dir}"
go_root="$(${go_command} env GOROOT)"
copy_required "${go_root}/LICENSE" "${license_dir}/GO-LICENSE.txt"
copy_required "${root_dir}/licenses/APACHE-2.0.txt" "${license_dir}/APACHE-2.0.txt"
copy_required "${root_dir}/licenses/Go/CMUDICT-LICENSE.txt" "${license_dir}/CMUDICT-LICENSE.txt"
copy_required "${root_dir}/licenses/Go/PINYIN-DATA-NOTICE.txt" "${license_dir}/PINYIN-DATA-NOTICE.txt"

while IFS= read -r module || [[ -n "${module}" ]]; do
  module="${module%$'\r'}"
  [[ -z "${module}" ]] && continue
  module_info="$(${go_command} list -m -f '{{.Dir}}|{{.Version}}' "${module}")"
  module_dir="${module_info%%|*}"
  module_version="${module_info#*|}"
  if [[ -z "${module_dir}" || "${module_dir}" == "${module_info}" || -z "${module_version}" ]]; then
    echo "could not resolve Go module metadata: ${module_info}" >&2
    exit 1
  fi
  module_name="${module//\//_}"
  module_name="${module_name//./_}"

  license_file=""
  for candidate in LICENSE LICENSE.txt COPYING COPYING.txt; do
    if [[ -f "${module_dir}/${candidate}" ]]; then
      license_file="${module_dir}/${candidate}"
      break
    fi
  done
  if [[ -z "${license_file}" ]]; then
    echo "license file was not found for Go module: ${module}" >&2
    exit 1
  fi
  copy_required "${license_file}" "${license_dir}/${module_name}-${module_version}-LICENSE.txt"

  for notice in NOTICE NOTICE.txt THIRD_PARTY_NOTICES.md THIRD_PARTY_NOTICES.txt DATA_LICENSES.md; do
    if [[ -f "${module_dir}/${notice}" ]]; then
      case "${notice}" in
        NOTICE|NOTICE.txt)
          destination="${license_dir}/${module_name}-${module_version}-NOTICE.txt"
          ;;
        *)
          destination="${license_dir}/${module_name}-${module_version}-${notice}"
          ;;
      esac
      copy_required "${module_dir}/${notice}" "${destination}"
    fi
  done
done < "${root_dir}/tools/go-license-modules.txt"
