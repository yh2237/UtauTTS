#!/usr/bin/env python3
"""人工的に壊した音素境界を局所的に復元する残差モデルを学習する。"""

from __future__ import annotations

import argparse
import functools
import json
import math
import random
import sys
import wave
from array import array
from pathlib import Path

import torch
from torch import nn
from torch.nn import functional as F

# 現在も使う学習処理と共有しているGPU補助だけtoolsから読み込む。
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "tools"))
from torch_device import device_description, move_batch, resolve_device  # noqa: E402


@functools.lru_cache(maxsize=32)
def read_pcm16(path: str) -> tuple[int, torch.Tensor]:
    with wave.open(path, "rb") as source:
        if source.getnchannels() != 1 or source.getsampwidth() != 2:
            raise ValueError(f"unsupported WAV format: {path}")
        sample_rate = source.getframerate()
        values = array("h")
        values.frombytes(source.readframes(source.getnframes()))
    if sys.byteorder != "little":
        values.byteswap()
    return sample_rate, torch.tensor(values, dtype=torch.float32).div_(32768)


def write_pcm16(path: Path, sample_rate: int, values: torch.Tensor) -> None:
    samples = array(
        "h",
        (
            max(-32768, min(32767, round(float(value) * 32767)))
            for value in values.detach().cpu().clamp(-1, 1)
        ),
    )
    if sys.byteorder != "little":
        samples.byteswap()
    path.parent.mkdir(parents=True, exist_ok=True)
    with wave.open(str(path), "wb") as destination:
        destination.setnchannels(1)
        destination.setsampwidth(2)
        destination.setframerate(sample_rate)
        destination.writeframes(samples.tobytes())


def load_manifest(path: Path) -> list[dict]:
    records = []
    with path.open(encoding="utf-8") as source:
        for number, line in enumerate(source, 1):
            try:
                record = json.loads(line)
            except json.JSONDecodeError as error:
                raise ValueError(f"{path}:{number}: {error}") from error
            if record.get("version") != 1:
                raise ValueError(f"{path}:{number}: unsupported version")
            records.append(record)
    return records


