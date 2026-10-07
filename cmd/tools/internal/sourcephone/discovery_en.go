package sourcephone

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var englishVowels = map[string]bool{"aa": true, "ae": true, "ah": true, "ao": true, "ax": true, "aw": true, "ay": true, "eh": true, "er": true, "ey": true, "ih": true, "iy": true, "ow": true, "oy": true, "uh": true, "uw": true}
var pitchSuffix = regexp.MustCompile(`[A-G][#b]?-?\d+$`)

func AliasHypotheses(unit Object, symbols Object, phonemizer string) []any {
	if String(unit["role"]) != "ending" || len(List(unit["assigned_coda_phones"])) == 0 {
		return nil
	}
	nuclei := map[string]bool{}
	for _, raw := range List(unit["requested_context"]) {
		p := Map(raw)
		if String(p["role"]) == "nucleus" {
			nuclei[String(p["symbol"])] = true
		}
	}
	if len(nuclei) != 1 {
		return nil
	}
	vowel := ""
	for v := range nuclei {
		vowel = v
	}
	coda := stringsOf(List(unit["assigned_coda_phones"]))
	if !englishVowels[vowel] {
		return nil
	}
	all := append([]string{vowel}, coda...)
	for _, phone := range all {
		if len(List(symbols[phone])) == 0 {
			return nil
		}
	}
	alias := strings.TrimSpace(pitchSuffix.ReplaceAllString(String(unit["alias"]), ""))
	terminal := strings.HasSuffix(alias, "-")
	compact := strings.Join(strings.Fields(strings.Trim(alias, "- ")), "")
	possible := []string{""}
	for _, phone := range all {
		next := []string{}
		for _, prefix := range possible {
			for _, raw := range List(symbols[phone]) {
				next = append(next, prefix+String(raw))
			}
		}
		possible = next
		if len(possible) > 4096 {
			return nil
		}
	}
	if !contains(possible, compact) {
		return nil
	}
	primary := "vc"
	if phonemizer == "en-vccv" && !terminal {
		primary = "vcv"
	}
	result := []any{Object{"kind": "vc", "phones": toAny(all), "preferred": primary == "vc"}}
	if phonemizer == "en-vccv" {
		vcv := append(append([]string{}, all...), vowel)
		result = append(result, Object{"kind": "vcv", "phones": toAny(vcv), "preferred": primary == "vcv"})
	}
	return result
}
func toAny(values []string) []any {
	result := make([]any, len(values))
	for i, v := range values {
		result[i] = v
	}
	return result
}
func AcousticLabel(phone string) string {
	label := strings.ToUpper(phone)
	if phone == "ax" {
		label = "AH"
	}
	if englishVowels[phone] {
		label += "0"
	}
	return label
}

