//go:build !js

package tts

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/refspeech"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

var englishLexicons sync.Map

// EnglishReferenceModelsは英語の合成に使うモデルの場所を返す。
func EnglishReferenceModels(dir string) (refspeech.Models, string, string) {
	return refspeech.Models{
		KokoroWeights: filepath.Join(dir, "kokoro-v1_0.pth"),
		KokoroConfig:  filepath.Join(dir, "config.json"),
		KokoroVoice:   filepath.Join(dir, "af_heart.pt"),
		HuBERT:        filepath.Join(dir, "hubert-base-ls960.safetensors"),
	}, filepath.Join(dir, "us_gold.json"), filepath.Join(dir, "us_silver.json")
}

// EnglishReferenceServiceは英語の合成と音源の索引作成を受け持つServiceを返す。
func EnglishReferenceService(dir string) (*refspeech.Service, error) {
	if dir == "" {
		return nil, fmt.Errorf("English synthesis models are not installed in models/%s", EnglishReferenceDir)
	}
	cacheDir, err := refspeech.DefaultCacheDir()
	if err != nil {
		return nil, err
	}
	models, _, _ := EnglishReferenceModels(dir)
	return refspeech.Shared(models, cacheDir), nil
}

// PrepareEnglishIndexは音源の英語の索引がなければ裏で作り始める。モデルがなければ何もしない。
func PrepareEnglishIndex(dir, bankRoot, worldEnginePath string) {
	if service, err := EnglishReferenceService(dir); err == nil {
		service.Prepare(bankRoot, worldEnginePath)
	}
}

// EnglishIndexProgressは索引の作成の進み具合を返す。
func EnglishIndexProgress() refspeech.Progress {
	return refspeech.CurrentProgress()
}

func englishLexicon(dir string) (*refspeech.Lexicon, error) {
	_, gold, silver := EnglishReferenceModels(dir)
	if cached, ok := englishLexicons.Load(gold); ok {
		return cached.(*refspeech.Lexicon), nil
	}
	lexicon, err := refspeech.LoadLexicon(gold, silver)
	if err != nil {
		return nil, fmt.Errorf("load English lexicon: %w", err)
	}
	actual, _ := englishLexicons.LoadOrStore(gold, lexicon)
	return actual.(*refspeech.Lexicon), nil
}

// englishReferenceInputsは、モーラ長を話速に、抑揚の強さをF0の動きの幅に、音高をF0の中心に直す。
func englishReferenceInputs(cfg Config, reading string) (*refspeech.Service, []refspeech.Piece, refspeech.Options, error) {
	service, err := EnglishReferenceService(cfg.EnglishReferenceDir)
	if err != nil {
		return nil, nil, refspeech.Options{}, err
	}
	lexicon, err := englishLexicon(cfg.EnglishReferenceDir)
	if err != nil {
		return nil, nil, refspeech.Options{}, err
	}
	options := refspeech.Options{Speed: 1, PitchScale: cfg.IntonationStrength / settings.Number("intonation_strength")}
	if cfg.MoraDurationMS > 0 {
		options.Speed = float32(plan.DefaultMoraDurationMS / cfg.MoraDurationMS)
	}
	if frequency, ok := voicebank.ToneFrequency(cfg.Tone); ok {
		options.CenterHz = frequency
	}
	return service, refspeech.KokoroPieces(lexicon, cfg.Text, reading, cfg.Dictionary), options, nil
}

