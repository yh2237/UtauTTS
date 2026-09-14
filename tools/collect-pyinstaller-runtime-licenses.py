#!/usr/bin/env python3
"""Inspect a PyInstaller archive and collect its runtime license notices."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import shutil
import sys
import sysconfig
from pathlib import Path
from typing import Any


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest().upper()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().upper()


NATIVE_SUFFIXES = (".dll", ".dylib", ".so", ".pyd")
SHA256_RE = re.compile(r"^[0-9A-Fa-f]{64}$")
MICROSOFT_RUNTIME_PATTERNS = (
    r"^MSVCP\d+(?:_\d+)?\.dll$",
    r"^VCRUNTIME\d+(?:_\d+)?\.dll$",
    r"^ucrtbase\.dll$",
    r"^api-ms-win-(?:core|crt)-[a-z0-9-]+\.dll$",
)


def stdlib_native_names() -> set[str]:
    """Return native extension names shipped by the active interpreter."""
    roots: set[Path] = set()
    for key in ("platstdlib", "stdlib"):
        value = sysconfig.get_path(key)
        if value:
            roots.add(Path(value))
    shared = sysconfig.get_config_var("DESTSHARED")
    if shared:
        roots.add(Path(shared))
    for root in tuple(roots):
        roots.add(root / "lib-dynload")
    # Windows stores the standard-library extensions beside the interpreter
    # in DLLs rather than under the Lib directory.
    executable_dir = Path(sys.executable).resolve().parent
    roots.update({executable_dir, executable_dir / "DLLs"})
    bindir = sysconfig.get_config_var("BINDIR")
    if bindir:
        roots.update({Path(bindir), Path(bindir) / "DLLs"})
    names: set[str] = set()
    for root in roots:
        if not root.is_dir():
            continue
        for path in root.iterdir():
            if path.is_file() and path.name.lower().endswith(NATIVE_SUFFIXES):
                names.add(path.name.lower())
    return names


def classify_entry(name: str, native_names: set[str]) -> tuple[str, list[str]]:
    lower = name.lower().replace("\\", "/")
    basename = lower.rsplit("/", 1)[-1]
    if basename.startswith(("libcrypto", "libssl")):
        return "OpenSSL (forbidden)", []
    if basename.startswith(("msvcp", "vcruntime", "ucrtbase", "api-ms-win")):
        return "Microsoft Visual C++/UCRT", [
            "runtime/licenses/MICROSOFT-RUNTIME-NOTICE.txt",
        ]
    if basename == "openjtalk.pyd" or basename.startswith(("openjtalk.", "htsengine.")):
        return "pyopenjtalk/Open JTalk", [
            "THIRD_PARTY_NOTICES.txt",
            "licenses/OpenJTalk/OPENJTALK_COPYING.txt",
        ]
    if re.fullmatch(r"python\d+\.dll", basename):
        return "CPython", ["runtime/licenses/PYTHON_LICENSE.txt"]
    # Linux and macOS builds usually carry the shared interpreter under a
    # libpython*.so/.dylib name instead of pythonXY.dll.
    if re.fullmatch(r"libpython\d+(?:\.\d+)+.*", basename):
        return "CPython", ["runtime/licenses/PYTHON_LICENSE.txt"]
    if basename in {"pyz.pyz", "base_library.zip"} or basename.startswith(
        ("pyi_", "pyiboot", "pyimod")
    ):
        return "PyInstaller", ["runtime/licenses/PYINSTALLER_COPYING.txt"]
    if basename == "openjtalk-feature-bridge":
        return "UtauTTS source", ["LICENSE"]
    if basename in native_names:
        return "CPython native extension", ["runtime/licenses/PYTHON_LICENSE.txt"]
    if basename.endswith(NATIVE_SUFFIXES):
        return "Unclassified native", []
    if basename.endswith((".py", ".pyc")) or basename == "struct" or basename.startswith("_"):
        return "CPython", ["runtime/licenses/PYTHON_LICENSE.txt"]
    return "CPython/PyInstaller", [
        "runtime/licenses/PYTHON_LICENSE.txt",
        "runtime/licenses/PYINSTALLER_COPYING.txt",
    ]


def is_microsoft_runtime_name(name: str) -> bool:
    return any(
        re.fullmatch(pattern, name, flags=re.IGNORECASE)
        for pattern in MICROSOFT_RUNTIME_PATTERNS
    )


def write_text(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text.rstrip() + "\n", encoding="utf-8")


def copy_file_if_needed(source: Path, destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    if source.resolve() != destination.resolve():
        shutil.copyfile(source, destination)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Inspect a PyInstaller CArchive and write runtime license notices"
    )
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--python-license", type=Path, required=True)
    parser.add_argument("--pyinstaller-license", type=Path, required=True)
    parser.add_argument(
        "--microsoft-runtime-source-manifest",
        type=Path,
        help="JSON manifest describing the official Microsoft runtime files used for staging",
    )
    args = parser.parse_args()

    archive_path = args.archive.resolve()
    output_dir = args.output_dir.resolve()
    if not archive_path.is_file():
        raise SystemExit(f"PyInstaller archive was not found: {archive_path}")
    for source in (args.python_license, args.pyinstaller_license):
        if not source.is_file():
            raise SystemExit(f"Required license source was not found: {source}")

    try:
        from PyInstaller.archive.readers import CArchiveReader
    except ImportError as exc:
        raise SystemExit(
            "PyInstaller is required to inspect the helper archive"
        ) from exc

    reader = CArchiveReader(str(archive_path))
    native_names = stdlib_native_names()
    entries: list[dict[str, Any]] = []
    for name in sorted(reader.toc):
        data = reader.extract(name)
        if not isinstance(data, bytes):
            raise SystemExit(f"Could not extract PyInstaller entry: {name}")
        component, license_files = classify_entry(name, native_names)
        entry = reader.toc[name]
        entries.append(
            {
                "name": name,
                "typecode": entry[4],
                "size": len(data),
                "sha256": sha256_bytes(data),
                "component": component,
                "license_files": license_files,
            }
        )

    native_unknown = [
        entry["name"]
        for entry in entries
        if entry["name"].lower().endswith((".dll", ".dylib", ".so", ".pyd"))
        and entry["component"] == "Unclassified native"
    ]
    if native_unknown:
        raise SystemExit(
            "Unclassified native entries require an explicit license mapping: "
            + ", ".join(native_unknown)
        )
    forbidden_openssl = [
        entry["name"] for entry in entries if entry["component"] == "OpenSSL (forbidden)"
    ]
    if forbidden_openssl:
        raise SystemExit(
            "OpenSSL runtime entries are not allowed in the helper; remove _hashlib: "
            + ", ".join(forbidden_openssl)
        )

    output_dir.mkdir(parents=True, exist_ok=True)
    for stale_name in ("OPENSSL-NOTICE.txt", "OPENSSL-LICENSE.txt"):
        stale_path = output_dir / stale_name
        if stale_path.exists():
            stale_path.unlink()
    copy_file_if_needed(args.python_license, output_dir / "PYTHON_LICENSE.txt")
    copy_file_if_needed(args.pyinstaller_license, output_dir / "PYINSTALLER_COPYING.txt")

    archive_relative_name = archive_path.name
    archive_hash = sha256_file(archive_path)
    microsoft_entries = [
        entry for entry in entries
        if entry["component"] == "Microsoft Visual C++/UCRT"
    ]
    unsupported_microsoft_entries = [
        entry["name"]
        for entry in microsoft_entries
        if not is_microsoft_runtime_name(entry["name"].rsplit("/", 1)[-1])
    ]
    if unsupported_microsoft_entries:
        raise SystemExit(
            "Microsoft runtime entries require an explicit redistribution mapping: "
            + ", ".join(unsupported_microsoft_entries)
        )

    microsoft_source_manifest: dict[str, Any] | None = None
    microsoft_source_files: dict[str, dict[str, Any]] = {}
    if microsoft_entries:
        if args.microsoft_runtime_source_manifest is None:
            raise SystemExit(
                "Microsoft runtime entries require the official runtime source manifest"
            )
        source_manifest_path = args.microsoft_runtime_source_manifest.resolve()
        try:
            microsoft_source_manifest = json.loads(
                source_manifest_path.read_text(encoding="utf-8-sig")
            )
        except (OSError, json.JSONDecodeError) as exc:
            raise SystemExit(
                f"invalid Microsoft runtime source manifest: {exc}"
            ) from exc
        if not isinstance(microsoft_source_manifest, dict) or microsoft_source_manifest.get(
            "kind"
        ) != "microsoft-runtime-source-manifest":
            raise SystemExit("unexpected Microsoft runtime source manifest kind")
        if microsoft_source_manifest.get("format_version") != 1:
            raise SystemExit("unsupported Microsoft runtime source manifest version")
        if microsoft_source_manifest.get("architecture") != "x64":
            raise SystemExit("Microsoft runtime source manifest must describe x64 files")
        source_files = microsoft_source_manifest.get("files")
        if not isinstance(source_files, list) or not source_files:
            raise SystemExit("Microsoft runtime source manifest has no file records")
        for source_file in source_files:
            if not isinstance(source_file, dict):
                raise SystemExit("Microsoft runtime source manifest contains a malformed file")
            source_name = source_file.get("name")
            source_hash = source_file.get("sha256")
            if not isinstance(source_name, str) or not isinstance(source_hash, str):
                raise SystemExit("Microsoft runtime source file record is incomplete")
            if not is_microsoft_runtime_name(source_name) or not SHA256_RE.fullmatch(
                source_hash.upper()
            ):
                raise SystemExit(
                    f"Microsoft runtime source file is outside the filename policy: {source_name}"
                )
            source_key = source_name.lower()
            if source_key in microsoft_source_files:
                raise SystemExit(
                    f"Microsoft runtime source manifest has a duplicate file: {source_name}"
                )
            microsoft_source_files[source_key] = source_file
        for entry in microsoft_entries:
            source_file = microsoft_source_files.get(
                entry["name"].rsplit("/", 1)[-1].lower()
            )
            if source_file is None:
                raise SystemExit(
                    "Microsoft runtime entry has no matching official source file: "
                    + entry["name"]
                )
            if entry["sha256"].upper() != str(source_file["sha256"]).upper():
                raise SystemExit(
                    "Microsoft runtime entry does not match the staged official source: "
                    + entry["name"]
                )

    if microsoft_entries:
        assert microsoft_source_manifest is not None
        source_sections = microsoft_source_manifest.get("sources")
        if not isinstance(source_sections, dict):
            raise SystemExit("Microsoft runtime source manifest has no source sections")
        visual_cpp_source = source_sections.get("visual_cpp_redist")
        ucrt_source = source_sections.get("windows_sdk_ucrt")
        if not isinstance(visual_cpp_source, dict) or not isinstance(ucrt_source, dict):
            raise SystemExit("Microsoft runtime source manifest has incomplete source sections")
        python_release_url = (
            "https://www.python.org/downloads/release/python-"
            + sys.version.split()[0].replace(".", "")
            + "/"
        )
        lines = [
            "Microsoft Visual C++ and UCRT runtime notice",
            "==============================================",
            "",
            "The PyInstaller helper contains the Microsoft runtime files listed",
            "below. The files are copied unmodified from the official Microsoft",
            "Visual C++ Redistributable and Windows SDK UCRT directories.",
            "Redistribution is permitted only for files covered by the applicable",
            "Microsoft Visual C++ Redistributable and UCRT terms.",
            "",
            f"Python build interpreter: {sys.version.splitlines()[0]}",
            f"Helper archive: {archive_relative_name}",
            f"Helper archive SHA-256: {archive_hash}",
            "",
            "Visual C++ source package:",
            f"{visual_cpp_source.get('family', 'Microsoft Visual C++ Redistributable')} {visual_cpp_source.get('version', 'unknown')}",
            "Windows SDK UCRT source package:",
            f"{ucrt_source.get('family', 'Windows SDK UCRT Redist')} {ucrt_source.get('version', 'unknown')}",
            "",
            "Microsoft redistribution guidance:",
            "https://learn.microsoft.com/en-us/cpp/windows/determining-which-dlls-to-redistribute?view=msvc-170",
            "https://learn.microsoft.com/en-us/cpp/windows/universal-crt-deployment?view=msvc-170",
            "https://learn.microsoft.com/en-us/cpp/windows/redistributing-visual-cpp-files?view=msvc-170",
            "",
            "Python build information (the helper build environment, not the Microsoft DLL source):",
            python_release_url,
            "",
            "Allowed runtime filename patterns:",
            *[f"{pattern}" for pattern in MICROSOFT_RUNTIME_PATTERNS],
            "",
            "Embedded Microsoft runtime entries:",
        ]
        for entry in microsoft_entries:
            lines.extend(
                [
                    f"{entry['name']} ({entry['size']} bytes)",
                    f"  SHA-256: {entry['sha256']}",
                ]
            )
        write_text(
            output_dir / "MICROSOFT-RUNTIME-NOTICE.txt",
            "\n".join(lines),
        )
    else:
        stale_microsoft_notice = output_dir / "MICROSOFT-RUNTIME-NOTICE.txt"
        if stale_microsoft_notice.exists():
            stale_microsoft_notice.unlink()

    print(f"Inspected PyInstaller archive: {archive_path}")
    print(f"entries: {len(entries)}")
    if microsoft_entries:
        print(f"Microsoft runtime entries: {len(microsoft_entries)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
