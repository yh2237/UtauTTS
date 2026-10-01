package openjtalk

import (
	"reflect"
	"strings"
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

// 実際のOpen JTalk出力（pyopenjtalk 0.4.1）。
var auxiliaryFixtures = []struct {
	text    string
	rows    []string
	phrases string
}{
	{
		// 前の句に核がある: 補助動詞の核は消える。
		text: "雨が降ってきた。",
		rows: []string{
			"雨\t名詞\t一般\t*\t*\t雨\tアメ\tアメ\t1\t2\tC3\t-1",
			"が\t助詞\t格助詞\t*\t*\tが\tガ\tガ\t0\t1\t名詞%F1\t1",
			"降っ\t動詞\t自立\t五段・ラ行\t連用タ接続\t降る\tフッ\tフッ\t1\t2\t*\t0",
			"て\t助詞\t接続助詞\t*\t*\tて\tテ\tテ\t0\t1\t動詞%F1/形容詞%F1/名詞%F5\t1",
			"き\t動詞\t非自立\tカ変・クル\t連用形\tくる\tキ\tキ\t1\t1\t*\t0",
			"た\t助動詞\t*\t特殊・タ\t基本形\tた\tタ\tタ\t0\t1\t動詞%F2@1/形容詞%F4@-2\t1",
			"。\t記号\t句点\t*\t*\t。\t、\t、\t0\t0\t*\t0",
		},
		phrases: "あ＼め|が/ふ＼っ|て|き|た",
	},
	{
		// 前の句が平板: 補助動詞の核を句内の位置へずらして残す。
		text: "遊んできた。",
		rows: []string{
			"遊ん\t動詞\t自立\t五段・バ行\t連用タ接続\t遊ぶ\tアソン\tアソン\t0\t3\t*\t-1",
			"で\t助詞\t接続助詞\t*\t*\tで\tデ\tデ\t1\t1\t動詞%F1\t1",
			"き\t動詞\t非自立\tカ変・クル\t連用形\tくる\tキ\tキ\t1\t1\t*\t0",
			"た\t助動詞\t*\t特殊・タ\t基本形\tた\tタ\tタ\t0\t1\t動詞%F2@1/形容詞%F4@-2\t1",
			"。\t記号\t句点\t*\t*\t。\t、\t、\t0\t0\t*\t0",
		},
		phrases: "あそん|で|き＼|た",
	},
	{
		// 補助動詞の後に助動詞が続く。
		text: "だんだん寒くなってきました。",
		rows: []string{
			"だんだん\t副詞\t一般\t*\t*\tだんだん\tダンダン\tダンダン\t0\t4\t*\t-1",
			"寒く\t形容詞\t自立\t形容詞・アウオ段\t連用テ接続\t寒い\tサムク\tサムク\t2\t3\t*\t0",
			"なっ\t動詞\t自立\t五段・ラ行\t連用タ接続\tなる\tナッ\tナッ\t1\t2\t*\t1",
			"て\t助詞\t接続助詞\t*\t*\tて\tテ\tテ\t0\t1\t動詞%F1/形容詞%F1/名詞%F5\t1",
			"き\t動詞\t非自立\tカ変・クル\t連用形\tくる\tキ\tキ\t2\t1\t*\t0",
			"まし\t助動詞\t*\t特殊・マス\t連用形\tます\tマシ\tマシ’\t1\t2\t動詞%F4@1/助詞%F2@1\t1",
			"た\t助動詞\t*\t特殊・タ\t基本形\tた\tタ\tタ\t0\t1\t動詞%F2@1/形容詞%F4@-2\t1",
			"。\t記号\t句点\t*\t*\t。\t、\t、\t0\t0\t*\t0",
		},
		phrases: "だんだん/さむ＼く|なっ|て|き|まし|た",
	},
	{
		// 自立動詞（いただける）はつなげない。
		text: "教えていただけますか。",
		rows: []string{
			"教え\t動詞\t自立\t一段\t連用形\t教える\tオシエ\tオシエ\t0\t3\t*\t-1",
			"て\t助詞\t接続助詞\t*\t*\tて\tテ\tテ\t0\t1\t動詞%F1/形容詞%F1/名詞%F5\t1",
			"いただけ\t動詞\t自立\t一段\t連用形\tいただける\tイタダケ\tイタダケ\t5\t4\t*\t0",
			"ます\t助動詞\t*\t特殊・マス\t基本形\tます\tマス\tマス’\t1\t2\t動詞%F4@1/助詞%F2@1\t1",
			"か\t助詞\t副助詞／並立助詞／終助詞\t*\t*\tか\tカ\tカ\t0\t1\t名詞%F1/動詞%F2@0/形容詞%F2@0\t1",
			"。\t記号\t句点\t*\t*\t。\t、\t、\t0\t0\t*\t0",
		},
		phrases: "おしえ|て/いただけ|ま＼す|か",
	},
	{
		// サ変名詞＋する: 補助動詞をつないだ後のするの句（していま＼す）に核があるのでつなぐ。
		text: "運転しています。",
		rows: []string{
			"運転	名詞	サ変接続	*	*	運転	ウンテン	ウンテン	0	4	C2	-1",
			"し	動詞	自立	サ変・スル	連用形	する	シ	シ’	0	1	*	0",
			"て	助詞	接続助詞	*	*	て	テ	テ	0	1	動詞%F1/形容詞%F1/名詞%F5	1",
			"い	動詞	非自立	一段	連用形	いる	イ	イ	2	1	*	0",
			"ます	助動詞	*	特殊・マス	基本形	ます	マス	マス’	1	2	動詞%F4@1/助詞%F2@1	1",
			"。	記号	句点	*	*	。	、	、	0	0	*	0",
		},
		phrases: "うんてん|し|て|い|ま＼す",
	},
	{
		// するの句が平板ならつながない。
		text: "服従するより。",
		rows: []string{
			"服従	名詞	サ変接続	*	*	服従	フクジュウ	フ’クジュー	0	4	C2	-1",
			"する	動詞	自立	サ変・スル	基本形	する	スル	スル	0	2	*	0",
			"より	助詞	格助詞	*	*	より	ヨリ	ヨリ	1	2	名詞%F2@1	1",
			"。	記号	句点	*	*	。	、	、	0	0	*	0",
		},
		phrases: "ふくじゅー/する|より",
	},
	{
		// 平板の句の「には」: 格助詞「に」へ核を置く。
		text: "性質には。",
		rows: []string{
			"性質	名詞	一般	*	*	性質	セイシツ	セーシ’ツ	0	4	C2	-1",
			"に	助詞	格助詞	*	*	に	ニ	ニ	0	1	動詞%F5/形容詞%F1/名詞%F1	1",
			"は	助詞	係助詞	*	*	は	ハ	ワ	0	1	名詞%F1/動詞%F2@0/形容詞%F2@0	1",
			"。	記号	句点	*	*	。	、	、	0	0	*	0",
		},
		phrases: "せーしつ|に＼|わ",
	},
	{
		// 平板の句の「ても」: 接続助詞「で」へ核を置く。
		text: "遊んでも。",
		rows: []string{
			"遊ん	動詞	自立	五段・バ行	連用タ接続	遊ぶ	アソン	アソン	0	3	*	-1",
			"で	助詞	接続助詞	*	*	で	デ	デ	1	1	動詞%F1	1",
			"も	助詞	係助詞	*	*	も	モ	モ	0	1	名詞%F1/動詞%F2@0/形容詞%F2@0	1",
			"。	記号	句点	*	*	。	、	、	0	0	*	0",
		},
		phrases: "あそん|で＼|も",
	},
	{
		// 核のある句は変えない。
		text: "雨が降っても。",
		rows: []string{
			"雨	名詞	一般	*	*	雨	アメ	アメ	1	2	C3	-1",
			"が	助詞	格助詞	*	*	が	ガ	ガ	0	1	名詞%F1	1",
			"降っ	動詞	自立	五段・ラ行	連用タ接続	降る	フッ	フッ	1	2	*	0",
			"て	助詞	接続助詞	*	*	て	テ	テ	0	1	動詞%F1/形容詞%F1/名詞%F5	1",
			"も	助詞	係助詞	*	*	も	モ	モ	0	1	名詞%F1/動詞%F2@0/形容詞%F2@0	1",
			"。	記号	句点	*	*	。	、	、	0	0	*	0",
		},
		phrases: "あ＼め|が/ふ＼っ|て|も",
	},
}

// renderPhrasesはアクセント句を'/'、語を'|'で区切り、核の直後に'＼'を置く。
func renderPhrases(tokens []moraToken) string {
	var builder strings.Builder
	for index, token := range tokens {
		if token.Pause {
			continue
		}
		if index > 0 && token.AccentPhraseStart {
			builder.WriteString("/")
		} else if index > 0 && token.WordStart {
			builder.WriteString("|")
		}
		builder.WriteString(token.Mora)
		if token.AccentPhrasePosition == token.AccentNucleus {
			builder.WriteString("＼")
		}
	}
	return builder.String()
}

func TestRefineAccentPhrases(t *testing.T) {
	for _, fixture := range auxiliaryFixtures {
		nodes, err := parseNJD(strings.Join(fixture.rows, "\n") + "\n")
		if err != nil {
			t.Fatalf("%s: parseNJD: %v", fixture.text, err)
		}
		refineAccentPhrases(nodes)
		_, tokens := analyzeNJD(nodes)
		if got := renderPhrases(tokens); got != fixture.phrases {
			t.Errorf("%s: phrases = %s, want %s", fixture.text, got, fixture.phrases)
		}
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