def cosine_mask(samples: int, repair_samples: int, fade_samples: int) -> torch.Tensor:
    mask = torch.zeros(samples)
    start = (samples - repair_samples) // 2
    end = start + repair_samples
    mask[start:end] = 1
    fade = min(fade_samples, repair_samples // 2)
    if fade > 0:
        phase = torch.linspace(0, math.pi / 2, fade)
        ramp = torch.sin(phase).square()
        mask[start : start + fade] = ramp
        mask[end - fade : end] = ramp.flip(0)
    return mask


def extract_window(record: dict, sample_rate: int, samples: int) -> torch.Tensor:
    source_rate, source = read_pcm16(record["audio_path"])
    center = int(record["boundary_sample"])
    source_samples = round(samples * source_rate / sample_rate)
    start = center - source_samples // 2
    end = start + source_samples
    if start < 0 or end > source.numel():
        raise ValueError(f"window outside source: {record['id']}")
    window = source[start:end]
    if source_rate != sample_rate or window.numel() != samples:
        window = F.interpolate(window.view(1, 1, -1), size=samples, mode="linear", align_corners=False).view(-1)
    peak = max(0.05, float(window.abs().max()))
    return window * (0.9 / peak)


def shifted(values: torch.Tensor, offset: int) -> torch.Tensor:
    positions = torch.arange(values.numel(), dtype=torch.float32) + offset
    positions.clamp_(0, values.numel() - 1)
    left = positions.floor().long()
    right = (left + 1).clamp_max(values.numel() - 1)
    fraction = positions - left
    return values[left] * (1 - fraction) + values[right] * fraction


def corrupt(clean: torch.Tensor, mask: torch.Tensor, generator: torch.Generator) -> torch.Tensor:
    if float(torch.rand((), generator=generator)) < 0.2:
        return clean.clone()
    samples = clean.numel()
    center = samples // 2
    max_shift = max(1, round(samples * 0.06))
    left_shift = int(torch.randint(-max_shift, max_shift + 1, (), generator=generator))
    right_shift = int(torch.randint(-max_shift, max_shift + 1, (), generator=generator))
    left = shifted(clean, left_shift)
    right = shifted(clean, right_shift)
    candidate = torch.where(torch.arange(samples) < center, left, right)
    gain = float(torch.empty(()).uniform_(0.72, 1.28, generator=generator))
    candidate[center:] *= gain
    attenuation = float(torch.empty(()).uniform_(0.35, 1.0, generator=generator))
    width = int(torch.randint(max(2, samples // 40), max(3, samples // 8), (), generator=generator))
    side = int(torch.randint(0, 2, (), generator=generator))
    if side == 0:
        candidate[max(0, center - width) : center] *= attenuation
    else:
        candidate[center : min(samples, center + width)] *= attenuation
    return clean * (1 - mask) + candidate * mask


class ResidualBlock(nn.Module):
    def __init__(self, channels: int, dilation: int):
        super().__init__()
        self.filter = nn.Conv1d(channels, channels * 2, 5, padding=dilation * 2, dilation=dilation)
        self.project = nn.Conv1d(channels, channels, 1)

    def forward(self, values: torch.Tensor) -> torch.Tensor:
        filtered, gate = self.filter(values).chunk(2, dim=1)
        return values + self.project(torch.tanh(filtered) * torch.sigmoid(gate))


class BoundaryRefiner(nn.Module):
    def __init__(self, channels: int):
        super().__init__()
        self.input = nn.Conv1d(2, channels, 7, padding=3)
        self.blocks = nn.ModuleList(ResidualBlock(channels, 2**index) for index in range(10))
        self.output = nn.Sequential(nn.SiLU(), nn.Conv1d(channels, 1, 1), nn.Tanh())

    def forward(self, values: torch.Tensor) -> torch.Tensor:
        wave = values[:, :1]
        mask = values[:, 1:2]
        state = self.input(values)
        for block in self.blocks:
            state = block(state)
        residual = self.output(state) * 0.5
        return (wave + residual * mask).clamp(-1, 1)


def batch_for(
    records: list[dict],
    indices: list[int],
    sample_rate: int,
    samples: int,
    mask: torch.Tensor,
    seed: int,
) -> tuple[torch.Tensor, torch.Tensor]:
    inputs, targets = [], []
    for index in indices:
        clean = extract_window(records[index], sample_rate, samples)
        generator = torch.Generator().manual_seed(seed * 1_000_003 + index)
        damaged = corrupt(clean, mask, generator)
        inputs.append(torch.stack((damaged, mask)))
        targets.append(clean.unsqueeze(0))
    return torch.stack(inputs), torch.stack(targets)


def loss_for(predicted: torch.Tensor, target: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    weight = 0.1 + mask.view(1, 1, -1) * 0.9
    waveform = ((predicted - target).abs() * weight).mean()
    derivative = ((torch.diff(predicted) - torch.diff(target)).abs() * weight[..., 1:]).mean()
    spectrum = predicted.new_zeros(())
    for size in (128, 256, 512):
        window = torch.hann_window(size, device=predicted.device)
        actual = torch.stft(predicted[:, 0], size, size // 4, window=window, return_complex=True)
        expected = torch.stft(target[:, 0], size, size // 4, window=window, return_complex=True)
        spectrum = spectrum + F.l1_loss(torch.log1p(actual.abs()), torch.log1p(expected.abs()))
    return waveform + 0.5 * derivative + 0.1 * spectrum / 3


@torch.no_grad()
def evaluate(
    model: nn.Module,
    records: list[dict],
    indices: list[int],
    args: argparse.Namespace,
    mask: torch.Tensor,
    device: torch.device,
) -> tuple[float, float]:
    model.eval()
    refined_losses = []
    damaged_losses = []
    for offset in range(0, len(indices), args.batch_size):
        inputs, targets = batch_for(records, indices[offset : offset + args.batch_size], args.sample_rate, args.samples, mask, 9173)
        inputs, targets = move_batch(device, inputs, targets)
        device_mask = mask.to(device)
        refined_losses.append(float(loss_for(model(inputs), targets, device_mask)))
        damaged_losses.append(float(loss_for(inputs[:, :1], targets, device_mask)))
    count = max(1, len(refined_losses))
    return sum(refined_losses) / count, sum(damaged_losses) / count


@torch.no_grad()
def export_examples(
    model: nn.Module,
    records: list[dict],
    indices: list[int],
    args: argparse.Namespace,
    mask: torch.Tensor,
    device: torch.device,
) -> None:
    model.eval()
    output = Path(args.out)
    example_dir = output.parent / "examples"
    rows = []
    for number, index in enumerate(indices[: args.examples], 1):
        inputs, targets = batch_for(records, [index], args.sample_rate, args.samples, mask, 9173)
        refined = model(inputs.to(device))[0, 0].cpu()
        prefix = example_dir / f"{number:02d}-{records[index]['left_phone']}-{records[index]['right_phone']}"
        write_pcm16(prefix.with_name(prefix.name + "-01-clean.wav"), args.sample_rate, targets[0, 0])
        write_pcm16(prefix.with_name(prefix.name + "-02-damaged.wav"), args.sample_rate, inputs[0, 0])
        write_pcm16(prefix.with_name(prefix.name + "-03-refined.wav"), args.sample_rate, refined)
        rows.append((number, records[index], prefix.name))
    parts = [
        "<!doctype html><meta charset='utf-8'><title>Boundary refiner examples</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>Boundary refiner examples</h1>",
        "<p>同じ境界について、自然音声、人工劣化、補修後の順に並べています。</p>",
    ]
    for number, record, name in rows:
        parts.append(f"<section><h2>{number:02d}: {record['left_phone']} → {record['right_phone']}</h2>")
        for suffix, label in (("01-clean", "自然音声"), ("02-damaged", "人工劣化"), ("03-refined", "補修後")):
            parts.append(f"<label>{label}</label><audio controls src='{name}-{suffix}.wav'></audio>")
        parts.append("</section>")
    (example_dir / "index.html").write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/boundary-refiner/jsut-boundaries.jsonl")
    parser.add_argument("--out", default="out/boundary-refiner/boundary-refiner-v1.pt")
    parser.add_argument("--device", default="auto")
    parser.add_argument("--sample-rate", type=int, default=24_000)
    parser.add_argument("--window-ms", type=float, default=200.0)
    parser.add_argument("--repair-ms", type=float, default=64.0)
    parser.add_argument("--fade-ms", type=float, default=8.0)
    parser.add_argument("--channels", type=int, default=32)
    parser.add_argument("--batch-size", type=int, default=16)
    parser.add_argument("--steps", type=int, default=2_000)
    parser.add_argument("--learning-rate", type=float, default=2e-4)
    parser.add_argument("--seed", type=int, default=2237)
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--examples", type=int, default=12)
    args = parser.parse_args()
    if args.sample_rate <= 0 or args.window_ms <= 0 or args.repair_ms <= 0 or args.steps <= 0:
        raise SystemExit("invalid training settings")
    args.samples = round(args.window_ms * args.sample_rate / 1000)
    repair_samples = round(args.repair_ms * args.sample_rate / 1000)
    fade_samples = round(args.fade_ms * args.sample_rate / 1000)
    mask = cosine_mask(args.samples, repair_samples, fade_samples)

    records = load_manifest(Path(args.dataset))
    if args.limit > 0:
        records = records[: args.limit]
    train = [index for index, record in enumerate(records) if record["split"] == "train"]
    validation = [index for index, record in enumerate(records) if record["split"] == "validation"]
    if not train or not validation:
        raise SystemExit("dataset requires train and validation records")

    random.seed(args.seed)
    torch.manual_seed(args.seed)
    device = resolve_device(args.device)
    model = BoundaryRefiner(args.channels).to(device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.learning_rate)
    print(f"device={device_description(device)} train={len(train)} validation={len(validation)}", flush=True)
    best = math.inf
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    for step in range(1, args.steps + 1):
        model.train()
        selected = random.choices(train, k=args.batch_size)
        inputs, targets = batch_for(records, selected, args.sample_rate, args.samples, mask, args.seed + step)
        inputs, targets = move_batch(device, inputs, targets)
        optimizer.zero_grad(set_to_none=True)
        loss = loss_for(model(inputs), targets, mask.to(device))
        loss.backward()
        nn.utils.clip_grad_norm_(model.parameters(), 1.0)
        optimizer.step()
        if step == 1 or step % 100 == 0 or step == args.steps:
            subset = validation[: min(len(validation), 128)]
            validation_loss, damaged_loss = evaluate(model, records, subset, args, mask, device)
            improvement = 100 * (damaged_loss - validation_loss) / max(1e-9, damaged_loss)
            print(
                f"step={step} train={float(loss.detach()):.6f} validation={validation_loss:.6f} "
                f"damaged={damaged_loss:.6f} improvement={improvement:.1f}%",
                flush=True,
            )
            if validation_loss < best:
                best = validation_loss
                torch.save(
                    {
                        "version": 1,
                        "sample_rate": args.sample_rate,
                        "window_samples": args.samples,
                        "repair_samples": repair_samples,
                        "fade_samples": fade_samples,
                        "channels": args.channels,
                        "validation_loss": best,
                        "damaged_loss": damaged_loss,
                        "state_dict": model.state_dict(),
                    },
                    output,
                )
    checkpoint = torch.load(output, map_location=device, weights_only=True)
    model.load_state_dict(checkpoint["state_dict"])
    traced = torch.jit.trace(model, torch.zeros(1, 2, args.samples, device=device))
    traced.save(str(output.with_suffix(".torchscript.pt")))
    export_examples(model, records, validation, args, mask, device)
    print(f"wrote {output} validation={best:.6f}")


if __name__ == "__main__":
    main()
