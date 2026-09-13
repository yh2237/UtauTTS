// jsutはJSUTのHTSラベルを学習用の音素列へ変換する。
package jsut

import (
	"bufio"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// AlignmentSchemaVersionはデータセットJSONLのスキーマ番号
const AlignmentSchemaVersion = 1

// AlignmentはJSUT BASIC5000の1発話分
// 時刻はjsut-labelがJuliusで推定した値
type Alignment struct {
	SchemaVersion   int        `json:"schema_version"`
	Kind            string     `json:"kind"`
	ID              string     `json:"id"`
	Text            string     `json:"text"`
	AudioPath       string     `json:"audio_path"`
	AudioSampleRate int        `json:"audio_sample_rate,omitempty"`
	AudioChannels   int        `json:"audio_channels,omitempty"`
	AudioDurationMS float64    `json:"audio_duration_ms,omitempty"`
	AnalysisFrameMS float64    `json:"analysis_frame_ms,omitempty"`
	SpectrumBands   int        `json:"spectrum_bands,omitempty"`
	AlignmentSource string     `json:"alignment_source"`
	TimingUnit      string     `json:"timing_unit"`
	Phones          []Phone    `json:"phones"`
	Boundaries      []Boundary `json:"boundaries"`
}

// PhoneはHTSラベルの1区間
// 無音と休止も残し後で接続と区別できるようにする
type Phone struct {
	Index                int     `json:"index"`
	Symbol               string  `json:"symbol"`
	Previous             string  `json:"previous,omitempty"`
	Next                 string  `json:"next,omitempty"`
	Context              string  `json:"context"`
	StartMS              float64 `json:"start_ms"`
	EndMS                float64 `json:"end_ms"`
	DurationMS           float64 `json:"duration_ms"`
	Silence              bool    `json:"silence,omitempty"`
	Pause                bool    `json:"pause,omitempty"`
	AccentPhrasePosition int     `json:"accent_phrase_position,omitempty"`
	AccentPhraseLength   int     `json:"accent_phrase_length,omitempty"`
	AccentNucleus        int     `json:"accent_nucleus,omitempty"`
	Frame                *Frame  `json:"frame,omitempty"`
}

// Boundaryは録音内で隣り合う区間の境界
// 無音と休止は接続学習から除外する
type Boundary struct {
	Index          int               `json:"index"`
	LeftPhone      string            `json:"left_phone"`
	RightPhone     string            `json:"right_phone"`
	BoundaryTimeMS float64           `json:"boundary_time_ms"`
	GapMS          float64           `json:"gap_ms"`
	JoinType       string            `json:"join_type"`
	Trainable      bool              `json:"trainable"`
	Label          *float64          `json:"label"`
	LabelSource    string            `json:"label_source"`
	Features       *BoundaryFeatures `json:"features,omitempty"`
}

// Frameはオフライン学習用の音響観測値
// acoustic.Frameと同じ項目を持つ
type Frame struct {
	Valid      bool      `json:"valid"`
	RMSDB      float64   `json:"rms_db"`
	F0Hz       float64   `json:"f0_hz"`
	SpectrumDB []float64 `json:"spectrum_db,omitempty"`
}

// BoundaryFeaturesはラベル境界の直前と直後の比較値
// これ自体は品質ラベルではない
type BoundaryFeatures struct {
	PreviousOutgoing Frame   `json:"previous_outgoing"`
	CurrentIncoming  Frame   `json:"current_incoming"`
	SpectrumDeltaDB  float64 `json:"spectrum_delta_db"`
	RMSDeltaDB       float64 `json:"rms_delta_db"`
	F0DeltaCents     float64 `json:"f0_delta_cents"`
	F0Comparable     bool    `json:"f0_comparable"`
	VoicingMismatch  bool    `json:"voicing_mismatch"`
}

// ParseHTSLabelsは3列のHTSフルコンテキストラベルを読む
// 時刻の単位はjsut-labelと同じ100ナノ秒
func ParseHTSLabels(text string) ([]Phone, []Boundary, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1024), 2*1024*1024)
	phones := make([]Phone, 0, 64)
	previousEnd := int64(0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, nil, fmt.Errorf("line %d: expected start, end, context", lineNumber)
		}
		start, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: invalid start %q: %w", lineNumber, fields[0], err)
		}
		end, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: invalid end %q: %w", lineNumber, fields[1], err)
		}
		if start != previousEnd {
			return nil, nil, fmt.Errorf("line %d: noncontiguous interval %d after %d", lineNumber, start, previousEnd)
		}
		if start < 0 || end <= start {
			return nil, nil, fmt.Errorf("line %d: invalid interval %d..%d", lineNumber, start, end)
		}
		context := strings.Join(fields[2:], " ")
		phone, previous, next, err := parsePhoneContext(context)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		position, length, nucleus, err := parseAccentContext(context, phone)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		phones = append(phones, Phone{
			Index: len(phones), Symbol: phone, Previous: previous, Next: next,
			Context: context, StartMS: ticksToMS(start), EndMS: ticksToMS(end),
			DurationMS: ticksToMS(end - start), Silence: phone == "sil", Pause: phone == "pau",
			AccentPhrasePosition: position, AccentPhraseLength: length, AccentNucleus: nucleus,
		})
		previousEnd = end
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if len(phones) == 0 {
		return nil, nil, fmt.Errorf("label file is empty")
	}
	boundaries := make([]Boundary, 0, len(phones)-1)
	for index := 0; index+1 < len(phones); index++ {
		left, right := phones[index], phones[index+1]
		joinType, trainable := "phone", true
		if left.Silence || right.Silence {
			joinType, trainable = "silence", false
		} else if left.Pause || right.Pause {
			joinType, trainable = "pause", false
		}
		boundaryTime := (left.EndMS + right.StartMS) / 2
		var label *float64
		if trainable {
			value := 1.0
			label = &value
		}
		boundaries = append(boundaries, Boundary{
			Index: index, LeftPhone: left.Symbol, RightPhone: right.Symbol,
			BoundaryTimeMS: boundaryTime, GapMS: right.StartMS - left.EndMS,
			JoinType: joinType, Trainable: trainable, Label: label,
			LabelSource: "natural_adjacent_boundary",
		})
	}
	return phones, boundaries, nil
}

