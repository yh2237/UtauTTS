package tts

import (
	"fmt"
	"math"
	"strings"

	"utautts/internal/diffsinger"
	"utautts/internal/engine"
	"utautts/internal/frontend"
	"utautts/internal/openutau"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

func synthesizeDiffSinger(cfg Config) (*Result, error) {
	singer, err := diffsinger.Load(cfg.VoicebankPath)
	if err != nil {
		return nil, fmt.Errorf("load DiffSinger singer: %w", err)
	}
	language, phonemizer, reading, morae, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, fmt.Errorf("phonemize DiffSinger input: %w", err)
	}
	if language != frontend.LanguageJapanese {
		return nil, fmt.Errorf("DiffSinger MVP currently supports Japanese input only")
	}
	cfg, preview, err := prepareDiffSingerProsody(cfg, reading, morae, singer.FrameMS())
	if err != nil {
		return nil, err
	}
	durations := preview.MoraDurationsMS
	phones, phoneDurations, phoneCounts, err := diffsingerPhones(singer, morae, durations)
	if err != nil {
		return nil, err
	}
	frameMS := singer.FrameMS()
	phoneDurations = append([]float64{diffsinger.HeadFrames * frameMS}, phoneDurations...)
	phoneDurations = append(phoneDurations, diffsinger.TailFrames*frameMS)
	frames := durationsMSToFrames(phoneDurations, frameMS)
	symbols := append(append([]string{"SP"}, phones...), "SP")
	totalFrames := 0
	for _, duration := range frames {
		totalFrames += int(duration)
	}
	f0, err := diffsingerF0(cfg, totalFrames, frameMS)
	if err != nil {
		return nil, err
	}
	midi, err := diffsingerMIDI(cfg.Tone)
	if err != nil {
		return nil, err
	}
	wordDiv := append(append([]int64{1}, phoneCounts...), 1)
	wordDur := groupedFrameDurations(frames, wordDiv)
	noteRest := append([]bool{true}, make([]bool, len(morae))...)
	for index, mora := range morae {
		noteRest[index+1] = mora.Pause
	}
	noteRest = append(noteRest, true)
	score := engine.NeuralScore{
		Symbols: symbols, Durations: frames, F0: f0, MIDI: midi,
		WordDiv: wordDiv, WordDur: wordDur, NoteRest: noteRest,
		UsePitchPredictor: singer.Pitch != nil && cfg.PitchCurve == nil,
	}
	bridgePath := cfg.Engine.Resource(engine.ResourceDiffSingerBridge)
	if bridgePath == "" {
		return nil, fmt.Errorf("DiffSinger bridge is not configured by the renderer plugin")
	}
	pcm, err := diffsinger.RenderScore(cfg.Context, bridgePath, singer, score)
	if err != nil {
		return nil, err
	}
	synthesisPlan := diffsingerPlan(cfg, reading, language, phonemizer, morae, durations, singer.FrameMS())
	positions := make([]float64, len(durations))
	pitchPoints := make([]float64, len(durations))
	cursor := synthesisPlan.LeadingMarginMS
	for index, duration := range durations {
		positions[index] = cursor + duration/2
		if !morae[index].Pause {
			pitchPoints[index] = preview.PitchPoints[index]
		}
		cursor += duration
	}
	return &Result{Plan: synthesisPlan, Audio: pcm, MoraDurationsMS: durations, MoraPositionsMS: positions, PitchPoints: pitchPoints}, nil
}

