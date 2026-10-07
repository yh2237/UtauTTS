package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"utautts/cmd/tools/internal/sourcephone"
	"utautts/internal/audio"
)

func TestPlotDetailAndOutputRefusal(t *testing.T) {
	os.MkdirAll("out", 0755)
	dir, err := os.MkdirTemp("out", "source-plot-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	wav := filepath.Join(dir, "source.wav")
	if err := audio.WriteWav(wav, &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 4800)}); err != nil {
		t.Fatal(err)
	}
	digest, _, err := sourcephone.ClipIdentity(wav)
	if err != nil {
		t.Fatal(err)
	}
	frames := []any{}
	for i := 0; i < 30; i++ {
		frames = append(frames, sourcephone.Object{"start_ms": float64(i * 10), "end_ms": float64((i + 1) * 10), "rms_dbfs": float64(-70), "periodicity": float64(0), "zero_crossing_rate": float64(0)})
	}
	report := sourcephone.Object{"units": []any{sourcephone.Object{"unit_index": float64(1), "alias": "a", "source_clip": "source.wav", "analysis": sourcephone.Object{"duration_ms": float64(300), "source_sha256": digest, "low_energy_threshold_dbfs": float64(-60), "frames": frames, "landmarks": []any{}, "regions": []any{}}}}}
	reportPath := filepath.Join(dir, "report.json")
	if err := sourcephone.Write(reportPath, report); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		detail        bool
		width, height int
	}{{"detail.png", true, 2400, 525}, {"overview.png", false, 1800, 450}} {
		out := filepath.Join(dir, tc.name)
		args := []string{"--report", reportPath, "--out", out, "--alias", "a"}
		if tc.detail {
			args = append(args, "--detail")
		}
		if err := run(args); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(file)
		file.Close()
		if err != nil || config.Width != tc.width || config.Height != tc.height {
			t.Fatalf("%v %v", config, err)
		}
		if err := run(args); err == nil {
			t.Fatal("overwrote plot")
		}
	}
}
