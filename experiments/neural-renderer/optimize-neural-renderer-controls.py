#!/usr/bin/env python3
"""単一phoneのcropとtime warpを正解音声へ直接最適化する。"""

from __future__ import annotations

import argparse
import importlib.util
import json
import math
import sys
from pathlib import Path

import torch
from torch.nn import functional as F

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import (  # noqa: E402
    SILENCE_PHONES,
    build_candidate_pool,
    load_index,
    read_pcm16,
    resample,
    write_pcm16,
)


def load_baseline_module():
    path = Path(__file__).with_name("render-neural-renderer-baseline.py")
    spec = importlib.util.spec_from_file_location("utautts_neural_renderer_baseline", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


baseline_module = load_baseline_module()


def sample_source(source: torch.Tensor, positions: torch.Tensor) -> torch.Tensor:
    positions = positions.clamp(0, source.numel() - 1)
    left = positions.floor().long()
    right = (left + 1).clamp_max(source.numel() - 1)
    fraction = positions - left
    return source[left] * (1 - fraction) + source[right] * fraction


def render_controlled(
    source: torch.Tensor,
    source_rate: int,
    segment: dict,
    target_samples: int,
    parameters: torch.Tensor,
    maximum_correction_ms: float,
    maximum_gain_db: float,
) -> tuple[torch.Tensor, dict[str, torch.Tensor]]:
    core_start = float(segment["start_sample"])
    core_end = float(segment["end_sample"])
    core_duration = max(1.0, core_end - core_start)
    correction_limit = min(
        maximum_correction_ms * source_rate / 1000,
        core_duration * 0.24,
    )
    start_correction = torch.tanh(parameters[0]) * correction_limit
    end_correction = torch.tanh(parameters[1]) * correction_limit
    start = parameters.new_tensor(core_start) + start_correction
    end = parameters.new_tensor(core_end) + end_correction
    midpoint_fraction = 0.5 + 0.22 * torch.tanh(parameters[2])
    midpoint = start + (end - start) * midpoint_fraction
    gain_db = maximum_gain_db * torch.tanh(parameters[3])
    gain = torch.pow(parameters.new_tensor(10.0), gain_db / 20)

    progress = torch.linspace(0, 1, target_samples, device=parameters.device)
    left_positions = start + progress * 2 * (midpoint - start)
    right_positions = midpoint + (progress - 0.5) * 2 * (end - midpoint)
    positions = torch.where(progress <= 0.5, left_positions, right_positions)
    rendered = sample_source(source, positions) * gain
    controls = {
        "start_correction_ms": start_correction * 1000 / source_rate,
        "end_correction_ms": end_correction * 1000 / source_rate,
        "midpoint_fraction": midpoint_fraction,
        "gain_db": gain_db,
    }
    return rendered.clamp(-1, 1), controls


def reconstruction_loss(rendered: torch.Tensor, target: torch.Tensor) -> torch.Tensor:
    loss = rendered.new_zeros(())
    count = 0
    for size in (64, 128, 256, 512):
        if target.numel() <= size // 2:
            continue
        window = torch.hann_window(size, device=rendered.device)
        actual = torch.stft(rendered, size, size // 4, window=window, return_complex=True)
        expected = torch.stft(target, size, size // 4, window=window, return_complex=True)
        loss = loss + F.l1_loss(torch.log1p(actual.abs()), torch.log1p(expected.abs()))
        count += 1
    spectral = loss / max(1, count)
    envelope_window = max(2, min(target.numel(), 120))
    actual_envelope = F.avg_pool1d(rendered.square().view(1, 1, -1), envelope_window, stride=max(1, envelope_window // 2))
    target_envelope = F.avg_pool1d(target.square().view(1, 1, -1), envelope_window, stride=max(1, envelope_window // 2))
    envelope = F.l1_loss(torch.sqrt(actual_envelope + 1e-6), torch.sqrt(target_envelope + 1e-6))
    return spectral + 0.5 * envelope


def target_phone(utterance: dict, segment: dict, output_rate: int) -> torch.Tensor:
    sample_rate, wave = read_pcm16(utterance["audio_path"])
    values = wave[segment["start_sample"] : segment["end_sample"]]
    samples = max(2, round(values.numel() * output_rate / sample_rate))
    return resample(values, samples)


def contextualized(
    natural: torch.Tensor,
    start: int,
    end: int,
    replacement: torch.Tensor,
    sample_rate: int,
    context_ms: float = 80,
    fade_ms: float = 4,
) -> torch.Tensor:
    replacement = resample(replacement, max(2, end - start))
    candidate = natural.clone()
    fade = min(max(1, round(fade_ms * sample_rate / 1000)), replacement.numel() // 2)
    weight = torch.ones(replacement.numel())
    if fade > 0:
        phase = torch.linspace(0, math.pi / 2, fade)
        ramp = torch.sin(phase).square()
        weight[:fade] = ramp
        weight[-fade:] = ramp.flip(0)
    candidate[start:end] = natural[start:end] * (1 - weight) + replacement * weight
    context = round(context_ms * sample_rate / 1000)
    return candidate[max(0, start - context) : min(candidate.numel(), end + context)]


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>Oracle renderer controls</title>",
        "<style>body{font-family:sans-serif;max-width:900px;margin:30px auto}section{margin:26px 0}audio{display:block;width:100%;margin:4px 0 10px}code{white-space:pre-wrap}</style>",
        "<h1>Phase 1 oracle controls</h1>",
        "<p>正解を見てcropとtime warpを直接最適化した上限比較です。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{row['phone']} / {row['source_id']}</h2>")
        for key, label in (("target", "自然音声"), ("baseline", "固定制御"), ("oracle", "oracle制御")):
            parts.append(f"<label>{label}</label><audio controls src='{row[key]}'></audio>")
        parts.append(f"<code>{json.dumps(row['controls'], ensure_ascii=False)}</code></section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--split", choices=("validation", "test"), default="validation")
    parser.add_argument("--utterance", default="")
    parser.add_argument("--out", default="out/neural-renderer/oracle-controls")
    parser.add_argument("--limit", type=int, default=16)
    parser.add_argument("--sample-rate", type=int, default=24_000)
    parser.add_argument("--steps", type=int, default=160)
    parser.add_argument("--learning-rate", type=float, default=0.04)
    parser.add_argument("--max-correction-ms", type=float, default=30.0)
    parser.add_argument("--max-gain-db", type=float, default=2.0)
    args = parser.parse_args()

    index = load_index(args.dataset)
    pool = build_candidate_pool(index)
    targets = [item for item in index["utterances"] if item["split"] == args.split]
    if args.utterance:
        targets = [item for item in targets if item["id"] == args.utterance]
    if not targets:
        raise SystemExit("target utterance not found")
    utterance = targets[0]
    natural_rate, natural_wave = read_pcm16(utterance["audio_path"])
    natural_output = resample(natural_wave, round(natural_wave.numel() * args.sample_rate / natural_rate))
    output = Path(args.out)
    output.mkdir(parents=True, exist_ok=True)
    rows = []
    report_items = []
    for segment in utterance["segments"]:
        phone = segment["phone"]
        if phone in SILENCE_PHONES or phone not in pool:
            continue
        source_utterance, source_segment = baseline_module.choose_candidate(utterance, segment, pool[phone])
        source_rate, source_wave = read_pcm16(source_utterance["audio_path"])
        target = target_phone(utterance, segment, args.sample_rate)
        parameters = torch.zeros(4, requires_grad=True)
        optimizer = torch.optim.Adam([parameters], lr=args.learning_rate)
        with torch.no_grad():
            fixed, _ = render_controlled(
                source_wave, source_rate, source_segment, target.numel(), parameters,
                args.max_correction_ms, args.max_gain_db,
            )
            fixed_loss = float(reconstruction_loss(fixed, target))
        best_loss = fixed_loss
        best_wave = fixed.detach().clone()
        best_controls = None
        for _ in range(args.steps):
            optimizer.zero_grad(set_to_none=True)
            rendered, controls = render_controlled(
                source_wave, source_rate, source_segment, target.numel(), parameters,
                args.max_correction_ms, args.max_gain_db,
            )
            loss = reconstruction_loss(rendered, target)
            control_penalty = 0.002 * parameters.square().mean()
            (loss + control_penalty).backward()
            optimizer.step()
            if float(loss.detach()) < best_loss:
                best_loss = float(loss.detach())
                best_wave = rendered.detach().clone()
                best_controls = {key: float(value.detach()) for key, value in controls.items()}
        number = len(rows) + 1
        prefix = f"{number:02d}-{phone}"
        names = {
            "target": prefix + "-01-natural.wav",
            "baseline": prefix + "-02-fixed.wav",
            "oracle": prefix + "-03-oracle.wav",
        }
        target_start = round(segment["start_sample"] * args.sample_rate / natural_rate)
        target_end = round(segment["end_sample"] * args.sample_rate / natural_rate)
        write_pcm16(
            output / names["target"], args.sample_rate,
            natural_output[max(0, target_start - round(0.08 * args.sample_rate)) : min(natural_output.numel(), target_end + round(0.08 * args.sample_rate))],
        )
        write_pcm16(
            output / names["baseline"], args.sample_rate,
            contextualized(natural_output, target_start, target_end, fixed, args.sample_rate),
        )
        write_pcm16(
            output / names["oracle"], args.sample_rate,
            contextualized(natural_output, target_start, target_end, best_wave, args.sample_rate),
        )
        improvement = 100 * (fixed_loss - best_loss) / max(1e-9, fixed_loss)
        item = {
            "phone": phone,
            "source_id": source_utterance["id"],
            "fixed_loss": fixed_loss,
            "oracle_loss": best_loss,
            "improvement_percent": improvement,
            "controls": best_controls or {},
        }
        report_items.append(item)
        rows.append({**names, **item})
        if len(rows) >= args.limit:
            break
    write_html(output / "index.html", rows)
    mean_improvement = sum(item["improvement_percent"] for item in report_items) / max(1, len(report_items))
    report = {
        "version": 1,
        "target": utterance["id"],
        "phones": len(report_items),
        "mean_improvement_percent": mean_improvement,
        "items": report_items,
    }
    (output / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"target": utterance["id"], "phones": len(report_items), "mean_improvement_percent": mean_improvement}, ensure_ascii=False))
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
