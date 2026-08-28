#!/usr/bin/env python3
"""学習済み境界補修をUtauTTSのWAVとPlanへ診断適用する。"""

from __future__ import annotations

import argparse
import importlib.util
import json
from pathlib import Path

import torch
from torch.nn import functional as F


def load_training_module():
    path = Path(__file__).with_name("train-boundary-refiner.py")
    spec = importlib.util.spec_from_file_location("utautts_boundary_refiner_training", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


training = load_training_module()


def boundary_positions(
    plan: dict, sample_rate: int, frames: int, position_mode: str
) -> list[tuple[int, int, str]]:
    result = []
    seen = set()
    for index, unit in enumerate(plan.get("units", [])):
        if unit.get("silent") or unit.get("role", "mora") != "mora":
            continue
        note_start_ms = float(unit.get("note_start_ms", 0))
        position_ms = note_start_ms
        if position_mode == "join":
            preutterance_ms = float(unit.get("effective_preutterance_ms", 0))
            overlap_ms = max(5.0, float(unit.get("effective_overlap_ms", 0)))
            position_ms = note_start_ms - preutterance_ms + overlap_ms / 2
        position = round(position_ms * sample_rate / 1000)
        if position <= 0 or position >= frames or position in seen:
            continue
        seen.add(position)
        result.append((index, position, str(unit.get("alias", ""))))
    return result


def resample(values: torch.Tensor, samples: int) -> torch.Tensor:
    if values.numel() == samples:
        return values
    return F.interpolate(values.view(1, 1, -1), size=samples, mode="linear", align_corners=False).view(-1)


@torch.no_grad()
def refine_boundary(
    wave: torch.Tensor,
    center: int,
    source_rate: int,
    model: torch.nn.Module,
    model_rate: int,
    model_samples: int,
    mask: torch.Tensor,
    strength: float,
) -> tuple[float, bool]:
    source_samples = round(model_samples * source_rate / model_rate)
    start = center - source_samples // 2
    end = start + source_samples
    if start < 0 or end > wave.numel():
        return 0.0, False
    original = wave[start:end].clone()
    normalized = resample(original, model_samples)
    peak = max(0.05, float(normalized.abs().max()))
    scale = 0.9 / peak
    normalized *= scale
    values = torch.stack((normalized, mask)).unsqueeze(0)
    refined = model(values)[0, 0]
    residual = resample((refined - normalized) / scale, source_samples) * strength
    candidate = original + residual
    if not bool(torch.isfinite(candidate).all()) or float(candidate.abs().max()) > 1.25:
        return 0.0, False
    wave[start:end] = candidate.clamp(-1, 1)
    return float(residual.abs().max()), True


def write_html(path: Path, original: Path, refined: Path) -> None:
    document = f"""<!doctype html><meta charset="utf-8"><title>Boundary refiner comparison</title>
<style>body{{font-family:sans-serif;max-width:800px;margin:40px auto}}audio{{display:block;width:100%;margin:8px 0 24px}}</style>
<h1>Boundary refiner comparison</h1>
<label>従来のClassic faithful</label><audio controls src="{original.name}"></audio>
<label>ニューラル境界補修</label><audio controls src="{refined.name}"></audio>
"""
    path.write_text(document, encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--plan", required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--report", default="")
    parser.add_argument("--strength", type=float, default=0.25)
    parser.add_argument("--position", choices=("join", "note"), default="join")
    args = parser.parse_args()
    if not 0 <= args.strength <= 1:
        raise SystemExit("--strength must be between 0 and 1")

    input_path = Path(args.input).resolve()
    output_path = Path(args.out).resolve()
    plan = json.loads(Path(args.plan).read_text(encoding="utf-8"))
    checkpoint = torch.load(args.model, map_location="cpu", weights_only=True)
    if checkpoint.get("version") != 1:
        raise SystemExit("unsupported boundary refiner model")
    model_rate = int(checkpoint["sample_rate"])
    model_samples = int(checkpoint["window_samples"])
    repair_samples = int(checkpoint["repair_samples"])
    fade_samples = int(checkpoint.get("fade_samples", round(model_rate * 0.008)))
    model = training.BoundaryRefiner(int(checkpoint["channels"]))
    model.load_state_dict(checkpoint["state_dict"])
    model.eval()
    mask = training.cosine_mask(model_samples, repair_samples, fade_samples)

    sample_rate, wave = training.read_pcm16(str(input_path))
    boundaries = boundary_positions(plan, sample_rate, wave.numel(), args.position)
    decisions = []
    for unit_index, center, alias in boundaries:
        peak, applied = refine_boundary(
            wave, center, sample_rate, model, model_rate, model_samples, mask, args.strength
        )
        decisions.append(
            {
                "unit_index": unit_index,
                "position_ms": center * 1000 / sample_rate,
                "alias": alias,
                "applied": applied,
                "peak_residual": peak,
            }
        )
    training.write_pcm16(output_path, sample_rate, wave)
    report_path = Path(args.report) if args.report else output_path.with_suffix(".json")
    report = {
        "version": 1,
        "input": str(input_path),
        "output": str(output_path),
        "model": str(Path(args.model).resolve()),
        "strength": args.strength,
        "position": args.position,
        "boundaries": len(decisions),
        "applied": sum(item["applied"] for item in decisions),
        "decisions": decisions,
    }
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    if input_path.parent == output_path.parent:
        write_html(output_path.parent / "index.html", input_path, output_path)
    print(json.dumps({"boundaries": len(decisions), "applied": report["applied"]}, ensure_ascii=False))
    print(f"wrote {output_path}")


if __name__ == "__main__":
    main()