func DiscoveryPrepare(reports, libraries []string, out string) (Object, error) {
	if err := FreshDir(out); err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, file := range libraries {
		library, err := Read(file)
		if err != nil {
			return nil, err
		}
		if library["language"] != "en" {
			return nil, fmt.Errorf("English source library required")
		}
		for _, raw := range List(library["entries"]) {
			known[String(Map(raw)["source_sha256"])] = true
		}
	}
	units, requests, provenance, skipped := []any{}, []any{}, []any{}, []any{}
	seen := map[string]bool{}
	for _, file := range reports {
		report, err := Read(file)
		if err != nil {
			return nil, err
		}
		phonemizer := String(report["phonemizer"])
		if report["language"] != "en" || (phonemizer != "en-vccv" && phonemizer != "en-delta") {
			return nil, fmt.Errorf("English Delta/VCCV source observations required")
		}
		hash, err := HashFile(file)
		if err != nil {
			return nil, err
		}
		for _, raw := range List(report["units"]) {
			unit := Map(raw)
			if len(List(unit["assigned_coda_phones"])) == 0 {
				continue
			}
			digest := String(Map(unit["analysis"])["source_sha256"])
			if known[digest] || seen[digest] {
				continue
			}
			hypotheses := AliasHypotheses(unit, Map(report["source_phone_symbols"]), phonemizer)
			if len(hypotheses) == 0 {
				skipped = append(skipped, Object{"report": Absolute(file), "unit_index": unit["unit_index"], "alias": unit["alias"], "reason": "unsupported-or-ambiguous-alias"})
				continue
			}
			seen[digest] = true
			for _, raw := range hypotheses {
				hypothesis := Map(raw)
				index := len(units)
				row := Object{}
				for k, v := range unit {
					row[k] = v
				}
				row["unit_index"] = index
				row["source_clip"] = ResolvedSource(file, String(unit["source_clip"]))
				units = append(units, row)
				phones := []any{}
				for _, phone := range List(hypothesis["phones"]) {
					phones = append(phones, AcousticLabel(String(phone)))
				}
				requests = append(requests, Object{"unit_index": index, "phones": phones})
				record := Object{"candidate_index": index, "source_sha256": digest, "source_report": Absolute(file), "source_report_sha256": hash, "source_unit_index": unit["unit_index"], "alias": unit["alias"]}
				for k, v := range hypothesis {
					record[k] = v
				}
				provenance = append(provenance, record)
			}
		}
	}
	if len(units) == 0 {
		return nil, fmt.Errorf("no supported unknown sources")
	}
	observation := filepath.Join(out, "observations.json")
	requestPath := filepath.Join(out, "requests.json")
	if err := Write(observation, Object{"language": "en", "units": units, "annotation_status": "unobserved"}); err != nil {
		return nil, err
	}
	if err := Write(requestPath, Object{"units": requests}); err != nil {
		return nil, err
	}
	if err := Write(filepath.Join(out, "hypotheses.json"), Object{"version": 1, "kind": "alias-derived-unverified-source-phone-hypotheses", "candidates": provenance, "skipped": skipped}); err != nil {
		return nil, err
	}
	if _, err := Prepare(observation, requestPath, filepath.Join(out, "mfa")); err != nil {
		return nil, err
	}
	return Object{"sources": len(seen), "candidates": len(units), "skipped": skipped}, nil
}

func ScoreCandidate(unit, hypothesis Object) (Object, error) {
	phones := List(unit["forced_phone_intervals"])
	wanted := stringsOf(List(hypothesis["phones"]))
	if len(phones) == 0 || len(phones) != len(wanted) {
		return Object{"accepted": false, "reason": "missing-or-mismatched-alignment"}, nil
	}
	for i, v := range wanted {
		if v == "ax" {
			v = "ah"
		}
		if String(Map(phones[i])["symbol"]) != v {
			return Object{"accepted": false, "reason": "missing-or-mismatched-alignment"}, nil
		}
	}
	audit, err := AcousticAudit(unit, "en")
	if err != nil {
		return nil, err
	}
	analysis := Map(unit["analysis"])
	frames := List(analysis["frames"])
	vowelScores := []any{}
	sumFraction := 0.0
	for _, raw := range phones {
		phone := Map(raw)
		symbol := String(phone["symbol"])
		if !englishVowels[symbol] {
			continue
		}
		start, end := Number(phone["start_ms"]), Number(phone["end_ms"])
		duration := end - start
		active, periodic := 0.0, 0.0
		for _, raw := range frames {
			frame := Map(raw)
			overlap := math.Max(0, math.Min(Number(frame["end_ms"]), end)-math.Max(Number(frame["start_ms"]), start))
			if Number(frame["rms_dbfs"]) >= Number(analysis["low_energy_threshold_dbfs"]) {
				active += overlap
				if Number(frame["periodicity"]) >= .65 {
					periodic += overlap
				}
			}
		}
		fraction := periodic / duration
		vowelScores = append(vowelScores, Object{"symbol": symbol, "start_ms": start, "end_ms": end, "periodic_ms": periodic, "periodic_fraction": fraction, "active_fraction": active / duration})
		sumFraction += fraction
		if duration < 40 || periodic < 25 || fraction < .25 {
			return Object{"accepted": false, "reason": "vowel-hypothesis-lacks-periodic-support", "vowels": vowelScores, "audit": audit}, nil
		}
	}
	warnings := warningStrings(audit["warnings"])
	for _, warning := range warnings {
		if strings.Contains(warning, "outside-alignment") {
			return Object{"accepted": false, "reason": "alignment-leaves-audible-source-activity", "vowels": vowelScores, "audit": audit}, nil
		}
	}
	if len(vowelScores) == 0 {
		return nil, fmt.Errorf("candidate has no vowel")
	}
	penalty := 1 - sumFraction/float64(len(vowelScores))
	if !Bool(hypothesis["preferred"]) {
		penalty += .02
	}
	return Object{"accepted": true, "penalty": penalty, "vowels": vowelScores, "audit": audit}, nil
}

