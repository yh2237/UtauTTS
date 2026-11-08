package main

import "fmt"

// JSON要求に併用できるのは資源配置・出力・実行制御だけ。合成・編集設定はJSONで指定する。
var requestTransportFlags = map[string]bool{
	"request": true, "json": true, "version": true, "timeout": true,
	"voicebank": true, "model-dir": true,
	"openjtalk-engine": true, "openjtalk-dictionary": true,
	"out": true, "plan-out": true, "ustx-out": true,
	"write-text": true, "write-lab": true, "text-encoding": true,
}

// requestJSONFieldsは--requestと併用できないフラグの、JSON側のフィールド名。
var requestJSONFields = map[string]string{
	"text": "text", "reading": "reading", "kana": "kana",
	"language": "language", "phonemizer": "phonemizer", "method": "method",
	"tone": "tone", "color": "color", "dictionary": "dictionary",
	"prosody": "model_id",
	"mora-ms": "mora_duration_ms", "pause-ms": "pause_duration_ms",
	"mora-durations": "mora_durations_ms", "phone-weights": "phone_weights",
	"manual-pitch": "manual_pitch", "alias-policy": "alias_policy",
	"leading-preutterance-ms": "leading_preutterance_ms", "release-ms": "release_ms",
	"intonation-strength": "intonation_strength", "apply-pitch": "apply_pitch",
	"context-duration": "context_duration", "boundary-tone": "boundary_tone",
	"stretch-adapt": "stretch_adapt", "pause-context": "pause_context",
	"context-duration-strength": "context_duration_strength",
	"boundary-tone-strength":    "boundary_tone_strength",
	"stretch-adapt-strength":    "stretch_adapt_strength",
	"pause-context-strength":    "pause_context_strength",
}

// requestJSONHintは--requestと併用できないフラグの置き換え先を返す。
func requestJSONHint(name string) string {
	if field, ok := requestJSONFields[name]; ok {
		return fmt.Sprintf("set %q in the request JSON instead", field)
	}
	return "put utterance settings in the request JSON"
}
