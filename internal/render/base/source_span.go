package base

// SourceSpanは原音区間と出力音素の対応を保持する。
type SourceSpan struct {
	Alias          string               `json:"alias"`
	SourceSHA256   string               `json:"source_sha256"`
	CoreStartMS    float64              `json:"core_start_ms"`
	CoreEndMS      float64              `json:"core_end_ms"`
	ContextStartMS float64              `json:"context_start_ms"`
	Mappings       []SourcePhoneMapping `json:"mappings"`
	Landmarks      []SourceLandmark     `json:"landmark_candidates"`
}

type SourceLandmark struct {
	Kind       string  `json:"kind"`
	SourceMS   float64 `json:"source_ms"`
	DurationMS float64 `json:"duration_ms"`
	Score      float64 `json:"heuristic_score"`
	RelativeDB float64 `json:"relative_to_peak_db"`
}

type SourcePhoneMapping struct {
	Symbol           string  `json:"symbol"`
	SourceStartMS    float64 `json:"source_start_ms"`
	SourceEndMS      float64 `json:"source_end_ms"`
	RequestedStartMS float64 `json:"requested_start_ms"`
	RequestedEndMS   float64 `json:"requested_end_ms"`
}

// 旧名は試聴ツールとの互換用。
type ExperimentalSourceSpan = SourceSpan
type ExperimentalPhoneMapping = SourcePhoneMapping
type ExperimentalLandmark = SourceLandmark
