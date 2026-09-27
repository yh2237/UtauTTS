package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestTemplatePreservesLinguisticUnitsAndIsNotAnObservation(t *testing.T) {
	root := t.TempDir()
	p := plan.Plan{Language: "en", Reading: "AA1", Morae: []frontend.Mora{{Language: "en", Stress: 1, StressKnown: true, Phones: []frontend.Phone{{Symbol: "aa", Role: "nucleus"}}}},
		PhoneTimings: []plan.PhoneTiming{{Position: 0, Symbol: "aa", Role: "nucleus", DurationMS: 200}}}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	input, output := filepath.Join(root, "plan.json"), filepath.Join(root, "template.json")
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(input, output, "example", 120); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var template map[string]any
	if err := json.Unmarshal(data, &template); err != nil {
		t.Fatal(err)
	}
	if template["observation_status"] != "unobserved" {
		t.Fatal("rule target was labelled observed")
	}
	row := template["phones"].([]any)[0].(map[string]any)
	if _, ok := row["duration_ms"]; ok {
		t.Fatal("template emitted a measured duration")
	}
	if row["baseline_ms"].(float64) == 200 {
		t.Fatal("template copied target duration instead of recomputing rule baseline")
	}
	p.PhoneTimings[0].Symbol = "incorrect"
	data, _ = json.Marshal(p)
	_ = os.WriteFile(input, data, 0600)
	if run(input, output, "example", 120) == nil {
		t.Fatal("mismatched linguistic identity accepted")
	}
}
