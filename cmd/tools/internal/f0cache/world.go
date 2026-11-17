package f0cache

import "github.com/yh2237/UtauWLD/worldrender"

func WorldF0(audioPath string, step float64, enginePath string) ([]float64, error) {
	return worldrender.AnalyzeFileF0(enginePath, audioPath, step)
}
