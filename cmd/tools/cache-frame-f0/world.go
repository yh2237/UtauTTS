package main

import "github.com/yh2237/UtauWLD/worldrender"

func worldF0(r record, step float64, path string) ([]float64, error) {
	return worldrender.AnalyzeFileF0(path, r.AudioPath, step)
}
