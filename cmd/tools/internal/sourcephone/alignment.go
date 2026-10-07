package sourcephone

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

func Prepare(reportPath, requestsPath, out string) (Object, error) {
	report, err := Read(reportPath)
	if err != nil {
		return nil, err
	}
	requests, err := Read(requestsPath)
	if err != nil {
		return nil, err
	}
	if err := FreshDir(out); err != nil {
		return nil, err
	}
	units := UnitByIndex(List(report["units"]))
	rows := []any{}
	seen := map[int]bool{}
	for _, raw := range List(requests["units"]) {
		r := Map(raw)
		index := Int(r["unit_index"])
		unit, ok := units[index]
		if seen[index] || !ok {
			return nil, fmt.Errorf("duplicate or unknown source unit")
		}
		seen[index] = true
		phones := List(r["phones"])
		if len(phones) == 0 {
			return nil, fmt.Errorf("explicit nonempty acoustic-model phone sequence required")
		}
		labels := make([]string, len(phones))
		for i, p := range phones {
			label := String(p)
			valid := label != ""
			for _, r := range label {
				if !unicode.IsPrint(r) || unicode.IsSpace(r) {
					valid = false
				}
			}
			if !valid {
				return nil, fmt.Errorf("explicit nonempty acoustic-model phone sequence required")
			}
			labels[i] = label
		}
		source := ResolvedSource(reportPath, String(unit["source_clip"]))
		digest, duration, err := ClipIdentity(source)
		if err != nil {
			return nil, err
		}
		if digest != String(Map(unit["analysis"])["source_sha256"]) {
			return nil, fmt.Errorf("source PCM changed since observation")
		}
		name := fmt.Sprintf("source%04d", index)
		wav := filepath.Join(out, "corpus", name+".wav")
		if _, err := os.Stat(wav); err == nil {
			return nil, fmt.Errorf("output source already exists; use a fresh directory")
		}
		if err := os.MkdirAll(filepath.Dir(wav), 0755); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(wav, data, 0644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(out, "corpus", name+".lab"), []byte(name), 0644); err != nil {
			return nil, err
		}
		row := Object{"unit_index": index, "id": name, "alias": unit["alias"], "phones": phones, "source_sha256": digest, "duration_ms": duration}
		if canon, ok := r["canonical_phones"]; ok {
			values := List(canon)
			if len(values) != len(phones) {
				return nil, fmt.Errorf("invalid canonical phone sequence")
			}
			for _, value := range values {
				if !regexp.MustCompile(`^[a-z]+$`).MatchString(String(value)) {
					return nil, fmt.Errorf("invalid canonical phone sequence")
				}
			}
			row["canonical_phones"] = values
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no source units requested")
	}
	var dictionary strings.Builder
	for _, raw := range rows {
		row := Map(raw)
		labels := []string{}
		for _, p := range List(row["phones"]) {
			labels = append(labels, String(p))
		}
		fmt.Fprintf(&dictionary, "%s\t%s\n", row["id"], strings.Join(labels, " "))
	}
	if err := os.WriteFile(filepath.Join(out, "dictionary.dict"), []byte(dictionary.String()), 0644); err != nil {
		return nil, err
	}
	manifest := Object{"version": 1, "report": Absolute(reportPath), "language": report["language"], "time_origin": "oto-offset", "units": rows}
	return manifest, Write(filepath.Join(out, "manifest.json"), manifest)
}

func Intervals(alignment, row Object) ([]any, error) {
	tiers := Map(alignment["tiers"])
	var entries []any
	count := 0
	for name, raw := range tiers {
		if name == "phones" || strings.HasSuffix(name, " phones") {
			entries = List(Map(raw)["entries"])
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("exactly one phone tier required")
	}
	result := []any{}
	previous := 0.0
	for _, raw := range entries {
		entry := List(raw)
		if len(entry) != 3 {
			return nil, fmt.Errorf("invalid interval")
		}
		start, end := Number(entry[0])*1000, Number(entry[1])*1000
		label := String(entry[2])
		if !Finite(start, end) {
			return nil, fmt.Errorf("non-finite interval")
		}
		if start < previous-.001 || end <= start || start < 0 || end > Number(row["duration_ms"])+1 {
			return nil, fmt.Errorf("non-monotone or out-of-source interval")
		}
		previous = end
		if label == "" || label == "sil" || label == "sp" {
			continue
		}
		if label == "spn" || label == "<unk>" {
			return nil, fmt.Errorf("unknown aligned phone")
		}
		result = append(result, Object{"symbol": Normalized(label), "acoustic_label": label, "start_ms": start, "end_ms": end})
	}
	expected := List(row["phones"])
	if len(expected) != len(result) {
		return nil, fmt.Errorf("aligned phone sequence mismatch")
	}
	for i, v := range expected {
		if String(Map(result[i])["symbol"]) != Normalized(String(v)) {
			return nil, fmt.Errorf("aligned phone sequence mismatch")
		}
	}
	if canonical, ok := row["canonical_phones"]; ok {
		values := List(canonical)
		if len(values) != len(result) {
			return nil, fmt.Errorf("canonical phone count mismatch")
		}
		for i, v := range values {
			Map(result[i])["symbol"] = v
		}
	}
	return result, nil
}

func Import(manifestPath, alignments, model, out string) (Object, error) {
	manifest, err := Read(manifestPath)
	if err != nil {
		return nil, err
	}
	reportPath := String(manifest["report"])
	report, err := Read(reportPath)
	if err != nil {
		return nil, err
	}
	units := UnitByIndex(List(report["units"]))
	accepted, rejected := []any{}, []any{}
	for _, raw := range List(manifest["units"]) {
		row := Map(raw)
		index := Int(row["unit_index"])
		unit, ok := units[index]
		if !ok {
			return nil, fmt.Errorf("source unit removed from report")
		}
		for _, clip := range []string{ResolvedSource(reportPath, String(unit["source_clip"])), filepath.Join(filepath.Dir(manifestPath), "corpus", String(row["id"])+".wav")} {
			digest, duration, err := ClipIdentity(clip)
			if err != nil {
				return nil, err
			}
			if digest != String(row["source_sha256"]) || digest != String(Map(unit["analysis"])["source_sha256"]) || math.Abs(duration-Number(row["duration_ms"])) > .01 {
				return nil, fmt.Errorf("source identity mismatch")
			}
		}
		file := filepath.Join(alignments, String(row["id"])+".json")
		alignment, err := Read(file)
		var phones []any
		if err == nil {
			phones, err = Intervals(alignment, row)
		}
		if err != nil {
			delete(unit, "forced_phone_intervals")
			delete(unit, "phone_alignment")
			rejected = append(rejected, Object{"unit_index": index, "reason": err.Error()})
			continue
		}
		hash, err := HashFile(file)
		if err != nil {
			return nil, err
		}
		unit["forced_phone_intervals"] = phones
		unit["phone_alignment"] = Object{"kind": "forced", "status": "unverified", "acoustic_model": model, "source_sha256": row["source_sha256"], "alignment_sha256": hash, "transcript_source": "explicit-source-phone-request"}
		accepted = append(accepted, index)
	}
	report["alignment_audit"] = Object{"accepted": accepted, "rejected": rejected, "verified_count": 0}
	for _, raw := range List(report["units"]) {
		unit := Map(raw)
		unit["source_clip"] = ResolvedSource(reportPath, String(unit["source_clip"]))
	}
	return report, Write(out, report)
}

func Evaluate(reportPath, referencesPath, out string) (Object, error) {
	report, err := Read(reportPath)
	if err != nil {
		return nil, err
	}
	references, err := Read(referencesPath)
	if err != nil {
		return nil, err
	}
	units := UnitByIndex(List(report["units"]))
	errors := []float64{}
	seen := map[int]bool{}
	perPhone := map[string][]float64{}
	for _, raw := range List(references["units"]) {
		reference := Map(raw)
		index := Int(reference["unit_index"])
		if seen[index] || String(reference["annotation_kind"]) != "manual" {
			return nil, fmt.Errorf("unique manual references required")
		}
		seen[index] = true
		unit, ok := units[index]
		if !ok {
			return nil, fmt.Errorf("unknown source unit")
		}
		digest := String(reference["source_sha256"])
		if digest != String(Map(unit["analysis"])["source_sha256"]) {
			return nil, fmt.Errorf("reference source mismatch")
		}
		current, _, err := ClipIdentity(ResolvedSource(reportPath, String(unit["source_clip"])))
		if err != nil {
			return nil, err
		}
		if current != digest {
			return nil, fmt.Errorf("reference audio changed")
		}
		predicted, actual := List(unit["forced_phone_intervals"]), List(reference["phones"])
		if len(predicted) != len(actual) {
			return nil, fmt.Errorf("reference phone sequence mismatch")
		}
		previous := 0.0
		for i, raw := range actual {
			manual, forced := Map(raw), Map(predicted[i])
			symbol := String(manual["symbol"])
			if symbol != String(forced["symbol"]) {
				return nil, fmt.Errorf("reference phone sequence mismatch")
			}
			a, b := Number(manual["start_ms"]), Number(manual["end_ms"])
			if !Finite(a, b) || a < previous || b <= a || b > Number(Map(unit["analysis"])["duration_ms"]) {
				return nil, fmt.Errorf("invalid manual interval")
			}
			previous = b
			pair := []float64{math.Abs(a - Number(forced["start_ms"])), math.Abs(b - Number(forced["end_ms"]))}
			errors = append(errors, pair...)
			perPhone[symbol] = append(perPhone[symbol], pair...)
		}
	}
	metrics := func(values []float64) Object {
		if len(values) == 0 {
			return Object{"boundaries": 0, "mae_ms": nil, "max_ms": nil, "within_20_ms": nil}
		}
		sum, max, within := 0.0, 0.0, 0
		for _, v := range values {
			sum += v
			if v > max {
				max = v
			}
			if v <= 20 {
				within++
			}
		}
		return Object{"boundaries": len(values), "mae_ms": sum / float64(len(values)), "max_ms": max, "within_20_ms": float64(within) / float64(len(values))}
	}
	byPhone := Object{}
	for symbol, values := range perPhone {
		byPhone[symbol] = metrics(values)
	}
	result := Object{"manual_units": len(seen), "unreviewed_units": len(units) - len(seen), "overall": metrics(errors), "by_phone": byPhone}
	return result, Write(out, result)
}
