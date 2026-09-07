#!/usr/bin/env python3
"""Validate model license metadata and copy the referenced notices.

Model metadata uses a package-root-relative POSIX path for ``license_notice``.
Only files below the repository's ``licenses/`` directory may be referenced.
"""

from __future__ import annotations

import argparse
import json
import shutil
import sys
from pathlib import Path, PurePosixPath


def parse_notice(value: object, model_path: Path) -> PurePosixPath:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{model_path.name}: license_notice is required")

    notice = value.strip()
    relative = PurePosixPath(notice)
    if (
        notice != value
        or relative.is_absolute()
        or notice != relative.as_posix()
        or len(relative.parts) < 2
        or relative.parts[0] != "licenses"
        or any(part in {"", ".", ".."} for part in relative.parts)
        or any("\\" in part or ":" in part for part in relative.parts)
    ):
        raise ValueError(
            f"{model_path.name}: license_notice must be a normalized path below licenses/"
        )
    return relative


def load_models(models_root: Path) -> list[tuple[Path, PurePosixPath]]:
    model_paths = sorted(models_root.glob("*.json"))
    if not model_paths:
        raise ValueError(f"no model JSON files found: {models_root}")

    notices: list[tuple[Path, PurePosixPath]] = []
    for model_path in model_paths:
        try:
            metadata = json.loads(model_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            raise ValueError(f"{model_path.name}: invalid model JSON: {exc}") from exc
        if not isinstance(metadata.get("license"), str) or not metadata["license"].strip():
            raise ValueError(f"{model_path.name}: license is required")
        notices.append((model_path, parse_notice(metadata.get("license_notice"), model_path)))
    return notices


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Validate model license metadata and copy referenced notices"
    )
    parser.add_argument("--models", type=Path, required=True)
    parser.add_argument("--package-root", type=Path, required=True)
    parser.add_argument("--repository-root", type=Path)
    parser.add_argument("--check-only", action="store_true")
    args = parser.parse_args()

    try:
        models_root = args.models.resolve()
        package_root = args.package_root.resolve()
        notices = load_models(models_root)
        if args.check_only:
            for model_path, relative in notices:
                destination = package_root.joinpath(*relative.parts)
                if not destination.is_file():
                    raise ValueError(
                        f"{model_path.name}: packaged license notice is missing: "
                        f"{relative.as_posix()}"
                    )
            return 0

        if args.repository_root is None:
            parser.error("--repository-root is required unless --check-only is used")
        repository_root = args.repository_root.resolve()
        licenses_root = (repository_root / "licenses").resolve()
        copied: set[PurePosixPath] = set()
        for model_path, relative in notices:
            source = (repository_root / Path(*relative.parts)).resolve()
            try:
                source.relative_to(licenses_root)
            except ValueError as exc:
                raise ValueError(
                    f"{model_path.name}: license notice escapes licenses/: {relative.as_posix()}"
                ) from exc
            if not source.is_file():
                raise ValueError(
                    f"{model_path.name}: license notice was not found: {relative.as_posix()}"
                )
            destination = package_root.joinpath(*relative.parts)
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, destination)
            copied.add(relative)
        for relative in sorted(copied, key=lambda path: path.as_posix()):
            print(f"model license notice: {relative.as_posix()}")
        return 0
    except (OSError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
