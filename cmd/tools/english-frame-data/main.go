package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"utautts/internal/frontend"
)

// 整列済みARPAbetを合成と同じ音節にまとめる。
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var input struct {
			Reading string `json:"reading"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &input); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_, units, err := frontend.ParseEnglishDelta("", input.Reading, nil)
		if err != nil {
			encoder.Encode(map[string]any{"error": err.Error()})
			continue
		}
		var tokens []map[string]any
		for _, unit := range units {
			var phones []map[string]string
			for _, phone := range unit.Phones {
				phones = append(phones, map[string]string{"symbol": phone.Symbol, "role": phone.Role})
			}
			tokens = append(tokens, map[string]any{"language": "en", "pause": unit.Pause, "phones": phones, "stress": unit.Stress, "stress_known": unit.StressKnown, "word_index": unit.WordIndex, "word_end": unit.WordEnd})
		}
		encoder.Encode(map[string]any{"tokens": tokens})
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
