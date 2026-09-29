package openjtalk

import (
	"reflect"
	"testing"
)

// 実際のOpen JTalk wasm出力（こんにちは、今日はいい天気です。）をフィクスチャにする。
const njdFixture = "こんにちは\t感動詞\t*\t*\t*\tこんにちは\tコンニチハ\tコンニチワ\t0\t5\t-1\t-1\n" +
	"、\t記号\t読点\t*\t*\t、\t、\t、\t0\t0\t*\t0\n" +
	"今日\t名詞\t副詞可能\t*\t*\t今日\tキョウ\tキョー\t1\t2\tC3\t0\n" +
	"は\t助詞\t係助詞\t*\t*\tは\tハ\tワ\t0\t1\t名詞%F1/動詞%F2@0/形容詞%F2@0\t1\n" +
	"いい\t動詞\t自立\t五段・ワ行促音便\t連用形\tいう\tイイ\tイイ\t1\t2\t*\t0\n" +
	"天気\t名詞\t一般\t*\t*\t天気\tテンキ\tテンキ\t1\t3\tC1\t0\n" +
	"です\t助動詞\t*\t特殊・デス\t基本形\tです\tデス\tデス’\t1\t2\t名詞%F2@1/動詞%F1/形容詞%F2@0\t1\n" +
	"。\t記号\t句点\t*\t*\t。\t、\t、\t0\t0\t*\t0\n"

func TestBuildAnalysisFromNJD(t *testing.T) {
	nodes, err := parseNJD(njdFixture)
	if err != nil {
		t.Fatalf("parseNJD: %v", err)
	}
	if len(nodes) != 8 {
		t.Fatalf("nodes = %d, want 8", len(nodes))
	}
	analysis := buildAnalysis(nodes)
	if analysis.Version != 1 {
		t.Errorf("Version = %d, want 1", analysis.Version)
	}
	if want := "コンニチワ、キョーワイイテンキデス。"; analysis.Reading != want {
		t.Errorf("Reading = %q, want %q", analysis.Reading, want)
	}
	wantMorae := []string{"こ", "ん", "に", "ち", "わ", "", "きょ", "ー", "わ", "い", "い", "て", "ん", "き", "で", "す", ""}
	if !reflect.DeepEqual(analysis.Morae, wantMorae) {
		t.Errorf("Morae = %#v\nwant %#v", analysis.Morae, wantMorae)
	}
	if len(analysis.Features) != len(wantMorae) {
		t.Fatalf("Features = %d, want %d", len(analysis.Features), len(wantMorae))
	}

	// 「こ」はアクセント句先頭の平板。位置1は低く、位置2以降は高い。
	if got := analysis.Features[0]["accent_type=heiban"]; got != 1 {
		t.Errorf("features[0] heiban = %v, want 1", got)
	}
	if got := analysis.Features[0]["accent_high"]; got != 0 {
		t.Errorf("features[0] accent_high = %v, want 0", got)
	}
	if got := analysis.Features[1]["accent_high"]; got != 1 {
		t.Errorf("features[1] accent_high = %v, want 1", got)
	}
	if got := analysis.Features[0]["pos=感動詞"]; got != 1 {
		t.Errorf("features[0] pos=感動詞 = %v, want 1", got)
	}

	// 「きょ」はアクセント核(1)の位置。
	if got := analysis.Features[6]["accent_type=nucleus"]; got != 1 {
		t.Errorf("features[6] nucleus = %v, want 1", got)
	}

	// ポーズは空フレーム。
	if len(analysis.Features[5]) != 0 {
		t.Errorf("pause features = %v, want empty", analysis.Features[5])
	}
}

func TestParseNJDRejectsBadFieldCount(t *testing.T) {
	if _, err := parseNJD("a\tb\tc\n"); err == nil {
		t.Fatal("expected an error for a short NJD row")
	}
}

func TestSplitMorae(t *testing.T) {
	cases := []struct {
		reading string
		morae   []string
		vowels  []string
	}{
		{"こんにちは", []string{"こ", "ん", "に", "ち", "は"}, []string{"o", "n", "i", "i", "a"}},
		{"きょう", []string{"きょ", "う"}, []string{"o", "u"}},
		{"キョー", []string{"きょ", "ー"}, []string{"o", "o"}},
		{"がっこう", []string{"が", "っ", "こ", "う"}, []string{"a", "cl", "o", "u"}},
	}
	for _, testCase := range cases {
		result := splitMorae(testCase.reading)
		var morae, vowels []string
		for _, mora := range result {
			morae = append(morae, mora.mora)
			vowels = append(vowels, mora.vowel)
		}
		if !reflect.DeepEqual(morae, testCase.morae) {
			t.Errorf("splitMorae(%q) morae = %#v, want %#v", testCase.reading, morae, testCase.morae)
		}
		if !reflect.DeepEqual(vowels, testCase.vowels) {
			t.Errorf("splitMorae(%q) vowels = %#v, want %#v", testCase.reading, vowels, testCase.vowels)
		}
	}
}

func TestSparseFeaturesAccentTypes(t *testing.T) {
	heiban := moraToken{Pause: false, AccentPhrasePosition: 1, AccentPhraseLength: 3, AccentNucleus: 0, Pos: "名詞", PosGroup1: "一般"}
	if got := heiban.sparseFeatures()["accent_type=heiban"]; got != 1 {
		t.Errorf("heiban = %v, want 1", got)
	}
	before := moraToken{Pause: false, AccentPhrasePosition: 1, AccentPhraseLength: 3, AccentNucleus: 2}
	if got := before.sparseFeatures()["accent_type=before"]; got != 1 {
		t.Errorf("before = %v, want 1", got)
	}
	nucleus := moraToken{Pause: false, AccentPhrasePosition: 2, AccentPhraseLength: 3, AccentNucleus: 2}
	if got := nucleus.sparseFeatures()["accent_type=nucleus"]; got != 1 {
		t.Errorf("nucleus = %v, want 1", got)
	}
	after := moraToken{Pause: false, AccentPhrasePosition: 3, AccentPhraseLength: 3, AccentNucleus: 2}
	if got := after.sparseFeatures()["accent_type=after"]; got != 1 {
		t.Errorf("after = %v, want 1", got)
	}
}
