package main

import "math"

func flatBaselineMetrics(items []example, c config) map[string]float64 {
	scale := math.Max(1, math.Max(math.Abs(c.Low), math.Abs(c.High)))
	var raw, rendered float64
	var count int
	for _, item := range items {
		target := make([]float64, len(item.Targets))
		for i, v := range item.Targets {
			target[i] = float64(v) * scale
		}
		reference := renderContour(target, item.Mask, c.Frame, c.RenderStrength, c.RenderSmoothing, c.RenderP99, c.RenderMax, c.Low, c.High)
		for i, valid := range item.Mask {
			if valid {
				raw += math.Abs(target[i])
				rendered += math.Abs(reference[i])
				count++
			}
		}
	}
	if count == 0 {
		return map[string]float64{"raw_mae_cents": 0, "rendered_mae_cents": 0}
	}
	return map[string]float64{"raw_mae_cents": raw / float64(count), "rendered_mae_cents": rendered / float64(count)}
}
