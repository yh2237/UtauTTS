package sourcephone

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
)

func BuildLibrary(paths []string, out string, preferLast bool) (Object, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one --spans is required")
	}
	entries := map[string]Object{}
	order := []string{}
	replacements := []any{}
	language := ""
	for _, file := range paths {
		report, err := Read(file)
		if err != nil {
			return nil, err
		}
		if report["time_origin"] != "oto-offset" {
			return nil, fmt.Errorf("oto-offset library required")
		}
		lang := String(report["language"])
		if language != "" && language != lang {
			return nil, fmt.Errorf("mixed library languages")
		}
		language = lang
		hash, err := HashFile(file)
		if err != nil {
			return nil, err
		}
		for _, raw := range List(report["units"]) {
			unit := Map(raw)
			digest, duration, err := ClipIdentity(String(unit["original_clip"]))
			if err != nil {
				return nil, err
			}
			if digest != String(unit["source_sha256"]) {
				return nil, fmt.Errorf("library source PCM changed")
			}
			phones := List(unit["source_clip_phone_intervals"])
			if err := CheckedPhones(phones, duration); err != nil {
				return nil, err
			}
			provenance := Object{"report": Absolute(file), "report_sha256": hash, "alias": unit["alias"]}
			row := Object{"source_sha256": digest, "duration_ms": duration, "phones": phones, "acoustic_model": unit["acoustic_model"], "alignment_sha256": unit["alignment_sha256"], "status": "unverified", "training_eligible": false, "provenance": provenance}
			if old, ok := entries[digest]; ok {
				a, _ := json.Marshal(old["phones"])
				b, _ := json.Marshal(phones)
				if string(a) != string(b) || old["acoustic_model"] != row["acoustic_model"] {
					if !preferLast {
						return nil, fmt.Errorf("conflicting source hypotheses; choose a library or explicitly prefer the last report")
					}
					replacements = append(replacements, Object{"source_sha256": digest, "previous": old["provenance"], "selected": provenance})
				}
			} else {
				order = append(order, digest)
			}
			entries[digest] = row
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty source library")
	}
	rows := []any{}
	for _, digest := range order {
		rows = append(rows, entries[digest])
	}
	result := Object{"version": 1, "language": language, "time_origin": "oto-offset", "kind": "experimental-unverified-source-phone-library", "entries": rows, "replacements": replacements}
	return result, Write(out, result)
}

func AttachLibrary(reportPath, libraryPath string) (Object, error) {
	report, err := Read(reportPath)
	if err != nil {
		return nil, err
	}
	library, err := Read(libraryPath)
	if err != nil {
		return nil, err
	}
	if Number(library["version"]) != 1 || library["time_origin"] != "oto-offset" || library["language"] != report["language"] {
		return nil, fmt.Errorf("incompatible source library")
	}
	entries := map[string]Object{}
	for _, raw := range List(library["entries"]) {
		row := Map(raw)
		digest := String(row["source_sha256"])
		if _, exists := entries[digest]; exists {
			return nil, fmt.Errorf("duplicate library source identity")
		}
		if err := CheckedPhones(List(row["phones"]), Number(row["duration_ms"])); err != nil {
			return nil, err
		}
		entries[digest] = row
	}
	reused, missing := []any{}, []any{}
	libraryHash, err := HashFile(libraryPath)
	if err != nil {
		return nil, err
	}
	for _, raw := range List(report["units"]) {
		unit := Map(raw)
		unit["source_clip"] = ResolvedSource(reportPath, String(unit["source_clip"]))
		delete(unit, "forced_phone_intervals")
		delete(unit, "phone_alignment")
		role := String(unit["role"])
		if (role != "ending" && !(report["language"] == "zh" && role == "mora")) || len(List(unit["assigned_coda_phones"])) == 0 {
			continue
		}
		digest, duration, err := ClipIdentity(String(unit["source_clip"]))
		if err != nil {
			return nil, err
		}
		analysis := Map(unit["analysis"])
		if digest != String(analysis["source_sha256"]) || math.Abs(duration-Number(analysis["duration_ms"])) > .01 {
			return nil, fmt.Errorf("current source PCM changed")
		}
		row, ok := entries[digest]
		if !ok {
			missing = append(missing, Object{"unit_index": unit["unit_index"], "alias": unit["alias"], "reason": "source-not-in-library"})
			continue
		}
		if math.Abs(duration-Number(row["duration_ms"])) > .01 {
			return nil, fmt.Errorf("library source duration mismatch")
		}
		unit["forced_phone_intervals"] = row["phones"]
		unit["phone_alignment"] = Object{"kind": "forced", "status": "unverified", "source_sha256": digest, "acoustic_model": row["acoustic_model"], "alignment_sha256": row["alignment_sha256"], "transcript_source": "source-library-reuse", "library_sha256": libraryHash, "provenance": row["provenance"]}
		reused = append(reused, unit["unit_index"])
	}
	report["source_library_audit"] = Object{"reused": reused, "missing": missing, "verified_count": 0}
	return report, nil
}
func MapReport(reportPath, libraryPath, out string) (Object, error) {
	report, err := AttachLibrary(reportPath, libraryPath)
	if err != nil {
		return nil, err
	}
	if err := FreshDir(out); err != nil {
		return nil, err
	}
	aligned := filepath.Join(out, "aligned-observations.json")
	if err := Write(aligned, report); err != nil {
		return nil, err
	}
	proposal, err := Propose(aligned, filepath.Join(out, "requests.json"))
	if err != nil {
		return nil, err
	}
	ending, selected := map[int]bool{}, map[int]bool{}
	for _, raw := range List(report["units"]) {
		unit := Map(raw)
		role := String(unit["role"])
		if (role == "ending" || (report["language"] == "zh" && role == "mora")) && len(List(unit["assigned_coda_phones"])) > 0 {
			ending[Int(unit["unit_index"])] = true
		}
	}
	for _, raw := range List(proposal["units"]) {
		selected[Int(Map(raw)["unit_index"])] = true
	}
	indices := []int{}
	for index := range selected {
		indices = append(indices, index)
	}
	for i := 1; i < len(indices); i++ {
		for j := i; j > 0 && indices[j] < indices[j-1]; j-- {
			indices[j], indices[j-1] = indices[j-1], indices[j]
		}
	}
	rejected := []any{}
	for _, raw := range List(proposal["rejected"]) {
		if ending[Int(Map(raw)["unit_index"])] {
			rejected = append(rejected, raw)
		}
	}
	coverage := Object{"ending_units": len(ending), "mapped_units": len(selected), "unmapped_units": len(ending) - len(selected), "selected_indices": indices, "library_audit": report["source_library_audit"], "rejected": rejected, "fallback": "unmapped-units-keep-existing-oto-timing", "training_eligible_units": 0}
	if err := Write(filepath.Join(out, "coverage.json"), coverage); err != nil {
		return nil, err
	}
	if len(selected) > 0 {
		if _, err := Select(aligned, filepath.Join(out, "requests.json"), filepath.Join(out, "selected")); err != nil {
			return nil, err
		}
	}
	return coverage, nil
}
