package connection

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadJoinAuditRowsAcceptsJSONLAndNormalizesLegacyVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rows.jsonl")
	content := `{"group_id":"a","features":{"spectrum_delta_db":2}}
{"schema_version":1,"group_id":"b","features":{"rms_delta_db":3}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadJoinAuditRows(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].SchemaVersion != JoinAuditSchemaVersion || rows[1].Features.RMSDelta != 3 {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestJoinRiskFlagsIdentifyOnlyTriageSignals(t *testing.T) {
	flags := JoinRiskFlags(PairFeatures{
		SpectrumDelta: 9, RMSDelta: 7, F0DeltaCents: 100,
		VoicingMismatch: true, WaveformCorrelation: 0.1,
		SameSource: true, ForwardInSource: false, CurrentVCV: true,
	})
	for _, want := range []string{"spectrum-jump", "level-jump", "pitch-jump", "voicing-change", "low-correlation", "source-backtrack", "vcv-boundary"} {
		if !containsString(flags, want) {
			t.Fatalf("flags=%v missing %q", flags, want)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