// synthesizeEnglishReferenceは英語の文を参照TTSで読み、音源のフレームで合成し直す。
func synthesizeEnglishReference(cfg Config, bank *voicebank.Bank, language, phonemizer, reading string, morae []frontend.Mora) (*Result, error) {
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	service, pieces, options, err := englishReferenceInputs(cfg, reading)
	if err != nil {
		return nil, err
	}
	speech, err := service.Speak(ctx, bank.Root, cfg.WorldEnginePath, pieces, options)
	if err != nil {
		return nil, fmt.Errorf("English synthesis: %w", err)
	}
	pcm := &audio.PCM{SampleRate: speech.SampleRate, Channels: 1, Data: make([]int16, len(speech.Audio))}
	for i, v := range speech.Audio {
		pcm.Data[i] = int16(math.Round(math.Max(-1, math.Min(1, v)) * 32767))
	}
	durations := englishMoraDurations(pieces, speech.PieceMS, speech.TrailingMS, reading, morae)
	synthesisPlan := &plan.Plan{
		Version: plan.Version, Voicebank: bank.Root, Text: cfg.Text, Reading: reading,
		Language: language, Phonemizer: phonemizer, Tone: cfg.Tone,
		SelectionMode: "reference", AliasPolicy: "reference", JoinCostMode: "none",
		LeadingMarginMS: speech.LeadingMS, Morae: append([]frontend.Mora(nil), morae...),
		DurationMS: float64(len(speech.Audio)) * 1000 / float64(speech.SampleRate),
	}
	result := &Result{Voicebank: bank, Plan: synthesisPlan, Audio: pcm,
		MoraDurationsMS: durations, MoraPositionsMS: make([]float64, len(morae)), PitchPoints: make([]float64, len(morae))}
	center := englishCenterHz(speech.F0)
	cursor := 0.0
	for index, mora := range morae {
		result.MoraPositionsMS[index] = cursor + durations[index]/2
		if !mora.Pause {
			synthesisPlan.Units = append(synthesisPlan.Units, plan.Unit{
				Position: index, Role: "mora", Mora: mora.Text, Alias: mora.Text,
				NoteStartMS: cursor, DurationMS: durations[index],
				PitchFactor: 1, EnergyFactor: 1, TimingScale: 1, IntonationFactor: 1,
			})
			start := speech.LeadingMS + cursor
			result.PitchPoints[index] = englishCentsBetween(speech.F0, center, start, start+durations[index])
		}
		cursor += durations[index]
	}
	return result, nil
}

// predictEnglishReferenceは、Kokoroの読みだけから英語のモーラ長と抑揚を返す。音源は使わない。
func predictEnglishReference(cfg Config, reading string, morae []frontend.Mora) (*ProsodyPreview, error) {
	service, pieces, options, err := englishReferenceInputs(cfg, reading)
	if err != nil {
		return nil, err
	}
	reference, err := service.Read(pieces, options.Speed)
	if err != nil {
		return nil, fmt.Errorf("English prosody: %w", err)
	}
	durations := englishMoraDurations(pieces, reference.PieceMS, reference.TrailingMS, reading, morae)
	f0 := reference.ReferenceF0()
	center := englishCenterHz(f0)
	result := &ProsodyPreview{
		Reading: reading, Morae: append([]frontend.Mora(nil), morae...), MoraDurationsMS: durations,
		MoraPositionsMS: make([]float64, len(morae)), PitchPoints: make([]float64, len(morae)),
		FramePitchCurve: englishPitchCurve(f0, center, reference.LeadingMS, options.PitchScale),
	}
	cursor := 0.0
	for index, mora := range morae {
		result.MoraPositionsMS[index] = cursor + durations[index]/2
		if !mora.Pause {
			start := reference.LeadingMS + cursor
			result.PitchPoints[index] = options.PitchScale * englishCentsBetween(f0, center, start, start+durations[index])
		}
		cursor += durations[index]
	}
	return result, nil
}

