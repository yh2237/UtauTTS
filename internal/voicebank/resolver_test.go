package voicebank

import (
	"errors"
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/connection"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func TestMissingEndingDoesNotDropRemainingConsonants(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"a":   {{Alias: "a", Filename: "a.wav"}},
		"s t": {{Alias: "s t", Filename: "st.wav"}},
	}}
	selections, err := bank.Resolve([]frontend.Mora{{Text: "a", Vowel: "a", Aliases: &frontend.AliasHints{
		Main: []string{"a"}, Endings: [][]string{{"a s"}, {"s t"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(selections[0].Endings) != 1 || selections[0].Endings[0].Alias != "s t" {
		t.Fatalf("endings = %#v", selections[0].Endings)
	}
}

func TestResolvePrefersVCVAndFallsBackToCV(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- こ": {{Alias: "- こ", Filename: "start.wav"}},
		"o ん": {{Alias: "o ん", Filename: "vcv.wav"}},
		"に":   {{Alias: "に", Filename: "cv.wav"}},
	}}
	morae, err := frontend.ParseKana("こんに")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Alias != "- こ" || got[1].Alias != "o ん" || got[2].Alias != "に" {
		t.Fatalf("selections = %#v", got)
	}
}

func TestResolveUsesCVVCTransitionWhenVCVIsUnavailable(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"あ":   {{Alias: "あ", Filename: "a.wav"}},
		"か":   {{Alias: "か", Filename: "ka.wav"}},
		"a k": {{Alias: "a k", Filename: "ak.wav"}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Alias != "か" || !got[1].Composite || got[1].Transition == nil || got[1].Transition.Alias != "a k" {
		t.Fatalf("selections = %#v", got)
	}
}

func TestResolveUsesPhonemizerAliasHints(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- ni": {{Alias: "- ni", Filename: "ni.wav"}},
		"hao":  {{Alias: "hao", Filename: "hao.wav"}},
		"i h":  {{Alias: "i h", Filename: "ih.wav"}},
	}}
	morae := []frontend.Mora{
		{Text: "ni", Vowel: "i", Aliases: &frontend.AliasHints{Main: []string{"- ni", "ni"}}},
		{Text: "hao", Consonant: "h", Vowel: "ao", Aliases: &frontend.AliasHints{Main: []string{"i hao", "hao"}, Transition: []string{"i h"}}},
	}
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Alias != "- ni" || got[1].Alias != "hao" || got[1].Transition == nil || got[1].Transition.Alias != "i h" {
		t.Fatalf("selections = %#v", got)
	}
}

func TestResolveUsesExplicitKindsAndEnding(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- ni": {{Alias: "- ni", Filename: "ni.wav"}},
		"hao":  {{Alias: "hao", Filename: "hao.wav"}},
		"i h":  {{Alias: "i h", Filename: "ih.wav"}},
		"ao R": {{Alias: "ao R", Filename: "aor.wav"}},
	}}
	morae := []frontend.Mora{
		{Text: "ni", Vowel: "i", Aliases: &frontend.AliasHints{Main: []string{"- ni", "ni"}, MainKinds: []string{"vcv", "cv"}}},
		{Text: "hao", Consonant: "h", Vowel: "ao", Aliases: &frontend.AliasHints{
			Main: []string{"i hao", "hao"}, MainKinds: []string{"vcv", "cv"},
			Transition: []string{"i h"}, Endings: [][]string{{"ao R"}},
		}},
	}
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Kind != AliasVCV || !got[1].Composite || got[1].Transition == nil {
		t.Fatalf("selections=%#v", got)
	}
	if len(got[1].Endings) != 1 || got[1].Endings[0].Alias != "ao R" {
		t.Fatalf("ending=%#v", got[1].Endings)
	}
}

func TestResolveDoesNotInventClosureTransition(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"っ":    {{Alias: "っ", Filename: "cl.wav"}},
		"か":    {{Alias: "か", Filename: "ka.wav"}},
		"cl k": {{Alias: "cl k", Filename: "clk.wav"}},
	}}
	morae, err := frontend.ParseKana("っか")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Composite || got[1].Alias != "か" {
		t.Fatalf("closure transition was selected: %#v", got)
	}
}

