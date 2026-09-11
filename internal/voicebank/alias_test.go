package voicebank

import (
	"strings"
	"testing"

	"utautts/internal/oto"
)

func TestClassifyAlias(t *testing.T) {
	tests := map[string]AliasKind{
		"あ":     AliasCV,
		"きゃ":    AliasCV,
		"- あ":   AliasVCV,
		"a か":   AliasVCV,
		"n だ":   AliasVCV,
		"* あ":   AliasCV,
		"あ k":   AliasVC,
		"a k":   AliasVC,
		"R":     AliasOther,
		"pau":   AliasOther,
		"a b c": AliasOther,
	}
	for alias, want := range tests {
		if got := ClassifyAlias(alias); got != want {
			t.Errorf("ClassifyAlias(%q) = %q, want %q", alias, got, want)
		}
	}
}

func TestAliasPolicyValues(t *testing.T) {
	for _, policy := range []AliasPolicy{AliasPolicyAuto, AliasPolicyVCVPrefer, AliasPolicyCVVCPrefer, AliasPolicyCVOnly} {
		if !policy.valid() {
			t.Errorf("policy %q was rejected", policy)
		}
	}
	if AliasPolicy("invalid").valid() {
		t.Fatal("invalid alias policy was accepted")
	}
}

func TestAliasCapabilitiesSummarizeVCVContexts(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"あ":   {{Alias: "あ"}},
		"- あ": {{Alias: "- あ"}},
		"a か": {{Alias: "a か"}},
		"n だ": {{Alias: "n だ"}},
		"あ k": {{Alias: "あ k"}},
	}}
	capabilities := bank.AliasCapabilities()
	if !capabilities.HasVCV || !capabilities.HasInitialVCV || !capabilities.HasNContextVCV {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	if capabilities.Counts[AliasCV] != 1 || capabilities.Counts[AliasVCV] != 3 || capabilities.Counts[AliasVC] != 1 {
		t.Fatalf("counts = %#v", capabilities.Counts)
	}
	if capabilities.VCVContexts["a"] != 1 || capabilities.VCVContexts["n"] != 1 || capabilities.VCVContexts["-"] != 1 {
		t.Fatalf("contexts = %#v", capabilities.VCVContexts)
	}
	if capabilities.InitialAliases != 1 || capabilities.ContextVCV != 2 {
		t.Fatalf("vcv context split = %+v", capabilities)
	}
	if !capabilities.HasVC || capabilities.VCContexts["あ"] != 1 {
		t.Fatalf("vc capabilities = %+v", capabilities)
	}
}

func TestInitialContextDoesNotCountAsRealVCV(t *testing.T) {
	if !IsInitialContextAlias("- か") || !IsInitialContextAlias("- か C4") || IsInitialContextAlias("a か") {
		t.Fatal("initial alias classification failed")
	}
	if !IsContextVCVAlias("a か") || !IsContextVCVAlias("a か C4") || IsContextVCVAlias("- か") {
		t.Fatal("context VCV classification failed")
	}
	selections := []Selection{
		{Alias: "- か C4"},
		{Alias: "き", Kind: AliasCV},
	}
	if !IsSingleCVSelections(selections) {
		t.Fatal("initial CV selection was not recognized")
	}
	selections[1] = Selection{Alias: "a き", Kind: AliasVCV}
	if IsSingleCVSelections(selections) {
		t.Fatal("real VCV selection was treated as standalone CV")
	}
}

func TestAliasCapabilitiesRecognizeToneSuffixedContext(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- か C4": {{Alias: "- か C4"}},
		"a か C4": {{Alias: "a か C4"}},
	}}
	capabilities := bank.AliasCapabilities()
	if capabilities.InitialAliases != 1 || capabilities.ContextVCV != 1 || capabilities.Counts[AliasVCV] != 2 || !capabilities.HasInitialVCV || !capabilities.HasVCV {
		t.Fatalf("tone-suffixed context = %+v", capabilities)
	}
}

func TestRecommendCVVCEnhancedUsesInventoryBalance(t *testing.T) {
	makeBank := func(vc, vcv int) *Bank {
		bank := &Bank{Entries: map[string][]oto.Entry{}}
		for index := 0; index < vc; index++ {
			bank.Entries[strings.Repeat("あ", index+1)+" k"] = nil
		}
		for index := 0; index < vcv; index++ {
			bank.Entries["a "+strings.Repeat("あ", index+1)] = nil
		}
		return bank
	}
	if !makeBank(180, 190).RecommendCVVCEnhanced() {
		t.Fatal("balanced CVVC inventory was not recognized")
	}
	if makeBank(126, 4736).RecommendCVVCEnhanced() {
		t.Fatal("VCV-dominant inventory was incorrectly recognized as CVVC")
	}
	if makeBank(1, 0).RecommendCVVCEnhanced() {
		t.Fatal("tiny incidental VC inventory was incorrectly recognized as CVVC")
	}
}
