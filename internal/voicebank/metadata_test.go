package voicebank

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/oto"
)

func TestLoadCharacterNameAndPrefixMap(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "oto.ini"), "a.wav=あC4,0,0,0,0,0\n")
	name, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte("name=テスト音源\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "character.txt"), name, 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "prefix.map"), "C4\t\tC4\nG4\t強\t\n")

	bank, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if bank.Name != "テスト音源" {
		t.Fatalf("name = %q", bank.Name)
	}
	if got := bank.PrefixMap["C4"]; got != (Affix{Suffix: "C4"}) {
		t.Fatalf("C4 affix = %+v", got)
	}
	if got, ok := bank.AffixForTone("F#4"); !ok || got != (Affix{Prefix: "強"}) {
		t.Fatalf("nearest affix = %+v, %v", got, ok)
	}
}

func TestMetadataSnapshotPreservesCaseAndRefreshesOnReload(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "oto.ini"), "a.wav=あ,0,0,0,0,0\n")
	write(t, filepath.Join(root, "CHARACTER.YML"), "default_phonemizer: ja-kana\nsubbanks:\n- color: \"\"\n  suffix: \" C4\"\n  tone_ranges: [C3-C5]\n")
	write(t, filepath.Join(root, "CHARACTER.TXT"), "name=最初\n")
	write(t, filepath.Join(root, "PREFIX.MAP"), "C4\t\t_C4\n")
	first, err := Load(root)
	if err != nil || first.Name != "最初" || first.DefaultPhonemizer != "ja-kana" || len(first.Subbanks) != 1 || first.PrefixMap["C4"].Suffix != "_C4" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	write(t, filepath.Join(root, "CHARACTER.TXT"), "name=更新\n")
	if err := os.Remove(filepath.Join(root, "PREFIX.MAP")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "PRESAMP.INI"), "[VOWEL]\na=a=あ=100\n")
	second, err := Load(root)
	if err != nil || second.Name != "更新" || len(second.PrefixMap) != 0 || second.Presamp == nil || second.Presamp.Vowels["あ"] != "a" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestResolveAtToneUsesAffixedAliases(t *testing.T) {
	bank := &Bank{
		Entries: map[string][]oto.Entry{
			"あ_C4":   {{Alias: "あ_C4"}},
			"か_C4":   {{Alias: "か_C4"}},
			"a k_C4": {{Alias: "a k_C4"}},
		},
		PrefixMap: map[string]Affix{"C4": {Suffix: "_C4"}},
	}
	morae, _ := frontend.ParseKana("あか")
	selections, err := bank.ResolveAtTone(morae, "C4")
	if err != nil {
		t.Fatal(err)
	}
	if len(selections) != 2 || selections[0].Alias != "あ_C4" || !selections[1].Composite ||
		selections[1].Transition == nil || selections[1].Transition.Alias != "a k_C4" {
		t.Fatalf("selections = %#v", selections)
	}
}

func TestEmptyPrefixMapEntryPreventsNearestToneFallback(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "oto.ini"), "a.wav=a,0,0,0,0,0\n")
	write(t, filepath.Join(root, "prefix.map"), "B3\t\t_B3\r\nC4\t\t\r\n")

	bank, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := bank.AffixForTone("C4"); !ok || got != (Affix{}) {
		t.Fatalf("C4 lookup = %+v, %t", got, ok)
	}
}

