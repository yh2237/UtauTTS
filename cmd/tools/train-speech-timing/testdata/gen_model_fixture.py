"""Generate a small full-width speech-timing PyTorch parity fixture in a temp directory."""
import argparse
import importlib.util
import json
import struct
from pathlib import Path

import numpy as np
import torch
import torch.nn.functional as F


class Target(torch.nn.Module):
    def __init__(self):
        super().__init__()
        self.phone = torch.nn.Embedding(40, 32)
        self.inp = torch.nn.Conv1d(100, 128, 1)
        self.blocks = torch.nn.ModuleList(torch.nn.Conv1d(128, 128, 5, dilation=d, padding=2*d)
                                          for d in (1, 2, 4, 8, 1, 2, 4, 8))
        self.norms = torch.nn.ModuleList(torch.nn.LayerNorm(128) for _ in range(8))
        self.out = torch.nn.Conv1d(128, 80, 1)


def write_safetensors(path, tensors):
    header = {}
    blobs = []
    offset = 0
    for name in sorted(tensors):
        values = np.ascontiguousarray(tensors[name], dtype="<f4")
        blob = values.tobytes()
        header[name] = {"dtype": "F32", "shape": list(values.shape),
                        "data_offsets": [offset, offset + len(blob)]}
        offset += len(blob)
        blobs.append(blob)
    encoded = json.dumps(header, separators=(",", ":")).encode()
    path.write_bytes(struct.pack("<Q", len(encoded)) + encoded + b"".join(blobs))


def keep_mask(count, seed, p):
    x = np.arange(count, dtype=np.uint32) ^ np.uint32(seed)
    x ^= x >> np.uint32(16)
    x *= np.uint32(0x7FEB352D)
    x ^= x >> np.uint32(15)
    x *= np.uint32(0x846CA68B)
    x ^= x >> np.uint32(16)
    return torch.from_numpy((x >= np.uint32(int(p * 4294967296))).astype(np.float32))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    parser.add_argument("--reference", help="optional read-only UtauTTS train script for architecture verification")
    args = parser.parse_args()
    if args.reference:
        spec = importlib.util.spec_from_file_location("speech_timing_reference", args.reference)
        reference = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(reference)
    torch.set_num_threads(4)
    torch.manual_seed(1234)
    rng = np.random.default_rng(713)
    model = reference.Target(4) if args.reference else Target()
    initial = {k: v.detach().cpu().clone().numpy() for k, v in model.state_dict().items()}
    ids = rng.integers(0, 40, size=(2, 7, 3), dtype=np.int64)
    cont = rng.normal(size=(2, 7, 4)).astype(np.float32)
    target = rng.normal(size=(2, 7, 80)).astype(np.float32)
    target[:, -1, :] = np.nan
    target[0, 3, :] = np.nan
    ids_t = torch.from_numpy(ids)
    cont_t = torch.from_numpy(cont)
    target_t = torch.from_numpy(target.copy())
    h = model.inp(torch.cat([model.phone(ids_t).flatten(2), cont_t], dim=2).transpose(1, 2))
    for i, (block, norm) in enumerate(zip(model.blocks, model.norms)):
        y = norm(block(h).transpose(1, 2)).transpose(1, 2)
        y = F.gelu(y)
        mask = keep_mask(y.numel(), 99 + i, .15).reshape(y.shape[0], y.shape[2], y.shape[1]).transpose(1, 2)
        h = h + y * mask / .85
    output = model.out(h).transpose(1, 2)
    mask = ~torch.isnan(target_t[..., 0])
    loss = (output - torch.nan_to_num(target_t)).abs()[mask].mean()
    loss.backward()
    grads = {k: p.grad.detach().flatten().tolist() for k, p in model.named_parameters()}
    norm = torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0).item()
    optimizer = torch.optim.AdamW(model.parameters(), lr=.002, weight_decay=.0001)
    optimizer.step()
    post = {k: p.detach().flatten().tolist() for k, p in model.named_parameters()}
    output_dir = Path(args.output)
    output_dir.mkdir(parents=True, exist_ok=True)
    write_safetensors(output_dir / "weights.safetensors", initial)
    with (output_dir / "fixture.json").open("w", encoding="utf-8") as stream:
        json.dump({"ids": ids.flatten().tolist(), "cont": cont.flatten().tolist(),
                   "target": np.nan_to_num(target).flatten().tolist(),
                   "nan_rows": [int(i) for i, bad in enumerate(np.isnan(target[..., 0]).flatten()) if bad],
                   "output": output.detach().flatten().tolist(), "loss": loss.item(),
                   "grads": grads, "clip_norm": norm, "post": post}, stream)
    print(output_dir / "fixture.json")


if __name__ == "__main__":
    main()
