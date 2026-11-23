package voicebank

// AliasHintsは音素列から波形合成の原音選択へ渡すUTAUエイリアス候補。
type AliasHints struct {
	Main            []string
	MainKinds       []string
	Transition      []string
	Endings         [][]string
	EndingPhones    [][]string          `json:",omitempty"`
	EndingFallbacks [][]CodaAlias       `json:",omitempty"`
	MainMissing     map[string][]string `json:",omitempty"`
}

// Emptyはエイリアス候補が何も設定されていないかを返す。
func (a AliasHints) Empty() bool {
	return len(a.Main) == 0 && len(a.MainKinds) == 0 && len(a.Transition) == 0 &&
		len(a.Endings) == 0 && len(a.EndingPhones) == 0 && len(a.EndingFallbacks) == 0 && len(a.MainMissing) == 0
}

// CodaAliasは語末子音の分割や代替候補を表す。
type CodaAlias struct {
	Aliases   []string
	Phones    []string
	CodaStart int
}

// PresampConfigはpresamp.iniから得た原音エイリアス用の設定。
type PresampConfig struct {
	Vowels       map[string]string
	Consonants   map[string]string
	Replacements map[string]string
	Endings      []string
}
