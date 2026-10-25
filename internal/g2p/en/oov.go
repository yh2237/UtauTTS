package en

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"sync"
)

// englishG2PWeightsはg2p_enのOOVモデル（GRU encoder-decoder）の重み。
// Apache License 2.0のg2p-en-plus（Kyubyong Park & Jongseok Kim）のcheckpoint20.npzを
// 固定の順序でfloat32のまま並べたもの。学習済みの重みだけを同梱し、Pythonのコードは含まない。
//
//go:embed english-g2p.bin
var englishG2PBinary []byte

const (
	englishG2PHidden    = 256
	englishG2PGates     = 3 * englishG2PHidden
	englishG2PGraphemes = 29
	englishG2PPhonemes  = 74
	englishG2PMaxSteps  = 20
)

var englishG2PVocabulary = []string{
	"<pad>", "<unk>", "</s>",
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
	"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
}

var englishG2PPhonemeVocabulary = []string{
	"<pad>", "<unk>", "<s>", "</s>",
	"AA0", "AA1", "AA2", "AE0", "AE1", "AE2", "AH0", "AH1", "AH2",
	"AO0", "AO1", "AO2", "AW0", "AW1", "AW2", "AY0", "AY1", "AY2",
	"B", "CH", "D", "DH", "EH0", "EH1", "EH2", "ER0", "ER1", "ER2",
	"EY0", "EY1", "EY2", "F", "G", "HH", "IH0", "IH1", "IH2",
	"IY0", "IY1", "IY2", "JH", "K", "L", "M", "N", "NG",
	"OW0", "OW1", "OW2", "OY0", "OY1", "OY2", "P", "R", "S", "SH",
	"T", "TH", "UH0", "UH1", "UH2", "UW", "UW0", "UW1", "UW2",
	"V", "W", "Y", "Z", "ZH",
}

type englishG2PWeights struct {
	encEmb []float32
	encWIH []float32
	encWHH []float32
	encBIH []float32
	encBHH []float32
	decEmb []float32
	decWIH []float32
	decWHH []float32
	decBIH []float32
	decBHH []float32
	fcW    []float32
	fcB    []float32
}

var (
	englishG2POnce    sync.Once
	englishG2PModel   *englishG2PWeights
	englishG2PLoadErr error
)

func loadEnglishG2P() (*englishG2PWeights, error) {
	englishG2POnce.Do(func() {
		englishG2PModel, englishG2PLoadErr = parseEnglishG2P(englishG2PBinary)
	})
	return englishG2PModel, englishG2PLoadErr
}

func parseEnglishG2P(data []byte) (*englishG2PWeights, error) {
	const header = 8
	if len(data) < header || string(data[:4]) != "UG2P" {
		return nil, fmt.Errorf("english g2p: invalid weights header")
	}
	if version := binary.LittleEndian.Uint32(data[4:8]); version != 1 {
		return nil, fmt.Errorf("english g2p: unsupported weights version %d", version)
	}
	offset := header
	take := func(count int) ([]float32, error) {
		size := count * 4
		if offset+size > len(data) {
			return nil, fmt.Errorf("english g2p: truncated weights")
		}
		values := make([]float32, count)
		for i := 0; i < count; i++ {
			values[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[offset+i*4:]))
		}
		offset += size
		return values, nil
	}
	weights := &englishG2PWeights{}
	var err error
	if weights.encEmb, err = take(englishG2PGraphemes * englishG2PHidden); err != nil {
		return nil, err
	}
	if weights.encWIH, err = take(englishG2PGates * englishG2PHidden); err != nil {
		return nil, err
	}
	if weights.encWHH, err = take(englishG2PGates * englishG2PHidden); err != nil {
		return nil, err
	}
	if weights.encBIH, err = take(englishG2PGates); err != nil {
		return nil, err
	}
	if weights.encBHH, err = take(englishG2PGates); err != nil {
		return nil, err
	}
	if weights.decEmb, err = take(englishG2PPhonemes * englishG2PHidden); err != nil {
		return nil, err
	}
	if weights.decWIH, err = take(englishG2PGates * englishG2PHidden); err != nil {
		return nil, err
	}
	if weights.decWHH, err = take(englishG2PGates * englishG2PHidden); err != nil {
		return nil, err
	}
	if weights.decBIH, err = take(englishG2PGates); err != nil {
		return nil, err
	}
	if weights.decBHH, err = take(englishG2PGates); err != nil {
		return nil, err
	}
	if weights.fcW, err = take(englishG2PPhonemes * englishG2PHidden); err != nil {
		return nil, err
	}
	if weights.fcB, err = take(englishG2PPhonemes); err != nil {
		return nil, err
	}
	if offset != len(data) {
		return nil, fmt.Errorf("english g2p: trailing weights")
	}
	return weights, nil
}

