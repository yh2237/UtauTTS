// Package settingsは合成設定の定義表。既定値・範囲・表示・適用するproviderをここ1か所で持つ。
//
// 合成の前段（読み・発話計画・抑揚）の設定はRendererによらず共通で、Providersは空にする。
// Renderer固有の設定はProvidersに対象のprovider IDを書く。Renderer manifestは、この表にない
// 外部providerの設定だけを宣言する。カタログはこの表をmanifestの設定へ合成してGUIへ渡す。
package settings

type Option struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

type Setting struct {
	ID    string
	Type  string // integer, number, boolean, enum, string
	Group string
	// Defaultはnumber・integerならfloat64、booleanならbool、enum・stringならstring。
	Default       any
	Min, Max      *float64
	Step          *float64
	Label         string
	Options       []Option
	OptionsSource string
	// Providersは設定を受け付けるprovider。空は全Renderer共通。
	Providers []string
	// Languagesは設定が効く言語（正規化済みの言語コード）。空は全言語。
	Languages []string
	// HiddenはGUIに出さない設定（CLI・APIの互換用）。
	Hidden bool
}

func value(v float64) *float64 { return &v }

const (
	worldPhrase = "utautts-world-phrase"
	classic     = "utau-external-resampler"
	diffSinger  = "diffsinger"
)

// Tableは設定の定義。順序はGUIの表示順。
var Table = []Setting{
	{ID: "mora_duration_ms", Type: "integer", Group: "timing", Default: 120.0, Min: value(20), Max: value(1000), Label: "settings.defaultMoraDuration"},
	{ID: "pause_duration_ms", Type: "integer", Group: "timing", Default: 180.0, Min: value(0), Max: value(3000), Label: "settings.defaultPauseDuration"},
	{ID: "leading_preutterance_ms", Type: "integer", Group: "timing", Default: 0.0, Min: value(0), Max: value(300), Label: "settings.defaultLeadingPreutterance"},
	{ID: "intonation_strength", Type: "number", Group: "prosody", Default: 4.0, Min: value(0), Max: value(8), Step: value(0.01), Label: "settings.defaultIntonation"},
	{ID: "boundary_tone", Type: "boolean", Group: "correction", Default: true, Label: "settings.boundaryTone", Languages: []string{"ja"}},
	{ID: "stretch_adapt", Type: "boolean", Group: "correction", Default: true, Label: "settings.stretchAdapt", Languages: []string{"ja"}},
	{ID: "pause_context", Type: "boolean", Group: "correction", Default: true, Label: "settings.pauseContext"},
	{ID: "timing_warp", Type: "boolean", Group: "correction", Default: true, Label: "settings.timingWarp", Providers: []string{worldPhrase}},
	{ID: "microprosody", Type: "boolean", Group: "prosody", Default: true, Label: "settings.microprosody", Providers: []string{worldPhrase}, Languages: []string{"ja"}},
	{ID: "english_weak_form", Type: "boolean", Group: "pronunciation", Default: true, Label: "settings.englishWeakForm", Languages: []string{"en"}},
	{ID: "resampler", Type: "enum", Group: "classic", Default: "", OptionsSource: "resamplers", Label: "main.param.resampler", Providers: []string{classic}},
	{ID: "wavtool", Type: "enum", Group: "classic", Default: "builtin", OptionsSource: "wavtools", Label: "main.param.wavtool", Providers: []string{classic}},
	{ID: "diffsinger_steps", Type: "integer", Group: "diffsinger", Default: 0.0, Min: value(0), Max: value(100), Label: "settings.defaultDiffSingerSteps", Providers: []string{diffSinger}},
	{ID: "diffsinger_expr", Type: "number", Group: "diffsinger", Default: 0.0, Min: value(0), Max: value(2), Step: value(0.01), Label: "settings.defaultDiffSingerExpr", Providers: []string{diffSinger}},
	{ID: "diffsinger_duration_mix", Type: "number", Group: "diffsinger", Default: 0.0, Min: value(0), Max: value(1), Step: value(0.01), Label: "settings.defaultDiffSingerDurationMix", Providers: []string{diffSinger}},
	{ID: "diffsinger_pitch_mix", Type: "number", Group: "diffsinger", Default: 0.0, Min: value(0), Max: value(1), Step: value(0.01), Label: "settings.defaultDiffSingerPitchMix", Providers: []string{diffSinger}},
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

// Stringはenum・string設定の既定値。
func String(id string) string { return mustLookup(id).Default.(string) }

func (setting Setting) AppliesTo(provider string) bool {
	if len(setting.Providers) == 0 {
		return true
	}
	for _, candidate := range setting.Providers {
		if candidate == provider {
			return true
		}
	}
	return false
}

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
func ForProvider(provider string) []Setting {
	var result []Setting
	for _, setting := range Table {
		if !setting.Hidden && setting.AppliesTo(provider) {
			result = append(result, setting)
		}
	}
	return result
}
