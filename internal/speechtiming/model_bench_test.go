package speechtiming

import "testing"

func BenchmarkDilatedConv(b *testing.B) {
	const frames, hidden, kernel = 320, 128, 5
	m := &TCN{hidden: hidden}
	input, output := make([]float32, frames*hidden), make([]float32, frames*hidden)
	weights := tensor{data: make([]float32, kernel*hidden*hidden)}
	bias := tensor{data: make([]float32, hidden)}
	for i := range input {
		input[i] = float32(i%17) / 17
	}
	for i := range weights.data {
		weights.data[i] = float32(i%13-6) / 100
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.dilatedConv(input, output, frames, weights, bias, kernel, 8)
	}
}
