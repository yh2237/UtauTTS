package voicebank

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type SourcePhoneInterval struct {
	Symbol  string  `json:"symbol"`
	StartMS float64 `json:"start_ms"`
	EndMS   float64 `json:"end_ms"`
}
type SourcePhoneRecord struct {
	SourceSHA256 string                `json:"source_sha256"`
	DurationMS   float64               `json:"duration_ms"`
	Phones       []SourcePhoneInterval `json:"phones"`
}
type SourcePhoneLibrary struct {
	Version    int                 `json:"version"`
	Language   string              `json:"language"`
	TimeOrigin string              `json:"time_origin"`
	Entries    []SourcePhoneRecord `json:"entries"`
}

// 不整合な区間ライブラリは使用しない。
func LoadSourcePhoneLibrary(path string) (*SourcePhoneLibrary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var library SourcePhoneLibrary
	if err = json.Unmarshal(data, &library); err != nil {
		return nil, err
	}
	if library.Version != 1 || library.TimeOrigin != "oto-offset" || (library.Language != "en" && library.Language != "zh") {
		return nil, fmt.Errorf("unsupported source phone library")
	}
	seen := map[string]bool{}
	for _, entry := range library.Entries {
		hash, err := hex.DecodeString(entry.SourceSHA256)
		if err != nil || len(hash) != 32 || seen[entry.SourceSHA256] || !sourceFinite(entry.DurationMS) || entry.DurationMS <= 0 || len(entry.Phones) == 0 {
			return nil, fmt.Errorf("invalid source identity")
		}
		seen[entry.SourceSHA256] = true
		previous := 0.0
		for _, p := range entry.Phones {
			if p.Symbol == "" || !sourceFinite(p.StartMS) || !sourceFinite(p.EndMS) || p.StartMS < previous-.001 || p.EndMS <= p.StartMS || p.EndMS > entry.DurationMS+1 {
				return nil, fmt.Errorf("invalid source phone interval")
			}
			previous = p.EndMS
		}
	}
	return &library, nil
}
func sourceFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
