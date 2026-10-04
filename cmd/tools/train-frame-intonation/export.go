package main

import (
	"fmt"
	"math"
	"path/filepath"

	"sort"
	"strings"
)

func matrix(x []float32, rows, cols int) [][]float64 {
	out := make([][]float64, rows)
	for i := range out {
		out[i] = make([]float64, cols)
		for j := range out[i] {
			out[i][j] = float64(x[i*cols+j])
		}
	}
	return out
}
func vector(x []float32) []float64 {
	out := make([]float64, len(x))
	for i, v := range x {
		out[i] = float64(v)
	}
	return out
}
func exportedFrame(m *tcn, names []string, c config) map[string]any {
	state := m.Module.StateDict()
	layers := make([]map[string]any, len(m.Layers))
	for i, d := range c.Dilations {
		w := state[fmt.Sprintf("layers.%d.weight", i)]
		cube := make([][][]float64, c.Hidden)
		for o := range cube {
			cube[o] = matrix(w[o*c.Hidden*3:(o+1)*c.Hidden*3], c.Hidden, 3)
		}
		layers[i] = map[string]any{"dilation": d, "weights": cube, "bias": vector(state[fmt.Sprintf("layers.%d.bias", i)])}
	}
	scale := math.Max(1, math.Max(math.Abs(c.Low), math.Abs(c.High)))
	output := vector(state["output.weight"])
	for i := range output {
		output[i] *= scale
	}
	return map[string]any{"feature_names": names, "input_weights": matrix(state["input.weight"], c.Hidden, len(names)), "input_bias": vector(state["input.bias"]), "layers": layers, "output_weight": output, "output_bias": float64(state["output.bias"][0]) * scale, "frame_ms": c.Frame, "low_cents": c.Low, "high_cents": c.High, "centered": true, "target_scale_cents": scale, "render_strength": c.RenderStrength, "render_smoothing_ms": c.RenderSmoothing, "render_p99_cents": c.RenderP99, "render_max_cents": c.RenderMax}
}
func ids(rows []record) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}
func frames(xs []example) int {
	n := 0
	for _, x := range xs {
		n += x.Frames
	}
	return n
}
func voiced(xs []example) int {
	n := 0
	for _, x := range xs {
		for _, b := range x.Mask {
			if b {
				n++
			}
		}
	}
	return n
}
func export(m *tcn, names []string, c config, sha string, trainRows, validRows, testRows []record, train, valid []example, validation, best float64, bestEpoch int, testRaw, testRendered float64, history []map[string]any) map[string]any {
	modelID := c.ID
	if modelID == "" {
		base := filepath.Base(c.Output)
		modelID = strings.TrimSuffix(base, filepath.Ext(base))
	}
	display := c.Name
	if display == "" {
		display = modelID
	}
	countTokens := 0
	for _, r := range trainRows {
		countTokens += len(r.Tokens)
	}
	metrics := map[string]any{"records": len(valid), "frames": frames(valid), "voiced_frames": voiced(valid), "pitch_mae_cents": validation, "validation_rendered_mae_cents": best}
	if len(testRows) > 0 {
		metrics["test_mae_cents"] = testRaw
		metrics["test_rendered_mae_cents"] = testRendered
	}
	mode := "intonation_frame_tcn_accent_bounded"
	accent := "openjtalk"
	if c.Language == "en" {
		mode = "intonation_frame_tcn_english_bounded"
		accent = "aligned-arpabet-stress"
	}
	trainMeta := map[string]any{"records": len(train), "tokens": countTokens, "frames": frames(train), "epochs": c.Epochs, "learning_rate": c.LR, "hidden": c.Hidden, "batch_size": c.Batch, "seed": c.Seed, "device": c.Device, "openjtalk_accent": c.OpenJTalkAccent, "f0_source": map[string]string{"internal": "internal_autocorrelation", "world": "utautts_world_harvest"}[c.F0Source], "target_smooth_ms": c.Smooth, "delta_weight": c.Delta, "best_epoch": bestEpoch, "evaluation_is_in_sample": c.AllDataTraining, "accent_source": accent, "dataset_sha256": sha, "selection_metric": "validation_rendered_contour_mae", "history": history, "split_ids": map[string]any{"train": ids(trainRows), "validation": ids(validRows), "test": ids(testRows)}}
	trainMeta["alignment"] = alignmentMetadata(trainRows, validRows, c.Language)
	if len(testRows) > 0 {
		trainMeta["test_alignment"] = alignmentStats(testRows, c.Language)
	}
	if c.Language == "en" {
		trainMeta["split_unit"] = "speaker"
		splits := map[string][]string{}
		for name, rows := range map[string][]record{"train": trainRows, "validation": validRows, "test": testRows} {
			set := map[string]bool{}
			for _, r := range rows {
				set[r.Speaker] = true
			}
			for speaker := range set {
				splits[name] = append(splits[name], speaker)
			}
			sort.Strings(splits[name])
		}
		trainMeta["speaker_splits"] = splits
	}
	description := c.Description
	if description == "" {
		description = "Frame-level learned intonation model"
	}
	renderers := c.RecommendedRenderers
	if len(renderers) == 0 {
		renderers = []string{"utautts-world-phrase"}
	}
	payload := map[string]any{"id": modelID, "display_name": display, "description": description, "license": c.License, "license_notices": c.Notices, "provenance": map[string]any{"training_corpus": c.Corpus}, "recommended_renderers": renderers, "version": 8, "feature_version": 1, "mode": mode, "language": c.Language, "duration_weights": map[string]float64{}, "frame_pitch": exportedFrame(m, names, c), "metrics": metrics, "training": trainMeta}
	if c.Language == "en" {
		payload["status"] = "experimental-requires-listening"
	}
	return payload
}
func alignmentStats(rows []record, language string) map[string]any {
	if language == "en" {
		return map[string]any{"alignment_records": 0, "alignment_moras": 0, "skipped_records": 0, "alignment_rate": 0.0, "fallback_records": len(rows)}
	}
	moras := 0
	types := map[string]int{}
	pos := map[string]int{}
	groups := map[string]int{}
	for _, r := range rows {
		for _, t := range r.Tokens {
			if t.Pause {
				continue
			}
			moras++
			kind := "heiban"
			if t.AccentNucleus > 0 {
				if t.AccentPosition < t.AccentNucleus {
					kind = "before"
				} else if t.AccentPosition == t.AccentNucleus {
					kind = "nucleus"
				} else {
					kind = "after"
				}
			}
			types[kind]++
			pos[t.POS]++
			groups[t.POSGroup]++
		}
	}
	return map[string]any{"alignment_records": len(rows), "alignment_moras": moras, "skipped_records": 0, "alignment_rate": 1.0, "fallback_records": 0, "accent_type_counts": types, "pos_counts": pos, "pos_group1_counts": groups}
}
func alignmentMetadata(train, valid []record, language string) map[string]any {
	sources := map[string]bool{}
	questions, questionFrames := 0, 0
	for _, r := range append(append([]record(nil), train...), valid...) {
		source := r.AlignmentSource
		if source == "" {
			source = "unspecified"
		}
		sources[source] = true
		if strings.ContainsAny(r.Text, "?？") {
			questions++
			questionFrames += len(timeGrid(r, 10))
		}
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	return map[string]any{"train": alignmentStats(train, language), "validation": alignmentStats(valid, language), "records": len(train) + len(valid), "aligned_records": len(train) + len(valid), "skipped_records": 0, "alignment_rate": 1.0, "question_records": questions, "final_distance_records": len(train) + len(valid), "question_distance_feature_count": questionFrames, "timing_sources": names}
}
