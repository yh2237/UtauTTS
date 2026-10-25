// Package g2pは、言語ごとの読みの前処理をまとめる。
// 数字・通貨・外来語・混在文字を扱い、音素化そのものはinternal/frontendの言語別実装に任せる。
// internal/frontendやvoicebankをimportしないので、あとから別リポジトリへ移せる。
package g2p

// Languageは読みの言語。
type Language string

const (
	Japanese   Language = "ja"
	English    Language = "en"
	Chinese    Language = "zh"
	Spanish    Language = "es"
	Portuguese Language = "pt"
)

// Normalizeは、その言語の読みに渡す前に数字・通貨・記号を語へ広げる。
// いまはポルトガル語だけ実装している。ほかの言語はfrontend側の既存処理を使う。
func Normalize(language Language, text string) string {
	switch language {
	case Portuguese:
		return normalizePortuguese(text)
	case Spanish:
		return normalizeSpanish(text)
	default:
		return text
	}
}