// englishNeuralPronunciationは辞書にない語をGRUで読む。
// 読みが出せない語はエラーにせず、空文字とfalseを返す。
func englishNeuralPronunciation(word string) (string, bool) {
	weights, err := loadEnglishG2P()
	if err != nil || weights == nil {
		return "", false
	}
	word = strings.ToLower(strings.ReplaceAll(word, "’", "'"))
	if word == "" || strings.Trim(word, "abcdefghijklmnopqrstuvwxyz") != "" {
		return "", false
	}
	runes := []rune(word)
	inputs := make([]int, 0, len(runes)+1)
	for _, r := range runes {
		if r >= 'a' && r <= 'z' {
			inputs = append(inputs, int(r-'a')+3)
		} else {
			inputs = append(inputs, 1)
		}
	}
	inputs = append(inputs, 2) // </s>

	hidden := make([]float32, englishG2PHidden)
	for _, input := range inputs {
		hidden = gruStep(weights.encEmb[input*englishG2PHidden:(input+1)*englishG2PHidden],
			hidden, weights.encWIH, weights.encWHH, weights.encBIH, weights.encBHH)
	}

	current := 2 // <s>
	phones := make([]string, 0, englishG2PMaxSteps)
	for step := 0; step < englishG2PMaxSteps; step++ {
		hidden = gruStep(weights.decEmb[current*englishG2PHidden:(current+1)*englishG2PHidden],
			hidden, weights.decWIH, weights.decWHH, weights.decBIH, weights.decBHH)
		best, bestScore := 0, float32(math.Inf(-1))
		for i := 0; i < englishG2PPhonemes; i++ {
			score := weights.fcB[i]
			row := weights.fcW[i*englishG2PHidden : (i+1)*englishG2PHidden]
			for j, value := range row {
				score += hidden[j] * value
			}
			if score > bestScore {
				best, bestScore = i, score
			}
		}
		if best == 3 { // </s>
			break
		}
		if best < 4 {
			break
		}
		phones = append(phones, englishG2PPhonemeVocabulary[best])
		current = best
	}
	if len(phones) == 0 {
		return "", false
	}
	return strings.Join(phones, " "), true
}

func gruStep(x, h, wIH, wHH, bIH, bHH []float32) []float32 {
	rznIH := make([]float32, englishG2PGates)
	rznHH := make([]float32, englishG2PGates)
	for i := 0; i < englishG2PGates; i++ {
		sum := bIH[i]
		row := wIH[i*englishG2PHidden : (i+1)*englishG2PHidden]
		for j, value := range row {
			sum += x[j] * value
		}
		rznIH[i] = sum

		sum = bHH[i]
		row = wHH[i*englishG2PHidden : (i+1)*englishG2PHidden]
		for j, value := range row {
			sum += h[j] * value
		}
		rznHH[i] = sum
	}
	rz := make([]float32, 2*englishG2PHidden)
	for i := 0; i < 2*englishG2PHidden; i++ {
		rz[i] = sigmoid32(rznIH[i] + rznHH[i])
	}
	next := make([]float32, englishG2PHidden)
	for i := 0; i < englishG2PHidden; i++ {
		n := tanh32(rznIH[2*englishG2PHidden+i] + rz[i]*rznHH[2*englishG2PHidden+i])
		z := rz[englishG2PHidden+i]
		next[i] = (1-z)*n + z*h[i]
	}
	return next
}

func sigmoid32(value float32) float32 {
	return 1 / (1 + float32(math.Exp(float64(-value))))
}

func tanh32(value float32) float32 {
	return float32(math.Tanh(float64(value)))
}