// 音響モデルを編集画面と同じ発話時間軸に置く。DiffSingerの先頭パディングは
// 出力側に属し、プロソディモデルの入力には含めない。
func prepareDiffSingerProsody(cfg Config, reading string, morae []frontend.Mora, frameMS float64) (Config, *ProsodyPreview, error) {
	preview, err := PredictProsody(cfg)
	if err != nil {
		return cfg, nil, fmt.Errorf("predict DiffSinger speech prosody: %w", err)
	}
	curve := cfg.PitchCurve
	if curve == nil {
		curve = preview.FramePitchCurve
	}
	timings := make([]prosody.MoraTiming, len(morae))
	cursor := 0.0
	for i, duration := range preview.MoraDurationsMS {
		timings[i] = prosody.MoraTiming{StartMS: cursor, DurationMS: duration}
		cursor += duration
	}
	manual := cfg.ManualPitch
	if manual == nil && cfg.ManualPitchPath != "" {
		manual, err = prosody.LoadManualPitch(cfg.ManualPitchPath)
		if err != nil {
			return cfg, nil, err
		}
	}
	if manual != nil {
		if err := manual.Validate(); err != nil {
			return cfg, nil, err
		}
		if manual.Reading != "" && manual.Reading != reading {
			return cfg, nil, fmt.Errorf("manual pitch reading does not match synthesis reading")
		}
		contour, err := manual.Curve(morae, timings, cursor)
		if err != nil {
			return cfg, nil, err
		}
		curve = render.ConstrainPitchCurve(mergeManualPitchCurve(curve, contour, manual.Mode), 20, 8)
	}
	if curve != nil {
		padding := diffsinger.HeadFrames * frameMS
		shifted := &render.PitchCurve{FrameMS: frameMS, Cents: make([]float64, int(math.Ceil((cursor+padding+diffsinger.TailFrames*frameMS)/frameMS))+1)}
		for i := range shifted.Cents {
			shifted.Cents[i] = pitchCurveCentsAt(curve, math.Max(0, float64(i)*frameMS-padding))
		}
		cfg.PitchCurve = shifted
	}
	return cfg, preview, nil
}

func diffsingerPhones(singer *diffsinger.Singer, morae []frontend.Mora, durations []float64) ([]string, []float64, []int64, error) {
	var phones []string
	var phoneDurations []float64
	var phoneCounts []int64
	for index, mora := range morae {
		if mora.Pause {
			phones = append(phones, "SP")
			phoneDurations = append(phoneDurations, durations[index])
			phoneCounts = append(phoneCounts, 1)
			continue
		}
		if symbols := diffsingerDictionarySymbols(singer, mora); len(symbols) > 0 {
			phones = append(phones, symbols...)
			phoneDurations = append(phoneDurations, diffsingerDictionaryDurations(mora, symbols, durations[index])...)
			phoneCounts = append(phoneCounts, int64(len(symbols)))
			continue
		}
		if mora.Vowel == "n" || mora.Vowel == "cl" {
			symbol := mora.Vowel
			if symbol == "n" {
				symbol = firstSupported(singer, "N", "n")
			} else {
				symbol = firstSupported(singer, "cl", "q")
			}
			if symbol == "" {
				return nil, nil, nil, fmt.Errorf("DiffSinger singer has no phoneme for %q", mora.Text)
			}
			phones = append(phones, symbol)
			phoneDurations = append(phoneDurations, durations[index])
			phoneCounts = append(phoneCounts, 1)
			continue
		}
		vowel := firstSupported(singer, mora.Vowel, strings.ToUpper(mora.Vowel))
		if vowel == "" {
			return nil, nil, nil, fmt.Errorf("DiffSinger singer has no vowel %q for %q", mora.Vowel, mora.Text)
		}
		if mora.Consonant == "" {
			phones = append(phones, vowel)
			phoneDurations = append(phoneDurations, durations[index])
			phoneCounts = append(phoneCounts, 1)
			continue
		}
		consonant := firstSupported(singer, mora.Consonant, strings.ToLower(mora.Consonant))
		if consonant == "" {
			return nil, nil, nil, fmt.Errorf("DiffSinger singer has no consonant %q for %q", mora.Consonant, mora.Text)
		}
		consonantMS := diffsingerConsonantDuration(consonant, durations[index])
		phones = append(phones, consonant, vowel)
		phoneDurations = append(phoneDurations, consonantMS, durations[index]-consonantMS)
		phoneCounts = append(phoneCounts, 2)
	}
	return phones, phoneDurations, phoneCounts, nil
}

func diffsingerDictionarySymbols(singer *diffsinger.Singer, mora frontend.Mora) []string {
	if symbols := singer.JapaneseDictionary[mora.Text]; len(symbols) > 0 {
		return symbols
	}
	if mora.Text != "ー" {
		return nil
	}
	vowels := map[string]string{"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お"}
	symbols := singer.JapaneseDictionary[vowels[mora.Vowel]]
	if len(symbols) == 0 {
		return nil
	}
	return symbols[len(symbols)-1:]
}

