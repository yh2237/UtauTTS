package connection

import (
	"math"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/yh2237/audiodsp/correlation"
	"github.com/yh2237/audiodsp/stretch"

	"github.com/yh2237/UtauTTS/internal/acoustic"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/sourceaudio"
)

type Boundary struct {
	Incoming     acoustic.Frame
	Outgoing     acoustic.Frame
	incomingWave []float64
	outgoingWave []float64
}

type PairFeatures struct {
	PreviousOutgoing       acoustic.Frame `json:"previous_outgoing"`
	CurrentIncoming        acoustic.Frame `json:"current_incoming"`
	SpectrumDelta          float64        `json:"spectrum_delta_db"`
	SpectralTiltDelta      float64        `json:"spectral_tilt_delta_db"`
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
	return pairFeatures(previous, current, left, right)
}

func pairFeatures(previous, current oto.Entry, left, right Boundary) PairFeatures {
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
	result.WaveformCorrelation = correlation.Max(left.outgoingWave, right.incomingWave, 40, 32, 1e-10)
	if !left.Outgoing.Valid || !right.Incoming.Valid {
		return result
	}
	result.SpectrumDelta = acoustic.MeanSpectrumDelta(left.Outgoing.SpectrumDB, right.Incoming.SpectrumDB)
	result.SpectralTiltDelta = acoustic.SpectralTiltDelta(left.Outgoing.SpectrumDB, right.Incoming.SpectrumDB)
	result.RMSDelta = math.Abs(left.Outgoing.RMSDB - right.Incoming.RMSDB)
	leftVoiced, rightVoiced := left.Outgoing.F0Hz > 0, right.Incoming.F0Hz > 0
	result.VoicingMismatch = leftVoiced != rightVoiced
	if leftVoiced && rightVoiced {
		result.F0DeltaCents = math.Abs(1200 * math.Log2(right.Incoming.F0Hz/left.Outgoing.F0Hz))
	}
	return result
}

// 判定なし・低信頼の場合は規則スコアへ戻す。
func (e *Extractor) ScoreEntries(previous, current oto.Entry) float64 {
	features := e.Pair(previous, current)
	return e.ScoreFeatures(features)
}

// ScoreSpeechContextは描画で保持する接続区間の中央を評価する。
// 入口はpreutterance前の最大40ms、出口は従来の推定位置を使う。
func (e *Extractor) ScoreSpeechContext(previous, current oto.Entry) float64 {
	current.Overlap = math.Min(20, math.Max(0, current.Preutterance)*.5)
	return e.ScoreEntries(previous, current)
}

func (e *Extractor) ScoreFeatures(features PairFeatures) float64 {
	return HandcraftedScore(features)
}

func HandcraftedScore(features PairFeatures) float64 {
	score := sourceContinuityScore(features)
	if !features.PreviousOutgoing.Valid || !features.CurrentIncoming.Valid {
		return score
	}
	spectrumWeight, rmsWeight := 0.8, 0.25
	if features.CurrentVCV {
		// VCV境界は閉鎖区間も含むため、接続点の減点を弱める。
		spectrumWeight, rmsWeight = 0.42, 0.13
	}
	score -= math.Min(18, features.SpectrumDelta*spectrumWeight)
	score -= math.Min(6, features.RMSDelta*rmsWeight)
	if features.PreviousOutgoing.F0Hz > 0 && features.CurrentIncoming.F0Hz > 0 {
		score -= math.Min(8, features.F0DeltaCents*0.015)
	} else if features.VoicingMismatch && !features.CurrentVCV {
		score -= 4
	}
	// 相関0.5を中立として補正する。
	score += 4 * (features.WaveformCorrelation - 0.5)
	score -= math.Min(4, features.SpectralTiltDelta*0.15)
	return score
}

func sourceContinuityScore(features PairFeatures) float64 {
	if !features.ForwardInSource {
		return 0
	}
	// 同一録音の連続性を保つため、距離が遠くても6〜9点の加点を残す。
	distance := features.SourceAnchorDistanceMS
	if !isFinite(distance) || distance < 0 {
		distance = 0
	}
	if distance <= 500 {
		return 9
	}
	if distance >= 2000 {
		return 6
	}
	return 9 - 3*(distance-500)/1500
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
	pcm, wave, err := sourceaudio.ReadMono(entry.Filename)
	if err != nil || pcm.SampleRate <= 0 || pcm.Channels <= 0 {
		return Boundary{}
	}
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
	return acoustic.AnalyzeFrame(frameWave, sampleRate, 10, true), stretch.Linear(frameWave, 240)
}

func SameSource(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
