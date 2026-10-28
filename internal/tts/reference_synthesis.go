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
	"time"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/refmatch"
	"github.com/yh2237/UtauTTS/internal/refspeech"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

var englishLexicons sync.Map

// ReferenceModelsは参照照合に使うモデルの場所を返す。
func ReferenceModels(dir string) (refspeech.Models, string, string) {
	shared := map[string]string{}
	for id, name := range sharedReferenceVoices {
		if path := filepath.Join(dir, name); exists(path) {
			shared[id] = path
		}
	}
	return refspeech.Models{
		SharedVoices:  shared,
		KokoroWeights: filepath.Join(dir, "kokoro-v1_0.pth"),
		KokoroConfig:  filepath.Join(dir, "config.json"),
		KokoroVoice:   filepath.Join(dir, "af_heart.pt"),
		ZhWeights:     filepath.Join(dir, "kokoro-zh", "kokoro-v1_1-zh.pth"),
		ZhConfig:      filepath.Join(dir, "kokoro-zh", "config.json"),
		ZhVoice:       filepath.Join(dir, "kokoro-zh", "zf_001.pt"),
		HuBERT:        filepath.Join(dir, "wavlm", "wavlm-base-plus.safetensors"),
	}, filepath.Join(dir, "us_gold.json"), filepath.Join(dir, "us_silver.json")
}

// ReferenceServiceは参照照合と音源の索引作成を受け持つServiceを返す。
func ReferenceService(dir string) (*refspeech.Service, error) {
	if dir == "" {
		return nil, fmt.Errorf("reference synthesis models are not installed in models/%s", ReferenceDir)
	}
	cacheDir, err := refspeech.DefaultCacheDir()
	if err != nil {
		return nil, err
	}
	models, _, _ := ReferenceModels(dir)
	return refspeech.Shared(models, cacheDir), nil
}

// PrepareReferenceIndexは音源の通常の声の参照の索引がなければ裏で作り始める。モデルがなければ何もしない。
func PrepareReferenceIndex(dir string, bank *voicebank.Bank, worldEnginePath string) {
	if service, err := ReferenceService(dir); err == nil {
		service.Prepare(bank.Root, referenceRecordings(bank, ""), worldEnginePath)
	}
}

// referenceRecordingsは索引に使う録音と、それを使う原音のoto.iniの範囲を返す。
func referenceRecordings(bank *voicebank.Bank, color string) []refmatch.Recording {
	recordings := bank.ReferenceRecordings(color)
	result := make([]refmatch.Recording, len(recordings))
	for i, recording := range recordings {
		result[i].Path = recording.Path
		for _, entry := range recording.Entries {
			result[i].Regions = append(result[i].Regions, refmatch.Region{Offset: entry.Offset, Blank: entry.Blank})
		}
	}
	return result
}

// ReferenceIndexProgressは索引の作成の進み具合を返す。
func ReferenceIndexProgress() refspeech.Progress {
	return refspeech.CurrentProgress()
}

