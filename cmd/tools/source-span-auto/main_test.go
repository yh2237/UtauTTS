package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestRealSpanPythonParity(t *testing.T) {
	if _, e := os.Stat("../../../out/source-span-mapping-20260928/nyui/spans.json"); e != nil {
		t.Skip("optional selected spans absent")
	}
	old, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir("../../.."); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(old)
	out := "out/python-migration/nyui-library-test.json"
	_ = os.Remove(out)
	defer os.Remove(out)
	got, e := build([]string{"out/source-span-mapping-20260928/nyui/spans.json"}, out, false)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile("out/python-migration/nyui-library-py.json")
	if e != nil {
		t.Skip("Python parity fixture absent")
	}
	var want map[string]any
	if e = json.Unmarshal(b, &want); e != nil {
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
		t.Fatal("source library differs from Python")
	}
}
func TestRejectInvalidPhones(t *testing.T) {
	if e := checkedPhones([]any{map[string]any{"symbol": "a", "start_ms": 8.0, "end_ms": 7.0}}, 100); e == nil {
		t.Fatal("invalid interval accepted")
	}
}
