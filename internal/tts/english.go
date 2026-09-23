package tts

import (
	"fmt"
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/voicebank"
)

// englishProfileは英語のphonemizer呼び分けと規則ベースの抑揚をまとめる。
type englishProfile struct{}

func (englishProfile) Language() string { return frontend.LanguageEnglish }

func (englishProfile) ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	switch phonemizer {
	case frontend.PhonemizerEnglish:
		return frontend.ParseEnglishARPAsingWithOptions(cfg.Text, cfg.Reading, cfg.Dictionary, englishOptions(cfg))
	case frontend.PhonemizerEnglishDelta:
		var presamp frontend.PresampConfig
		if cfg.Voicebank != nil {
			presamp = cfg.Voicebank.Presamp.FrontendConfig()
		}
		return frontend.ParseEnglishDeltaWithOptions(cfg.Text, cfg.Reading, cfg.Dictionary, presamp, englishOptions(cfg))
	case frontend.PhonemizerEnglishVCCV:
		return frontend.ParseEnglishVCCVWithOptions(cfg.Text, cfg.Reading, cfg.Dictionary, englishOptions(cfg))
	case frontend.PhonemizerEnglishCV:
		return frontend.ParseEnglishCVWithOptions(cfg.Text, cfg.Reading, cfg.Dictionary, englishOptions(cfg))
	default:
		return "", nil, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, frontend.LanguageEnglish)
	}
}

func (englishProfile) ApplySpeechProfile(cfg *Config) {
	if cfg == nil {
		return
	}
	// 英語の語境界では遷移音を少し強める
	if cfg.AliasPolicy == voicebank.AliasPolicyCVVCPrefer && cfg.CVVCTransitionGain == 0.35 {
		cfg.CVVCTransitionGain = 0.55
	}
}

func (englishProfile) ProsodyModelFallback(configuredPath string) string {
	return englishFallbackProsodyModelPath(configuredPath)
}

func (englishProfile) SupportsStretchAdapt() bool { return false }

func (englishProfile) PhoneTiming(_ Config, morae []frontend.Mora, _ bool) ([][]float64, string) {
	return languagePhoneWeights(frontend.LanguageEnglish, morae), "language-phone-v1"
}

func (englishProfile) Predict(morae []frontend.Mora) []prosody.Prediction {
	return englishPredictions(morae)
}

func (englishProfile) AdjustPredictions(_ Config, _ *prosody.Model, _ []frontend.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return predictions
}

func (englishProfile) AutomaticPitchCurve(cfg Config, model *prosody.Model, morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) (*render.PitchCurve, bool) {
	if !applyPitchEnabled(cfg) || shouldPredictFrameContour(cfg, model) {
		return nil, false
	}
	return scaleAutomaticPitchCurve(englishSpeechCurve(morae, timings, durationMS, cfg.Text), cfg.IntonationStrength), false
}

func (englishProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}

func (englishProfile) ExperimentalPitchAllowed() bool { return false }

// englishOptionsは英語phonemizerの前処理オプションを組み立てる。
// E1の弱形は未指定(nil)で既定ON。
func englishOptions(cfg Config) frontend.EnglishOptions {
	return frontend.EnglishOptions{WeakForms: cfg.EnglishWeakForm == nil || *cfg.EnglishWeakForm}
}

// 日本語アクセントモデルを使えない英語向けの保守的なフォールバック。
// 辞書に強勢があればそれを使う。学習済みモデルではなく規則ベースの基準実装。
func englishPredictions(morae []frontend.Mora) []prosody.Prediction {
	result := make([]prosody.Prediction, len(morae))
	for i, mora := range morae {
		factor := 1.0
		energy := 1.0
		if mora.Vowel != "" && !mora.Pause {
			switch mora.Stress {
			case 1:
				factor = 1.2
				energy = 1.06
			case 2:
				factor = 1.1
				energy = 1.03
			case 0:
				if mora.StressKnown {
					factor = 0.85
					energy = 0.92
				}
			}
			// 音節単位の音素化では末子音の時間を確保する。
			if mora.DurationScale == 0 && mora.Aliases != nil {
				factor += math.Min(0.5, float64(len(mora.Aliases.Endings))*0.15)
			}
		}
		if !mora.Pause && (i+1 == len(morae) || morae[i+1].Pause) {
			factor *= 1.12
		}
		result[i] = prosody.Prediction{DurationFactor: factor, EnergyFactor: energy, PitchFactor: 1}
	}
	return result
}

func englishSpeechCurve(morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64, text string) *render.PitchCurve {
	if len(morae) == 0 || len(morae) != len(timings) || durationMS <= 0 {
		return nil
	}
	curve := &render.PitchCurve{FrameMS: 10, Cents: make([]float64, int(math.Ceil(durationMS/10))+1)}
	for start := 0; start < len(morae); {
		if morae[start].Pause {
			start++
			continue
		}
		end := start
		for end+1 < len(morae) && !morae[end+1].Pause {
			end++
		}
		left := timings[start].StartMS
		right := timings[end].StartMS + timings[end].DurationMS
		for i := start; i <= end; i++ {
			timing := timings[i]
			for frame := max(0, int(math.Ceil(timing.StartMS/10))); frame < len(curve.Cents) && float64(frame)*10 <= timing.StartMS+timing.DurationMS; frame++ {
				t := float64(frame) * 10
				phase := (t - left) / math.Max(1, right-left)
				cents := 55 - 110*phase
				if morae[i].Vowel != "" && morae[i].Stress > 0 {
					local := (t - timing.StartMS) / math.Max(1, timing.DurationMS)
					cents += englishStressAccent(morae[i].Stress, local)
				} else if morae[i].Vowel != "" && morae[i].StressKnown && morae[i].Stress == 0 {
					local := (t - timing.StartMS) / math.Max(1, timing.DurationMS)
					cents -= 18 * math.Sin(math.Pi*math.Max(0, math.Min(1, local)))
				}
				if strings.HasSuffix(strings.TrimSpace(text), "?") && end >= len(morae)-2 && phase > 0.7 {
					cents += 140 * (phase - 0.7) / 0.3
				}
				curve.Cents[frame] = cents
			}
		}
		start = end + 1
	}
	return render.ConstrainPitchCurve(curve, 16, 7)
}

func englishStressAccent(stress int, local float64) float64 {
	if stress <= 0 {
		return 0
	}
	local = math.Max(0, math.Min(1, local))
	strength := 62.0 / float64(stress)
	// 強勢母音の前に小さな下降を置き上昇下降の輪郭にする
	dip := 0.0
	if local < 0.28 {
		dip = -12 * (1 - local/0.28) / float64(stress)
	}
	return dip + strength*math.Sin(math.Pi*local)
}
