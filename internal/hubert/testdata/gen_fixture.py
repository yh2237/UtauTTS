import argparse
from pathlib import Path

import torch
from safetensors.torch import load_file
from transformers import HubertConfig, HubertModel


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("weights")
    parser.add_argument("config")
    args = parser.parse_args()
    torch.set_num_threads(8)
    config = HubertConfig.from_json_file(args.config)
    model = HubertModel(config)
    missing, unexpected = model.load_state_dict(load_file(args.weights), strict=False)
    if missing or unexpected:
        raise RuntimeError(f"state_dict mismatch: {missing}, {unexpected}")
    model.eval()
    t = torch.arange(24000, dtype=torch.float32)
    wave = 0.13 * torch.sin(t * 0.017) + 0.07 * torch.sin(t * 0.041) + 0.01 * torch.cos(t * 0.003)
    root = Path(__file__).resolve().parent
    root.mkdir(parents=True, exist_ok=True)
    wave.numpy().astype("<f4").tofile(root / "wave.f32")
    with torch.no_grad():
        output = model(wave[None], output_hidden_states=True)
    for layer in [0, 6]:
        output.hidden_states[layer][0].numpy().astype("<f4").tofile(root / f"layer{layer}.f32")
    print("frames", output.hidden_states[6].shape[1])


if __name__ == "__main__":
    main()
