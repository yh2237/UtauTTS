// 抑揚モデルのランタイム輪郭（平滑化・クリップ込み）を、コーパスの自然時間軸で生成する。
// 統合韻律モデルの蒸留教師用。リポジトリ直下で実行する。
// 使い方: go run ./cmd/tools/prosody-teacher --dataset out/mfa-align-20261002/all-mfa.jsonl --model models/frame-intonation-tcn-v12.json --out out/speech-timing-target/ja-teacher.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yh2237/UtauTTS/cmd/tools/internal/toolutil"
	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/prosody"
)

type token struct {
	Mora                 string  `json:"mora"`
	Vowel                string  `json:"vowel"`
	Pause                bool    `json:"pause"`
	StartMS              float64 `json:"start_ms"`
	EndMS                float64 `json:"end_ms"`
	AccentPhrasePosition int     `json:"accent_phrase_position"`
	AccentPhraseLength   int     `json:"accent_phrase_length"`
	AccentNucleus        int     `json:"accent_nucleus"`
	AccentHigh           bool    `json:"accent_high"`
	AccentPhraseStart    bool    `json:"accent_phrase_start"`
	AccentPhraseEnd      bool    `json:"accent_phrase_end"`
	WordStart            bool    `json:"word_start"`
	WordEnd              bool    `json:"word_end"`
	Pos                  string  `json:"pos"`
	PosGroup1            string  `json:"pos_group1"`
}

type record struct {
	ID     string  `json:"id"`
	Text   string  `json:"text"`
	Tokens []token `json:"tokens"`
}

func boolFeature(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

// accentFrameはinternal/openjtalkのsparseFeaturesと同じキーを組む。
func accentFrame(t token) prosody.FeatureFrame {
	if t.Pause {
		return prosody.FeatureFrame{}
	}
	length := t.AccentPhraseLength
	if length < 1 {
		length = 1
	}
	pos := t.Pos
	if pos == "" {
		pos = "*"
	}
	group := t.PosGroup1
	if group == "" {
		group = "*"
	}
	frame := prosody.FeatureFrame{
		"accent_position":         float64(t.AccentPhrasePosition) / float64(length),
		"accent_from_end":         float64(length-t.AccentPhrasePosition) / float64(length),
		"accent_nucleus_position": float64(t.AccentNucleus) / float64(length),
		"accent_high":             boolFeature(t.AccentHigh),
		"accent_phrase_start":     boolFeature(t.AccentPhraseStart),
		"accent_phrase_end":       boolFeature(t.AccentPhraseEnd),
		"word_start":              boolFeature(t.WordStart),
		"word_end":                boolFeature(t.WordEnd),
		"pos=" + pos:              1,
		"pos_group1=" + group:     1,
	}
	switch {
	case t.AccentNucleus == 0:
		frame["accent_type=heiban"] = 1
	case t.AccentPhrasePosition < t.AccentNucleus:
		frame["accent_type=before"] = 1
	case t.AccentPhrasePosition == t.AccentNucleus:
		frame["accent_type=nucleus"] = 1
	default:
		frame["accent_type=after"] = 1
	}
	return frame
}

type teacherLine struct {
	ID      string    `json:"id"`
	FrameMS float64   `json:"frame_ms"`
	Cents   []float64 `json:"cents"`
}

func main() {
	dataset := flag.String("dataset", "out/mfa-align-20261002/all-mfa.jsonl", "corpus JSONL")
	modelPath := flag.String("model", "models/frame-intonation-tcn-v12.json", "prosody model JSON")
	outPath := flag.String("out", "out/speech-timing-target/ja-v10-teacher.jsonl", "teacher output JSONL")
	flag.Parse()

	count, err := run(*dataset, *modelPath, *outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("teacher done: %d\n", count)
}

func run(dataset, modelPath, outPath string) (count int, err error) {
	model, err := prosody.LoadModel(modelPath)
	if err != nil {
		return 0, fmt.Errorf("load model: %w", err)
	}
	output, err := toolutil.CreateExclusive(outPath)
	if err != nil {
		return 0, err
	}
	defer func() {
		err = errors.Join(err, output.Close())
		if err != nil {
			err = errors.Join(err, os.Remove(output.Name()))
		}
	}()
	return writeTeacher(model, dataset, output)
}

func writeTeacher(model *prosody.Model, dataset string, output io.Writer) (int, error) {
	writer := bufio.NewWriter(output)
	encoder := json.NewEncoder(writer)
	count := 0
	err := toolutil.ScanJSONL(dataset, func(line []byte) error {
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		morae := make([]linguistics.Mora, len(r.Tokens))
		frames := make([]prosody.FeatureFrame, len(r.Tokens))
		timings := make([]prosody.MoraTiming, len(r.Tokens))
		durationMS := 0.0
		for i, t := range r.Tokens {
			morae[i] = linguistics.Mora{Language: "ja", Text: t.Mora, Vowel: t.Vowel, Pause: t.Pause}
			frames[i] = accentFrame(t)
			timings[i] = prosody.MoraTiming{StartMS: t.StartMS, DurationMS: t.EndMS - t.StartMS}
			if t.EndMS > durationMS {
				durationMS = t.EndMS
			}
		}
		text := strings.TrimSpace(r.Text)
		question := strings.HasSuffix(text, "?") || strings.HasSuffix(text, "？")
		contour := model.PredictFrameContour(morae, frames, timings, durationMS, question)
		if contour == nil {
			return fmt.Errorf("no contour: %s", r.ID)
		}
		if err := encoder.Encode(teacherLine{ID: r.ID, FrameMS: contour.FrameMS, Cents: contour.Cents}); err != nil {
			return err
		}
		count++
		if count%200 == 0 {
			fmt.Printf("teacher %d\n", count)
		}
		return nil
	})
	if err != nil {
		return count, err
	}
	return count, writer.Flush()
}
