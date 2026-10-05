package toolutil

import "testing"

func TestUnderOut(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"out", true},
		{"out/model.json", true},
		{"out/a/b", true},
		{"out2/model.json", false},
		{"models/model.json", false},
		{"../out/model.json", false},
	}
	for _, test := range tests {
		if got := UnderOut(test.path); got != test.want {
			t.Errorf("UnderOut(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}

func TestUnderOutChild(t *testing.T) {
	if UnderOutChild("out") {
		t.Error("UnderOutChild(out) = true, want false")
	}
	if !UnderOutChild("out/model.json") {
		t.Error("UnderOutChild(out/model.json) = false, want true")
	}
}

func TestScanJSONLBytes(t *testing.T) {
	data := []byte("\ufeff{\"id\":\"a\"}\n\n  \n{\"id\":\"b\"}\n")
	var ids []string
	if err := ScanJSONLBytes(data, func(line []byte) error {
		ids = append(ids, string(line))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "{\"id\":\"a\"}" || ids[1] != "{\"id\":\"b\"}" {
		t.Fatalf("lines = %q", ids)
	}
}
