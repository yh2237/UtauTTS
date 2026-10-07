// 抑揚モデルのランタイム輪郭（平滑化・クリップ込み）を、コーパスの自然時間軸で生成する。
// 統合韻律モデルの蒸留教師用。リポジトリ直下で実行する。
// 使い方: go run ./cmd/tools/prosody-teacher --dataset out/mfa-align-20261002/all-mfa.jsonl --model models/frame-intonation-tcn-v10.json --out out/speech-timing-target/ja-v10-teacher.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
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
	modelPath := flag.String("model", "models/frame-intonation-tcn-v10.json", "prosody model JSON")
	outPath := flag.String("out", "out/speech-timing-target/ja-v10-teacher.jsonl", "teacher output JSONL")
	flag.Parse()

	model, err := prosody.LoadModel(*modelPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load model:", err)
		os.Exit(1)
	}
	file, err := os.Open(*dataset)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	output, err := os.Create(*outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer output.Close()
	writer := bufio.NewWriter(output)
	defer writer.Flush()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 16<<20)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		morae := make([]frontend.Mora, len(r.Tokens))
		frames := make([]prosody.FeatureFrame, len(r.Tokens))
		timings := make([]prosody.MoraTiming, len(r.Tokens))
		durationMS := 0.0
		for i, t := range r.Tokens {
			morae[i] = frontend.Mora{Language: "ja", Text: t.Mora, Vowel: t.Vowel, Pause: t.Pause}
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
			fmt.Fprintln(os.Stderr, "no contour:", r.ID)
			os.Exit(1)
		}
		encoded, err := json.Marshal(teacherLine{ID: r.ID, FrameMS: contour.FrameMS, Cents: contour.Cents})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		writer.Write(encoded)
		writer.WriteByte('\n')
		count++
		if count%200 == 0 {
			fmt.Printf("teacher %d\n", count)
		}
	}
	fmt.Printf("teacher done: %d\n", count)
}
