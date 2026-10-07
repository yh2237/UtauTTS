package openjtalk

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCapturedPyopenjtalkBridgeParity(t *testing.T) {
	data, err := os.ReadFile("testdata/bridge_parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Text     string   `json:"text"`
			TSV      string   `json:"tsv"`
			Expected Analysis `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Text, func(t *testing.T) {
			nodes, err := parseNJD(tc.TSV)
			if err != nil {
				t.Fatal(err)
			}
			got := buildAnalysis(nodes)
			if got.Version != tc.Expected.Version || got.Reading != tc.Expected.Reading || !reflect.DeepEqual(got.Morae, tc.Expected.Morae) || !reflect.DeepEqual(got.Features, tc.Expected.Features) {
				t.Errorf("Go NJD conversion differs from pyopenjtalk helper\nreading: %q vs %q\nmorae: %#v vs %#v\nfeatures: %#v vs %#v", got.Reading, tc.Expected.Reading, got.Morae, tc.Expected.Morae, got.Features, tc.Expected.Features)
			}
		})
	}
}
