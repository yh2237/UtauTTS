package reference

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/yh2237/UtauTTS/internal/tts"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/yh2237/UtauIDX/speechpool"
	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/refspeech"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"github.com/yh2237/audiodsp/wav"
)

var englishLexicons sync.Map

// ReferenceModelsは参照照合に使うモデルの場所を返す。
func ReferenceModels(dir string) (refspeech.Models, string, string) {
	shared := map[string]string{}
	have := referenceFiles(dir)
	for id, name := range sharedReferenceVoices {
		if have(name) {
			shared[id] = filepath.Join(dir, name)
		}
	}
	models := refspeech.Models{
		SharedVoices:  shared,
		KokoroWeights: filepath.Join(dir, "kokoro-v1_0.pth"),
		KokoroConfig:  filepath.Join(dir, "config.json"),
		KokoroVoice:   filepath.Join(dir, "af_heart.pt"),
		ZhWeights:     filepath.Join(dir, "kokoro-zh", "kokoro-v1_1-zh.pth"),
		ZhConfig:      filepath.Join(dir, "kokoro-zh", "config.json"),
		ZhVoice:       filepath.Join(dir, "kokoro-zh", "zf_001.pt"),
		WavLM:         filepath.Join(dir, "wavlm", "wavlm-base-plus.safetensors"),
	}
	models.Espeak = firstReferenceFile(dir, "espeak/model.safetensors", "espeak/wav2vec2-espeak.safetensors")
	models.EspeakConfig = firstReferenceFile(dir, "espeak/config.json")
	models.EspeakPreprocessor = firstReferenceFile(dir, "espeak/preprocessor_config.json")
	models.Whisper = firstReferenceFile(dir, "whisper/model.safetensors", "whisper/whisper-small.safetensors")
	models.WhisperConfig = firstReferenceFile(dir, "whisper/config.json")
	if data, err := os.ReadFile(filepath.Join(dir, "inference.json")); err == nil {
		var manifest struct {
			Encoders []string `json:"encoders"`
		}
		if json.Unmarshal(data, &manifest) == nil {
			models.ExternalEncoders = manifest.Encoders
		}
	}
	return models, filepath.Join(dir, "us_gold.json"), filepath.Join(dir, "us_silver.json")
}

func firstReferenceFile(dir string, names ...string) string {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
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
		service.Prepare(bank.Root, "", referenceRecordings(bank, "", ""), worldEnginePath)
	}
}

// PrepareReferenceIndexStatusは音高と声色に合う索引を裏で準備し、作成中かどうかを返す。
func PrepareReferenceIndexStatus(dir string, bank *voicebank.Bank, tone, color, world string) (bool, error) {
	service, err := ReferenceService(dir)
	if err != nil {
		return false, err
	}
	status := service.PrepareStatus(bank.Root, color, referenceRecordings(bank, tone, color), world)
	return status.State == refspeech.IndexBuilding, nil
}

// referenceRecordingsは索引に使う録音と、それを使う原音のoto.iniの範囲を返す。
func referenceRecordings(bank *voicebank.Bank, tone, color string) []speechpool.Entry {
	recordings := bank.ReferenceRecordings(tone, color)
	result := make([]speechpool.Entry, len(recordings))
	for i, recording := range recordings {
		result[i].Path = recording.Path
		result[i].Layer = speechpool.LayerRaw
		for _, entry := range recording.Entries {
			result[i].Regions = append(result[i].Regions, speechpool.Region{Offset: entry.Offset, Blank: entry.Blank})
		}
	}
	return result
}

// ReferenceIndexProgressは索引の作成の進み具合を返す。
func ReferenceIndexProgress() refspeech.Progress {
	return refspeech.CurrentProgress()
}

// ReferenceIndexesは保存済みの索引を返す。
func ReferenceIndexes() ([]refspeech.IndexInfo, error) {
	dir, err := refspeech.DefaultCacheDir()
	if err != nil {
		return nil, err
	}
	return refspeech.ListIndexes(dir)
}

