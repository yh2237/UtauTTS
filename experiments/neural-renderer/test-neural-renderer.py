#!/usr/bin/env python3
"""ニューラル制御型renderer実験基盤の回帰test。"""

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path

import torch

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import (  # noqa: E402
    build_candidate_pool,
    build_diphone_pool,
    build_ngram_pool,
    estimate_f0_median,
    fold_f0,
    parse_hts_label,
    split_for,
    td_psola,
)


def load_baseline_module():
    path = Path(__file__).with_name("render-neural-renderer-baseline.py")
    spec = importlib.util.spec_from_file_location("utautts_neural_renderer_baseline_test", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


baseline = load_baseline_module()


class NeuralRendererDataTest(unittest.TestCase):
    def test_f0_estimator_finds_sine(self) -> None:
        sample_rate = 24_000
        time = torch.arange(sample_rate // 4) / sample_rate
        wave = 0.2 * torch.sin(2 * torch.pi * 220 * time)
        self.assertAlmostEqual(estimate_f0_median(wave, sample_rate), 220, delta=3)

    def test_f0_octave_folding(self) -> None:
        self.assertAlmostEqual(fold_f0(77, 310), 308)
        self.assertAlmostEqual(fold_f0(506, 240), 253)

    def test_td_psola_changes_pitch_without_changing_length(self) -> None:
        sample_rate = 24_000
        time = torch.arange(sample_rate // 4) / sample_rate
        source = 0.2 * torch.sin(2 * torch.pi * 220 * time)
        rendered = td_psola(source, sample_rate // 3, sample_rate, 220, 260)
        self.assertEqual(rendered.numel(), sample_rate // 3)
        self.assertAlmostEqual(estimate_f0_median(rendered, sample_rate), 260, delta=8)

    def test_split_is_stable(self) -> None:
        self.assertEqual(split_for("BASIC5000_0001"), split_for("BASIC5000_0001"))
        self.assertIn(split_for("BASIC5000_0002"), {"train", "validation", "test"})

    def test_hts_phone_and_context(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "sample.lab"
            path.write_text(
                "0 1000000 xx^xx-sil+k=o/A:x\n"
                "1000000 2000000 xx^sil-k+o=N/A:x\n"
                "2000000 3000000 sil^k-o+N=xx/A:x\n",
                encoding="utf-8",
            )
            segments = parse_hts_label(path, 48_000)
        self.assertEqual([item["phone"] for item in segments], ["sil", "k", "o"])
        self.assertEqual(segments[1]["left_phone"], "sil")
        self.assertEqual(segments[1]["right_phone"], "o")
        self.assertEqual(segments[1]["start_sample"], 4_800)

    def test_candidate_pool_uses_train_only(self) -> None:
        index = {
            "utterances": [
                {"id": "train", "split": "train", "segments": [{"index": 0, "phone": "a"}]},
                {"id": "test", "split": "test", "segments": [{"index": 0, "phone": "i"}]},
            ]
        }
        pool = build_candidate_pool(index)
        self.assertEqual(set(pool), {"a"})
        self.assertEqual(pool["a"][0][0]["id"], "train")

    def test_diphone_pool_preserves_natural_transition(self) -> None:
        segments = [
            {"index": 0, "phone": "k"},
            {"index": 1, "phone": "o"},
            {"index": 2, "phone": "N"},
        ]
        index = {"utterances": [{"id": "train", "split": "train", "segments": segments}]}
        pool = build_diphone_pool(index)
        self.assertEqual(set(pool), {"k->o", "o->N"})
        self.assertIs(pool["k->o"][0][1], segments[0])
        self.assertIs(pool["k->o"][0][2], segments[1])

    def test_ngram_pool_preserves_long_transition(self) -> None:
        segments = [
            {"index": 0, "phone": "k"},
            {"index": 1, "phone": "o"},
            {"index": 2, "phone": "N"},
        ]
        index = {"utterances": [{"id": "train", "split": "train", "segments": segments}]}
        pool = build_ngram_pool(index, 3)
        self.assertEqual(set(pool), {"k->o->N"})
        self.assertEqual(pool["k->o->N"][0][1], segments)

    def test_selection_rejects_target_utterance(self) -> None:
        target_utterance = {"id": "target"}
        target = {
            "index": 0,
            "phone": "a",
            "left_phone": "k",
            "right_phone": "s",
            "start_sample": 0,
            "end_sample": 100,
        }
        same = ({"id": "target"}, {**target, "index": 1})
        other = ({"id": "other"}, {**target, "index": 2})
        selected = baseline.choose_candidate(target_utterance, target, [same, other])
        self.assertEqual(selected[0]["id"], "other")

    def test_duration_first_selection_avoids_large_pitch_shift(self) -> None:
        target_utterance = {"id": "target"}
        target_left = {
            "index": 1, "phone": "a", "left_phone": "k", "right_phone": "i",
            "start_sample": 100, "end_sample": 200,
        }
        target_right = {
            "index": 2, "phone": "i", "left_phone": "a", "right_phone": "s",
            "start_sample": 200, "end_sample": 300,
        }
        context_match = (
            {"id": "context"},
            {**target_left, "start_sample": 0, "end_sample": 50},
            {**target_right, "start_sample": 50, "end_sample": 100},
        )
        duration_match = (
            {"id": "duration"},
            {**target_left, "left_phone": "n", "start_sample": 0, "end_sample": 100},
            {**target_right, "right_phone": "t", "start_sample": 100, "end_sample": 200},
        )
        selected = baseline.choose_diphone_candidate(
            target_utterance, target_left, target_right,
            [context_match, duration_match], "duration-first",
        )
        self.assertEqual(selected[0]["id"], "duration")

    def test_f0_oracle_rejects_target_utterance(self) -> None:
        target_wave = 0.2 * torch.sin(2 * torch.pi * 220 * torch.arange(4000) / 24_000)
        target_left = {
            "index": 0, "phone": "a", "left_phone": "sil", "right_phone": "i",
            "start_sample": 0, "end_sample": 2000,
        }
        target_right = {
            "index": 1, "phone": "i", "left_phone": "a", "right_phone": "sil",
            "start_sample": 2000, "end_sample": 4000,
        }
        with self.assertRaises(ValueError):
            baseline.choose_f0_oracle_candidate(
                {"id": "target"}, target_left, target_right,
                [({"id": "target", "sample_rate": 24_000}, target_left, target_right)],
                target_wave, 24_000,
            )

    def test_anchored_time_warp_maps_phone_boundary(self) -> None:
        source = torch.arange(64, dtype=torch.float32)
        positions = torch.tensor([0.0, 10.0, 30.0])
        rendered = baseline.anchored_source_values(
            source,
            positions,
            target_start=0,
            target_anchor=10,
            target_end=30,
            source_start=10,
            source_anchor=30,
            source_end=50,
        )
        self.assertTrue(torch.equal(rendered, torch.tensor([10.0, 30.0, 50.0])))

    def test_zero_controls_equal_linear_time_warp(self) -> None:
        source = torch.linspace(-1, 1, 100)
        positions = torch.linspace(8, 42, 71)
        linear = baseline.source_values(
            source, 24_000, {"start_sample": 20, "end_sample": 70},
            positions, 10, 40, 24_000,
        )
        controlled = baseline.controlled_source_values(
            source, positions, 10, 40, 20.0, 70.0, 0.5, 0.0,
        )
        self.assertTrue(torch.allclose(linear, controlled, atol=1e-6))

    def test_phase_alignment_finds_sine_offset(self) -> None:
        time = torch.arange(240, dtype=torch.float32)
        reference = torch.sin(2 * torch.pi * time / 60)
        candidate = baseline.shift_values(reference, 13)
        shift = baseline.best_phase_shift(reference, candidate, 20)
        aligned = baseline.shift_values(candidate, shift)
        correlation = torch.corrcoef(torch.stack((reference, aligned)))[0, 1]
        self.assertGreater(float(correlation), 0.98)

    def test_viterbi_pitch_smoothing_uses_local_median(self) -> None:
        path = Path(__file__).with_name("render-neural-renderer-viterbi.py")
        spec = importlib.util.spec_from_file_location("utautts_viterbi_smoothing_test", path)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        self.assertEqual(module.smooth_f0_targets([220, 440, 225], 3), [330.0, 225, 332.5])


if __name__ == "__main__":
    unittest.main()
