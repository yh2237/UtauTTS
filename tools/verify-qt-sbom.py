#!/usr/bin/env python3
"""Verify Qt SBOM files and reject bundled FFmpeg in a GUI package."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path


SHA256_RE = re.compile(r"^[0-9A-Fa-f]{64}$")
MODULES = ("qtbase", "qtdeclarative", "qtmultimedia")
QT_DLL_MODULES = {
    "qtcore": "qtbase",
    "qtgui": "qtbase",
    "qtnetwork": "qtbase",
    "qtopengl": "qtbase",
    "qtconcurrent": "qtbase",
    "qtwidgets": "qtbase",
    "qtprintsupport": "qtbase",
    "qtqml": "qtdeclarative",
    "qtqmlmeta": "qtdeclarative",
    "qtqmlmodels": "qtdeclarative",
    "qtqmlworkerscript": "qtdeclarative",
    "qtquick": "qtdeclarative",
    "qtquickcontrols2": "qtdeclarative",
    "qtquicktemplates2": "qtdeclarative",
    "qtmultimedia": "qtmultimedia",
    "qtmultimediaquick": "qtmultimedia",
    "qtsvg": "qtsvg",
    "qt6core": "qtbase",
    "qt6gui": "qtbase",
    "qt6network": "qtbase",
    "qt6opengl": "qtbase",
    "qt6concurrent": "qtbase",
    "qt6widgets": "qtbase",
    "qt6printsupport": "qtbase",
    "qt6qml": "qtdeclarative",
    "qt6qmlmeta": "qtdeclarative",
    "qt6qmlmodels": "qtdeclarative",
    "qt6qmlworkerscript": "qtdeclarative",
    "qt6quick": "qtdeclarative",
    "qt6quickcontrols2": "qtdeclarative",
    "qt6quicktemplates2": "qtdeclarative",
    "qt6multimedia": "qtmultimedia",
    "qt6multimediaquick": "qtmultimedia",
    "qt6svg": "qtsvg",
}

QT_PREFIX_MODULES = (
    ("qt6multimedia", "qtmultimedia"),
    ("qtmultimedia", "qtmultimedia"),
    ("qt6svg", "qtsvg"),
    ("qtsvg", "qtsvg"),
    ("qt6qml", "qtdeclarative"),
    ("qtqml", "qtdeclarative"),
    ("qt6quick", "qtdeclarative"),
    ("qtquick", "qtdeclarative"),
    ("qt6labs", "qtdeclarative"),
    ("qtlabs", "qtdeclarative"),
    ("qt6core", "qtbase"),
    ("qtcore", "qtbase"),
    ("qt6gui", "qtbase"),
    ("qtgui", "qtbase"),
    ("qt6network", "qtbase"),
    ("qtnetwork", "qtbase"),
    ("qt6opengl", "qtbase"),
    ("qtopengl", "qtbase"),
    ("qtuiotouch", "qtbase"),
)

QT_PATH_MODULES = (
    ("/qml/qtmultimedia/", "qtmultimedia"),
    ("/qml/qtqml/", "qtdeclarative"),
    ("/qml/qtquick/", "qtdeclarative"),
    ("/qml/qt/labs/", "qtdeclarative"),
    ("/multimedia/", "qtmultimedia"),
    ("/imageformats/", "qtbase"),
    ("/iconengines/", "qtsvg"),
    ("/platforms/", "qtbase"),
    ("/generic/", "qtbase"),
    ("/networkinformation/", "qtbase"),
    ("/tls/", "qtbase"),
    ("/qml/", "qtdeclarative"),
)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().upper()


def load_json(path: Path) -> dict[str, object]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SystemExit(f"invalid Qt SBOM JSON: {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise SystemExit(f"Qt SBOM must be an object: {path}")
    return value


def ffmpeg_file(path: Path) -> bool:
    name = path.name.lower()
    if not (
        path.suffix.lower() in {".dll", ".dylib"}
        or re.search(r"\.so(?:\.\d+)*$", name) is not None
    ):
        return False
    if name.startswith("lib"):
        name = name[3:]
    return (
        "ffmpeg" in name
        or re.fullmatch(r"(?:avcodec|avformat|avutil|swresample|swscale)[-._].+", name)
        is not None
    )


def qt_module_for_stem(stem: str) -> str | None:
    stem = stem.lower()
    exact = QT_DLL_MODULES.get(stem)
    if exact is not None:
        return exact
    for prefix, module in QT_PREFIX_MODULES:
        if stem.startswith(prefix):
            return module
    return None


def normalized_binary_stem(path: Path) -> str:
    """Normalize Qt DLL, shared-object, dylib, and framework filenames."""
    name = path.name.lower()
    name = re.sub(r"\.(?:dll|dylib)(?:\.\d+)*$", "", name)
    name = re.sub(r"\.so(?:\.\d+)*$", "", name)
    if name.startswith("lib"):
        name = name[3:]
    return name


def qt_module_for_path(path: Path, root: Path) -> str | None:
    relative = "/" + path.relative_to(root).as_posix().lower() + "/"
    stem = normalized_binary_stem(path)
    if "/imageformats/" in relative and stem.startswith("qsvg"):
        return "qtsvg"
    for marker, module in QT_PATH_MODULES:
        if marker in relative:
            return module
    return qt_module_for_stem(stem)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--package-root", type=Path, required=True)
    parser.add_argument(
        "--sbom-root",
        type=Path,
        help="directory containing the raw Qt SPDX JSON files from the build audit",
    )
    args = parser.parse_args()
    root = args.package_root.resolve()
    qt_root = root / "licenses" / "Qt"
    sbom_root = (
        args.sbom_root.resolve()
        if args.sbom_root is not None
        else qt_root / "sbom"
    )
    if not sbom_root.is_dir():
        hint = " (pass --sbom-root for an audit directory)" if args.sbom_root is None else ""
        raise SystemExit(f"Qt SBOM directory is missing: {sbom_root}{hint}")
    files = sorted(sbom_root.glob("*.spdx.json"))
    if len(files) != len(MODULES):
        raise SystemExit(f"expected {len(MODULES)} Qt SBOM files, found {len(files)}")

    by_module: dict[str, Path] = {}
    for module in MODULES:
        candidates = sorted(sbom_root.glob(f"{module}-*.spdx.json"))
        if len(candidates) != 1:
            raise SystemExit(f"expected one Qt SBOM for {module}, found {len(candidates)}")
        by_module[module] = candidates[0]

    manifest_path = qt_root / "Qt-SBOM-MANIFEST.txt"
    if not manifest_path.is_file():
        raise SystemExit(f"Qt SBOM manifest is missing: {manifest_path}")
    manifest_text = manifest_path.read_text(encoding="utf-8", errors="replace")
    for module, path in by_module.items():
        expected_hash = sha256_file(path)
        line = re.search(rf"(?m)^{re.escape(path.name)}\s*$\n\s*SHA-256:\s*([0-9A-Fa-f]+)\s*$", manifest_text)
        if line is None or not SHA256_RE.fullmatch(line.group(1)) or line.group(1).upper() != expected_hash:
            raise SystemExit(f"Qt SBOM hash is missing or incorrect: {path.name}")
        data = load_json(path)
        if not str(data.get("spdxVersion", "")).startswith("SPDX-"):
            raise SystemExit(f"Qt SBOM has no SPDX version: {path.name}")
        if not isinstance(data.get("packages"), list) or not data["packages"]:
            raise SystemExit(f"Qt SBOM has no package list: {path.name}")
        if not str(data.get("name", "")).startswith(f"{module}-"):
            raise SystemExit(f"Qt SBOM document name does not match {module}: {path.name}")

    multimedia = load_json(by_module["qtmultimedia"])
    ffmpeg = [
        item
        for item in multimedia.get("packages", [])
        if isinstance(item, dict) and item.get("name") == "FFmpeg"
    ]
    if len(ffmpeg) != 1:
        raise SystemExit("qtmultimedia SBOM does not identify exactly one FFmpeg package")
    packaged_ffmpeg = {
        path.relative_to(root).as_posix(): sha256_file(path)
        for path in root.rglob("*")
        if path.is_file() and ffmpeg_file(path)
    }
    if packaged_ffmpeg:
        names = ", ".join(sorted(packaged_ffmpeg))
        raise SystemExit(f"FFmpeg files must not be bundled: {names}")
    optional_notice = qt_root / "FFmpeg-OPTIONAL.txt"
    if not optional_notice.is_file():
        raise SystemExit("FFmpeg-OPTIONAL.txt is missing")
    for stale_notice in (
        qt_root / "FFmpeg-SOURCE-AND-LICENSE.txt",
        qt_root / "FFmpeg-SOURCE-OFFER.txt",
        qt_root / "LGPL-2.1.txt",
    ):
        if stale_notice.exists():
            raise SystemExit(f"obsolete bundled-FFmpeg license file is present: {stale_notice.name}")

    detected_modules: set[str] = set()
    for path in root.rglob("*"):
        if not path.is_file():
            continue
        stem = normalized_binary_stem(path)
        is_framework_binary = ".framework" in path.parts
        if not (
            path.suffix.lower() in {".dll", ".dylib", ".so"} or is_framework_binary
        ):
            continue
        module = qt_module_for_path(path, root)
        if module is None:
            if stem.startswith("qt"):
                raise SystemExit(f"Qt library has no matching SBOM module: {path.name}")
            continue
        detected_modules.add(module)
    if not detected_modules:
        raise SystemExit("no Qt libraries were found in the package")
    missing = detected_modules - set(by_module)
    if missing:
        raise SystemExit(f"Qt libraries have no matching SBOM: {', '.join(sorted(missing))}")
    print(f"verified Qt SBOM: {sbom_root}")
    print(f"modules: {', '.join(MODULES)}; FFmpeg bundled: no")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
