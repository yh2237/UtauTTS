package sourcephone

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"utautts/internal/audio"
)

func testDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("out", 0755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("out", "sourcephone-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
func acousticUnit(tailActive bool) Object {
	frames := []any{}
	for i := 0; i < 32; i++ {
		rms := -80
		if i < 18 || tailActive {
			rms = -25
		}
		frames = append(frames, Object{"start_ms": float64(i * 10), "end_ms": float64((i + 1) * 10), "rms_dbfs": float64(rms), "periodicity": .9, "zero_crossing_rate": .03})
	}
	return Object{"analysis": Object{"duration_ms": float64(320), "frames": frames, "window_ms": float64(20), "periodicity_method": "normalized-autocorrelation-local-peak-80-500hz-v2", "low_energy_threshold_dbfs": float64(-60), "landmarks": []any{Object{"kind": "energy-rise", "source_ms": float64(110), "heuristic_score": .8, "relative_to_peak_db": float64(-10)}}}, "forced_phone_intervals": []any{Object{"symbol": "l", "start_ms": float64(0), "end_ms": float64(80)}, Object{"symbol": "d", "start_ms": float64(80), "end_ms": float64(180)}}}
}
func TestIntervals(t *testing.T) {
	row := Object{"phones": []any{"x", "ə", "n"}, "canonical_phones": []any{"h", "e", "n"}, "duration_ms": float64(300)}
	alignment := Object{"tiers": Object{"phones": Object{"entries": []any{[]any{float64(0), .04, "x"}, []any{.04, .2, "ə"}, []any{.2, .3, "n"}}}}}
	phones, err := Intervals(alignment, row)
	if err != nil {
		t.Fatal(err)
	}
	if Map(phones[1])["symbol"] != "e" || Map(phones[1])["acoustic_label"] != "ə" {
		t.Fatal(phones)
	}
	List(List(Map(Map(alignment["tiers"])["phones"])["entries"])[1])[2] = "a"
	if _, err := Intervals(alignment, row); err == nil {
		t.Fatal("accepted mismatched acoustic label")
	}
}
func TestAcousticAudit(t *testing.T) {
	unit := acousticUnit(true)
	result, err := AcousticAudit(unit, "en")
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "needs-review" || !contains(warningStrings(result["warnings"]), "periodic-activity-outside-alignment") {
		t.Fatal(result)
	}
	if Number(Map(List(result["uncovered_intervals"])[0])["periodic_ms"]) != 100 {
		t.Fatal(result)
	}
	result, err = AcousticAudit(acousticUnit(false), "en")
	if err != nil || len(warningStrings(result["warnings"])) != 0 {
		t.Fatalf("%v %v", result, err)
	}
	bad := acousticUnit(false)
	frames := List(Map(bad["analysis"])["frames"])
	Map(bad["analysis"])["frames"] = append(frames[:10], frames[11:]...)
	if _, err := AcousticAudit(bad, "en"); err == nil {
		t.Fatal("accepted missing frame")
	}
}
func TestPrepareImportAndSelect(t *testing.T) {
	dir := testDir(t)
	source := filepath.Join(dir, "source.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 4800)}
	for i := range pcm.Data {
		pcm.Data[i] = 1
	}
	if err := audio.WriteWav(source, pcm); err != nil {
		t.Fatal(err)
	}
	digest, duration, err := ClipIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	unit := acousticUnit(false)
	unit["unit_index"] = float64(6)
	unit["position"] = float64(1)
	unit["role"] = "ending"
	unit["alias"] = "l d-"
	unit["source_clip"] = "source.wav"
	unit["assigned_coda_phones"] = []any{"d"}
	unit["requested_context"] = []any{Object{"symbol": "l", "role": "nucleus", "start_ms": float64(10), "duration_ms": float64(90)}, Object{"symbol": "d", "role": "coda", "start_ms": float64(100), "duration_ms": float64(100)}}
	analysis := Map(unit["analysis"])
	analysis["source_sha256"] = digest
	analysis["duration_ms"] = duration
	analysis["frames"] = List(analysis["frames"])[:30]
	reportPath := filepath.Join(dir, "report.json")
	requestPath := filepath.Join(dir, "requests.json")
	if err := Write(reportPath, Object{"language": "en", "units": []any{unit}, "annotation_status": "unobserved"}); err != nil {
		t.Fatal(err)
	}
	if err := Write(requestPath, Object{"units": []any{Object{"unit_index": float64(6), "phones": []any{"L", "D"}}}}); err != nil {
		t.Fatal(err)
	}
	prepared := filepath.Join(dir, "prepared")
	manifest, err := Prepare(reportPath, requestPath, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if len(List(manifest["units"])) != 1 {
		t.Fatal(manifest)
	}
	alignments := filepath.Join(dir, "alignments")
	os.MkdirAll(alignments, 0755)
	if err := Write(filepath.Join(alignments, "source0006.json"), Object{"tiers": Object{"phones": Object{"entries": []any{[]any{float64(0), .1, "L"}, []any{.1, .3, "D"}}}}}); err != nil {
		t.Fatal(err)
	}
	aligned := filepath.Join(dir, "aligned.json")
	result, err := Import(filepath.Join(prepared, "manifest.json"), alignments, "test", aligned)
	if err != nil {
		t.Fatal(err)
	}
	if len(List(Map(result["alignment_audit"])["accepted"])) != 1 {
		t.Fatal(result)
	}
	audited, err := Audit(aligned, filepath.Join(dir, "audit.json"))
	if err != nil || audited["aligned_units"] != 1 {
		t.Fatalf("%v %v", audited, err)
	}
	reference := Object{"unit_index": float64(6), "source_sha256": digest, "annotation_kind": "manual", "phones": []any{Object{"symbol": "l", "start_ms": float64(10), "end_ms": float64(120)}, Object{"symbol": "d", "start_ms": float64(120), "end_ms": float64(290)}}}
	manualPath := filepath.Join(dir, "manual.json")
	if err := Write(manualPath, Object{"units": []any{reference}}); err != nil {
		t.Fatal(err)
	}
	metrics, err := Evaluate(aligned, manualPath, filepath.Join(dir, "metrics.json"))
	if err != nil || Number(Map(metrics["overall"])["mae_ms"]) != 15 {
		t.Fatalf("%v %v", metrics, err)
	}
	proposalPath := filepath.Join(dir, "proposal.json")
	proposal, err := Propose(aligned, proposalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(List(proposal["units"])) != 1 {
		t.Fatal(proposal)
	}
	selected, err := Select(aligned, proposalPath, filepath.Join(dir, "selected"))
	if err != nil {
		t.Fatal(err)
	}
	row := Map(List(selected["units"])[0])
	if Number(row["core_start_ms"]) != 100 || Number(row["core_end_ms"]) != 300 {
		t.Fatal(row)
	}
	if _, err := Select(aligned, proposalPath, filepath.Join(dir, "selected")); err == nil || !strings.Contains(err.Error(), "fresh") {
		t.Fatalf("expected overwrite refusal: %v", err)
	}
	badAlignment := Object{"tiers": Object{"phones": Object{"entries": []any{[]any{float64(0), .1, "L"}, []any{.1, .3, "T"}}}}}
	if err := os.WriteFile(filepath.Join(alignments, "source0006.json"), mustJSON(t, badAlignment), 0644); err != nil {
		t.Fatal(err)
	}
	rejected, err := Import(filepath.Join(prepared, "manifest.json"), alignments, "test", filepath.Join(dir, "rejected.json"))
	if err != nil || len(List(Map(rejected["alignment_audit"])["rejected"])) != 1 {
		t.Fatalf("%v %v", rejected, err)
	}
}
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAliasHypotheses(t *testing.T) {
	unit := Object{"role": "ending", "alias": "6 kB3", "assigned_coda_phones": []any{"k"}, "requested_context": []any{Object{"symbol": "uh", "role": "nucleus"}, Object{"symbol": "k", "role": "coda"}}}
	symbols := Object{"uh": []any{"6"}, "k": []any{"k"}}
	rows := AliasHypotheses(unit, symbols, "en-vccv")
	if len(rows) != 2 || !Bool(Map(rows[1])["preferred"]) {
		t.Fatal(rows)
	}
	unit["alias"] = "6k-B3"
	rows = AliasHypotheses(unit, symbols, "en-vccv")
	if len(rows) != 2 || !Bool(Map(rows[0])["preferred"]) {
		t.Fatal(rows)
	}
	unit["alias"] = "6 q"
	if len(AliasHypotheses(unit, symbols, "en-vccv")) != 0 {
		t.Fatal("guessed unknown alias")
	}
}
func TestChinesePhones(t *testing.T) {
	unit := Object{"role": "mora", "alias": "hen", "assigned_coda_phones": []any{"n"}, "requested_context": []any{Object{"symbol": "h", "role": "onset"}, Object{"symbol": "e", "role": "nucleus"}, Object{"symbol": "n", "role": "coda"}}}
	acoustic, canonical := PhonesForChineseUnit(unit)
	if strings.Join(acoustic, ",") != "x,ə,n" || strings.Join(canonical, ",") != "h,e,n" {
		t.Fatalf("%v %v", acoustic, canonical)
	}
	unit["alias"] = "other"
	if acoustic, _ := PhonesForChineseUnit(unit); acoustic != nil {
		t.Fatal(acoustic)
	}
	unit["alias"] = "hangB3"
	unit["assigned_coda_phones"] = []any{"ng"}
	Map(List(unit["requested_context"])[1])["symbol"] = "a"
	Map(List(unit["requested_context"])[2])["symbol"] = "ng"
	acoustic, _ = PhonesForChineseUnit(unit)
	if strings.Join(acoustic, ",") != "x,a,ŋ" {
		t.Fatal(acoustic)
	}
}

func TestDiscoveryAcousticScreen(t *testing.T) {
	frames := []any{}
	for i := 0; i < 55; i++ {
		frames = append(frames, Object{"start_ms": float64(i * 10), "end_ms": float64((i + 1) * 10), "rms_dbfs": float64(-20), "periodicity": .9, "zero_crossing_rate": .03})
	}
	unit := Object{"forced_phone_intervals": []any{Object{"symbol": "uh", "start_ms": float64(0), "end_ms": float64(200)}, Object{"symbol": "k", "start_ms": float64(200), "end_ms": float64(340)}, Object{"symbol": "uh", "start_ms": float64(340), "end_ms": float64(500)}}, "analysis": Object{"duration_ms": float64(550), "window_ms": float64(20), "periodicity_method": "normalized-autocorrelation-local-peak-80-500hz-v2", "low_energy_threshold_dbfs": float64(-60), "frames": frames, "landmarks": []any{}}}
	hypothesis := Object{"phones": []any{"uh", "k", "uh"}, "preferred": true}
	score, err := ScoreCandidate(unit, hypothesis)
	if err != nil || !Bool(score["accepted"]) {
		t.Fatalf("%v %v", score, err)
	}
	for _, raw := range frames {
		frame := Map(raw)
		if Number(frame["start_ms"]) >= 340 {
			frame["periodicity"] = float64(0)
			frame["rms_dbfs"] = float64(-80)
		}
	}
	score, err = ScoreCandidate(unit, hypothesis)
	if err != nil || score["reason"] != "vowel-hypothesis-lacks-periodic-support" {
		t.Fatalf("%v %v", score, err)
	}
}

func TestLibraryMap(t *testing.T) {
	dir := testDir(t)
	source := filepath.Join(dir, "source.wav")
	pcm := &audio.PCM{SampleRate: 10000, Channels: 1, Data: make([]int16, 5500)}
	if err := audio.WriteWav(source, pcm); err != nil {
		t.Fatal(err)
	}
	digest, duration, err := ClipIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	phones := []any{Object{"symbol": "eh", "start_ms": float64(0), "end_ms": float64(210)}, Object{"symbol": "k", "start_ms": float64(210), "end_ms": float64(340)}, Object{"symbol": "eh", "start_ms": float64(340), "end_ms": float64(530)}}
	span := Object{"alias": "e k", "original_clip": Absolute(source), "source_sha256": digest, "source_clip_phone_intervals": phones, "acoustic_model": "test", "alignment_sha256": "alignment"}
	spanPath, libraryPath, reportPath := filepath.Join(dir, "spans.json"), filepath.Join(dir, "library.json"), filepath.Join(dir, "report.json")
	if err := Write(spanPath, Object{"language": "en", "time_origin": "oto-offset", "units": []any{span}}); err != nil {
		t.Fatal(err)
	}
	library, err := BuildLibrary([]string{spanPath}, libraryPath, false)
	if err != nil || len(List(library["entries"])) != 1 {
		t.Fatalf("%v %v", library, err)
	}
	frames := []any{}
	for i := 0; i < 55; i++ {
		frames = append(frames, Object{"start_ms": float64(i * 10), "end_ms": float64((i + 1) * 10), "rms_dbfs": float64(-70), "periodicity": float64(0), "zero_crossing_rate": float64(0)})
	}
	unit := Object{"unit_index": float64(29), "position": float64(12), "alias": "e k", "role": "ending", "source_clip": "source.wav", "assigned_coda_phones": []any{"k"}, "requested_context": []any{Object{"symbol": "eh", "role": "nucleus", "start_ms": float64(700), "duration_ms": float64(140)}, Object{"symbol": "k", "role": "coda", "start_ms": float64(840), "duration_ms": float64(70)}}, "analysis": Object{"source_sha256": digest, "duration_ms": duration, "window_ms": float64(20), "periodicity_method": "normalized-autocorrelation-local-peak-80-500hz-v2", "low_energy_threshold_dbfs": float64(-60), "frames": frames, "landmarks": []any{}}}
	if err := Write(reportPath, Object{"language": "en", "units": []any{unit}}); err != nil {
		t.Fatal(err)
	}
	coverage, err := MapReport(reportPath, libraryPath, filepath.Join(dir, "mapped"))
	if err != nil {
		t.Fatal(err)
	}
	if Number(coverage["mapped_units"]) != 1 && coverage["mapped_units"] != 1 {
		t.Fatal(coverage)
	}
	selected, err := Read(filepath.Join(dir, "mapped", "selected", "spans.json"))
	if err != nil {
		t.Fatal(err)
	}
	row := Map(List(selected["units"])[0])
	if Number(row["core_start_ms"]) != 210 || Number(row["core_end_ms"]) != 340 {
		t.Fatal(row)
	}
	aligned, err := Read(filepath.Join(dir, "mapped", "aligned-observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	requests, err := Read(filepath.Join(dir, "mapped", "requests.json"))
	if err != nil {
		t.Fatal(err)
	}
	mappedUnit := Map(List(aligned["units"])[0])
	request := Map(List(requests["units"])[0])
	request["source_sha256"] = "stale"
	if _, err := SelectUnit(mappedUnit, request, "en"); err == nil {
		t.Fatal("accepted stale source")
	}
	request["source_sha256"] = digest
	request["mappings"] = append(List(request["mappings"]), List(request["mappings"])[0])
	if _, err := SelectUnit(mappedUnit, request, "en"); err == nil {
		t.Fatal("accepted duplicate phone index")
	}
}
