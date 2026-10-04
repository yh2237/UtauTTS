package worldline

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

// microprosodyStartMSは、微細韻律のテンプレートの最初の点の母音の開始からの時刻。点は10ms間隔。
const microprosodyStartMS = -20.0

// microprosodyTemplatesは子音の種類ごとの、母音の開始の前後のF0の小さな上下（セント）。
// つくよみちゃんコーパスとみんなで作るJSUT BASIC5000_0001-0600（MFA整列、WORLD 5ms）で、
// 150msの移動中央値からの残差を子音の種類ごとに平均し、母音→母音の平均を引いた値（−20〜+40ms）。
// WORLDは目標のF0曲線で置き換えるので、元の録音にあったこの動きは失われる。それを曲線へ戻す。
var microprosodyTemplates = map[string][]float64{
	"voiceless_stop":      {-7, 52, 62, 37, 12, -1, -8},
	"voiceless_fricative": {-28, -1, 21, 21, 3, -11, -10},
	"voiced_stop":         {31, 50, 25, 1, -8, -10, -8},
	"voiced_fricative":    {13, 32, 36, 19, 3, -1, -4},
	"nasal":               {-7, 2, 2, -2, -4, -6, -8},
	"liquid_glide":        {2, 13, 7, 0, -5, -5, -3},
}

func microprosodyClass(consonant string) string {
	switch consonant {
	case "k", "t", "p", "ky", "py", "ty":
		return "voiceless_stop"
	case "s", "sh", "h", "f", "hy", "ch", "ts":
		return "voiceless_fricative"
	case "g", "d", "b", "gy", "by", "dy":
		return "voiced_stop"
	case "z", "j":
		return "voiced_fricative"
	case "m", "n", "my", "ny":
		return "nasal"
	case "r", "ry", "y", "w", "v":
		return "liquid_glide"
	}
	return ""
}

// applyMicroprosodyは日本語のモーラの母音の開始（ノートの開始）の前後へ、子音の種類ごとのテンプレートを掛ける。
// curveは合成計画の時刻curveStartMSからframeMS間隔のF0（Hz）。
func applyMicroprosody(synthesisPlan *plan.Plan, curve []float64, curveStartMS, frameMS float64) {
	if synthesisPlan == nil || frameMS <= 0 {
		return
	}
	for _, unit := range synthesisPlan.Units {
		if unit.Role != "mora" || unit.Silent {
			continue
		}
		parsed, err := frontend.ParseKana(unit.Mora)
		if err != nil || len(parsed) == 0 {
			continue
		}
		template := microprosodyTemplates[microprosodyClass(parsed[0].Consonant)]
		for index, cents := range template {
			timeMS := unit.NoteStartMS + microprosodyStartMS + float64(index)*10
			frame := int(math.Round((timeMS - curveStartMS) / frameMS))
			if frame >= 0 && frame < len(curve) && curve[frame] > 0 {
				curve[frame] *= math.Pow(2, cents/1200)
			}
		}
	}
}
