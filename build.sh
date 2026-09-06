#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
system_name="$(uname -s)"

default_target="linux"
case "${system_name}" in
  MINGW*|MSYS*|CYGWIN*) default_target="win" ;;
  Darwin*) default_target="macos" ;;
esac

target="${1:-${default_target}}"
if [ "$#" -gt 0 ]; then
  shift
fi

run_linux_build_through_wsl() {
  if ! command -v wsl.exe >/dev/null 2>&1; then
    echo 'WSL is required to build the Linux package from Windows.' >&2
    echo 'Install WSL2 with a Linux distribution, then run ./tools/setup-linux.sh inside WSL.' >&2
    exit 1
  fi

  local windows_root="${root_dir}"
  if command -v cygpath >/dev/null 2>&1; then
    windows_root="$(cygpath -w "${root_dir}")"
  elif command -v wslpath >/dev/null 2>&1; then
    windows_root="$(wslpath -w "${root_dir}")"
  fi

  local wsl_root
  wsl_root="$(MSYS_NO_PATHCONV=1 wsl.exe wslpath -a "${windows_root}" 2>/dev/null | tr -d '\r' || true)"
  if [ -z "${wsl_root}" ]; then
    echo 'WSL could not translate the project path.' >&2
    echo 'Ensure a default Linux distribution is installed and initialized.' >&2
    exit 1
  fi

  # MSYS/Git BashによるWSLパスの変換を抑止する。
  MSYS_NO_PATHCONV=1 wsl.exe --cd "${wsl_root}" -- \
    bash -lc 'exec bash ./tools/build-linux.sh "$@"' -- "$@"
}

case "${target}" in
  linux)
    case "${system_name}" in
      MINGW*|MSYS*|CYGWIN*)
        run_linux_build_through_wsl
        ;;
      *)
        exec bash "${root_dir}/tools/build-linux.sh" "$@"
        ;;
    esac
    ;;
  mac|macos|darwin)
    if [[ "${system_name}" != "Darwin" ]]; then
      echo 'The macOS package must be built on macOS; use GitHub Actions or a Mac.' >&2
      exit 1
    fi
    exec bash "${root_dir}/tools/build-macos.sh" "$@"
    ;;
  win|windows)
    case "${system_name}" in
      MINGW*|MSYS*|CYGWIN*)
        if command -v powershell.exe >/dev/null 2>&1; then
          exec powershell.exe -NoProfile -ExecutionPolicy Bypass -File \
            "${root_dir}/tools/build-release.ps1"
        elif command -v pwsh >/dev/null 2>&1; then
          exec pwsh -NoProfile -File "${root_dir}/tools/build-release.ps1"
        fi
        ;;
    esac
    echo 'The Windows package must be built on Windows; use build.bat win there.' >&2
    exit 1
    ;;
  both)
    echo 'Build Windows with build.bat win, Linux with ./build.sh linux, and macOS with ./build.sh macos.' >&2
    exit 1
    ;;
  help|--help|-h)
    cat <<'EOF'
Usage: ./build.sh [linux|macos]

Linux builds run natively on Linux. On Windows, build.bat (or this script from
Git Bash) delegates the Linux target to WSL; the Windows target is native.
macOS builds run natively on macOS or in the macOS GitHub Actions workflow.
EOF
    ;;
  *)
    echo "Unknown build target: ${target}" >&2
    echo 'Usage: ./build.sh [linux|macos]' >&2
    exit 2
    ;;
esac
