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
	if _, err := os.Stat(filepath.Join(dir, "kokoro-v1_0.pth")); err != nil {
		return nil
	}
	return []ReferenceProviderInfo{{ID: "kokoro-en", Language: "en", Name: "Kokoro-82M"}}
}
