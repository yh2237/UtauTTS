"""Train the v9 candidate using the frame TCN, without installing it."""
import importlib.util
import sys
from pathlib import Path

spec = importlib.util.spec_from_file_location("frame_trainer", Path(__file__).with_name("train-frame-intonation-tcn.py"))
trainer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(trainer)

if __name__ == "__main__":
    raise SystemExit(trainer.main([
        "--model-id", "frame-intonation-v9",
        "--display-name", "Frame intonation TCN v9 candidate",
        "--out", "out/prosody/frame-intonation-v9-candidate.json",
        "--hidden", "32", "--epochs", "24", "--holdout-test",
        *sys.argv[1:],
    ]))
