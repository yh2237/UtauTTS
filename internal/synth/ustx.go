package synth

import (
	"context"
	"math"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/openutau"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/tts"
)

// USTXFromSynthesisは、合成した1発話からUSTXを作る。
func USTXFromSynthesis(resolved ResolvedRequest, result *Result, voicebankID string) ([]byte, error) {
	project := projectFromPlan(resolved.Config, result.Plan, voicebankID)
	return openutau.ExportUSTX(project, openutau.ExportOptions{Curves: synthesisCurves(resolved.Config)})
}

// ExportUSTXは、GUIのプロジェクトからUSTXを作る。読みと抑揚の曲線がない発話は予測して補う。
func (s *Service) ExportUSTX(ctx context.Context, project *openutau.UtauTTSProject) ([]byte, error) {
	return openutau.ExportUSTX(project, openutau.ExportOptions{Curves: s.completeProject(ctx, project)})
}

// completeProjectは、読みのない発話に解析結果を入れ、抑揚の曲線を返す。
// 曲線を作れない発話はnilのままにし、書き出し側でモーラごとの値を使う。
func (s *Service) completeProject(ctx context.Context, project *openutau.UtauTTSProject) []openutau.FrameCurve {
	curves := make([]openutau.FrameCurve, len(project.Utterances))
	for index := range project.Utterances {
		utterance := &project.Utterances[index]
		if utterance.Text == "" && utterance.AnalysisCache.Reading == "" {
			continue
		}
		needsAnalysis := len(utterance.AnalysisCache.Morae) == 0
		needsCurve := utterance.ModelID != "" && len(utterance.AutomaticFramePitch) == 0
		if !needsAnalysis && !needsCurve {
			continue
		}
		strength := utterance.Intonation
		if strength <= 0 {
			strength = 1
		}
		language := utterance.Language
		if language == "" {
			language = linguistics.LanguageJapanese
		}
		// 曲線は波形合成の抑揚モデルで作るため、発話の方式によらず波形合成で予測する。
		preview, err := s.PredictProsodyContext(ctx, Request{
			Text:               utterance.Text,
			Kana:               utterance.AnalysisCache.Reading,
			Language:           language,
			Phonemizer:         utterance.Phonemizer,
			Method:             MethodWaveformSynthesis,
			ModelID:            utterance.ModelID,
			MoraDurationMS:     utterance.MoraDurationMS,
			PauseDurationMS:    utterance.PauseDurationMS,
			MoraDurationsMS:    utterance.MoraDurationsMS,
			IntonationStrength: strength,
			ApplyPitch:         true,
		})
		if err != nil || preview == nil {
			continue
		}
		if needsAnalysis {
			utterance.AnalysisCache.Reading = preview.Reading
			utterance.AnalysisCache.Morae = projectMorae(preview.Morae)
		}
		if preview.FramePitchCurve != nil {
			curves[index] = openutau.FrameCurve{FrameMS: preview.FramePitchCurve.FrameMS, Cents: preview.FramePitchCurve.Cents}
		}
	}
	return curves
}

func synthesisCurves(cfg tts.Config) []openutau.FrameCurve {
	curves := make([]openutau.FrameCurve, 1)
	if cfg.ProsodyModelPath == "" {
		return curves
	}
	cfg.ApplyPitch = true
	if cfg.IntonationStrength <= 0 {
		cfg.IntonationStrength = 1
	}
	preview, err := tts.PredictProsody(cfg)
	if err != nil || preview == nil || preview.FramePitchCurve == nil {
		return curves
	}
	curves[0] = openutau.FrameCurve{FrameMS: preview.FramePitchCurve.FrameMS, Cents: preview.FramePitchCurve.Cents}
	return curves
}

func projectFromPlan(cfg tts.Config, p *plan.Plan, voicebankID string) *openutau.UtauTTSProject {
	utterance := openutau.UtauTTSUtterance{
		Text:            cfg.Text,
		VoicebankID:     voicebankID,
		Tone:            p.Tone,
		MoraDurationMS:  cfg.MoraDurationMS,
		PauseDurationMS: cfg.PauseDurationMS,
		ApplyPitch:      cfg.ApplyPitch,
		Intonation:      cfg.IntonationStrength,
		AnalysisCache: openutau.UtauTTSAnalysisCache{
			Reading: p.Reading,
			Morae:   planMorae(p),
		},
	}
	count := len(utterance.AnalysisCache.Morae)
	// フレームの曲線がないときに使う、モーラごとの音高。
	cents := make([]float64, count)
	durationsMS := make([]float64, count)
	positionsMS := make([]float64, count)
	for _, unit := range p.Units {
		if unit.Silent || unit.Role == "transition" || unit.Position < 0 || unit.Position >= count {
			continue
		}
		factor := unit.PitchFactor
		if factor <= 0 {
			factor = 1
		}
		cents[unit.Position] = 1200 * math.Log2(factor)
		positionsMS[unit.Position] = unit.NoteStartMS
		if unit.DurationMS > 0 {
			durationsMS[unit.Position] = unit.DurationMS
		}
	}
	utterance.AutomaticPitchPoints = cents
	utterance.AutomaticMoraDurMS = durationsMS
	utterance.AutomaticMoraPosMS = positionsMS
	return &openutau.UtauTTSProject{
		Format:        "utautts-project",
		FormatVersion: 5,
		Utterances:    []openutau.UtauTTSUtterance{utterance},
	}
}

// planMoraeは、長音の母音を保つため計画と同じ方法で読みを解析し直す。
func planMorae(p *plan.Plan) []openutau.UtauTTSMora {
	if morae, err := linguistics.ParseKana(p.Reading); err == nil && len(morae) > 0 {
		return projectMorae(morae)
	}
	// 解析できなければ計画のユニットから戻す。母音は分からない。
	var fallback []openutau.UtauTTSMora
	seen := make(map[int]bool)
	for _, unit := range p.Units {
		if seen[unit.Position] {
			continue
		}
		seen[unit.Position] = true
		if unit.Silent {
			fallback = append(fallback, openutau.UtauTTSMora{Position: unit.Position, Pause: true})
			continue
		}
		fallback = append(fallback, openutau.UtauTTSMora{Position: unit.Position, Mora: unit.Mora})
	}
	return fallback
}

func projectMorae(morae []linguistics.Mora) []openutau.UtauTTSMora {
	result := make([]openutau.UtauTTSMora, len(morae))
	for index, mora := range morae {
		result[index] = openutau.UtauTTSMora{
			Position: index, Mora: mora.Text, Pause: mora.Pause,
			Consonant: mora.Consonant, Vowel: mora.Vowel,
		}
	}
	return result
}
