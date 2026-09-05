#!/usr/bin/env bash

# Linux用開発スクリプトで共有する補助関数。
#
# 通常の環境では.envは不要。GoはPATHまたはsetup-linux.shの配置先、Pythonは.venv、
# CMake/Ninja/Qtはシステムから解決する。.envはQT_ROOTなどの例外的な上書きに使う。

utautts_load_linux_env() {
  local root_dir="$1"
  if [ -f "${root_dir}/.env" ]; then
    set -a
    source <(sed 's/\r$//' "${root_dir}/.env")
    set +a
  fi
}

utautts_resolve_executable() {
  local requested="$1"
  if [[ "${requested}" == */* ]]; then
    [ -x "${requested}" ] || return 1
    printf '%s\n' "${requested}"
    return 0
  fi
  command -v "${requested}"
}

utautts_go_install_root() {
  local user_data_dir="${XDG_DATA_HOME:-${HOME}/.local/share}"
  printf '%s\n' "${UTAUTTS_GO_ROOT:-${user_data_dir}/utautts/go/1.27.0}"
}

utautts_resolve_go() {
  local root_dir="$1"
  local candidate=""

  if [ -n "${GO_BIN:-}" ]; then
    candidate="$(utautts_resolve_executable "${GO_BIN}" || true)"
  fi
  if [ -z "${candidate}" ]; then
    candidate="$(utautts_resolve_executable "$(utautts_go_install_root)/bin/go" || true)"
  fi
  if [ -z "${candidate}" ] && [ -n "${root_dir}" ]; then
    candidate="$(utautts_resolve_executable "${root_dir}/build/toolchain/go/1.27.0/bin/go" || true)"
  fi
  if [ -z "${candidate}" ]; then
    candidate="$(utautts_resolve_executable go || true)"
  fi
  if [ -z "${candidate}" ]; then
    candidate="$(utautts_resolve_executable /usr/local/go/bin/go || true)"
  fi
  [ -n "${candidate}" ] || return 1
  printf '%s\n' "${candidate}"
}

utautts_resolve_python() {
  local root_dir="$1"
  local candidate="${PYTHON:-}"
  if [ -z "${candidate}" ] && [ -x "${root_dir}/.venv/bin/python" ]; then
    candidate="${root_dir}/.venv/bin/python"
  elif [ -z "${candidate}" ]; then
    candidate="python3"
  fi

  candidate="$(utautts_resolve_executable "${candidate}" || true)"
  [ -n "${candidate}" ] || return 1
  printf '%s\n' "${candidate}"
}

utautts_prepend_colon_path() {
  local variable_name="$1"
  local value="$2"
  local current_value="${!variable_name:-}"
  [ -d "${value}" ] || return 0
  if [ -n "${current_value}" ]; then
    printf -v "${variable_name}" '%s:%s' "${value}" "${current_value}"
  else
    printf -v "${variable_name}" '%s' "${value}"
  fi
  export "${variable_name}"
}

utautts_find_qt_config_dir() {
  [ -n "${QT_ROOT:-}" ] || return 1
  local candidate
  for candidate in \
    "${QT_ROOT}/lib/cmake/Qt6" \
    "${QT_ROOT}/lib/x86_64-linux-gnu/cmake/Qt6" \
    "${QT_ROOT}/usr/lib/cmake/Qt6" \
    "${QT_ROOT}/usr/lib/x86_64-linux-gnu/cmake/Qt6"; do
    if [ -f "${candidate}/Qt6Config.cmake" ]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  return 1
}

utautts_configure_qt_environment() {
  [ -n "${QT_ROOT:-}" ] || return 0

  local directory
  for directory in \
    "${QT_ROOT}/bin" \
    "${QT_ROOT}/usr/bin" \
    "${QT_ROOT}/usr/lib/qt6/bin" \
    "${QT_ROOT}/usr/lib/qt6/libexec"; do
    if [ -d "${directory}" ]; then
      case ":${PATH}:" in
        *":${directory}:"*) ;;
        *) PATH="${directory}:${PATH}" ;;
      esac
    fi
  done
  export PATH

  for directory in \
    "${QT_ROOT}/lib" \
    "${QT_ROOT}/lib/x86_64-linux-gnu" \
    "${QT_ROOT}/usr/lib" \
    "${QT_ROOT}/usr/lib/x86_64-linux-gnu"; do
    utautts_prepend_colon_path LD_LIBRARY_PATH "${directory}"
  done

  for directory in \
    "${QT_ROOT}/plugins" \
    "${QT_ROOT}/lib/plugins" \
    "${QT_ROOT}/usr/lib/qt6/plugins" \
    "${QT_ROOT}/usr/lib/x86_64-linux-gnu/qt6/plugins"; do
    utautts_prepend_colon_path QT_PLUGIN_PATH "${directory}"
  done

  for directory in \
    "${QT_ROOT}/qml" \
    "${QT_ROOT}/lib/qml" \
    "${QT_ROOT}/usr/lib/qt6/qml" \
    "${QT_ROOT}/usr/lib/x86_64-linux-gnu/qt6/qml"; do
    utautts_prepend_colon_path QML2_IMPORT_PATH "${directory}"
  done

  for directory in \
    "${QT_ROOT}/lib/pkgconfig" \
    "${QT_ROOT}/lib/x86_64-linux-gnu/pkgconfig" \
    "${QT_ROOT}/usr/lib/pkgconfig" \
    "${QT_ROOT}/usr/lib/x86_64-linux-gnu/pkgconfig" \
    "${QT_ROOT}/share/pkgconfig" \
    "${QT_ROOT}/usr/share/pkgconfig"; do
    utautts_prepend_colon_path PKG_CONFIG_PATH "${directory}"
  done
}
