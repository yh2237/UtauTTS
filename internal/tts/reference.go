package tts

import (
	"os"
	"path/filepath"
)

// ReferenceProviderInfoは参照TTSの種類。画面の一覧に使う。
type ReferenceProviderInfo struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Name     string `json:"display_name"`
}

// ReferenceProvidersは、モデルが入っている参照TTSを返す。
// 参照モデルを増やしたら、ここに足す。
func ReferenceProviders(dir string) []ReferenceProviderInfo {
	if dir == "" {
		return nil
	}
	var providers []ReferenceProviderInfo
	if exists(filepath.Join(dir, "kokoro-v1_0.pth")) {
		providers = append(providers, ReferenceProviderInfo{ID: "kokoro-en", Language: "en", Name: "Kokoro-82M"})
	}
	if exists(filepath.Join(dir, "kokoro-zh", "kokoro-v1_1-zh.pth")) {
		providers = append(providers, ReferenceProviderInfo{ID: "kokoro-zh", Language: "zh", Name: "Kokoro-82M v1.1-zh"})
	}
	if exists(filepath.Join(dir, "kokoro-v1_0.pth")) && exists(filepath.Join(dir, sharedReferenceVoices["kokoro-es"])) {
		providers = append(providers, ReferenceProviderInfo{ID: "kokoro-es", Language: "es", Name: "Kokoro-82M"})
	}
	return providers
}

// sharedReferenceVoicesは、英語と同じKokoro v1.0のモデルで読む言語の声のファイル。
var sharedReferenceVoices = map[string]string{"kokoro-es": "ef_dora.pt"}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
