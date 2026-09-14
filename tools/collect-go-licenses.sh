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
for stale_file in "${license_dir}"/*; do
  [[ -f "${stale_file}" ]] && rm -f "${stale_file}"
done
go_root="$(${go_command} env GOROOT)"
copy_required "${go_root}/LICENSE" "${license_dir}/GO-LICENSE.txt"
copy_required "${root_dir}/licenses/Go/CMUDICT-LICENSE.txt" "${license_dir}/CMUDICT-LICENSE.txt"
copy_required "${root_dir}/licenses/Go/PINYIN-DATA-NOTICE.txt" "${license_dir}/PINYIN-DATA-NOTICE.txt"
license_destinations=("${license_dir}/GO-LICENSE.txt")

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

  license_count=0
  while IFS= read -r source; do
    relative="${source#"${module_dir}/"}"
    basename="${source##*/}"
    # macOS ships Bash 3.2, which does not support ${parameter^^}.
    basename_upper="$(printf '%s' "${basename}" | tr '[:lower:]' '[:upper:]')"
    case "${basename_upper}" in
      LICENSE|LICENSE.*|COPYING|COPYING.*|PATENTS|PATENTS.*|NOTICE|NOTICE.*|THIRD_PARTY_NOTICES*|DATA_LICENSES*) ;;
      *) continue ;;
    esac
    is_primary_license=0
    case "${basename_upper}" in
      LICENSE|LICENSE.*|COPYING|COPYING.*)
        is_primary_license=1
        license_count=$((license_count + 1))
        ;;
    esac
    if [[ "${is_primary_license}" -eq 1 ]]; then
      duplicate=0
      for existing_destination in "${license_destinations[@]}"; do
        if cmp -s "${source}" "${existing_destination}"; then
          duplicate=1
          break
        fi
      done
      [[ "${duplicate}" -eq 1 ]] && continue
    fi
    destination_name="${relative//\//__}"
    case "${relative}" in
      LICENSE) destination_name='LICENSE.txt' ;;
      NOTICE) destination_name='NOTICE.txt' ;;
      PATENTS) destination_name='PATENTS.txt' ;;
    esac
    destination="${license_dir}/${module_name}-${module_version}-${destination_name}"
    copy_required "${source}" "${destination}"
    if [[ "${is_primary_license}" -eq 1 ]]; then
      license_destinations+=("${destination}")
    fi
  done < <(find "${module_dir}" -type f -print | LC_ALL=C sort)
  if [[ "${license_count}" -eq 0 ]]; then
    echo "license file was not found for Go module: ${module}" >&2
    exit 1
  fi
done < "${root_dir}/tools/go-license-modules.txt"
