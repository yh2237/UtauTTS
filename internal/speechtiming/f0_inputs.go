package speechtiming

import (
	"fmt"
	"math"
)

// F0InputsはF0ブランチの入力をタイムラインの音素区間とフレームごとの追加特徴から組む。
// extras[t]の幅はF0Context()-2（アクセント・POS等）。無いフレームはゼロ。
func (m *TCN) F0Inputs(spans []Span, extras [][]float32, frames int) ([][3]int, [][]float32, error) {
	if !m.hasF0 {
		return nil, nil, fmt.Errorf("speech timing model: no F0 head")
	}
	extra := m.f0Context - 2
	if extra < 1 {
		return nil, nil, fmt.Errorf("speech timing model: unsupported f0 context %d", m.f0Context)
	}
	index := make(map[string]int, len(m.phones))
	for i, name := range m.phones {
		index[name] = i
	}
	id := func(label string) int {
		if value, ok := index[label]; ok {
			return value
		}
		return index["<unk>"]
	}
	silence := id("sil")
	ids := make([][3]int, frames)
	cont := make([][]float32, frames)
	for t := range ids {
		ids[t] = [3]int{silence, silence, silence}
		cont[t] = make([]float32, m.f0Context)
	}
	for i, span := range spans {
		a := int(math.Round(span.Start * 1000 / FrameMS))
		b := int(math.Round(span.End * 1000 / FrameMS))
		a = max(0, a)
		b = min(frames, max(b, a+1))
		if a >= frames {
			break
		}
		previous, next := "sil", "sil"
		if i > 0 {
			previous = spans[i-1].Label
		}
		if i+1 < len(spans) {
			next = spans[i+1].Label
		}
		duration := float32(math.Log((span.End-span.Start)*1000+1) / 6)
		for t := a; t < b; t++ {
			ids[t] = [3]int{id(span.Label), id(previous), id(next)}
			cont[t][0] = (float32(t-a) + 0.5) / float32(max(1, b-a))
			cont[t][1] = duration
			if t < len(extras) && len(extras[t]) == extra {
				copy(cont[t][2:], extras[t])
			}
		}
	}
	return ids, cont, nil
}