// englishPitchCurveは、最初のモーラの始まりを0とするセントの曲線。無声区間は前後の有声区間を直線でつなぐ。
func englishPitchCurve(f0 []float64, center, leadingMS, scale float64) *render.PitchCurve {
	first := int(leadingMS / refspeech.F0FrameMS)
	if center <= 0 || first >= len(f0) {
		return nil
	}
	cents := make([]float64, len(f0)-first)
	last := -1
	for i := range cents {
		v := f0[first+i]
		if v <= 0 {
			continue
		}
		cents[i] = scale * 1200 * math.Log2(v/center)
		if last < 0 {
			for j := range i {
				cents[j] = cents[i]
			}
		} else {
			for j := last + 1; j < i; j++ {
				cents[j] = cents[last] + (cents[i]-cents[last])*float64(j-last)/float64(i-last)
			}
		}
		last = i
	}
	if last < 0 {
		return nil
	}
	for j := last + 1; j < len(cents); j++ {
		cents[j] = cents[last]
	}
	return &render.PitchCurve{FrameMS: refspeech.F0FrameMS, Cents: cents}
}

// englishMoraDurationsはKokoroの音素長を、読みの語（SPを含む）ごとに足し、語の中のモーラに等分する。
// 語の間の空白は直前の語に、文末の記号と末尾の境界記号は最後の語か休止に入れる。
func englishMoraDurations(pieces []refspeech.Piece, pieceMS []float64, trailingMS float64, reading string, morae []frontend.Mora) []float64 {
	partMS := map[int]float64{}
	lastPart := -1
	for i, piece := range pieces {
		if piece.Part >= 0 {
			lastPart = piece.Part
		}
		if lastPart >= 0 {
			partMS[lastPart] += pieceMS[i]
		}
	}
	var pauseParts []int
	for index, part := range splitEnglishReading(reading) {
		if part == "SP" {
			pauseParts = append(pauseParts, index)
		}
	}
	if n := len(pauseParts); n > 0 && pauseParts[n-1] > lastPart {
		wordMS := englishWordOnlyMS(pieces, pieceMS, lastPart)
		partMS[pauseParts[n-1]] += partMS[lastPart] - wordMS + trailingMS
		partMS[lastPart] = wordMS
	} else if lastPart >= 0 {
		partMS[lastPart] += trailingMS
	}
	counts := map[int]int{}
	for _, mora := range morae {
		if !mora.Pause {
			counts[mora.WordIndex]++
		}
	}
	durations := make([]float64, len(morae))
	pause := 0
	for index, mora := range morae {
		if mora.Pause {
			if pause < len(pauseParts) {
				durations[index] = partMS[pauseParts[pause]]
			}
			pause++
			continue
		}
		durations[index] = partMS[mora.WordIndex] / float64(counts[mora.WordIndex])
	}
	return durations
}

func englishWordOnlyMS(pieces []refspeech.Piece, pieceMS []float64, part int) float64 {
	var total float64
	for i, piece := range pieces {
		if piece.Part == part {
			total += pieceMS[i]
		}
	}
	return total
}

func splitEnglishReading(reading string) []string {
	var parts []string
	for _, part := range strings.Split(reading, "|") {
		if fields := strings.Fields(part); len(fields) > 0 {
			parts = append(parts, strings.Join(fields, " "))
		}
	}
	return parts
}

// englishCenterHzは出力のF0の有声区間の中央値。
func englishCenterHz(f0 []float64) float64 {
	var voiced []float64
	for _, v := range f0 {
		if v > 0 {
			voiced = append(voiced, v)
		}
	}
	if len(voiced) == 0 {
		return 0
	}
	sort.Float64s(voiced)
	return voiced[len(voiced)/2]
}

// englishCentsBetweenはstartMSからendMSまでの有声区間の平均F0を、centerからのセントで返す。
func englishCentsBetween(f0 []float64, center, startMS, endMS float64) float64 {
	if center <= 0 {
		return 0
	}
	var sum float64
	count := 0
	for frame := int(startMS / refspeech.F0FrameMS); frame < int(endMS/refspeech.F0FrameMS) && frame < len(f0); frame++ {
		if frame >= 0 && f0[frame] > 0 {
			sum += math.Log2(f0[frame] / center)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return 1200 * sum / float64(count)
}