// DeleteReferenceIndexは保存済みの索引を名前で消す。
func DeleteReferenceIndex(name string) (bool, error) {
	dir, err := refspeech.DefaultCacheDir()
	if err != nil {
		return false, err
	}
	return refspeech.DeleteIndex(dir, name)
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
func referenceInputs(cfg tts.Config, utterance linguistics.Utterance) (*refspeech.Service, refspeech.Provider, []refspeech.Piece, refspeech.Options, error) {
	if !refspeech.SupportsProviderLanguage(utterance.Language) {
		return nil, nil, nil, refspeech.Options{}, fmt.Errorf("reference synthesis is not available for %q", utterance.Language)
	}
	service, err := ReferenceService(cfg.ReferenceDir)
	if err != nil {
		return nil, nil, nil, refspeech.Options{}, err
	}
	provider, err := refspeech.NewProvider(utterance.Language, refspeech.ProviderConfig{
		Service:     service,
		LoadLexicon: func() (*refspeech.Lexicon, error) { return englishLexicon(cfg.ReferenceDir) },
		Dictionary:  cfg.Dictionary,
	})
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
	pieces, err := provider.Pieces(utterance)
	if err != nil {
		return nil, nil, nil, refspeech.Options{}, err
	}
	return service, provider, pieces, options, nil
}

// referenceManualPitchは手動の抑揚を、最初のモーラの始まりからのミリ秒ごとのセントにする。
func referenceManualPitch(cfg tts.Config, reading string, morae []linguistics.Mora, durations []float64) (func(float64) float64, bool, error) {
	timings := make([]prosody.MoraTiming, len(morae))
	cursor := 0.0
	for i, d := range durations {
		timings[i] = prosody.MoraTiming{StartMS: cursor, DurationMS: d}
		cursor += d
	}
	contour, mode, err := tts.ResolveManualPitchCurve(cfg, reading, morae, timings, cursor)
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
func synthesizeReference(cfg tts.Config, bank *voicebank.Bank, utterance linguistics.Utterance) (*tts.Result, error) {
	morae := utterance.Morae
	language := utterance.Language
	phonemizerID := utterance.Phonemizer
	reading := utterance.Reading
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
	if surface := utterance.EffectiveReading(); surface != reading {
		log(fmt.Sprintf("参照の読み: %s", surface))
	}
	service, provider, pieces, options, err := referenceInputs(cfg, utterance)
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
	_, plans, sourceDurations, planned, err := referenceTiming(cfg, provider, pieces, reference.Timing, utterance)
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
	speech, err := service.SpeakReference(ctx, bank.Root, cfg.Color, referenceRecordings(bank, cfg.Tone, cfg.Color), cfg.WorldEnginePath, reference, options)
	refspeech.SetStage("")
	if err != nil {
		return nil, fmt.Errorf("reference synthesis: %w", err)
	}
	durations := planned
	leading := speech.LeadingMS()
	pcm := referenceAudio(speech, durations, unitGain)
	synthesisPlan := &plan.Plan{
		Version: plan.Version, Voicebank: bank.Root, Text: cfg.Text, Reading: reading,
		Language: language, Phonemizer: phonemizerID, Tone: cfg.Tone, Color: cfg.Color,
		SelectionMode: "reference", AliasPolicy: "reference", JoinCostMode: "none",
		LeadingMarginMS: leading, Morae: append([]linguistics.Mora(nil), morae...),
		DurationMS: float64(len(speech.Audio)) * 1000 / float64(speech.SampleRate),
	}
	result := &tts.Result{Voicebank: bank, Plan: synthesisPlan, Audio: pcm,
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
func predictReference(cfg tts.Config, utterance linguistics.Utterance) (*tts.ProsodyPreview, error) {
	morae := utterance.Morae
	reading := utterance.Reading
	if err := plan.ValidateMoraDurations(morae, cfg.MoraDurationsMS); err != nil {
		return nil, err
	}
	_, provider, pieces, options, err := referenceInputs(cfg, utterance)
	if err != nil {
		return nil, fmt.Errorf("reference prosody: %w", err)
	}
	reference, err := provider.ReadProsody(pieces, options.Speed, nil)
	if err != nil {
		return nil, fmt.Errorf("reference prosody: %w", err)
	}
	_, plans, sourceDurations, durations, err := referenceTiming(cfg, provider, pieces, reference.Timing, utterance)
	if err != nil {
		return nil, fmt.Errorf("reference prosody: %w", err)
	}
	f0 := reference.ReferenceF0()
	leading := reference.LeadingMS()
	if hasManualMoraDurations(cfg.MoraDurationsMS) || len(plans) > 0 {
		f0 = refspeech.WarpF0(refspeech.NewWarp(sourceDurations, durations), leading, f0)
	}
	center := referenceCenterHz(f0)
	result := &tts.ProsodyPreview{
		Reading: reading, Morae: append([]linguistics.Mora(nil), morae...), MoraDurationsMS: durations,
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

// referenceTimingは、参照の記号の範囲と、参照・出力それぞれのモーラの長さを求める。
func referenceTiming(cfg tts.Config, provider refspeech.Provider, pieces []refspeech.Piece, timing refspeech.Timing, utterance linguistics.Utterance) (moraTokens [][]int, plans []phonePlan, source []float64, target []float64, err error) {
	morae := utterance.Morae
	moraTokens = provider.AlignMoraTokens(pieces, timing, utterance)
	plans, err = buildPhonePlans(cfg, morae)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	source = referenceMoraDurations(moraTokens, timing, nil)
	target = referencePlannedDurations(morae, moraTokens, timing, cfg.MoraDurationsMS,
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
// 参照TTSの休止は短いことがあるので、手で指定されていない休止には下限を設ける。
func referencePlannedDurations(morae []linguistics.Mora, moraTokens [][]int, timing refspeech.Timing, manual []float64, plans []moraPhonePlan) []float64 {
	durations := referenceMoraDurations(moraTokens, timing, manual)
	for i := range durations {
		if i < len(plans) && plans[i].total > 0 {
			durations[i] = plans[i].total
		}
		if i < len(morae) && morae[i].Pause {
			if _, ok := configuredMoraDuration(i, manual); !ok {
				durations[i] = math.Max(durations[i], referencePauseFloor(morae[i].PauseKind))
			}
		}
	}
	return durations
}

// referencePauseFloorは、休止の種類ごとの下限の長さを返す。
func referencePauseFloor(kind string) float64 {
	switch kind {
	case linguistics.PauseKindComma:
		return 150
	case linguistics.PauseKindPeriod, linguistics.PauseKindQuestion:
		return 250
	case linguistics.PauseKindEllipsis:
		return 350
	}
	return 150
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

// referenceAudioは、合成した波形にモーラごとの音量を掛け、16ビットへ直す。
func referenceAudio(speech refspeech.Speech, durations []float64, unitGain map[int]float64) *wav.PCM {
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
	pcm := &wav.PCM{SampleRate: speech.SampleRate, Channels: 1, Data: make([]int16, len(speech.Audio))}
	for i, v := range speech.Audio {
		pcm.Data[i] = int16(math.Round(math.Max(-1, math.Min(1, v*scale)) * 32767))
	}
	return pcm
}

// methodは、参照TTSの読み上げに近いフレームを音源から選んで合成する方式。
type method struct{}

func init() { tts.RegisterMethod(tts.MethodReferenceMatching, method{}) }

func (method) Synthesize(cfg tts.Config, utterance linguistics.Utterance) (*tts.Result, error) {
	return synthesizeReference(cfg, cfg.Voicebank, utterance)
}

func (method) PredictProsody(cfg tts.Config, utterance linguistics.Utterance) (*tts.ProsodyPreview, error) {
	return predictReference(cfg, utterance)
}
