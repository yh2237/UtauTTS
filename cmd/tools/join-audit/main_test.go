package main

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/plan"
)

func TestAuditPlanExportsAdjacentAudibleUnits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tone.wav")
	const sampleRate = 16000
	data := make([]int16, sampleRate/3)
	for index := range data {
		data[index] = int16(7000 * math.Sin(2*math.Pi*220*float64(index)/sampleRate))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	synthesisPlan := &plan.Plan{
		Voicebank: "bank", Reading: "あい", JoinCostMode: "handcrafted",
		Units: []plan.Unit{
			{Position: 0, Role: "mora", Mora: "あ", Alias: "- あ", Source: path, NoteStartMS: 0, DurationMS: 120, OffsetMS: 20, ConsonantMS: 60, PreutteranceMS: 30, OverlapMS: 10},
			{Position: 1, Role: "mora", Mora: "い", Alias: "あ い", Source: path, NoteStartMS: 120, DurationMS: 120, OffsetMS: 120, ConsonantMS: 160, PreutteranceMS: 130, OverlapMS: 110},
		},
	}
	rows := auditPlan(synthesisPlan, nil)
	if len(rows) != 1 || rows[0].Previous.Index != 0 || rows[0].Current.Index != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0].Features.WaveformCorrelation <= 0 {
		t.Fatalf("features = %#v", rows[0].Features)
	}
}

func TestAuditPlanDoesNotJoinAcrossSilentUnit(t *testing.T) {
	synthesisPlan := &plan.Plan{Units: []plan.Unit{
		{Source: "a.wav", NoteStartMS: 0, DurationMS: 100},
		{Silent: true, NoteStartMS: 100, DurationMS: 100},
		{Source: "b.wav", NoteStartMS: 200, DurationMS: 100},
	}}
	if rows := auditPlan(synthesisPlan, nil); len(rows) != 0 {
		t.Fatalf("rows across silence = %#v", rows)
	}
}
