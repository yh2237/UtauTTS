package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
	"utautts/internal/audio"
	"utautts/internal/openjtalk"
)

func TestUniformPythonParity(t *testing.T) {
	raw, e := os.ReadFile("../../../out/python-migration/jsut-frame-py.jsonl")
	if e != nil {
		t.Skip("optional real corpus parity fixture absent")
	}
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var row struct {
			Text             string           `json:"text"`
			AudioPath        string           `json:"audio_path"`
			Tokens           []map[string]any `json:"tokens"`
			OpenJTalkReading string           `json:"openjtalk_reading"`
		}
		if e := json.Unmarshal(line, &row); e != nil {
			t.Fatal(e)
		}
		a, e := openjtalk.Analyze(row.Text, openjtalk.Config{HelperPath: "../../../tools/openjtalk-feature-bridge/bin/utautts-openjtalk-features.exe", DictionaryPath: "../../../.tmp-openjtalk/pyopenjtalk/open_jtalk_dic_utf_8-1.11"})
		if e != nil {
			t.Fatal(e)
		}
		if a.Reading != row.OpenJTalkReading {
			t.Fatal("reading differs")
		}
		wav, e := audio.ReadWav(row.AudioPath)
		if e != nil {
			t.Fatal(e)
		}
		start, end, e := activeBounds(wav.Data, wav.SampleRate)
		if e != nil {
			t.Fatal(e)
		}
		got, e := tokensFor(a, start, end)
		if e != nil {
			t.Fatal(e)
		}
		if len(got) != len(row.Tokens) {
			t.Fatal("token count differs")
		}
		for i, want := range row.Tokens {
			for k, v := range want {
				actual, ok := got[i][k]
				if !ok {
					t.Fatalf("token %d missing %s", i, k)
				}
				if k == "start_ms" || k == "end_ms" || k == "duration_ms" {
					if math.Abs(actual.(float64)-v.(float64)) > 1e-8 {
						t.Fatalf("token %d %s: %v vs %v", i, k, actual, v)
					}
				} else if number, ok := v.(float64); ok {
					if math.Abs(float64(actual.(int))-number) > 1e-8 {
						t.Fatalf("token %d %s: %v vs %v", i, k, actual, v)
					}
				} else if actual != v {
					t.Fatalf("token %d %s: %v vs %v", i, k, actual, v)
				}
			}
		}
	}
}

func TestRequiresExplicitReadingMismatch(t *testing.T) {
	if _, _, e := run("unused", "out/unused.jsonl", 0, false, openjtalk.Config{}); e == nil {
		t.Fatal("missing G2P validation was silently accepted")
	}
}
