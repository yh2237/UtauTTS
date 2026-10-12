package main

// JSON要求に併用できるのは資源配置・出力・実行制御だけ。合成・編集設定はJSONで指定する。
var requestTransportFlags = map[string]bool{
	"request": true, "json": true, "version": true, "timeout": true,
	"voicebank": true, "renderer-dir": true, "model-dir": true,
	"worldline-bridge": true, "openjtalk-features": true, "openjtalk-dictionary": true,
	"out": true, "plan-out": true, "ustx-out": true,
	"write-text": true, "write-lab": true, "text-encoding": true,
}
