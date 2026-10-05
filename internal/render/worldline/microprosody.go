package worldline

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

// 母音開始を基準としたテンプレートの始点(ms)。点は10ms間隔。
const microprosodyStartMS = -20.0

// つくよみちゃんコーパスとみんなで作るJSUT BASIC5000_0001-0600から測定したF0補正(cent)。
// MFA整列・WORLD 5ms、150ms移動中央値からの残差を平均し、母音連続の平均を引いた値。
// WORLDで失われる子音前後の微細な音高変化を補う。
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

// curveは計画時刻curveStartMSからframeMS間隔のF0(Hz)。
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
