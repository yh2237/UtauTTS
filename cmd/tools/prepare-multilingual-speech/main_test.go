package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPythonFixtureParity(t *testing.T) {
	root := "../../../out/python-migration/multilingual/"
	data, e := os.ReadFile(root + "python.jsonl")
	if e != nil {
		t.Skip("optional Python parity fixture absent")
	}
	var want map[string]any
	if e = json.Unmarshal(data, &want); e != nil {
		t.Fatal(e)
	}
	template, e := readJSON(root + "template.json")
	if e != nil {
		t.Fatal(e)
	}
	source, e := readJSON(root + "observations.json")
	if e != nil {
		t.Fatal(e)
	}
	obs := source["utterances"].([]any)[0].(map[string]any)
	got, e := prepare(template, obs, root)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := json.Marshal(got)
	if e != nil {
		t.Fatal(e)
	}
	var normalized map[string]any
	if e = json.Unmarshal(encoded, &normalized); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(want, normalized) {
		t.Fatalf("Python and Go preparation differ: %v vs %v", want, normalized)
	}
}
func TestRejectGeneratedObservation(t *testing.T) {
	template := map[string]any{"version": float64(1), "feature_version": float64(1), "id": "x"}
	obs := map[string]any{"id": "x", "speaker": "s", "corpus": "c", "license": "l", "audio_path": "x.wav", "kind": "generated", "alignment": "manual", "split": "train"}
	if _, e := prepare(template, obs, "out"); e == nil {
		t.Fatal("generated observation accepted")
	}
}

func TestRealAudioPythonParity(t *testing.T) {
	root := "../../../out/python-migration/multilingual-real/"
	data, e := os.ReadFile(root + "python.jsonl")
	if e != nil {
		t.Skip("optional JSUT audio parity fixture absent")
	}
	var want map[string]any
	if e = json.Unmarshal(data, &want); e != nil {
		t.Fatal(e)
	}
	template, e := readJSON(root + "template.json")
	if e != nil {
		t.Fatal(e)
	}
	source, e := readJSON(root + "observations.json")
	if e != nil {
		t.Fatal(e)
	}
	obs := source["utterances"].([]any)[0].(map[string]any)
	got, e := prepare(template, obs, root)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := json.Marshal(got)
	if e != nil {
		t.Fatal(e)
	}
	var normalized map[string]any
	if e = json.Unmarshal(encoded, &normalized); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(want, normalized) {
		t.Fatalf("JSUT audio differs from Python: %v vs %v", want, normalized)
	}
}
