// Package settingsは合成設定の既定値・範囲・表示・適用先を定義する。
package settings

type Option struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

type Setting struct {
	ID    string `json:"id"`
	Type  string `json:"type"` // integer, number, boolean, enum, string
	Group string `json:"group,omitempty"`
	// Defaultはnumber・integerならfloat64、booleanならbool、enum・stringならstring。
	Default any      `json:"default,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Step    *float64 `json:"step,omitempty"`
	Label   string   `json:"label,omitempty"`
	Options []Option `json:"options,omitempty"`
	// Methodsは設定が効く方式。空は全方式。
	Methods []string `json:"methods,omitempty"`
	// Languagesは設定が効く言語（正規化済みの言語コード）。空は全言語。
	Languages []string `json:"languages,omitempty"`
	// HiddenはGUIに出さない設定（CLI・APIの互換用）。
	Hidden bool `json:"-"`
}

func value(v float64) *float64 { return &v }

// waveformは波形合成の方式名（tts.MethodWaveformSynthesis）。
const waveform = "waveform-synthesis"

// Tableは設定の定義。順序はGUIの表示順。
var Table = []Setting{
	{ID: "mora_duration_ms", Type: "integer", Group: "timing", Default: 120.0, Min: value(20), Max: value(1000), Label: "settings.defaultMoraDuration"},
	{ID: "pause_duration_ms", Type: "integer", Group: "timing", Default: 180.0, Min: value(0), Max: value(3000), Label: "settings.defaultPauseDuration"},
	{ID: "leading_preutterance_ms", Type: "integer", Group: "timing", Default: 0.0, Min: value(0), Max: value(300), Label: "settings.defaultLeadingPreutterance"},
	{ID: "intonation_strength", Type: "number", Group: "prosody", Default: 4.0, Min: value(0), Max: value(8), Step: value(0.01), Label: "settings.defaultIntonation"},
	{ID: "boundary_tone", Type: "boolean", Group: "correction", Default: true, Label: "settings.boundaryTone", Languages: []string{"ja"}},
	{ID: "stretch_adapt", Type: "boolean", Group: "correction", Default: true, Label: "settings.stretchAdapt", Languages: []string{"ja"}},
	{ID: "pause_context", Type: "boolean", Group: "correction", Default: true, Label: "settings.pauseContext"},
	{ID: "timing_warp", Type: "boolean", Group: "correction", Default: true, Label: "settings.timingWarp", Methods: []string{waveform}},
	{ID: "microprosody", Type: "boolean", Group: "prosody", Default: true, Label: "settings.microprosody", Methods: []string{waveform}, Languages: []string{"ja"}},
	// GUIに出さない設定。強度の0は既定1.0として扱う。
	{ID: "context_duration", Type: "boolean", Default: false, Hidden: true, Languages: []string{"ja"}},
	{ID: "context_duration_strength", Type: "number", Default: 1.0, Hidden: true},
	{ID: "boundary_tone_strength", Type: "number", Default: 1.0, Hidden: true},
	{ID: "stretch_adapt_strength", Type: "number", Default: 1.0, Hidden: true},
	{ID: "pause_context_strength", Type: "number", Default: 1.0, Hidden: true},
}

func Lookup(id string) (Setting, bool) {
	for _, setting := range Table {
		if setting.ID == id {
			return setting, true
		}
	}
	return Setting{}, false
}

func mustLookup(id string) Setting {
	setting, ok := Lookup(id)
	if !ok {
		panic("unknown setting " + id)
	}
	return setting
}

// Boolはboolean設定の既定値。
func Bool(id string) bool { return mustLookup(id).Default.(bool) }

// Numberはnumber・integer設定の既定値。
func Number(id string) float64 { return mustLookup(id).Default.(float64) }

// AppliesToLanguageは設定が言語（正規化済み）に効くかを返す。
func (setting Setting) AppliesToLanguage(language string) bool {
	if len(setting.Languages) == 0 {
		return true
	}
	for _, candidate := range setting.Languages {
		if candidate == language {
			return true
		}
	}
	return false
}

// ForProviderはproviderのGUIに出す設定を表の順で返す。
// Visibleは、GUIに出す設定を表示順に返す。
func Visible() []Setting {
	var result []Setting
	for _, setting := range Table {
		if !setting.Hidden {
			result = append(result, setting)
		}
	}
	return result
}
