// Package entryinputは既存入口の互換規則を保って入力を変換する。
// 公開APIの厳密検証はルートパッケージが担当する。
package entryinput

import (
	utautts "github.com/yh2237/UtauTTS"
	"github.com/yh2237/UtauTTS/internal/synth"
)

// Applyは編集・評価設定を保ち、通常入力を加える。CLI・HTTPの休止長0は既定値扱いのまま。
func Apply(request utautts.Request, input synth.Request) synth.Request {
	input = synth.ApplyCommonOptions(input, synth.CommonOptions(request.Options))
	input.PauseDurationSet = false
	input.Text, input.Reading = request.Text, request.Reading
	input.Language, input.Phonemizer = request.Language, request.Phonemizer
	input.VoicebankID, input.Renderer, input.ModelID = request.VoicebankID, request.RendererID, request.ModelID
	input.Tone, input.Color = request.Tone, request.Color
	input.Method = request.Method
	input.Dictionary = make([]synth.DictionaryEntry, len(request.Dictionary))
	for i, entry := range request.Dictionary {
		input.Dictionary[i] = synth.DictionaryEntry{Surface: entry.Surface, Reading: entry.Reading}
	}
	// 共通設定を含め、mapの値を型付き互換フィールドより優先する。
	input.RendererSettings = request.RendererSettings
	return input
}
