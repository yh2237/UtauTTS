#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
required_go_version="1.27.0"
go_archive_sha256="675c26c449cbb18fc24b74650de1eabbae6e16f64326fd85a283fb3b58280685"

# shellcheck source=linux-environment.sh
source "${root_dir}/tools/linux-environment.sh"
utautts_load_linux_env "${root_dir}"

usage() {
  cat <<'EOF'
Usage: tools/setup-linux.sh

Install Debian/Ubuntu build prerequisites, create .venv, install the pinned
Open JTalk/PyInstaller build dependencies, and configure the Go 1.27 toolchain.
The resulting tools are found automatically; a .env file is not normally needed.

Set UTAUTTS_SKIP_APT=1 when the system packages are already available or when
you are using a preconfigured container. Set UTAUTTS_GO_ROOT to choose the
user-local directory used for the official Go archive.
EOF
}

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  usage
  exit 0
fi

case "$(uname -s)" in
  Linux) ;;
  *) echo 'This setup script is for Linux.' >&2; exit 1 ;;
esac

if [ "${UTAUTTS_SKIP_APT:-0}" != "1" ]; then
  if ! command -v apt-get >/dev/null 2>&1; then
    echo 'apt-get was not found. Install the Debian/Ubuntu prerequisites manually.' >&2
    exit 1
  fi

  packages=(
    build-essential cmake ninja-build pkg-config unzip zip curl ca-certificates
    python3-dev python3-pip python3-venv
    qt6-base-dev qt6-declarative-dev qt6-multimedia-dev qt6-tools-dev qt6-l10n-tools
    qml6-module-qtquick-controls qml6-module-qtmultimedia
    fontconfig fonts-noto-cjk
  )
  if [ "$(id -u)" -eq 0 ]; then
    apt_command=(apt-get)
  elif command -v sudo >/dev/null 2>&1; then
    apt_command=(sudo apt-get)
  else
    echo 'Root privileges are required for the Debian packages.' >&2
    printf 'Run: sudo apt-get update && sudo apt-get install -y %s\n' "${packages[*]}" >&2
    exit 1
  fi
  "${apt_command[@]}" update
  "${apt_command[@]}" install -y "${packages[@]}"
fi

go_is_usable() {
  local candidate="$1"
  local version
  version="$("${candidate}" version 2>/dev/null | awk '{print $3}' | sed 's/^go//')"
  [ -n "${version}" ] || return 1
  [ "$(printf '%s\n' "${required_go_version}" "${version}" | sort -V | head -n 1)" = "${required_go_version}" ]
}

go_command=""
if [ -n "${GO_BIN:-}" ] && go_command="$(utautts_resolve_executable "${GO_BIN}")" && go_is_usable "${go_command}"; then
  :
elif candidate_go="$(utautts_resolve_executable go 2>/dev/null || true)" && go_is_usable "${candidate_go}"; then
  go_command="${candidate_go}"
elif candidate_go="$(utautts_resolve_executable "$(utautts_go_install_root)/bin/go" 2>/dev/null || true)" && go_is_usable "${candidate_go}"; then
  go_command="${candidate_go}"
else
  go_install_parent="$(utautts_go_install_root)"
  temporary_dir="$(mktemp -d -t utautts-go-XXXXXX)"
  trap 'rm -rf "${temporary_dir}"' EXIT
  archive_path="${temporary_dir}/go.tar.gz"
  echo "Downloading Go ${required_go_version}..."
  curl -fL --retry 3 --connect-timeout 15 \
    -o "${archive_path}" \
    "https://go.dev/dl/go${required_go_version}.linux-amd64.tar.gz"
  printf '%s  %s\n' "${go_archive_sha256}" "${archive_path}" | sha256sum -c -
  mkdir -p "${temporary_dir}/extract" "$(dirname "${go_install_parent}")"
  tar -xzf "${archive_path}" -C "${temporary_dir}/extract"
  if [ -e "${go_install_parent}" ]; then
    rm -rf "${go_install_parent}"
  fi
  mv "${temporary_dir}/extract/go" "${go_install_parent}"
  go_command="${go_install_parent}/bin/go"
fi

python_command="$(utautts_resolve_executable "${PYTHON:-python3}" || true)"
[ -n "${python_command}" ] || { echo 'python3 is required' >&2; exit 1; }

venv_dir="${UTAUTTS_VENV:-${root_dir}/.venv}"
if [ ! -x "${venv_dir}/bin/python" ]; then
  "${python_command}" -m venv "${venv_dir}" || {
    echo 'Python venv creation failed; install python3-venv and retry.' >&2
    exit 1
  }
fi
"${venv_dir}/bin/python" -m pip install --disable-pip-version-check \
  --requirement "${root_dir}/tools/requirements-build.txt"

utautts_configure_qt_environment
cmake_command="$(utautts_resolve_executable "${CMAKE:-cmake}" || true)"
ninja_command="$(utautts_resolve_executable "${NINJA:-ninja}" || true)"
for required_command in "${cmake_command}" "${ninja_command}" \
  "$(utautts_resolve_executable pkg-config || true)" \
  "$(utautts_resolve_executable zip || true)" "$(utautts_resolve_executable unzip || true)" \
  "$(utautts_resolve_executable curl || true)"; do
  if [ -z "${required_command}" ]; then
    echo 'cmake, ninja, pkg-config, zip, unzip, and curl are required; install the Debian prerequisites and retry.' >&2
    exit 1
  fi
done

pkg_config_command="$(utautts_resolve_executable pkg-config)"
if ! "${pkg_config_command}" --exists Qt6Core Qt6Quick Qt6QuickControls2 Qt6Multimedia 2>/dev/null; then
  echo 'Qt 6.5+ development modules were not found through pkg-config.' >&2
  echo 'Install qt6-base-dev, qt6-declarative-dev, qt6-multimedia-dev, and the QML modules.' >&2
  exit 1
fi

if [ -e "${root_dir}/.env" ]; then
  echo "Using optional overrides from ${root_dir}/.env"
else
  echo 'No .env file was created; standard tool locations are detected automatically.'
fi

echo "Go: $(${go_command} version)"
echo "Python: $(${venv_dir}/bin/python --version)"
echo "Qt: $("${pkg_config_command}" --modversion Qt6Core)"
echo 'Linux development environment is ready. Run ./build.sh linux or ./tools/test.sh.'
