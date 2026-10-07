package sourcephone

import (
	"fmt"
	"math"
	"path/filepath"

	"utautts/internal/audio"
)

func symbols(raw []any) []string {
	result := make([]string, len(raw))
	for i, value := range raw {
		result[i] = String(Map(value)["symbol"])
	}
	return result
}
func stringsOf(raw []any) []string {
	result := make([]string, len(raw))
	for i, value := range raw {
		result[i] = String(value)
	}
	return result
}
func warningStrings(value any) []string {
	if values, ok := value.([]string); ok {
		return append([]string{}, values...)
	}
	return stringsOf(List(value))
}
func matches(sequence, wanted []string) []int {
	result := []int{}
	if len(wanted) == 0 {
		return result
	}
	for i := 0; i+len(wanted) <= len(sequence); i++ {
		same := true
		for j, v := range wanted {
			if sequence[i+j] != v {
				same = false
				break
			}
		}
		if same {
			result = append(result, i)
		}
	}
	return result
}

func Propose(reportPath, out string) (Object, error) {
	report, err := Read(reportPath)
	if err != nil {
		return nil, err
	}
	rows, rejected := []any{}, []any{}
	language := String(report["language"])
	for _, raw := range List(report["units"]) {
		unit := Map(raw)
		wanted := stringsOf(List(unit["assigned_coda_phones"]))
		chinese := language == "zh" && String(unit["role"]) == "mora" && len(wanted) > 0
		if chinese {
			wanted = symbols(List(unit["requested_context"]))
		}
		reason := ""
		if len(wanted) == 0 {
			reason = "no assigned coda sequence"
		} else {
			source := List(unit["forced_phone_intervals"])
			sourceMatches := matches(symbols(source), wanted)
			if len(sourceMatches) != 1 {
				reason = "missing or ambiguous source sequence"
			} else {
				context := List(unit["requested_context"])
				targetMatches := []int{}
				for _, i := range matches(symbols(context), wanted) {
					allCoda := true
					for _, raw := range context[i : i+len(wanted)] {
						if String(Map(raw)["role"]) != "coda" {
							allCoda = false
						}
					}
					if chinese || allCoda {
						targetMatches = append(targetMatches, i)
					}
				}
				if len(targetMatches) != 1 {
					reason = "missing or ambiguous requested coda sequence"
				} else {
					start, target := sourceMatches[0], targetMatches[0]
					mappings := []any{}
					for i := range wanted {
						mappings = append(mappings, Object{"source_phone_index": start + i, "requested_phone_index": target + i})
					}
					left := 0
					if start > 0 {
						left = 1
					}
					rows = append(rows, Object{"unit_index": unit["unit_index"], "source_sha256": Map(unit["analysis"])["source_sha256"], "alignment_sha256": Map(unit["phone_alignment"])["alignment_sha256"], "mappings": mappings, "left_context_count": left, "right_context_count": 0})
				}
			}
		}
		if reason != "" {
			rejected = append(rejected, Object{"unit_index": unit["unit_index"], "alias": unit["alias"], "reason": reason})
		}
	}
	hash, err := HashFile(reportPath)
	if err != nil {
		return nil, err
	}
	result := Object{"version": 1, "report_sha256": hash, "proposal_status": "unverified-unique-sequence-match", "units": rows, "rejected": rejected}
	return result, Write(out, result)
}
func checkedIndices(mappings []any, key string, length int) ([]int, error) {
	if len(mappings) == 0 {
		return nil, fmt.Errorf("invalid phone index")
	}
	indices := make([]int, len(mappings))
	for i, raw := range mappings {
		value, ok := Map(raw)[key].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid phone index")
		}
		index := int(value)
		if value != float64(index) || index < 0 || index >= length {
			return nil, fmt.Errorf("invalid phone index")
		}
		indices[i] = index
		if i > 0 && index != indices[0]+i {
			return nil, fmt.Errorf("phone indices must be unique, ordered and contiguous")
		}
	}
	return indices, nil
}
func SelectUnit(unit, request Object, language string) (Object, error) {
	phones, context := List(unit["forced_phone_intervals"]), List(unit["requested_context"])
	metadata := Map(unit["phone_alignment"])
	analysis := Map(unit["analysis"])
	if request["source_sha256"] != analysis["source_sha256"] || request["source_sha256"] != metadata["source_sha256"] {
		return nil, fmt.Errorf("selection source identity mismatch")
	}
	if request["alignment_sha256"] != metadata["alignment_sha256"] || metadata["kind"] != "forced" {
		return nil, fmt.Errorf("selection alignment identity mismatch")
	}
	check, err := AcousticAudit(unit, language)
	if err != nil {
		return nil, err
	}
	mappings := List(request["mappings"])
	sourceIndices, err := checkedIndices(mappings, "source_phone_index", len(phones))
	if err != nil {
		return nil, err
	}
	targetIndices, err := checkedIndices(mappings, "requested_phone_index", len(context))
	if err != nil {
		return nil, err
	}
	selected, targets := make([]any, len(mappings)), make([]any, len(mappings))
	for i := range mappings {
		selected[i], targets[i] = phones[sourceIndices[i]], context[targetIndices[i]]
		if String(Map(selected[i])["symbol"]) != String(Map(targets[i])["symbol"]) {
			return nil, fmt.Errorf("selected source and requested phone sequence mismatch")
		}
	}
	assigned := stringsOf(List(unit["assigned_coda_phones"]))
	chinese := language == "zh" && String(unit["role"]) == "mora" && len(assigned) > 0
	if chinese {
		if len(targets) != len(context) {
			return nil, fmt.Errorf("Chinese selection must cover the whole syllable")
		}
		assigned = nil
	}
	if len(assigned) > 0 {
		if len(assigned) != len(targets) {
			return nil, fmt.Errorf("selection must cover exactly the assigned coda phones")
		}
		for i, raw := range targets {
			target := Map(raw)
			if String(target["symbol"]) != assigned[i] || String(target["role"]) != "coda" {
				return nil, fmt.Errorf("selection must cover exactly the assigned coda phones")
			}
		}
	}
	previous := math.Inf(-1)
	rows := []any{}
	warnings := warningStrings(check["warnings"])
	for i := range mappings {
		phone, target := Map(selected[i]), Map(targets[i])
		start, duration := Number(target["start_ms"]), Number(target["duration_ms"])
		if !Finite(start, duration) || start < 0 || duration <= 0 || start < previous-.001 {
			return nil, fmt.Errorf("invalid requested phone timing")
		}
		previous = start + duration
		sourceDuration := Number(phone["end_ms"]) - Number(phone["start_ms"])
		if sourceDuration <= 0 {
			return nil, fmt.Errorf("invalid source phone duration")
		}
		ratio := duration / sourceDuration
		if ratio < .5 || ratio > 2 {
			warnings = append(warnings, "duration-ratio-outside-experimental-range")
		}
		rows = append(rows, Object{"source_phone_index": sourceIndices[i], "requested_phone_index": targetIndices[i], "symbol": phone["symbol"], "source_start_ms": phone["start_ms"], "source_end_ms": phone["end_ms"], "requested_start_ms": start, "requested_end_ms": previous, "duration_ratio": ratio})
	}
	left, right := Int(request["left_context_count"]), Int(request["right_context_count"])
	if left < 0 || right < 0 || left > sourceIndices[0] || sourceIndices[len(sourceIndices)-1]+right >= len(phones) {
		return nil, fmt.Errorf("invalid adjacent context count")
	}
	first, last := sourceIndices[0]-left, sourceIndices[len(sourceIndices)-1]+right
	adjacent, excluded := []int{}, []int{}
	selectedSet := map[int]bool{}
	for _, i := range sourceIndices {
		selectedSet[i] = true
	}
	for i := range phones {
		if i >= first && i <= last {
			if !selectedSet[i] {
				adjacent = append(adjacent, i)
			}
		} else {
			excluded = append(excluded, i)
		}
	}
	coreStart, coreEnd := Number(Map(selected[0])["start_ms"]), Number(Map(selected[len(selected)-1])["end_ms"])
	landmarks := []any{}
	for _, raw := range List(analysis["landmarks"]) {
		landmark := Map(raw)
		position := Number(landmark["source_ms"])
		if coreStart <= position && position < coreEnd {
			record := Object{}
			for k, v := range landmark {
				record[k] = v
			}
			inside := position+Number(landmark["duration_ms"]) <= coreEnd
			record["fully_inside_core"] = inside
			landmarks = append(landmarks, record)
			if !inside {
				warnings = append(warnings, "landmark-extends-past-selected-core")
			}
		}
	}
	return Object{"unit_index": unit["unit_index"], "position": unit["position"], "alias": unit["alias"], "role": unit["role"], "source_sha256": request["source_sha256"], "alignment_sha256": request["alignment_sha256"], "acoustic_model": metadata["acoustic_model"], "selection_status": "unverified", "training_eligible": false, "source_phone_indices": sourceIndices, "adjacent_context_phone_indices": adjacent, "excluded_phone_indices": excluded, "source_clip_phone_intervals": phones, "mappings": rows, "warnings": UniqueStrings(warnings), "core_start_ms": coreStart, "core_end_ms": coreEnd, "context_start_ms": Map(phones[first])["start_ms"], "context_end_ms": Map(phones[last])["end_ms"], "landmark_candidates": landmarks}, nil
}
func writeClip(source, destination string, startMS, endMS float64) (Object, error) {
	pcm, err := audio.ReadWav(source)
	if err != nil {
		return nil, err
	}
	frames := len(pcm.Data) / pcm.Channels
	first := int(math.Max(0, math.Floor(startMS*float64(pcm.SampleRate)/1000)))
	last := int(math.Min(float64(frames), math.Ceil(endMS*float64(pcm.SampleRate)/1000)))
	if first >= last || first >= frames {
		return nil, fmt.Errorf("empty selected PCM interval")
	}
	clip := &audio.PCM{SampleRate: pcm.SampleRate, Channels: pcm.Channels, Data: pcm.Data[first*pcm.Channels : last*pcm.Channels]}
	if err := audio.WriteWav(destination, clip); err != nil {
		return nil, err
	}
	digest, duration, err := ClipIdentity(destination)
	if err != nil {
		return nil, err
	}
	return Object{"path": filepath.Base(destination), "source_start_sample": first, "source_end_sample": last, "sample_rate": pcm.SampleRate, "actual_start_ms": float64(first) * 1000 / float64(pcm.SampleRate), "actual_end_ms": float64(last) * 1000 / float64(pcm.SampleRate), "duration_ms": duration, "pcm_sha256": digest}, nil
}
func Select(reportPath, requestsPath, out string) (Object, error) {
	report, err := Read(reportPath)
	if err != nil {
		return nil, err
	}
	requests, err := Read(requestsPath)
	if err != nil {
		return nil, err
	}
	reportHash, err := HashFile(reportPath)
	if err != nil {
		return nil, err
	}
	if requests["report_sha256"] != reportHash {
		return nil, fmt.Errorf("observation report changed since span proposal")
	}
	units := UnitByIndex(List(report["units"]))
	rows := []any{}
	sources := []string{}
	seen := map[int]bool{}
	for _, raw := range List(requests["units"]) {
		request := Map(raw)
		index := Int(request["unit_index"])
		unit, ok := units[index]
		if !ok || seen[index] {
			return nil, fmt.Errorf("unknown or duplicate source unit")
		}
		seen[index] = true
		source := ResolvedSource(reportPath, String(unit["source_clip"]))
		digest, duration, err := ClipIdentity(source)
		if err != nil {
			return nil, err
		}
		if digest != String(request["source_sha256"]) || math.Abs(duration-Number(Map(unit["analysis"])["duration_ms"])) > .01 {
			return nil, fmt.Errorf("selected source PCM changed")
		}
		row, err := SelectUnit(unit, request, String(report["language"]))
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		sources = append(sources, source)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no unambiguous spans selected")
	}
	if err := FreshDir(out); err != nil {
		return nil, err
	}
	for i, raw := range rows {
		row := Map(raw)
		row["original_clip"] = sources[i]
		for _, kind := range []string{"core", "context"} {
			destination := filepath.Join(out, fmt.Sprintf("unit-%03d-%s.wav", Int(row["unit_index"]), kind))
			clip, err := writeClip(sources[i], destination, Number(row[kind+"_start_ms"]), Number(row[kind+"_end_ms"]))
			if err != nil {
				return nil, err
			}
			row[kind+"_clip"] = clip
		}
	}
	requestHash, err := HashFile(requestsPath)
	if err != nil {
		return nil, err
	}
	result := Object{"version": 1, "language": report["language"], "time_origin": "oto-offset", "observation_report_sha256": reportHash, "selection_request_sha256": requestHash, "requested_timing_kind": "synthesis-plan-not-natural-reference", "mapping_kind": "unverified-source-to-requested-phone-span", "training_eligible_units": 0, "units": rows}
	if err := Write(filepath.Join(out, "spans.json"), result); err != nil {
		return nil, err
	}
	return result, nil
}