func TestResolveDoesNotMakeCVVCFromAffixedWildcard(t *testing.T) {
	bank := &Bank{
		Entries: map[string][]oto.Entry{
			"強あ_C4":   {{Alias: "強あ_C4", Filename: "a.wav"}},
			"強* か_C4": {{Alias: "強* か_C4", Filename: "wild.wav"}},
			"強a k_C4": {{Alias: "強a k_C4", Filename: "ak.wav"}},
		},
		PrefixMap: map[string]Affix{"C4": {Prefix: "強", Suffix: "_C4"}},
	}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.ResolveAtTone(morae, "C4")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Composite || got[1].Alias != "強* か_C4" {
		t.Fatalf("wildcard was used as CVVC main: %#v", got)
	}
}

func TestResolveVCVBeatsCVVCInAutoButCVVCPreferCanOverride(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"あ":   {{Alias: "あ", Filename: "a.wav"}},
		"か":   {{Alias: "か", Filename: "ka.wav"}},
		"a k": {{Alias: "a k", Filename: "ak.wav"}},
		"a か": {{Alias: "a か", Filename: "aka.wav"}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	auto, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if auto[1].Kind != AliasVCV || auto[1].Composite {
		t.Fatalf("auto selections = %#v", auto)
	}
	prefer, err := bank.ResolveWithConfig(morae, ResolveConfig{AliasPolicy: AliasPolicyCVVCPrefer})
	if err != nil {
		t.Fatal(err)
	}
	if !prefer[1].Composite || prefer[1].Transition == nil || prefer[1].Transition.Alias != "a k" {
		t.Fatalf("cvvc-prefer selections = %#v", prefer)
	}
}

func TestAuditLatticeReportsCVVCSelection(t *testing.T) {
	bank := &Bank{Root: "bank", Entries: map[string][]oto.Entry{
		"あ":   {{Alias: "あ", Filename: "a.wav"}},
		"か":   {{Alias: "か", Filename: "ka.wav"}},
		"a k": {{Alias: "a k", Filename: "ak.wav"}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	audit, err := bank.AuditLattice(morae, "C4", nil)
	if err != nil {
		t.Fatal(err)
	}
	if audit.CVVCSelectedPositions != 1 || audit.CVSelectedPositions != 1 {
		t.Fatalf("selection counts = %+v", audit)
	}
	if audit.Positions[1].CVVCCandidateCount == 0 || !audit.Positions[1].SelectedComposite || audit.Positions[1].SelectedTransition != "a k" {
		t.Fatalf("position audit = %#v", audit.Positions[1])
	}
}

func TestResolveCVOnlySuppressesVCVCandidates(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ"}},
		"a か": {{Alias: "a か"}},
		"あ":   {{Alias: "あ"}},
		"か":   {{Alias: "か"}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.ResolveWithConfig(morae, ResolveConfig{AliasPolicy: AliasPolicyCVOnly})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Alias != "あ" || got[1].Alias != "か" {
		t.Fatalf("cv-only selections = %#v", got)
	}
	if got[0].Kind != AliasCV || got[1].Kind != AliasCV {
		t.Fatalf("cv-only kinds = %#v", got)
	}
}

func TestResolveVCVPreferKeepsUsableVCVAboveCV(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ", Filename: "vcv-start.wav"}},
		"a か": {{Alias: "a か", Filename: "vcv.wav"}},
		"あ":   {{Alias: "あ", Filename: "cv.wav"}},
		"か":   {{Alias: "か", Filename: "cv.wav"}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.ResolveWithConfig(morae, ResolveConfig{AliasPolicy: AliasPolicyVCVPrefer})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Kind != AliasVCV || got[1].Kind != AliasVCV {
		t.Fatalf("vcv-prefer selections = %#v", got)
	}
}

func TestResolveVCVPreferFallsBackFromBrokenVCV(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ", Fixed: 0, Preutterance: 200, Overlap: 250, Offset: -1}},
		"a か": {{Alias: "a か", Fixed: 0, Preutterance: 200, Overlap: 250, Offset: -1}},
		"あ":   {{Alias: "あ", Fixed: 100, Preutterance: 50, Overlap: 10}},
		"か":   {{Alias: "か", Fixed: 100, Preutterance: 50, Overlap: 10}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.ResolveWithConfig(morae, ResolveConfig{AliasPolicy: AliasPolicyVCVPrefer})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Kind != AliasCV || got[1].Kind != AliasCV {
		t.Fatalf("broken VCV fallback = %#v", got)
	}
}

func TestAliasCandidatesHandleSpecialMoraContexts(t *testing.T) {
	contains := func(candidates []aliasCandidate, name string) bool {
		for _, candidate := range candidates {
			if candidate.name == name {
				return true
			}
		}
		return false
	}
	if !contains(aliasCandidatesWithPolicy("か", "n", false, AliasPolicyAuto), "n か") {
		t.Fatal("n-context VCV candidate was not generated")
	}
	if contains(aliasCandidatesWithPolicy("か", "cl", false, AliasPolicyAuto), "cl か") {
		t.Fatal("closure was incorrectly used as a VCV context")
	}
	if contains(aliasCandidatesWithPolicy("っ", "", true, AliasPolicyAuto), "- っ") {
		t.Fatal("closure was incorrectly offered as an initial VCV target")
	}
	if !contains(aliasCandidatesWithPolicy("ー", "u", false, AliasPolicyAuto), "u う") {
		t.Fatal("long-vowel candidate did not use the preceding vowel")
	}
	// 同音の仮名へフォールバックする。
	for mora, equivalent := range map[string]string{"を": "お", "ぢ": "じ", "づ": "ず", "ゐ": "い", "ゑ": "え"} {
		candidates := aliasCandidatesWithPolicy(mora, "", true, AliasPolicyAuto)
		if !contains(candidates, mora) {
			t.Fatalf("mora %q candidate was not generated", mora)
		}
		if !contains(candidates, equivalent) {
			t.Fatalf("mora %q did not fall back to equivalent %q", mora, equivalent)
		}
		if !contains(candidates, toKatakana(equivalent)) {
			t.Fatalf("mora %q did not fall back to katakana %q", mora, toKatakana(equivalent))
		}
	}
	// 元の表記を同音候補より先に試す。
	wo := aliasCandidatesWithPolicy("を", "", true, AliasPolicyAuto)
	originalIndex, fallbackIndex := -1, -1
	for index, candidate := range wo {
		if candidate.name == "を" {
			originalIndex = index
		}
		if candidate.name == "お" {
			fallbackIndex = index
		}
	}
	if originalIndex < 0 || fallbackIndex < 0 || originalIndex > fallbackIndex {
		t.Fatalf("を must precede お in candidates: %v", wo)
	}
	// 同音候補にはペナルティを付け、両方あれば元の表記を選ぶ。
	originalTier, fallbackTier := -1, -1
	for _, candidate := range wo {
		if candidate.name == "を" {
			originalTier = candidate.tier
		}
		if candidate.name == "お" {
			fallbackTier = candidate.tier
		}
	}
	if originalTier < 0 || fallbackTier <= originalTier {
		t.Fatalf("equivalent fallback must have a worse tier than the original: %v", wo)
	}
	// 小書き仮名の組み合わせは別音なのでフォールバックしない。
	if contains(aliasCandidatesWithPolicy("てぃ", "", true, AliasPolicyAuto), "ち") {
		t.Fatal("てぃ must not fall back to ち")
	}
}

func TestResolvePrefersOriginalKanaWhenBothRecordingsExist(t *testing.T) {
	morae, err := frontend.ParseKana("あを")
	if err != nil {
		t.Fatal(err)
	}
	both := &Bank{Entries: map[string][]oto.Entry{
		"あ":   {{Alias: "あ", Filename: "a.wav"}},
		"- あ": {{Alias: "- あ", Filename: "a-start.wav"}},
		"a あ": {{Alias: "a あ", Filename: "a-a.wav"}},
		"を":   {{Alias: "を", Filename: "wo.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
		"- を": {{Alias: "- を", Filename: "wo-start.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
		"a を": {{Alias: "a を", Filename: "a-wo.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
		"お":   {{Alias: "お", Filename: "o.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
		"- お": {{Alias: "- お", Filename: "o-start.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
		"a お": {{Alias: "a お", Filename: "a-o.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
	}}
	got, err := both.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("selections = %d, want 2", len(got))
	}
	if got[1].Alias != "a を" {
		t.Fatalf("alias = %q, want the dedicated を recording: %#v", got[1].Alias, got[1])
	}
	if got[1].FallbackTier != 0 {
		t.Fatalf("fallback tier = %d, want the original kana to keep the better tier: %#v", got[1].FallbackTier, got[1])
	}

	onlyO := &Bank{Entries: map[string][]oto.Entry{
		"あ":   {{Alias: "あ", Filename: "a.wav"}},
		"- あ": {{Alias: "- あ", Filename: "a-start.wav"}},
		"a あ": {{Alias: "a あ", Filename: "a-a.wav"}},
		"お":   {{Alias: "お", Filename: "o.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
		"- お": {{Alias: "- お", Filename: "o-start.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
		"a お": {{Alias: "a お", Filename: "a-o.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
	}}
	fallback, err := onlyO.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if fallback[1].Alias != "a お" {
		t.Fatalf("alias = %q, want the equivalent お fallback when を is absent: %#v", fallback[1].Alias, fallback[1])
	}
	if fallback[1].FallbackTier != 1 {
		t.Fatalf("fallback tier = %d, want 1 for the equivalent alias: %#v", fallback[1].FallbackTier, fallback[1])
	}
}

func TestResolvePrefersOriginalCVOverEquivalentVCV(t *testing.T) {
	morae, err := frontend.ParseKana("あを")
	if err != nil {
		t.Fatal(err)
	}
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ", Filename: "a-start.wav"}},
		"を":   {{Alias: "を", Filename: "wo.wav"}},
		"a お": {{Alias: "a お", Filename: "a-o.wav", Preutterance: 30, Overlap: 20, Fixed: 40}},
	}}

	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Alias != "を" {
		t.Fatalf("alias = %q, want the dedicated CV recording を: %#v", got[1].Alias, got[1])
	}

	bank.Root = "."
	bank.Entries["を"] = []oto.Entry{{Alias: "を", OtoPath: "oto.ini"}}
	fallback, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if fallback[1].Alias != "a お" {
		t.Fatalf("alias = %q, want the equivalent VCV fallback when を is unusable: %#v", fallback[1].Alias, fallback[1])
	}
}

func TestAuditLatticeReportsAliasKindsAndSelection(t *testing.T) {
	bank := &Bank{Root: "bank", Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ", Filename: "vcv-start.wav"}},
		"a か": {{Alias: "a か", Filename: "vcv.wav"}},
		"あ":   {{Alias: "あ", Filename: "cv.wav"}},
		"か":   {{Alias: "か", Filename: "cv.wav"}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	audit, err := bank.AuditLattice(morae, "C4", nil)
	if err != nil {
		t.Fatal(err)
	}
	if audit.VCVSelectedPositions != 2 || audit.CVSelectedPositions != 0 {
		t.Fatalf("selection counts = %+v", audit)
	}
	if len(audit.Positions) != 2 || audit.Positions[0].SelectedAliasKind != string(AliasVCV) || audit.Positions[1].VCVCandidateCount == 0 || audit.Positions[1].CVCandidateCount == 0 {
		t.Fatalf("position audit = %#v", audit.Positions)
	}
}

func TestResolveResetsContextAtPause(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ"}},
		"- い": {{Alias: "- い"}},
	}}
	morae, err := frontend.ParseKana("あ、い")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Alias != "- い" {
		t.Fatalf("second alias = %q", got[1].Alias)
	}
}

func TestResolveReturnsMissingAlias(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	morae, _ := frontend.ParseKana("あ")
	_, err := bank.Resolve(morae)
	var missing *MissingAliasError
	if !errors.As(err, &missing) || missing.Position != 0 {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveLongMarkUsesPreviousVowel(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"す":   {{Alias: "す"}},
		"u う": {{Alias: "u う"}},
	}}
	morae, _ := frontend.ParseKana("スー")
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Mora.Text != "ー" || got[1].Alias != "u う" {
		t.Fatalf("long vowel selection = %+v", got[1])
	}
}

func TestResolveScoresDuplicateEntriesByOtoConsistency(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {
			{Alias: "- あ", Filename: "broken.wav", Fixed: 20, Preutterance: 80, Overlap: 100},
			{Alias: "- あ", Filename: "usable.wav", Fixed: 100, Preutterance: 60, Overlap: 20},
		},
	}}
	morae, _ := frontend.ParseKana("あ")
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Entry.Filename != "usable.wav" || got[0].TargetScore == 0 {
		t.Fatalf("selection = %#v", got[0])
	}
}

func TestResolveCanRejectBrokenVCVForCVFallback(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ", Fixed: 100, Preutterance: 60, Overlap: 20}},
		"a い": {{Alias: "a い", Filename: "broken.wav", Fixed: 0, Preutterance: 200, Overlap: 250}},
		"い":   {{Alias: "い", Filename: "cv.wav", Fixed: 100, Preutterance: 50, Overlap: 10}},
	}}
	morae, _ := frontend.ParseKana("あい")
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Alias != "い" {
		t.Fatalf("fallback = %#v", got[1])
	}
}

func TestResolveUsesPhrasePathInsteadOfGreedyDuplicateChoice(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {
			{Alias: "- あ", Filename: "isolated.wav", Fixed: 100, Preutterance: 60, Overlap: 20},
			{Alias: "- あ", Filename: "continuous.wav", Offset: 10, Fixed: 100, Preutterance: 60, Overlap: 20},
		},
		"a い": {{Alias: "a い", Filename: "continuous.wav", Offset: 200, Fixed: 100, Preutterance: 60, Overlap: 20}},
	}}
	morae, _ := frontend.ParseKana("あい")
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Entry.Filename != "continuous.wav" {
		t.Fatalf("path did not retain source continuity: %#v", got)
	}
	if got[0].CandidateCount != 2 || got[0].TargetScore != 114 || got[0].JoinScore != 0 || got[0].PathScore != 114 {
		t.Fatalf("first score audit = %#v", got[0])
	}
	if got[1].JoinScore != 8 || got[1].TargetScore+got[1].JoinScore != 122 || got[1].PathScore != 236 {
		t.Fatalf("second score audit = %#v", got[1])
	}

	greedy, err := bank.ResolveWithConfig(morae, ResolveConfig{Mode: SelectionGreedy})
	if err != nil {
		t.Fatal(err)
	}
	if greedy[0].Entry.Filename != "isolated.wav" || greedy[1].JoinScore != 0 || greedy[1].PathScore != 228 {
		t.Fatalf("greedy path = %#v", greedy)
	}

	targetOnly, err := bank.ResolveWithConfig(morae, ResolveConfig{Mode: SelectionTargetOnly})
	if err != nil {
		t.Fatal(err)
	}
	if targetOnly[0].Entry.Filename != "isolated.wav" || targetOnly[1].JoinScore != 0 {
		t.Fatalf("target-only path = %#v", targetOnly)
	}

	scales := make([]float64, 14)
	for index := range scales {
		scales[index] = 1
	}
	learned, err := bank.ResolveWithConfig(morae, ResolveConfig{
		Mode: SelectionViterbi,
		JoinModel: &connection.LearnedModel{
			Means: make([]float64, 14), Scales: scales, Weights: make([]float64, 14),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if learned[0].Entry.Filename != "continuous.wav" || learned[1].JoinScore != 8 || learned[1].JoinProbability != 0.5 {
		t.Fatalf("learned path audit = %#v", learned)
	}
}

func TestResolveRejectsUnknownSelectionMode(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	morae, _ := frontend.ParseKana("あ")
	if _, err := bank.ResolveWithConfig(morae, ResolveConfig{Mode: "unknown"}); err == nil {
		t.Fatal("unknown selection mode was accepted")
	}
}

func TestResolveUsesSilentClosureWhenVoicebankHasNoSmallTsu(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Alias: "- あ", Filename: "a.wav"}},
		"か":   {{Alias: "か", Filename: "ka.wav"}},
	}}
	morae, err := frontend.ParseKana("あっか")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[1].Alias != "<closure>" || got[1].Entry.Filename != "" {
		t.Fatalf("selections=%#v", got)
	}
}

func TestAliasCandidateTiersDoNotPenalizeScriptVariants(t *testing.T) {
	candidates := aliasCandidates("こ", "", true)
	want := map[string]int{"- こ": 0, "- コ": 0, "こ": 1, "コ": 1}
	for _, candidate := range candidates {
		if tier, ok := want[candidate.name]; ok {
			if candidate.tier != tier {
				t.Errorf("candidate %q tier=%d, want %d", candidate.name, candidate.tier, tier)
			}
			delete(want, candidate.name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing candidates: %v", want)
	}
}

func TestJoinScorePrefersMatchingAcousticBoundary(t *testing.T) {
	dir := t.TempDir()
	previousPath := filepath.Join(dir, "previous.wav")
	matchingPath := filepath.Join(dir, "matching.wav")
	mismatchPath := filepath.Join(dir, "mismatch.wav")
	writeResolverTone(t, previousPath, 200)
	writeResolverTone(t, matchingPath, 200)
	writeResolverTone(t, mismatchPath, 400)
	entry := func(path string) oto.Entry {
		return oto.Entry{Filename: path, Preutterance: 30, Overlap: 10}
	}
	cache := connection.NewExtractor()
	matching := joinScore(entry(previousPath), entry(matchingPath), cache)
	mismatch := joinScore(entry(previousPath), entry(mismatchPath), cache)
	if matching <= mismatch {
		t.Fatalf("matching score %.3f <= mismatching score %.3f", matching, mismatch)
	}
}

func writeResolverTone(t *testing.T, path string, hz float64) {
	t.Helper()
	const sampleRate = 16000
	data := make([]int16, sampleRate/3)
	for i := range data {
		data[i] = int16(8000 * math.Sin(2*math.Pi*hz*float64(i)/sampleRate))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
}
