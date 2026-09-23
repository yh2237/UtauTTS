package render

import (
	"os"
	"strings"
)

// E2a/E2bは聴取A/B用の開発スイッチ。既定は両方ON。
var (
	e2aEnabled = featureEnabledFromEnv("UTAUTTS_E2A", true)
	e2bEnabled = featureEnabledFromEnv("UTAUTTS_E2B", true)
)

func featureEnabledFromEnv(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

// E2AEnabledは英語codaの閉鎖/解放分離が有効かを返す。
func E2AEnabled() bool { return e2aEnabled }

// SetE2AはE2aを切り替える。既定はON。
func SetE2A(enabled bool) { e2aEnabled = enabled }

// E2BEnabledは過渡音ゲートの一般化が有効かを返す。
func E2BEnabled() bool { return e2bEnabled }

// SetE2BはE2bを切り替える。既定はON。
func SetE2B(enabled bool) { e2bEnabled = enabled }
