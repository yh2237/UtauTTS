package openutau

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ProjectAudit struct {
	Path               string              `json:"path"`
	SHA256             string              `json:"sha256"`
	Name               string              `json:"name,omitempty"`
	USTXVersion        string              `json:"ustx_version,omitempty"`
	Resolution         int                 `json:"resolution,omitempty"`
	BPM                float64             `json:"bpm,omitempty"`
	ExpressionDefaults map[string]float64  `json:"expression_defaults,omitempty"`
	Tracks             []TrackRendererInfo `json:"tracks"`
}

type TrackRendererInfo struct {
	Index      int    `json:"index"`
	Name       string `json:"name,omitempty"`
	Singer     string `json:"singer,omitempty"`
	Phonemizer string `json:"phonemizer,omitempty"`
	Renderer   string `json:"renderer,omitempty"`
	Resampler  string `json:"resampler,omitempty"`
	Wavtool    string `json:"wavtool,omitempty"`
}

func InspectProject(path string) (*ProjectAudit, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OpenUtau project: %w", err)
	}
	sum := sha256.Sum256(data)
	result := &ProjectAudit{
		Path: path, SHA256: hex.EncodeToString(sum[:]),
		ExpressionDefaults: map[string]float64{},
	}

	section := ""
	expression := ""
	trackIndex := -1
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		raw := strings.TrimPrefix(strings.TrimSuffix(scanner.Text(), "\r"), "\ufeff")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if indent == 0 && !strings.HasPrefix(trimmed, "-") {
			key, value, ok := splitYAMLField(trimmed)
			if ok && value == "" {
				section = key
				expression = ""
				continue
			}
			if ok {
				switch key {
				case "name":
					result.Name = yamlScalar(value)
				case "ustx_version":
					result.USTXVersion = yamlScalar(value)
				case "resolution":
					result.Resolution, _ = strconv.Atoi(yamlScalar(value))
				case "bpm":
					result.BPM, _ = strconv.ParseFloat(yamlScalar(value), 64)
				}
			}
		}

		switch section {
		case "expressions":
			if indent == 2 && strings.HasSuffix(trimmed, ":") {
				expression = strings.TrimSuffix(trimmed, ":")
				continue
			}
			if indent == 4 && expression != "" {
				key, value, ok := splitYAMLField(trimmed)
				if ok && key == "default_value" {
					if number, parseErr := strconv.ParseFloat(yamlScalar(value), 64); parseErr == nil {
						result.ExpressionDefaults[expression] = number
					}
				}
			}
		case "tracks":
			if indent == 0 && strings.HasPrefix(trimmed, "-") {
				result.Tracks = append(result.Tracks, TrackRendererInfo{Index: len(result.Tracks)})
				trackIndex = len(result.Tracks) - 1
				trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			}
			if trackIndex < 0 {
				continue
			}
			key, value, ok := splitYAMLField(trimmed)
			if !ok {
				continue
			}
			track := &result.Tracks[trackIndex]
			switch key {
			case "singer":
				track.Singer = yamlScalar(value)
			case "phonemizer":
				track.Phonemizer = yamlScalar(value)
			case "track_name":
				track.Name = yamlScalar(value)
			case "renderer":
				track.Renderer = yamlScalar(value)
			case "resampler":
				track.Resampler = yamlScalar(value)
			case "wavtool":
				track.Wavtool = yamlScalar(value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan OpenUtau project: %w", err)
	}
	return result, nil
}

func splitYAMLField(line string) (string, string, bool) {
	index := strings.IndexByte(line, ':')
	if index < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+1:]), true
}

func yamlScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if decoded, err := strconv.Unquote(value); err == nil {
			return decoded
		}
	}
	return strings.Trim(value, "'")
}
