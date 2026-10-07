package sourcephone

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var chineseInitials = map[string]string{"b": "p", "p": "pʰ", "m": "m", "f": "f", "d": "t", "t": "tʰ", "n": "n", "l": "l", "g": "k", "k": "kʰ", "h": "x", "j": "tɕ", "q": "tɕʰ", "x": "ɕ", "s": "s", "z": "ts", "c": "tsʰ"}
var chineseVowels = map[string]string{"a": "a", "e": "ə", "i": "i", "u": "u", "v": "y"}

func PhonesForChineseUnit(unit Object) ([]string, []string) {
	context := List(unit["requested_context"])
	assigned := stringsOf(List(unit["assigned_coda_phones"]))
	if String(unit["role"]) != "mora" || len(assigned) != 1 || (assigned[0] != "n" && assigned[0] != "ng") {
		return nil, nil
	}
	roles := []string{}
	canonical := []string{}
	vowel := ""
	for _, raw := range context {
		p := Map(raw)
		role, symbol := String(p["role"]), String(p["symbol"])
		roles = append(roles, role)
		canonical = append(canonical, symbol)
		if role == "nucleus" {
			vowel = symbol
		}
	}
	pattern := strings.Join(roles, ",")
	if pattern != "onset,nucleus,coda" && pattern != "nucleus,coda" {
		return nil, nil
	}
	if _, ok := chineseVowels[vowel]; !ok {
		return nil, nil
	}
	alias := strings.Trim(pitchSuffix.ReplaceAllString(String(unit["alias"]), ""), "- ")
	if alias != strings.Join(canonical, "") {
		return nil, nil
	}
	labels := []string{}
	for _, raw := range context {
		p := Map(raw)
		role, symbol := String(p["role"]), String(p["symbol"])
		switch role {
		case "onset":
			label, ok := chineseInitials[symbol]
			if !ok {
				return nil, nil
			}
			labels = append(labels, label)
		case "nucleus":
			labels = append(labels, chineseVowels[symbol])
		default:
			if symbol == "ng" {
				labels = append(labels, "ŋ")
			} else {
				labels = append(labels, "n")
			}
		}
	}
	return labels, canonical
}
func ChinesePrepare(reportPath, out string) (int, error) {
	if err := FreshDir(out); err != nil {
		return 0, err
	}
	report, err := Read(reportPath)
	if err != nil {
		return 0, err
	}
	if report["language"] != "zh" {
		return 0, fmt.Errorf("Chinese observations required")
	}
	requests, selected, rejected := []any{}, []any{}, []any{}
	for _, raw := range List(report["units"]) {
		unit := Map(raw)
		labels, canonical := PhonesForChineseUnit(unit)
		if len(labels) == 0 {
			if len(List(unit["assigned_coda_phones"])) > 0 {
				rejected = append(rejected, Object{"unit_index": unit["unit_index"], "reason": "unsupported-compound-or-alias"})
			}
			continue
		}
		row := Object{}
		for k, v := range unit {
			row[k] = v
		}
		row["source_clip"] = ResolvedSource(reportPath, String(unit["source_clip"]))
		selected = append(selected, row)
		requests = append(requests, Object{"unit_index": unit["unit_index"], "phones": toAny(labels), "canonical_phones": toAny(canonical)})
	}
	if len(selected) == 0 {
		return 0, fmt.Errorf("no supported nasal syllables")
	}
	report["units"] = selected
	if err := Write(filepath.Join(out, "observations.json"), report); err != nil {
		return 0, err
	}
	if err := Write(filepath.Join(out, "requests.json"), Object{"units": requests, "rejected": rejected}); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(out, "mfa-config.yaml"), []byte("tokenization: simple\n"), 0644); err != nil {
		return 0, err
	}
	if _, err := Prepare(filepath.Join(out, "observations.json"), filepath.Join(out, "requests.json"), filepath.Join(out, "mfa")); err != nil {
		return 0, err
	}
	return len(selected), nil
}
func ChineseFinish(work, alignments, out string) (int, error) {
	if err := FreshDir(out); err != nil {
		return 0, err
	}
	report, err := Import(filepath.Join(work, "mfa", "manifest.json"), alignments, "mandarin_mfa", filepath.Join(out, "aligned-observations.json"))
	if err != nil {
		return 0, err
	}
	selected, audit := []any{}, []any{}
	for _, raw := range List(report["units"]) {
		unit := Map(raw)
		check, err := AcousticAudit(unit, "zh")
		if err != nil {
			return 0, err
		}
		reason := ""
		if len(List(unit["forced_phone_intervals"])) == 0 {
			reason = "not-aligned"
		} else {
			for _, warning := range warningStrings(check["warnings"]) {
				if strings.Contains(warning, "outside-alignment") {
					reason = "uncovered-audible-source"
				}
			}
			if reason == "" {
				for _, raw := range List(check["phones"]) {
					phone := Map(raw)
					symbol := String(phone["symbol"])
					if (symbol == "n" || symbol == "ng") && (Number(phone["end_ms"])-Number(phone["start_ms"]) < 30-.001 || Number(phone["periodic_ms"]) < 20-.001) {
						reason = "nasal-without-periodic-support"
					}
				}
			}
		}
		var reasonValue any
		if reason != "" {
			reasonValue = reason
		}
		audit = append(audit, Object{"unit_index": unit["unit_index"], "alias": unit["alias"], "accepted": reason == "", "reason": reasonValue, "acoustic_audit": check})
		if reason == "" {
			Map(unit["phone_alignment"])["transcript_source"] = "alias-derived-simple-mandarin-nasal-syllable"
			selected = append(selected, unit)
		}
	}
	report["units"] = selected
	selectedPath := filepath.Join(out, "selected-observations.json")
	if err := Write(selectedPath, report); err != nil {
		return 0, err
	}
	if err := Write(filepath.Join(out, "audit.json"), Object{"kind": "acoustic-consistency-not-boundary-accuracy", "units": audit, "verified_units": 0, "training_eligible_units": 0}); err != nil {
		return 0, err
	}
	if len(selected) > 0 {
		proposal, err := Propose(selectedPath, filepath.Join(out, "requests.json"))
		if err != nil {
			return 0, err
		}
		if len(List(proposal["units"])) > 0 {
			selectedDir := filepath.Join(out, "selected")
			if _, err := Select(selectedPath, filepath.Join(out, "requests.json"), selectedDir); err != nil {
				return 0, err
			}
			if _, err := BuildLibrary([]string{filepath.Join(selectedDir, "spans.json")}, filepath.Join(out, "library.json"), false); err != nil {
				return 0, err
			}
		}
	}
	return len(selected), nil
}
