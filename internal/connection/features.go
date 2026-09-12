// connectionパッケージはUTAUユニット間の音響的な相性を測定する。
package connection

import (
	"math"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/oto"
)

// Boundaryはユニット接合部のフレーム群を保持する。
type Boundary struct {
	Incoming     acoustic.Frame
	Outgoing     acoustic.Frame
	incomingWave []float64
	outgoingWave []float64
}

// PairFeaturesはモデルとヒューリスティックで共有する入力。
type PairFeatures struct {
	PreviousOutgoing       acoustic.Frame `json:"previous_outgoing"`
	CurrentIncoming        acoustic.Frame `json:"current_incoming"`
	SpectrumDelta          float64        `json:"spectrum_delta_db"`
	RMSDelta               float64        `json:"rms_delta_db"`
	F0DeltaCents           float64        `json:"f0_delta_cents"`
	VoicingMismatch        bool           `json:"voicing_mismatch"`
	WaveformCorrelation    float64        `json:"waveform_correlation"`
	SameSource             bool           `json:"same_source"`
	ForwardInSource        bool           `json:"forward_in_source"`
	SourceAnchorDistanceMS float64        `json:"source_anchor_distance_ms,omitempty"`
	// CurrentVCVは境界の無声閉鎖を含むことがあるVCV原音を示す。
	CurrentVCV bool `json:"current_vcv,omitempty"`
}

// Extractorは複数ペアで使うWAV分析結果をキャッシュする。
type Extractor struct {
	mutex sync.Mutex
	cache map[oto.Entry]Boundary
}

func NewExtractor() *Extractor {
	return &Extractor{cache: map[oto.Entry]Boundary{}}
}

func (e *Extractor) Boundary(entry oto.Entry) Boundary {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.cache == nil {
		e.cache = make(map[oto.Entry]Boundary)
	}
	if value, ok := e.cache[entry]; ok {
		return value
	}
	value := measureBoundary(entry)
	e.cache[entry] = value
	return value
}

func (e *Extractor) Pair(previous, current oto.Entry) PairFeatures {
	left, right := e.Boundary(previous), e.Boundary(current)
	result := PairFeatures{
		PreviousOutgoing: left.Outgoing,
		CurrentIncoming:  right.Incoming,
		SameSource:       SameSource(previous.Filename, current.Filename),
		CurrentVCV:       IsContextVCVAlias(current.Alias),
	}
	result.ForwardInSource = result.SameSource && current.Offset > previous.Offset
	if result.SameSource {
		result.SourceAnchorDistanceMS = current.Offset + current.Preutterance - previous.Offset - previous.Preutterance
	}
	result.WaveformCorrelation = maxCorrelation(left.outgoingWave, right.incomingWave, 40)
	if !left.Outgoing.Valid || !right.Incoming.Valid {
		return result
	}
	result.SpectrumDelta = acoustic.MeanSpectrumDelta(left.Outgoing.SpectrumDB, right.Incoming.SpectrumDB)
	result.RMSDelta = math.Abs(left.Outgoing.RMSDB - right.Incoming.RMSDB)
	leftVoiced, rightVoiced := left.Outgoing.F0Hz > 0, right.Incoming.F0Hz > 0
	result.VoicingMismatch = leftVoiced != rightVoiced
	if leftVoiced && rightVoiced {
		result.F0DeltaCents = math.Abs(1200 * math.Log2(right.Incoming.F0Hz/left.Outgoing.F0Hz))
	}
	return result
}

// HandcraftedScoreは学習モデルとの比較基準となる。
func HandcraftedScore(features PairFeatures) float64 {
	score := sourceContinuityScore(features)
	if !features.PreviousOutgoing.Valid || !features.CurrentIncoming.Valid {
		return score
	}
	spectrumWeight, rmsWeight := 0.8, 0.25
	if features.CurrentVCV {
		// VCVの境界は閉鎖区間になることがあるため接続点の減点を弱める。
		// 原音の適性は母音側の音響scoreで判断する。
		spectrumWeight, rmsWeight = 0.42, 0.13
	}
	score -= math.Min(18, features.SpectrumDelta*spectrumWeight)
	score -= math.Min(6, features.RMSDelta*rmsWeight)
	if features.PreviousOutgoing.F0Hz > 0 && features.CurrentIncoming.F0Hz > 0 {
		score -= math.Min(8, features.F0DeltaCents*0.015)
	} else if features.VoicingMismatch && !features.CurrentVCV {
		score -= 4
	}
	return score
}

