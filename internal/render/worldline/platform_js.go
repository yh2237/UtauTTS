//go:build js

package worldline

// isWasmはブラウザ/Node向けビルドかを返す。ネイティブの外部ファイル解決を迂回するのに使う。
func isWasm() bool { return true }