func parseAccentContext(context, phone string) (position, length, nucleus int, err error) {
	if phone == "sil" || phone == "pau" {
		return 0, 0, 0, nil
	}
	accent, hasAccent := contextSection(context, "/A:")
	frame, hasFrame := contextSection(context, "/F:")
	if !hasAccent && !hasFrame {
		return 0, 0, 0, nil
	}
	if !hasAccent || !hasFrame {
		return 0, 0, 0, fmt.Errorf("incomplete accent context: %q", context)
	}
	accentParts := strings.Split(accent, "+")
	frameParts := strings.SplitN(frame, "_", 2)
	if len(accentParts) < 2 || len(frameParts) != 2 {
		return 0, 0, 0, fmt.Errorf("malformed accent context: %q", context)
	}
	position, err = strconv.Atoi(accentParts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid accent position in %q", context)
	}
	length, err = parseLeadingInt(frameParts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid accent length in %q", context)
	}
	nucleus, err = parseLeadingInt(frameParts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid accent nucleus in %q", context)
	}
	if position < 1 || position > length || nucleus < 0 || nucleus > length {
		return 0, 0, 0, fmt.Errorf("invalid accent range in %q", context)
	}
	return position, length, nucleus, nil
}

func parseLeadingInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty number")
	}
	end := 0
	if value[0] == '+' || value[0] == '-' {
		end = 1
	}
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == 0 || (end == 1 && (value[0] == '+' || value[0] == '-')) {
		return 0, fmt.Errorf("invalid number %q", value)
	}
	return strconv.Atoi(value[:end])
}

func contextSection(context, marker string) (string, bool) {
	start := strings.Index(context, marker)
	if start < 0 {
		return "", false
	}
	value := context[start+len(marker):]
	if end := strings.IndexByte(value, '/'); end >= 0 {
		value = value[:end]
	}
	return value, true
}

func parsePhoneContext(context string) (phone, previous, next string, err error) {
	dash := strings.IndexByte(context, '-')
	if dash < 0 || dash+1 >= len(context) {
		return "", "", "", fmt.Errorf("context has no phone separator: %q", context)
	}
	rest := context[dash+1:]
	plus := strings.IndexByte(rest, '+')
	if plus <= 0 {
		return "", "", "", fmt.Errorf("context has no phone pair: %q", context)
	}
	phone = rest[:plus]
	if phone == "" {
		return "", "", "", fmt.Errorf("context contains an empty phone: %q", context)
	}
	leftContext := context[:dash]
	if caret := strings.LastIndexByte(leftContext, '^'); caret >= 0 {
		previous = leftContext[caret+1:]
	} else {
		previous = leftContext
	}
	rightContext := rest[plus+1:]
	if separator := strings.IndexAny(rightContext, "=/"); separator >= 0 {
		rightContext = rightContext[:separator]
	}
	next = rightContext
	return phone, previous, next, nil
}

func ticksToMS(value int64) float64 {
	return float64(value) / 10000
}

// NewAlignmentは解析済みラベルから1発話の記録を作る
func NewAlignment(id, text, audioPath string, phones []Phone, boundaries []Boundary) Alignment {
	return Alignment{
		SchemaVersion:   AlignmentSchemaVersion,
		Kind:            "jsut_phone_alignment",
		ID:              id,
		Text:            text,
		AudioPath:       audioPath,
		AlignmentSource: "jsut-label:julius_estimated_timing",
		TimingUnit:      "milliseconds",
		Phones:          phones,
		Boundaries:      boundaries,
	}
}

// Validateはデータセットへ書き込む前に記録を検証する
func (alignment Alignment) Validate() error {
	if alignment.SchemaVersion != AlignmentSchemaVersion {
		return fmt.Errorf("unsupported schema version %d", alignment.SchemaVersion)
	}
	if alignment.Kind != "jsut_phone_alignment" {
		return fmt.Errorf("unsupported kind %q", alignment.Kind)
	}
	if alignment.ID == "" || alignment.AudioPath == "" {
		return fmt.Errorf("id and audio_path are required")
	}
	if len(alignment.Phones) == 0 {
		return fmt.Errorf("phones are empty")
	}
	previousEnd := 0.0
	for index, phone := range alignment.Phones {
		if phone.Index != index || !finite(phone.StartMS) || !finite(phone.EndMS) || phone.StartMS < previousEnd || phone.EndMS <= phone.StartMS {
			return fmt.Errorf("invalid phone interval at index %d", index)
		}
		previousEnd = phone.EndMS
	}
	if len(alignment.Boundaries) != len(alignment.Phones)-1 {
		return fmt.Errorf("boundary count %d, want %d", len(alignment.Boundaries), len(alignment.Phones)-1)
	}
	for index, boundary := range alignment.Boundaries {
		if boundary.Index != index || boundary.LeftPhone != alignment.Phones[index].Symbol || boundary.RightPhone != alignment.Phones[index+1].Symbol || !finite(boundary.BoundaryTimeMS) || !finite(boundary.GapMS) {
			return fmt.Errorf("invalid boundary at index %d", index)
		}
		if boundary.Trainable != (boundary.Label != nil) {
			return fmt.Errorf("boundary %d label/trainable mismatch", index)
		}
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
