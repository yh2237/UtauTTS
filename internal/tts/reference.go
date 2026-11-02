package tts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ReferenceDirはモデルのフォルダの下で、参照照合のモデル（参照TTSと特徴量のモデル）を置くフォルダ。
const ReferenceDir = "reference"

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
	have := referenceFiles(dir)
	var providers []ReferenceProviderInfo
	if have("kokoro-v1_0.pth") {
		providers = append(providers, ReferenceProviderInfo{ID: "kokoro-en", Language: "en", Name: "Kokoro-82M"})
	}
	if have("kokoro-zh/kokoro-v1_1-zh.pth") {
		providers = append(providers, ReferenceProviderInfo{ID: "kokoro-zh", Language: "zh", Name: "Kokoro-82M v1.1-zh"})
	}
	for _, voice := range []struct{ id, language string }{{"kokoro-es", "es"}, {"kokoro-pt", "pt"}} {
		if have("kokoro-v1_0.pth") && have(sharedReferenceVoices[voice.id]) {
			providers = append(providers, ReferenceProviderInfo{ID: voice.id, Language: voice.language, Name: "Kokoro-82M"})
		}
	}
	return providers
}

// referenceFilesは、参照照合のモデルのファイルがあるかを返す関数を作る。
// Web版は重みの代わりにinference.json（ONNX Runtime Webで読むモデルと声の名前）を置く。
func referenceFiles(dir string) func(name string) bool {
	var listed struct {
		Models []string `json:"models"`
		Voices []string `json:"voices"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "inference.json")); err == nil {
		_ = json.Unmarshal(data, &listed)
	}
	return func(name string) bool {
		if exists(filepath.Join(dir, name)) {
			return true
		}
		stem := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
		return slices.Contains(listed.Models, stem) || slices.Contains(listed.Voices, stem)
	}
}

// sharedReferenceVoicesは、英語と同じKokoro v1.0のモデルで読む言語の声のファイル。
var sharedReferenceVoices = map[string]string{"kokoro-es": "ef_dora.pt", "kokoro-pt": "pf_dora.pt"}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
