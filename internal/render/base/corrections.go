package base

import "strings"

// Correctionは言語別の補正。適用言語と設定キーを1箇所で宣言する。
type Correction struct {
	ID        string
	Languages []string // 空は全言語。
	Setting   string   // renderer設定キー。空は設定なし。
	Stage     string   // plan, pitch, frontend, worldline, bridge
}

// Correctionsは補正の一覧。言語ゲートと設定キーの対応をここで持つ。
var Corrections = []Correction{
	{ID: "timing_warp", Setting: "timing_warp", Stage: "worldline+bridge"},
	{ID: "microprosody", Languages: []string{"ja"}, Setting: "microprosody", Stage: "worldline"},
	{ID: "boundary_tone", Languages: []string{"ja"}, Setting: "boundary_tone", Stage: "pitch"},
	{ID: "stretch_adapt", Languages: []string{"ja"}, Setting: "stretch_adapt", Stage: "plan"},
	{ID: "context_duration", Languages: []string{"ja"}, Setting: "context_duration", Stage: "plan"},
	{ID: "pause_context", Setting: "pause_context", Stage: "plan"},
	{ID: "english_weak_form", Languages: []string{"en"}, Setting: "english_weak_form", Stage: "frontend"},
	{ID: "coda_release", Languages: []string{"en", "zh"}, Stage: "worldline"},
	{ID: "japanese_stop_gate", Languages: []string{"ja"}, Stage: "worldline"},
	{ID: "source_phone_library", Languages: []string{"en", "zh"}, Stage: "worldline"},
	{ID: "single_cv_legato", Stage: "worldline"},
	{ID: "cvvc_transition", Stage: "plan"},
}

// CorrectionAppliesToは補正が言語へ適用されるかを返す。空言語は日本語として扱う。
func CorrectionAppliesTo(id, language string) bool {
	normalized := NormalizeLanguage(language)
	for _, correction := range Corrections {
		if correction.ID != id {
			continue
		}
		if len(correction.Languages) == 0 {
			return true
		}
		for _, supported := range correction.Languages {
			if supported == normalized {
				return true
			}
		}
		return false
	}
	return false
}

// NormalizeLanguageは言語コードをja/en/zhへ揃える。空は日本語。
func NormalizeLanguage(language string) string {
	value := strings.ToLower(strings.TrimSpace(language))
	if value == "" {
		return "ja"
	}
	if index := strings.IndexAny(value, "-_"); index > 0 {
		value = value[:index]
	}
	if value == "cmn" {
		return "zh"
	}
	return value
}

// JapanesePlanは言語または音素化器が日本語かを返す。
func JapanesePlan(language, phonemizer string) bool {
	if NormalizeLanguage(language) == "ja" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(phonemizer)), "ja")
}