func DiscoveryFinish(work, alignments, model, out string) (Object, error) {
	if err := FreshDir(out); err != nil {
		return nil, err
	}
	report, err := Import(filepath.Join(work, "mfa", "manifest.json"), alignments, model, filepath.Join(out, "aligned-observations.json"))
	if err != nil {
		return nil, err
	}
	hypotheses, err := Read(filepath.Join(work, "hypotheses.json"))
	if err != nil {
		return nil, err
	}
	units := UnitByIndex(List(report["units"]))
	groups := map[string][]struct {
		penalty float64
		index   int
	}{}
	scores := []any{}
	for _, raw := range List(hypotheses["candidates"]) {
		hypothesis := Map(raw)
		index := Int(hypothesis["candidate_index"])
		score, err := ScoreCandidate(units[index], hypothesis)
		if err != nil {
			return nil, err
		}
		record := Object{"hypothesis": hypothesis}
		for k, v := range score {
			record[k] = v
		}
		scores = append(scores, record)
		if Bool(score["accepted"]) {
			digest := String(hypothesis["source_sha256"])
			groups[digest] = append(groups[digest], struct {
				penalty float64
				index   int
			}{Number(score["penalty"]), index})
		}
	}
	digests := map[string]bool{}
	for _, raw := range List(hypotheses["candidates"]) {
		digests[String(Map(raw)["source_sha256"])] = true
	}
	ordered := []string{}
	for digest := range digests {
		ordered = append(ordered, digest)
	}
	sort.Strings(ordered)
	chosen, rejected, indices := []any{}, []any{}, []any{}
	for _, digest := range ordered {
		ranked := groups[digest]
		sort.Slice(ranked, func(i, j int) bool {
			if ranked[i].penalty == ranked[j].penalty {
				return ranked[i].index < ranked[j].index
			}
			return ranked[i].penalty < ranked[j].penalty
		})
		reason := ""
		if len(ranked) == 0 {
			reason = "no-acoustically-supported-hypothesis"
		} else if len(ranked) > 1 && ranked[1].penalty-ranked[0].penalty < .1 {
			reason = "ambiguous-hypothesis-score"
		}
		if reason != "" {
			rejected = append(rejected, Object{"source_sha256": digest, "reason": reason})
			continue
		}
		unit := units[ranked[0].index]
		alignment := Map(unit["phone_alignment"])
		alignment["transcript_source"] = "alias-derived-acoustically-screened-hypothesis"
		alignment["hypothesis_candidate_index"] = ranked[0].index
		chosen = append(chosen, unit)
		indices = append(indices, unit["unit_index"])
	}
	report["units"] = chosen
	selectedPath := filepath.Join(out, "chosen-observations.json")
	if err := Write(selectedPath, report); err != nil {
		return nil, err
	}
	proposal, err := Propose(selectedPath, filepath.Join(out, "requests.json"))
	if err != nil {
		return nil, err
	}
	result := Object{"version": 1, "kind": "heuristic-acoustic-consistency-not-boundary-accuracy", "model": model, "candidates": scores, "chosen_candidates": indices, "rejected": rejected, "verified_units": 0, "training_eligible_units": 0}
	if err := Write(filepath.Join(out, "discovery-audit.json"), result); err != nil {
		return nil, err
	}
	if len(List(proposal["units"])) > 0 {
		selectedDir := filepath.Join(out, "selected")
		if _, err := Select(selectedPath, filepath.Join(out, "requests.json"), selectedDir); err != nil {
			return nil, err
		}
		if _, err := BuildLibrary([]string{filepath.Join(selectedDir, "spans.json")}, filepath.Join(out, "library.json"), false); err != nil {
			return nil, err
		}
	}
	return result, nil
}
