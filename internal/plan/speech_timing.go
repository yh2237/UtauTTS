package plan

// PhoneTimingは合成目標であり音響解析の結果ではない。
type PhoneTiming struct {
	Position   int     `json:"position"`
	Symbol     string  `json:"symbol"`
	Role       string  `json:"role"`
	StartMS    float64 `json:"start_ms"`
	DurationMS float64 `json:"duration_ms"`
}