func sourceContinuityScore(features PairFeatures) float64 {
	if !features.ForwardInSource {
		return 0
	}
	score := 8.0
	// 同じ録音でも遠い位置への移動は連続性の利点として扱わない。
	const safeAnchorDistanceMS = 560.0
	if features.SourceAnchorDistanceMS > safeAnchorDistanceMS {
		score -= math.Min(4, (features.SourceAnchorDistanceMS-safeAnchorDistanceMS)/140)
	}
	return score
}

// IsContextVCVAliasは英語のVCCVやVCと区別して日本語VCV表記を判定する。
func IsContextVCVAlias(alias string) bool {
	parts := strings.Fields(strings.TrimSpace(alias))
	if len(parts) < 2 || (parts[0] != "-" && !isVowelContext(parts[0])) {
		return false
	}
	for _, r := range parts[1] {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}

func isVowelContext(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "a", "i", "u", "e", "o", "n", "あ", "い", "う", "え", "お", "ん", "ア", "イ", "ウ", "エ", "オ", "ン":
		return true
	default:
		return false
	}
}

func measureBoundary(entry oto.Entry) Boundary {
	pcm, err := audio.ReadWav(entry.Filename)
	if err != nil || pcm.SampleRate <= 0 || pcm.Channels <= 0 {
		return Boundary{}
	}
	wave := acoustic.Mono(pcm)
	trimEndMS := float64(len(wave)) * 1000 / float64(pcm.SampleRate)
	if entry.Blank < 0 {
		trimEndMS = entry.Offset - entry.Blank
	} else {
		trimEndMS -= entry.Blank
	}
	incomingMS := entry.Offset + math.Max(0, entry.Preutterance-entry.Overlap)
	stableStartMS := entry.Offset + math.Max(entry.Fixed, entry.Preutterance)
	outgoingMS := math.Min(stableStartMS+60, trimEndMS-15)
	if outgoingMS < stableStartMS {
		outgoingMS = stableStartMS
	}
	incomingFrame, incomingWave := frameFeatures(wave, pcm.SampleRate, incomingMS)
	outgoingFrame, outgoingWave := frameFeatures(wave, pcm.SampleRate, outgoingMS)
	return Boundary{Incoming: incomingFrame, Outgoing: outgoingFrame, incomingWave: incomingWave, outgoingWave: outgoingWave}
}

func frameFeatures(wave []float64, sampleRate int, centerMS float64) (acoustic.Frame, []float64) {
	halfWindow := max(16, int(math.Round(0.015*float64(sampleRate))))
	if len(wave) < halfWindow*2 {
		return acoustic.Frame{}, nil
	}
	center := int(math.Round(centerMS * float64(sampleRate) / 1000))
	center = max(halfWindow, min(len(wave)-halfWindow, center))
	start, end := center-halfWindow, center+halfWindow
	frameWave := wave[start:end]
	return acoustic.AnalyzeFrame(frameWave, sampleRate, 10, true), resampleFixed(frameWave, 240)
}

func resampleFixed(values []float64, length int) []float64 {
	if len(values) == 0 || length <= 0 {
		return nil
	}
	result := make([]float64, length)
	for index := range result {
		position := float64(index) * float64(len(values)-1) / float64(max(1, length-1))
		left := int(position)
		right := min(len(values)-1, left+1)
		fraction := position - float64(left)
		result[index] = values[left]*(1-fraction) + values[right]*fraction
	}
	return result
}

func maxCorrelation(left, right []float64, maxLag int) float64 {
	if len(left) < 32 || len(right) < 32 {
		return 0
	}
	best := -1.0
	for lag := -maxLag; lag <= maxLag; lag++ {
		leftStart, rightStart := max(0, -lag), max(0, lag)
		length := min(len(left)-leftStart, len(right)-rightStart)
		if length < 32 {
			continue
		}
		leftMean, rightMean := 0.0, 0.0
		for index := 0; index < length; index++ {
			leftMean += left[leftStart+index]
			rightMean += right[rightStart+index]
		}
		leftMean /= float64(length)
		rightMean /= float64(length)
		cross, leftEnergy, rightEnergy := 0.0, 0.0, 0.0
		for index := 0; index < length; index++ {
			a := left[leftStart+index] - leftMean
			b := right[rightStart+index] - rightMean
			cross += a * b
			leftEnergy += a * a
			rightEnergy += b * b
		}
		if leftEnergy > 1e-10 && rightEnergy > 1e-10 {
			best = max(best, cross/math.Sqrt(leftEnergy*rightEnergy))
		}
	}
	return max(0, best)
}

func SameSource(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}
