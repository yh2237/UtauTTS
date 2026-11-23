package linguistics

// Moraは合成・韻律予測・参照照合で共有する言語中間単位。
type Mora struct {
	Language      string  `json:",omitempty"`
	WordIndex     int     `json:",omitempty"`
	WordEnd       bool    `json:",omitempty"`
	SourceText    string  `json:",omitempty"`
	Phones        []Phone `json:",omitempty"`
	Text          string
	Consonant     string
	Vowel         string
	Pause         bool
	PauseKind     string `json:",omitempty"`
	Stress        int
	StressKnown   bool `json:",omitempty"`
	Tone          int
	DurationScale float64
}

// Phoneはモーラや音節の内部の音素区分を表す。
type Phone struct {
	Symbol string
	Role   string
}

const (
	PauseKindComma    = "comma"
	PauseKindPeriod   = "period"
	PauseKindQuestion = "question"
	PauseKindEllipsis = "ellipsis"
	PauseKindSpace    = "space"
	PauseKindBracket  = "bracket"
	PauseKindOther    = "other"
)

// Utteranceは言語解析が返す発話全体の中間表現。
type Utterance struct {
	Language       string
	Phonemizer     string
	Text           string
	Reading        string
	SurfaceReading string
	Automatic      string
	Words          []string
	Morae          []Mora
}

// EffectiveReadingは発音時に使う読みを返す。SurfaceReadingが空ならReadingを使う。
func (u Utterance) EffectiveReading() string {
	if u.SurfaceReading != "" {
		return u.SurfaceReading
	}
	return u.Reading
}
