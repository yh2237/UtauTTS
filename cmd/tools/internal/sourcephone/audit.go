package sourcephone

import (
	"fmt"
	"math"
)

func AcousticAudit(unit Object, language string) (Object, error) {
	analysis := Map(unit["analysis"])
	duration := Number(analysis["duration_ms"])
	phones := List(unit["forced_phone_intervals"])
	if len(phones) == 0 {
		return Object{"status": "not-aligned", "training_eligible": false, "warnings": []string{}, "phones": []any{}}, nil
	}
	if String(analysis["periodicity_method"]) != "normalized-autocorrelation-local-peak-80-500hz-v2" {
		return nil, fmt.Errorf("regenerate source observations with local-peak periodicity v2")
	}
	previous := 0.0
	gaps := [][2]float64{}
	for _, raw := range phones {
		p := Map(raw)
		start, end := Number(p["start_ms"]), Number(p["end_ms"])
		if !Finite(start, end) || start < previous-.001 || end <= start || end > duration+1 {
			return nil, fmt.Errorf("invalid stored alignment interval")
		}
		if start > previous {
			gaps = append(gaps, [2]float64{previous, start})
		}
		previous = end
	}
	if previous < duration {
		gaps = append(gaps, [2]float64{previous, duration})
	}
	frames := List(analysis["frames"])
	previous = 0
	for _, raw := range frames {
		frame := Map(raw)
		start, end := Number(frame["start_ms"]), Number(frame["end_ms"])
		rms, periodicity, zero := Number(frame["rms_dbfs"]), Number(frame["periodicity"]), Number(frame["zero_crossing_rate"])
		if !Finite(start, end, rms, periodicity, zero) || math.Abs(start-previous) > .001 || end <= start || end > duration+.001 {
			return nil, fmt.Errorf("invalid acoustic frame coverage")
		}
		if periodicity < 0 || periodicity > 1 || zero < 0 || zero > 1 {
			return nil, fmt.Errorf("invalid acoustic frame feature")
		}
		previous = end
	}
	if len(frames) == 0 || math.Abs(previous-duration) > .001 {
		return nil, fmt.Errorf("incomplete acoustic frame coverage")
	}
	overlap := func(frame Object, start, end float64) float64 {
		return math.Max(0, math.Min(Number(frame["end_ms"]), end)-math.Max(Number(frame["start_ms"]), start))
	}
	threshold := Number(analysis["low_energy_threshold_dbfs"])
	guard := math.Max(40, Number(analysis["window_ms"]))
	uncovered := []any{}
	warnings := []string{}
	for _, gap := range gaps {
		start, end := gap[0], gap[1]
		checkedStart, checkedEnd := start, end
		if start > 0 {
			checkedStart += guard
		}
		if end < duration {
			checkedEnd -= guard
		}
		checkedStart = math.Min(end, checkedStart)
		checkedEnd = math.Max(checkedStart, checkedEnd)
		active, periodic := 0.0, 0.0
		for _, raw := range frames {
			f := Map(raw)
			weight := overlap(f, checkedStart, checkedEnd)
			if Number(f["rms_dbfs"]) >= threshold {
				active += weight
				if Number(f["periodicity"]) >= .65 {
					periodic += weight
				}
			}
		}
		uncovered = append(uncovered, Object{"start_ms": start, "end_ms": end, "checked_start_ms": checkedStart, "checked_end_ms": checkedEnd, "active_ms": active, "periodic_ms": periodic})
		if periodic >= 30 {
			warnings = append(warnings, "periodic-activity-outside-alignment")
		} else if active >= 30 {
			warnings = append(warnings, "acoustic-activity-outside-alignment")
		}
	}
	details := []any{}
	for _, raw := range phones {
		phone := Map(raw)
		start, end := Number(phone["start_ms"]), Number(phone["end_ms"])
		power, periodic, active := 0.0, 0.0, 0.0
		for _, value := range frames {
			f := Map(value)
			weight := overlap(f, start, end)
			power += weight * math.Pow(10, Number(f["rms_dbfs"])/10)
			if Number(f["rms_dbfs"]) >= threshold {
				active += weight
				if Number(f["periodicity"]) >= .65 {
					periodic += weight
				}
			}
		}
		detail := Object{"symbol": phone["symbol"], "start_ms": start, "end_ms": end, "windowed_rms_dbfs": 10 * math.Log10(math.Max(1e-14, power/(end-start))), "active_ms": active, "periodic_ms": periodic}
		if language == "en" && contains([]string{"p", "b", "t", "d", "k", "g"}, String(phone["symbol"])) {
			candidates := []any{}
			strong := 0
			for _, raw := range List(analysis["landmarks"]) {
				landmark := Map(raw)
				if source := Number(landmark["source_ms"]); start <= source && source < end {
					candidates = append(candidates, landmark)
					if Number(landmark["relative_to_peak_db"]) >= -25 && Number(landmark["heuristic_score"]) >= .4 {
						strong++
					}
				}
			}
			detail["stop_landmark_candidates"] = candidates
			detail["strong_candidate_count"] = strong
			if strong == 0 {
				warnings = append(warnings, "stop-without-strong-landmark-candidate")
			}
		}
		details = append(details, detail)
	}
	status := "acoustically-consistent-unverified"
	if len(warnings) > 0 {
		status = "needs-review"
	}
	return Object{"status": status, "training_eligible": false, "warnings": UniqueStrings(warnings), "boundary_guard_ms": guard, "uncovered_intervals": uncovered, "phones": details}, nil
}
func contains(values []string, wanted string) bool {
	for _, v := range values {
		if v == wanted {
			return true
		}
	}
	return false
}

func Audit(reportPath, out string) (Object, error) {
	report, err := Read(reportPath)
	if err != nil {
		return nil, err
	}
	rows := []any{}
	aligned, review := 0, 0
	for _, raw := range List(report["units"]) {
		unit := Map(raw)
		if len(List(unit["forced_phone_intervals"])) > 0 {
			digest, duration, err := ClipIdentity(ResolvedSource(reportPath, String(unit["source_clip"])))
			if err != nil {
				return nil, err
			}
			analysis := Map(unit["analysis"])
			if digest != String(analysis["source_sha256"]) || math.Abs(duration-Number(analysis["duration_ms"])) > .01 {
				return nil, fmt.Errorf("source identity mismatch in acoustic audit")
			}
			alignment := Map(unit["phone_alignment"])
			if String(alignment["source_sha256"]) != digest || String(alignment["kind"]) != "forced" {
				return nil, fmt.Errorf("alignment identity mismatch in acoustic audit")
			}
		}
		row, err := AcousticAudit(unit, String(report["language"]))
		if err != nil {
			return nil, err
		}
		row["unit_index"] = unit["unit_index"]
		row["alias"] = unit["alias"]
		row["source_sha256"] = Map(unit["analysis"])["source_sha256"]
		rows = append(rows, row)
		if row["status"] != "not-aligned" {
			aligned++
		}
		if row["status"] == "needs-review" {
			review++
		}
	}
	hash, err := HashFile(reportPath)
	if err != nil {
		return nil, err
	}
	result := Object{"version": 1, "audit_kind": "acoustic-consistency-not-boundary-accuracy", "language": report["language"], "observation_report_sha256": hash, "parameters": Object{"periodicity_threshold": .65, "review_activity_ms": 30, "strong_landmark_min_score": .4, "strong_landmark_min_relative_db": -25, "minimum_boundary_guard_ms": 40}, "aligned_units": aligned, "review_units": review, "verified_units": 0, "training_eligible_units": 0, "units": rows}
	return result, Write(out, result)
}
