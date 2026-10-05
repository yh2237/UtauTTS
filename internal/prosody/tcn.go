package prosody

import "math"

// tcnForwardはdilation付き残差TCNを順伝播する。sameGroupが非nilなら参照元を同じgroupに限定する。
func tcnForward(state [][]float64, layers []SequencePitchLayer, sameGroup func(source, position int) bool) [][]float64 {
	if len(state) == 0 {
		return state
	}
	for _, layer := range layers {
		hidden := len(layer.Bias)
		next := make([][]float64, len(state))
		for position := range state {
			next[position] = make([]float64, hidden)
			for output := 0; output < hidden; output++ {
				value := state[position][output] + layer.Bias[output]
				for input := 0; input < hidden; input++ {
					for kernel := 0; kernel < 3; kernel++ {
						source := position + (kernel-1)*layer.Dilation
						if source < 0 || source >= len(state) {
							continue
						}
						if sameGroup != nil && !sameGroup(source, position) {
							continue
						}
						value += layer.Weights[output][input][kernel] * state[source][input]
					}
				}
				next[position][output] = math.Tanh(value)
			}
		}
		state = next
	}
	return state
}