func englishLexicon(dir string) (*refspeech.Lexicon, error) {
	_, gold, silver := ReferenceModels(dir)
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

// referenceInputsは、言語に合う参照TTSを選び、モーラ長を話速に、抑揚の強さをF0の動きの幅に、
// 音高をF0の中心に直す。
func referenceInputs(cfg Config, language, reading string) (*refspeech.Service, refspeech.Provider, []refspeech.Piece, refspeech.Options, error) {
	service, err := ReferenceService(cfg.ReferenceDir)
	if err != nil {
		return nil, nil, nil, refspeech.Options{}, err
	}
	provider, err := referenceProvider(cfg, language, reading, service)
	if err != nil {
		return nil, nil, nil, refspeech.Options{}, err
	}
	options := refspeech.Options{
		Speed:      1,
		PitchScale: cfg.IntonationStrength / settings.Number("intonation_strength"),
	}
	if cfg.MoraDurationMS > 0 {
		options.Speed = float32(plan.DefaultMoraDurationMS / cfg.MoraDurationMS)
	}
	if frequency, ok := voicebank.ToneFrequency(cfg.Tone); ok {
		options.CenterHz = frequency
	}
	pieces, err := provider.Pieces(cfg.Text, reading)
	if err != nil {
		return nil, nil, nil, refspeech.Options{}, err
	}
	return service, provider, pieces, options, nil
}

// referenceProviderは、言語に合う参照TTSを返す。
func referenceProvider(cfg Config, language, reading string, service *refspeech.Service) (refspeech.Provider, error) {
	switch frontend.NormalizeLanguage(language) {
	case frontend.LanguageChinese:
		return refspeech.NewKokoroChineseProvider(service), nil
	case frontend.LanguageSpanish, frontend.LanguagePortuguese:
		return refspeech.NewKokoroSyllableProvider(service, frontend.NormalizeLanguage(language)), nil
	case frontend.LanguageEnglish:
		lexicon, err := englishLexicon(cfg.ReferenceDir)
		if err != nil {
			return nil, err
		}
		return refspeech.NewKokoroProvider(service, lexicon, cfg.Dictionary), nil
	}
	return nil, fmt.Errorf("reference synthesis is not available for %q", language)
}

// englishManualPitchは手動の抑揚を、最初のモーラの始まりからのミリ秒ごとのセントにする。
func referenceManualPitch(cfg Config, reading string, morae []frontend.Mora, durations []float64) (func(float64) float64, bool, error) {
	timings := make([]prosody.MoraTiming, len(morae))
	cursor := 0.0
	for i, d := range durations {
		timings[i] = prosody.MoraTiming{StartMS: cursor, DurationMS: d}
		cursor += d
	}
	contour, mode, err := resolveManualPitchCurve(cfg, reading, morae, timings, cursor)
	if err != nil || contour == nil {
		return nil, false, err
	}
	curve := &render.PitchCurve{FrameMS: contour.FrameMS, Cents: contour.Cents}
	return curve.CentsAt, mode == "replace", nil
}

func hasManualMoraDurations(durations []float64) bool {
	for _, d := range durations {
		if d > 0 {
			return true
		}
	}
	return false
}

// synthesizeReferenceは文を参照TTSで読み、音源のフレームで合成し直す。
func synthesizeReference(cfg Config, bank *voicebank.Bank, language, phonemizer, reading string, morae []frontend.Mora) (*Result, error) {
	if err := plan.ValidateMoraDurations(morae, cfg.MoraDurationsMS); err != nil {
		return nil, err
	}
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	log := cfg.Logf
	if log == nil {
		log = func(string) {}
	}
	started := time.Now()
	// 参照TTSへは、連続変調を反映した読みを渡す。編集画面などに出す読みは辞書の声調のまま。
	referenceReading := reading
	if frontend.NormalizeLanguage(language) == frontend.LanguageChinese {
		referenceReading = chineseSurfaceReading(reading, morae)
		if referenceReading != reading {
			log(fmt.Sprintf("参照の読み: %s", referenceReading))
		}
	}
	service, provider, pieces, options, err := referenceInputs(cfg, language, referenceReading)
	if err != nil {
		return nil, err
	}
	options.Logf = log
	log(fmt.Sprintf("参照TTS: %s（%d 記号）", provider.ID(), len(pieces)))
	unitCents, unitGain, err := referenceUnitAdjustments(cfg, morae)
	if err != nil {
		return nil, err
	}
	refspeech.SetStage("参照音声を生成しています")
	readStarted := time.Now()
	reference, err := provider.Read(pieces, options.Speed, nil)
	if err != nil {
		refspeech.SetStage("")
		return nil, fmt.Errorf("reference synthesis: %w", err)
	}
	rate := reference.SampleRate
	if rate <= 0 {
		rate = 24000
	}
	log(fmt.Sprintf("参照音声: %d Hz、%.2f 秒（生成 %d ms）",
		rate, float64(len(reference.Audio))/float64(rate), time.Since(readStarted).Milliseconds()))
	// 手動の長さがあれば、読み直さずに参照の時間軸を引き伸ばす。
	_, plans, sourceDurations, planned, err := referenceTiming(cfg, language, pieces, reference.Timing, reading, morae)
	if err != nil {
		return nil, fmt.Errorf("reference synthesis: %w", err)
	}
	if hasManualMoraDurations(cfg.MoraDurationsMS) || len(plans) > 0 {
		options.Warp = refspeech.NewWarp(sourceDurations, planned)
		if options.Warp == nil {
			log("指定した長さを反映できませんでした")
		}
	}
	if options.PitchCents, options.ReplacePitch, err = referenceManualPitch(cfg, reading, morae, planned); err != nil {
		return nil, err
	}
	// 拡張編集のPITは、手動の抑揚の上に足す。
	if pit := moraEnvelope(planned, unitCents, 0); pit != nil {
		manual := options.PitchCents
		options.PitchCents = func(ms float64) float64 {
			if manual != nil {
				return manual(ms) + pit(ms)
			}
			return pit(ms)
		}
	}
	refspeech.SetStage("索引を準備しています")
	speech, err := service.SpeakReference(ctx, bank.Root, referenceRecordings(bank, cfg.Color), cfg.WorldEnginePath, reference, options)
	refspeech.SetStage("")
	if err != nil {
		return nil, fmt.Errorf("reference synthesis: %w", err)
	}
	durations := planned
	leading := speech.LeadingMS()
	// 拡張編集のVOLは、モーラの区間の音量に掛ける。
	if volume := moraEnvelope(durations, unitGain, 1); volume != nil {
		for i := range speech.Audio {
			speech.Audio[i] *= volume(float64(i)*1000/float64(speech.SampleRate) - leading)
		}
	}
	peak := 0.0
	for _, v := range speech.Audio {
		peak = math.Max(peak, math.Abs(v))
	}
	scale := 1.0
	if peak > 0.98 {
		scale = 0.98 / peak
	}
	pcm := &audio.PCM{SampleRate: speech.SampleRate, Channels: 1, Data: make([]int16, len(speech.Audio))}
	for i, v := range speech.Audio {
		pcm.Data[i] = int16(math.Round(math.Max(-1, math.Min(1, v*scale)) * 32767))
	}
	synthesisPlan := &plan.Plan{
		Version: plan.Version, Voicebank: bank.Root, Text: cfg.Text, Reading: reading,
		Language: language, Phonemizer: phonemizer, Tone: cfg.Tone, Color: cfg.Color,
		SelectionMode: "reference", AliasPolicy: "reference", JoinCostMode: "none",
		LeadingMarginMS: leading, Morae: append([]frontend.Mora(nil), morae...),
		DurationMS: float64(len(speech.Audio)) * 1000 / float64(speech.SampleRate),
	}
	result := &Result{Voicebank: bank, Plan: synthesisPlan, Audio: pcm,
		MoraDurationsMS: durations, MoraPositionsMS: make([]float64, len(morae)), PitchPoints: make([]float64, len(morae))}
	center := referenceCenterHz(speech.AutomaticF0)
	cursor := 0.0
	for index, mora := range morae {
		result.MoraPositionsMS[index] = cursor + durations[index]/2
		if !mora.Pause {
			synthesisPlan.Units = append(synthesisPlan.Units, plan.Unit{
				Position: index, Role: "mora", Mora: mora.Text, Alias: mora.Text,
				NoteStartMS: cursor, DurationMS: durations[index],
				PitchFactor: 1, EnergyFactor: 1, TimingScale: 1, IntonationFactor: 1,
			})
			start := leading + cursor
			result.PitchPoints[index] = referenceCentsBetween(speech.AutomaticF0, center, start, start+durations[index])
		}
		cursor += durations[index]
	}
	log(fmt.Sprintf("合成: 合計 %.2f 秒", time.Since(started).Seconds()))
	return result, nil
}

// predictReferenceは、参照TTSの読みだけからモーラ長と抑揚を返す。音源は使わない。
// 手動のモーラ長があれば、その長さで読んだときの抑揚を返す。
func predictReference(cfg Config, language, reading string, morae []frontend.Mora) (*ProsodyPreview, error) {
	if err := plan.ValidateMoraDurations(morae, cfg.MoraDurationsMS); err != nil {
		return nil, err
	}
	referenceReading := reading
	if frontend.NormalizeLanguage(language) == frontend.LanguageChinese {
		referenceReading = chineseSurfaceReading(reading, morae)
	}
	_, provider, pieces, options, err := referenceInputs(cfg, language, referenceReading)
	if err != nil {
		return nil, fmt.Errorf("reference prosody: %w", err)
	}
	reference, err := provider.ReadProsody(pieces, options.Speed, nil)
	if err != nil {
		return nil, fmt.Errorf("reference prosody: %w", err)
	}
	_, plans, sourceDurations, durations, err := referenceTiming(cfg, language, pieces, reference.Timing, reading, morae)
	if err != nil {
		return nil, fmt.Errorf("reference prosody: %w", err)
	}
	f0 := reference.ReferenceF0()
	leading := reference.LeadingMS()
	if hasManualMoraDurations(cfg.MoraDurationsMS) || len(plans) > 0 {
		f0 = refspeech.WarpF0(refspeech.NewWarp(sourceDurations, durations), leading, f0)
	}
	center := referenceCenterHz(f0)
	result := &ProsodyPreview{
		Reading: reading, Morae: append([]frontend.Mora(nil), morae...), MoraDurationsMS: durations,
		MoraPositionsMS: make([]float64, len(morae)), PitchPoints: make([]float64, len(morae)),
		FramePitchCurve: referencePitchCurve(f0, center, leading, options.PitchScale),
	}
	cursor := 0.0
	for index, mora := range morae {
		result.MoraPositionsMS[index] = cursor + durations[index]/2
		if !mora.Pause {
			start := leading + cursor
			result.PitchPoints[index] = options.PitchScale * referenceCentsBetween(f0, center, start, start+durations[index])
		}
		cursor += durations[index]
	}
	return result, nil
}

// englishMoraTokensは、Kokoroの入力記号をモーラへ割り当てる。
// 語の記号は、語の中のモーラへ音素の数の比で順に分け、語の後ろの空白は語の最後のモーラに入れる。
// 休止のモーラには読点を、文末の休止には文末の記号と末尾の境界記号も入れる。
// 文末に休止がなければ、それらは最後のモーラに入れる。先頭の境界記号はどのモーラにも入れない。
func englishMoraTokens(pieces []refspeech.Piece, timing refspeech.Timing, reading string, morae []frontend.Mora) [][]int {
	parts := splitEnglishReading(reading)
	own := map[int][]int{}
	tail := map[int][]int{}
	lastPart := -1
	for i, piece := range pieces {
		tokens := make([]int, 0)
		for token := timing.PieceTokens[i][0]; token < timing.PieceTokens[i][1]; token++ {
			tokens = append(tokens, token)
		}
		if piece.Part >= 0 {
			lastPart = piece.Part
			own[piece.Part] = append(own[piece.Part], tokens...)
		} else if lastPart >= 0 {
			tail[lastPart] = append(tail[lastPart], tokens...)
		}
	}
	if lastPart >= 0 && len(timing.Durations) > 0 {
		tail[lastPart] = append(tail[lastPart], len(timing.Durations)-1)
		if last := len(parts) - 1; last > lastPart && parts[last] == "SP" {
			own[last] = append(own[last], tail[lastPart]...)
			tail[lastPart] = nil
		}
	}
	result := make([][]int, len(morae))
	pause := 0
	var pauseParts []int
	for index, part := range parts {
		if part == "SP" {
			pauseParts = append(pauseParts, index)
		}
	}
	for start := 0; start < len(morae); {
		mora := morae[start]
		if mora.Pause {
			if pause < len(pauseParts) {
				part := pauseParts[pause]
				result[start] = append(append([]int(nil), own[part]...), tail[part]...)
			}
			pause++
			start++
			continue
		}
		end := start
		for end+1 < len(morae) && !morae[end+1].Pause && morae[end+1].WordIndex == mora.WordIndex {
			end++
		}
		tokens := own[mora.WordIndex]
		if assigned := englishWordAssignment(pieces, timing, tokens, morae[start:end+1]); assigned != nil {
			for i, mora := range assigned {
				result[start+i] = mora
			}
			result[end] = append(result[end], tail[mora.WordIndex]...)
			start = end + 1
			continue
		}
		weights := make([]int, end-start+1)
		total := 0
		for i := range weights {
			weights[i] = max(1, len(morae[start+i].Phones))
			total += weights[i]
		}
		cumulative, at := 0, 0
		for i := range weights {
			cumulative += weights[i]
			next := int(math.Round(float64(len(tokens)) * float64(cumulative) / float64(total)))
			result[start+i] = append([]int(nil), tokens[at:next]...)
			at = next
		}
		result[end] = append(result[end], tail[mora.WordIndex]...)
		start = end + 1
	}
	return result
}

// referenceMoraDurationsは、モーラに割り当てた記号の長さを足す。
// 手動の長さがあるモーラはその値にする。
// referenceTimingは、参照の記号の範囲と、参照・出力それぞれのモーラの長さを求める。
func referenceTiming(cfg Config, language string, pieces []refspeech.Piece, timing refspeech.Timing, reading string, morae []frontend.Mora) (moraTokens [][]int, plans []phonePlan, source []float64, target []float64, err error) {
	moraTokens = referenceMoraTokens(language, pieces, timing, reading, morae)
	plans, err = buildPhonePlans(cfg, morae)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	source = referenceMoraDurations(moraTokens, timing, nil)
	target = referencePlannedDurations(moraTokens, timing, cfg.MoraDurationsMS,
		referencePhonePlans(morae, moraTokens, timing, cfg.MoraDurationsMS, plans))
	return moraTokens, plans, source, target, nil
}

func referenceMoraDurations(moraTokens [][]int, timing refspeech.Timing, manual []float64) []float64 {
	durations := make([]float64, len(moraTokens))
	for i, tokens := range moraTokens {
		if value, ok := plan.ConfiguredMoraDuration(i, manual); ok {
			durations[i] = value
			continue
		}
		for _, token := range tokens {
			durations[i] += timing.MS(token)
		}
	}
	return durations
}

// referencePlannedDurationsは、合成後の各モーラの長さを返す。
// 音素の指定があるモーラは、その合計長を使う。
func referencePlannedDurations(moraTokens [][]int, timing refspeech.Timing, manual []float64, plans []moraPhonePlan) []float64 {
	durations := referenceMoraDurations(moraTokens, timing, manual)
	for i := range durations {
		if i < len(plans) && plans[i].total > 0 {
			durations[i] = plans[i].total
		}
	}
	return durations
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

// referencePitchCurveは、最初のモーラの始まりを0とするセントの曲線。無声区間は前後の有声区間を直線でつなぐ。
func referencePitchCurve(f0 []float64, center, leadingMS, scale float64) *render.PitchCurve {
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

// referenceCenterHzは出力のF0の有声区間の中央値。
func referenceCenterHz(f0 []float64) float64 {
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

// referenceCentsBetweenはstartMSからendMSまでの有声区間の平均F0を、centerからのセントで返す。
func referenceCentsBetween(f0 []float64, center, startMS, endMS float64) float64 {
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
