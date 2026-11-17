package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
)

const teacherRecord = `{"id":"sample","text":"あ","tokens":[{"mora":"あ","vowel":"a","start_ms":0,"end_ms":100}]}`

func teacherFixture(t *testing.T) (dataset, modelPath, output string, model *prosody.Model) {
	t.Helper()
	root := t.TempDir()
	dataset = filepath.Join(root, "dataset.jsonl")
	modelPath = filepath.Join(root, "model.json")
	output = filepath.Join(root, "teacher.jsonl")
	model = &prosody.Model{
		Version: prosody.FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
		FramePitch: &prosody.FramePitchModel{
			FeatureNames: []string{"mora_progress"}, InputWeights: [][]float64{{2}}, InputBias: []float64{0},
			OutputWeight: []float64{100}, FrameMS: 10, LowCents: -60, HighCents: 60,
		},
	}
	if err := model.Save(modelPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dataset, []byte(teacherRecord+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return
}

func TestRunPreservesExistingFiles(t *testing.T) {
	for _, target := range []string{"output", "dataset", "model"} {
		t.Run(target, func(t *testing.T) {
			dataset, modelPath, output, _ := teacherFixture(t)
			switch target {
			case "dataset":
				output = dataset
			case "model":
				output = modelPath
			default:
				if err := os.WriteFile(output, []byte("existing teacher"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := run(dataset, modelPath, output); !errors.Is(err, os.ErrExist) {
				t.Fatalf("run error = %v, want existing file error", err)
			}
			after, err := os.ReadFile(output)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("existing %s changed: %v", target, err)
			}
		})
	}
}

func TestRunWritesRuntimeContour(t *testing.T) {
	dataset, modelPath, output, model := teacherFixture(t)
	if err := os.WriteFile(dataset, []byte("\ufeff\n  \n"+teacherRecord+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	count, err := run(dataset, modelPath, output)
	if err != nil || count != 1 {
		t.Fatalf("count=%d error=%v", count, err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var got teacherLine
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	contour := model.PredictFrameContour([]frontend.Mora{{Language: "ja", Text: "あ", Vowel: "a"}},
		[]prosody.FeatureFrame{accentFrame(token{})}, []prosody.MoraTiming{{DurationMS: 100}}, 100, false)
	want, err := json.Marshal(teacherLine{ID: "sample", FrameMS: contour.FrameMS, Cents: contour.Cents})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(data), want) {
		t.Fatalf("teacher = %s, want %s", data, want)
	}
}

func TestRunRemovesIncompleteOutput(t *testing.T) {
	for _, input := range []string{
		teacherRecord + "\n{invalid}",
		`{"id":"empty","tokens":[]}`,
		strings.Repeat("x", (16<<20)+1),
	} {
		dataset, modelPath, output, _ := teacherFixture(t)
		if err := os.WriteFile(dataset, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := run(dataset, modelPath, output); err == nil {
			t.Fatal("invalid input was accepted")
		}
		if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("incomplete output remains: %v", err)
		}
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteTeacherReturnsWriteAndFlushErrors(t *testing.T) {
	for _, records := range []int{1, 100} {
		dataset, _, _, model := teacherFixture(t)
		if err := os.WriteFile(dataset, []byte(strings.Repeat(teacherRecord+"\n", records)), 0600); err != nil {
			t.Fatal(err)
		}
		failure := errors.New("write failed")
		if _, err := writeTeacher(model, dataset, failingWriter{failure}); !errors.Is(err, failure) {
			t.Fatalf("records=%d error=%v, want %v", records, err, failure)
		}
	}
}