func diffsingerDictionaryDurations(mora frontend.Mora, symbols []string, durationMS float64) []float64 {
	if len(symbols) <= 1 {
		return []float64{durationMS}
	}
	onset := diffsingerConsonantDuration(mora.Consonant, durationMS)
	result := make([]float64, len(symbols))
	for index := 0; index < len(result)-1; index++ {
		result[index] = onset / float64(len(result)-1)
	}
	result[len(result)-1] = durationMS - onset
	return result
}

func diffsingerConsonantDuration(consonant string, durationMS float64) float64 {
	ratio := 0.4
	symbol := strings.TrimPrefix(consonant, "ja/")
	switch symbol {
	case "ch", "ts":
		ratio = 0.55
	case "s", "sh":
		ratio = 0.52
	case "z", "f", "v":
		ratio = 0.5
	case "h":
		ratio = 0.48
	case "k", "t", "p":
		ratio = 0.45
	case "g", "d", "b", "n", "m", "w":
		ratio = 0.42
	case "r", "y":
		ratio = 0.38
	default:
		if strings.HasSuffix(symbol, "y") {
			ratio = 0.46
		}
	}
	target := math.Min(75, math.Max(30, durationMS*ratio))
	maximum := math.Max(durationMS*0.4, durationMS-45)
	return math.Min(target, maximum)
}

func groupedFrameDurations(frames, groups []int64) []int64 {
	result := make([]int64, len(groups))
	cursor := 0
	for groupIndex, count := range groups {
		for index := int64(0); index < count; index++ {
			result[groupIndex] += frames[cursor]
			cursor++
		}
	}
	return result
}

func firstSupported(singer *diffsinger.Singer, candidates ...string) string {
	for _, candidate := range candidates {
		for _, symbol := range []string{candidate, "ja/" + candidate} {
			if _, ok := singer.Tokens[symbol]; ok {
				return symbol
			}
		}
	}
	return ""
}

func durationsMSToFrames(durations []float64, frameMS float64) []int64 {
	result := make([]int64, len(durations))
	accumulated := 0.0
	previous := int64(0)
	for index, duration := range durations {
		accumulated += duration
		frame := int64(math.RoundToEven(accumulated/frameMS + 0.5))
		result[index] = frame - previous
		previous = frame
	}
	return result
}

func diffsingerF0(cfg Config, frames int, frameMS float64) ([]float32, error) {
	midi, err := diffsingerMIDI(cfg.Tone)
	if err != nil {
		return nil, err
	}
	base := 440 * math.Pow(2, float64(midi-69)/12)
	result := make([]float32, frames)
	for frame := range result {
		cents := 0.0
		if cfg.PitchCurve != nil {
			cents = pitchCurveCentsAt(cfg.PitchCurve, float64(frame)*frameMS)
		}
		result[frame] = float32(base * math.Pow(2, cents/1200))
	}
	return result, nil
}

func diffsingerMIDI(tone string) (int, error) {
	if tone == "" {
		return 60, nil
	}
	midi, err := openutau.ToneToMIDI(tone)
	if err != nil {
		return 0, fmt.Errorf("DiffSinger tone: %w", err)
	}
	return midi, nil
}

func diffsingerPlan(cfg Config, reading, language, phonemizer string, morae []frontend.Mora, durations []float64, frameMS float64) *plan.Plan {
	result := &plan.Plan{
		Version: plan.Version, Voicebank: cfg.VoicebankPath, Text: cfg.Text, Reading: reading,
		Language: language, Phonemizer: phonemizer, Tone: cfg.Tone,
		SelectionMode: "neural", AliasPolicy: "neural", JoinCostMode: "none",
		LeadingMarginMS: diffsinger.HeadFrames * frameMS, Morae: append([]frontend.Mora(nil), morae...),
	}
	cursor := 0.0
	for index, mora := range morae {
		if !mora.Pause {
			result.Units = append(result.Units, plan.Unit{
				Position: index, Role: "mora", Mora: mora.Text, Alias: mora.Text,
				Silent: false, NoteStartMS: cursor, DurationMS: durations[index],
				PitchFactor: 1, EnergyFactor: 1, TimingScale: 1, IntonationFactor: 1,
			})
		}
		cursor += durations[index]
	}
	result.DurationMS = cursor + result.LeadingMarginMS + diffsinger.TailFrames*frameMS
	return result
}
