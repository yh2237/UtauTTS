//go:build js

package worldrender

// wasmのWORLDはJS側に読み込み済みなので、パスは使わない。
func ResolveEnginePath() (string, error) { return "wasm", nil }
