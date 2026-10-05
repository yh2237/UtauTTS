package main

import (
	"testing"
)

func TestNormalizePhones(t *testing.T) {
	for input, want := range map[string]string{"AH0": "ah", "ER1": "er", "sp": "sp"} {
		if got := normalize(input); got != want {
			t.Fatalf("%s -> %s want %s", input, got, want)
		}
	}
}