func TestCharacterYAMLSubbankSelectionByToneAndColor(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "character.yaml"), `subbanks:
- color: ""
  prefix: ""
  suffix: " C4"
  tone_ranges:
  - B3-B4
- color: "power"
  prefix: ""
  suffix: " P"
  tone_ranges: [C3-C5]
`)
	subbanks, path, diagnostics := loadCharacterYAML(root)
	if path == "" || len(diagnostics) != 0 || len(subbanks) != 2 {
		t.Fatalf("subbanks=%+v path=%q diagnostics=%+v", subbanks, path, diagnostics)
	}

	bank := &Bank{
		Entries: map[string][]oto.Entry{
			"あ C4": {{Alias: "あ C4"}},
			"あ P":  {{Alias: "あ P"}},
		},
		Subbanks: subbanks,
	}
	morae, err := frontend.ParseKana("あ")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := bank.ResolveWithConfig(morae, ResolveConfig{Tone: "C4"})
	if err != nil || len(selected) != 1 || selected[0].Alias != "あ C4" || selected[0].SubbankID != "subbank-0" {
		t.Fatalf("normal selection=%+v err=%v", selected, err)
	}
	selected, err = bank.ResolveWithConfig(morae, ResolveConfig{Tone: "C4", Color: "power"})
	if err != nil || len(selected) != 1 || selected[0].Alias != "あ P" || selected[0].Color != "power" {
		t.Fatalf("color selection=%+v err=%v", selected, err)
	}
}

func TestSubbankOptionsExposeEveryDeclaredType(t *testing.T) {
	bank := &Bank{Subbanks: []Subbank{
		{ID: "normal", Color: "", Prefix: "", Suffix: "_N", ToneRanges: []ToneRange{{Low: 48, High: 60}}},
		{ID: "power", Color: "power", Prefix: "", Suffix: "_P", ToneRanges: []ToneRange{{Low: 48, High: 72}}},
	}}
	options := bank.SubbankOptions()
	if len(options) != 2 || options[0].ID != "normal" || options[1].Color != "power" {
		t.Fatalf("options=%+v", options)
	}
	options[0].ToneRanges[0].Low = 0
	if bank.Subbanks[0].ToneRanges[0].Low == 0 {
		t.Fatal("subbank options leaked the bank's tone range slice")
	}
}

func TestCharacterYAMLDiagnosticsCarryFilePath(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "oto.ini"), "a.wav=あ,0,0,0,0,0\n")
	path := filepath.Join(root, "CHARACTER.YML")
	write(t, path, "subbanks:\n- color: power\n  tone_ranges: [bad, C3-C5]\n")
	bank, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(bank.Diagnostics) != 1 || bank.Diagnostics[0].Path != path || bank.Diagnostics[0].Line != 3 {
		t.Fatalf("diagnostics=%+v", bank.Diagnostics)
	}
	_, subbank, ok := bank.AffixForToneAndColor("C4", "power")
	if !ok || subbank.ID != "subbank-0" || subbank.Source != path || subbank.Tone != "C4" {
		t.Fatalf("subbank=%+v ok=%t", subbank, ok)
	}
}

func TestResolverRejectsUnreadableAndSilentWAVCandidates(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.wav")
	bad := filepath.Join(root, "bad.wav")
	writeTestTone(t, good, 220, 7000)
	if err := audio.WriteWav(bad, &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 1600)}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "oto.ini"), "bad.wav=あ,0,0,0,0,0\ngood.wav=あ,0,0,0,0,0\n")
	bank, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	morae, err := frontend.ParseKana("あ")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := bank.Resolve(morae)
	if err != nil || len(selected) != 1 {
		t.Fatalf("selection=%+v err=%v", selected, err)
	}
	if selected[0].Entry.Filename != good || selected[0].EntryStatus != "usable" {
		t.Fatalf("selected entry=%+v", selected[0])
	}
	if len(selected[0].CandidateRejections) != 1 || selected[0].CandidateRejections[0].Source != bad {
		t.Fatalf("rejections=%+v", selected[0].CandidateRejections)
	}
}

func writeTestTone(t *testing.T, path string, frequency float64, amplitude int16) {
	t.Helper()
	const sampleRate = 16000
	data := make([]int16, sampleRate/10)
	for index := range data {
		data[index] = int16(float64(amplitude) * math.Sin(2*math.Pi*frequency*float64(index)/sampleRate))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
}
