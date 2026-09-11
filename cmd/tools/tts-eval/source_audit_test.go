package main

import (
	"testing"
	"utautts/internal/audio"
)

func TestAuditClipClampsStereoAndCopies(t *testing.T) {
	p := &audio.PCM{SampleRate: 1000, Channels: 2, Data: []int16{1, 2, 3, 4, 5, 6, 7, 8}}
	c, start, end := auditClip(p, -20, 3)
	if start != 0 || end != 3 || len(c.Data) != 6 || c.Data[5] != 6 {
		t.Fatal(c, start, end)
	}
	c.Data[0] = 99
	if p.Data[0] != 1 {
		t.Fatal("input mutated")
	}
	c, start, end = auditClip(p, 3, 30)
	if start != 3 || end != 4 || len(c.Data) != 2 {
		t.Fatal(c, start, end)
	}
}
