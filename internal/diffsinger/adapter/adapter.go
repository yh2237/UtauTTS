// Package adapterはDiffSinger実装をprovider非依存のニューラル契約へ適合させる。
// このパッケージはinternal/ttsをimportせず、低層のneural.Inputだけを入力に取る。
package adapter

import (
	"fmt"
	"math"
	"strings"

	"utautts/internal/diffsinger"
	"utautts/internal/engine"
	"utautts/internal/frontend"
	"utautts/internal/neural"
	"utautts/internal/openutau"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// SynthesizerはDiffSinger実装をニューラルprovider契約へ適合させる。
type Synthesizer struct{}

func (Synthesizer) ProviderID() engine.ProviderID { return diffsinger.ProviderID }

// DiffSinger実装はprovider IDでレジストリへ登録し、tts側にprovider名の分岐を持たせない。
func init() {
	neural.Register(diffsinger.ProviderID, func() neural.Synthesizer {
		return Synthesizer{}
	})
	neural.RegisterCloser(diffsinger.CloseProviderSessions)
}

func (Synthesizer) Synthesize(in neural.Input) (*neural.Output, error) {
	singer, err := diffsinger.Load(in.VoicebankPath)
	if err != nil {
		return nil, fmt.Errorf("load DiffSinger singer: %w", err)
	}
	if in.Language != frontend.LanguageJapanese {
		return nil, fmt.Errorf("DiffSinger MVP currently supports Japanese input only")
	}
	morae := in.Morae
	durations := in.MoraDurationsMS
	frameMS := singer.FrameMS()
	// 先頭パディングはプロソディモデルの入力に含めない。provider側で曲線をずらす。
	curve := shiftPitchCurve(in.PitchCurve, frameMS, totalDurationMS(durations))
	phones, phoneDurations, phoneCounts, err := diffsingerPhones(singer, morae, durations, in.PhoneWeights)
	if err != nil {
		return nil, err
	}
	phoneDurations = append([]float64{diffsinger.HeadFrames * frameMS}, phoneDurations...)
	phoneDurations = append(phoneDurations, diffsinger.TailFrames*frameMS)
	frames := durationsMSToFrames(phoneDurations, frameMS)
	symbols := append(append([]string{"SP"}, phones...), "SP")
	totalFrames := 0
	for _, duration := range frames {
		totalFrames += int(duration)
	}
	f0, err := diffsingerF0(in.Tone, curve, totalFrames, frameMS)
	if err != nil {
		return nil, err
	}
	midi, err := diffsingerMIDI(in.Tone)
	if err != nil {
		return nil, err
	}
	wordDiv, noteRest := diffsingerWordGroups(morae, phoneCounts, in.Features)
	wordDur := groupedFrameDurations(frames, wordDiv)
	automaticPitch := in.AutomaticPitch
	noteMIDI, phMIDI := diffsingerMIDICurves(f0, wordDur, frames, midi)
	score := engine.NeuralScore{
		Symbols: symbols, Durations: frames, F0: f0, MIDI: midi,
		NoteMIDI: noteMIDI, PhMIDI: phMIDI,
		WordDiv: wordDiv, WordDur: wordDur, NoteRest: noteRest,
		Steps: in.ProviderOptions.DiffSinger.Steps, DurationPredictorMix: float32(in.ProviderOptions.DiffSinger.DurationMix),
		Expr:              float32(in.ProviderOptions.DiffSinger.Expr),
		UsePitchPredictor: singer.Pitch != nil && (in.PitchCurve == nil || automaticPitch),
	}
	if in.ProviderOptions.DiffSinger.PitchMix > 0 {
		score.PitchPredictorMix = float32(in.ProviderOptions.DiffSinger.PitchMix)
	} else if automaticPitch && singer.Pitch != nil {
		// 話声用の輪郭を基準に、音源側の滑らかな微小変化だけを混ぜる。
		score.PitchPredictorMix = .10
	}
	bridgePath := in.Engine.Resource(engine.ResourceDiffSingerBridge)
	if bridgePath == "" {
		return nil, fmt.Errorf("DiffSinger bridge is not configured by the renderer plugin")
	}
	pcm, err := diffsinger.RenderScore(in.Context, bridgePath, singer, score)
	if err != nil {
		return nil, err
	}
	synthesisPlan := diffsingerPlan(in, morae, durations, phones, phoneDurations[1:len(phoneDurations)-1], phoneCounts, frameMS)
	positions := make([]float64, len(durations))
	pitchPoints := make([]float64, len(durations))
	cursor := synthesisPlan.LeadingMarginMS
	for index, duration := range durations {
		positions[index] = cursor + duration/2
		if !morae[index].Pause {
			pitchPoints[index] = in.PitchPoints[index]
		}
		cursor += duration
	}
	return &neural.Output{Plan: synthesisPlan, Audio: pcm, MoraDurationsMS: durations, MoraPositionsMS: positions, PitchPoints: pitchPoints}, nil
}

func totalDurationMS(durations []float64) float64 {
	total := 0.0
	for _, duration := range durations {
		total += duration
	}
	return total
}

// shiftPitchCurveは先頭パディングぶん曲線を後ろへずらし、フレーム長を音源に合わせる。
func shiftPitchCurve(curve *render.PitchCurve, frameMS, durationMS float64) *render.PitchCurve {
	if curve == nil {
		return nil
	}
	padding := diffsinger.HeadFrames * frameMS
	shifted := &render.PitchCurve{FrameMS: frameMS, Cents: make([]float64, int(math.Ceil((durationMS+padding+diffsinger.TailFrames*frameMS)/frameMS))+1)}
	for i := range shifted.Cents {
		shifted.Cents[i] = curve.CentsAt(math.Max(0, float64(i)*frameMS-padding))
	}
	return shifted
}

func diffsingerPhones(singer *diffsinger.Singer, morae []frontend.Mora, durations []float64, weights [][]float64) ([]string, []float64, []int64, error) {
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
			phoneDurations = append(phoneDurations, diffsingerDictionaryDurations(mora, symbols, durations[index], phoneWeightsAt(weights, index))...)
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
		// 話声では子音を短くしすぎると潰れる。共有重みと話声向け比率の長い方を採る。
		consonantMS := diffsingerConsonantDuration(consonant, durations[index])
		if index < len(weights) && len(weights[index]) == 2 {
			if spans := phoneSpansFromWeights(weights[index], durations[index]); spans[0] > consonantMS {
				consonantMS = spans[0]
			}
		}
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

func diffsingerDictionaryDurations(mora frontend.Mora, symbols []string, durationMS float64, weights []float64) []float64 {
	if len(symbols) <= 1 {
		return []float64{durationMS}
	}
	if len(weights) == len(symbols) {
		return phoneSpansFromWeights(weights, durationMS)
	}
	onset := diffsingerConsonantDuration(mora.Consonant, durationMS)
	result := make([]float64, len(symbols))
	for index := 0; index < len(result)-1; index++ {
		result[index] = onset / float64(len(result)-1)
	}
	result[len(result)-1] = durationMS - onset
	return result
}

func phoneWeightsAt(weights [][]float64, index int) []float64 {
	if index >= 0 && index < len(weights) {
		return weights[index]
	}
	return nil
}

// phoneSpansFromWeightsは重みの比率でモーラ長を音素へ配分する。
func phoneSpansFromWeights(weights []float64, duration float64) []float64 {
	result := make([]float64, len(weights))
	total := 0.0
	for _, weight := range weights {
		if weight > 0 && !math.IsNaN(weight) && !math.IsInf(weight, 0) {
			total += weight
		}
	}
	if total <= 0 {
		return result
	}
	for i, weight := range weights {
		result[i] = duration * weight / total
	}
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

func diffsingerWordGroups(morae []frontend.Mora, phoneCounts []int64, features []prosody.FeatureFrame) ([]int64, []bool) {
	groups := []int64{1}
	rests := []bool{true}
	hasWordBoundaries := false
	for index, mora := range morae {
		if mora.WordEnd || (index < len(features) && features[index]["word_end"] > 0) {
			hasWordBoundaries = true
			break
		}
	}
	count := int64(0)
	for index, mora := range morae {
		phones := int64(1)
		if index < len(phoneCounts) && phoneCounts[index] > 0 {
			phones = phoneCounts[index]
		}
		if mora.Pause {
			if count > 0 {
				groups = append(groups, count)
				rests = append(rests, false)
				count = 0
			}
			groups = append(groups, phones)
			rests = append(rests, true)
			continue
		}
		count += phones
		wordEnd := mora.WordEnd
		if index < len(features) && features[index]["word_end"] > 0 {
			wordEnd = true
		}
		if wordEnd || !hasWordBoundaries || index+1 == len(morae) || morae[index+1].Pause {
			groups = append(groups, count)
			rests = append(rests, false)
			count = 0
		}
	}
	if count > 0 {
		groups = append(groups, count)
		rests = append(rests, false)
	}
	return append(groups, 1), append(rests, true)
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

func diffsingerF0(tone string, curve *render.PitchCurve, frames int, frameMS float64) ([]float32, error) {
	midi, err := diffsingerMIDI(tone)
	if err != nil {
		return nil, err
	}
	base := 440 * math.Pow(2, float64(midi-69)/12)
	result := make([]float32, frames)
	for frame := range result {
		cents := 0.0
		if curve != nil {
			cents = curve.CentsAt(float64(frame) * frameMS)
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

// diffsingerMIDICurvesは話声向けに、音符(単語)ごとと音素ごとのMIDIをF0から求める。
// 定数のMIDIではDiffSingerのピッチ・長さ予測器の基準が実際の高さとずれる。
func diffsingerMIDICurves(f0 []float32, noteFrames, phoneFrames []int64, fallback int) ([]float32, []int64) {
	values := make([]float64, len(f0))
	for index, hz := range f0 {
		if hz > 0 {
			values[index] = 69 + 12*math.Log2(float64(hz)/440)
			continue
		}
		values[index] = math.NaN()
	}
	base := float64(fallback)
	notes := midiGroupAverages(values, noteFrames, base)
	phones := midiGroupAverages(values, phoneFrames, base)
	noteMIDI := make([]float32, len(notes))
	for index, value := range notes {
		noteMIDI[index] = float32(value)
	}
	phMIDI := make([]int64, len(phones))
	for index, value := range phones {
		phMIDI[index] = int64(math.Round(value))
	}
	return noteMIDI, phMIDI
}

// midiGroupAveragesはフレームごとのMIDIを連続した区間へ平均する。無声区間は直前の値を保つ。
func midiGroupAverages(values []float64, counts []int64, fallback float64) []float64 {
	result := make([]float64, len(counts))
	cursor := 0
	last := fallback
	for group, count := range counts {
		sum, voiced := 0.0, 0
		for offset := int64(0); offset < count && cursor < len(values); offset++ {
			if value := values[cursor]; !math.IsNaN(value) {
				sum += value
				voiced++
			}
			cursor++
		}
		if voiced > 0 {
			last = sum / float64(voiced)
		}
		result[group] = last
	}
	return result
}

func diffsingerPlan(in neural.Input, morae []frontend.Mora, durations []float64, phones []string, phoneDurations []float64, phoneCounts []int64, frameMS float64) *plan.Plan {
	result := &plan.Plan{
		Version: plan.Version, Voicebank: in.VoicebankPath, Text: in.Text, Reading: in.Reading,
		Language: in.Language, Phonemizer: in.Phonemizer, Tone: in.Tone,
		SelectionMode: "neural", AliasPolicy: "neural", JoinCostMode: "none",
		LeadingMarginMS: diffsinger.HeadFrames * frameMS, Morae: append([]frontend.Mora(nil), morae...),
		PhoneTimingSource: "language-phone-v1",
	}
	cursor := 0.0
	phoneCursor := 0
	for index, mora := range morae {
		count := 0
		if index < len(phoneCounts) {
			count = int(phoneCounts[index])
		}
		localCursor := cursor
		for offset := 0; offset < count && phoneCursor < len(phones) && phoneCursor < len(phoneDurations); offset++ {
			role := "nucleus"
			if offset+1 < count {
				role = "onset"
			}
			if mora.Pause {
				role = "pause"
			}
			result.PhoneTimings = append(result.PhoneTimings, plan.PhoneTiming{Position: index, Symbol: phones[phoneCursor], Role: role, StartMS: localCursor, DurationMS: phoneDurations[phoneCursor]})
			localCursor += phoneDurations[phoneCursor]
			phoneCursor++
		}
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
