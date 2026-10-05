package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/frontend"
)

type manifestRow struct {
	ID        string `json:"id"`
	Speaker   string `json:"speaker"`
	AudioPath string `json:"audio_path"`
	Text      string `json:"text"`
}
type interval struct {
	Start, End float64
	Label      string
}

func intervals(raw any) ([]interval, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid interval list")
	}
	out := make([]interval, 0, len(items))
	for _, item := range items {
		triple, ok := item.([]any)
		if !ok || len(triple) < 3 {
			return nil, fmt.Errorf("invalid interval")
		}
		a, aok := triple[0].(float64)
		b, bok := triple[1].(float64)
		label, lok := triple[2].(string)
		if !aok || !bok || !lok {
			return nil, fmt.Errorf("invalid interval fields")
		}
		out = append(out, interval{a, b, label})
	}
	return out, nil
}
func tierEntries(alignment map[string]any, suffix string) ([]interval, error) {
	tiers, ok := alignment["tiers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing tiers")
	}
	for name, tier := range tiers {
		if strings.HasSuffix(name, suffix) {
			obj, ok := tier.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid tier")
			}
			return intervals(obj["entries"])
		}
	}
	return nil, fmt.Errorf("missing %s tier", suffix)
}
func normalize(phone string) string { return strings.ToLower(strings.TrimRight(phone, "012")) }

func build(row manifestRow, alignment map[string]any) (map[string]any, error) {
	words, e := tierEntries(alignment, "words")
	if e != nil {
		return nil, e
	}
	phones, e := tierEntries(alignment, "phones")
	if e != nil {
		return nil, e
	}
	filtered := make([]interval, 0, len(phones))
	for _, p := range phones {
		if p.Label == "spn" {
			return nil, fmt.Errorf("unknown phone")
		}
		if p.Label == "" || p.Label == "sil" || p.Label == "sp" {
			continue
		}
		if p.End <= p.Start {
			return nil, fmt.Errorf("invalid phone interval")
		}
		filtered = append(filtered, p)
	}
	phones = filtered
	if len(phones) == 0 {
		return nil, fmt.Errorf("invalid phone interval")
	}
	var groups [][]interval
	for _, word := range words {
		if word.Label == "" || word.Label == "<eps>" || word.Label == "sil" {
			continue
		}
		var group []interval
		for _, p := range phones {
			if word.Start <= (p.Start+p.End)/2 && (p.Start+p.End)/2 < word.End {
				group = append(group, p)
			}
		}
		if len(group) == 0 || word.Label == "<unk>" {
			return nil, fmt.Errorf("unknown/unmapped word")
		}
		groups = append(groups, group)
	}
	count := 0
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		count += len(group)
		labels := make([]string, len(group))
		for i, p := range group {
			labels[i] = p.Label
		}
		parts = append(parts, strings.Join(labels, " "))
	}
	if count != len(phones) {
		return nil, fmt.Errorf("unmapped phones")
	}
	_, units, e := frontend.ParseEnglishDelta("", strings.Join(parts, " | "), nil)
	if e != nil {
		return nil, e
	}
	tokens := make([]map[string]any, 0, len(units))
	offset := 0
	for _, unit := range units {
		if unit.Pause {
			return nil, fmt.Errorf("unexpected parsed pause")
		}
		n := len(unit.Phones)
		if offset+n > len(phones) || n == 0 {
			return nil, fmt.Errorf("phone sequence mismatch")
		}
		aligned := phones[offset : offset+n]
		ph := make([]map[string]string, n)
		for i, p := range unit.Phones {
			if normalize(aligned[i].Label) != p.Symbol {
				return nil, fmt.Errorf("phone sequence mismatch")
			}
			ph[i] = map[string]string{"symbol": p.Symbol, "role": p.Role}
		}
		start, end := aligned[0].Start*1000, aligned[n-1].End*1000
		if end-start < 20 || end-start > 1500 {
			return nil, fmt.Errorf("extreme syllable duration")
		}
		if len(tokens) > 0 && start-tokens[len(tokens)-1]["end_ms"].(float64) >= 80 {
			tokens = append(tokens, map[string]any{"language": "en", "pause": true, "start_ms": tokens[len(tokens)-1]["end_ms"], "end_ms": start, "word_index": -1})
		}
		tokens = append(tokens, map[string]any{"language": "en", "pause": false, "phones": ph, "stress": unit.Stress, "stress_known": unit.StressKnown, "word_index": unit.WordIndex, "word_end": unit.WordEnd, "start_ms": start, "end_ms": end})
		offset += n
	}
	if offset != len(phones) {
		return nil, fmt.Errorf("unused phones")
	}
	audio, e := os.ReadFile(row.AudioPath)
	if e != nil {
		return nil, e
	}
	digest := sha256.Sum256(audio)
	end, ok := alignment["end"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing alignment end")
	}
	return map[string]any{"version": 1, "id": row.ID, "language": "en", "speaker": row.Speaker, "text": row.Text, "audio_path": row.AudioPath, "audio_sha256": fmt.Sprintf("%x", digest), "alignment_source": "MFA english_us_arpa; matched phoneme intervals", "start_ms": 0, "end_ms": end * 1000, "tokens": tokens}, nil
}

func run(manifest, alignments, failed, out string) (map[string]any, error) {
	if !toolutil.UnderOutChild(out) {
		return nil, fmt.Errorf("output must be under out/")
	}
	data, e := os.ReadFile(manifest)
	if e != nil {
		return nil, e
	}
	var rows []manifestRow
	if e = json.Unmarshal(data, &rows); e != nil {
		return nil, e
	}
	badData, e := os.ReadFile(failed)
	if e != nil {
		return nil, e
	}
	bad := map[string]bool{}
	for _, line := range strings.Split(string(badData), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			bad[strings.TrimSuffix(filepath.Base(line), filepath.Ext(line))] = true
		}
	}
	set := map[string]bool{}
	for _, r := range rows {
		set[r.Speaker] = true
	}
	speakers := make([]string, 0, len(set))
	for s := range set {
		speakers = append(speakers, s)
	}
	sort.Slice(speakers, func(i, j int) bool {
		a := sha256.Sum256([]byte(speakers[i]))
		b := sha256.Sum256([]byte(speakers[j]))
		return string(a[:]) < string(b[:])
	})
	if len(speakers) < 10 {
		return nil, fmt.Errorf("at least 10 speakers required")
	}
	splits := map[string]string{}
	for i, s := range speakers {
		split := "train"
		if i < 3 {
			split = "test"
		} else if i < 6 {
			split = "validation"
		}
		splits[s] = split
	}
	accepted := []map[string]any{}
	rejected := []map[string]string{}
	seen := map[string]string{}
	cleanup := regexp.MustCompile(`\W+`)
	for _, r := range rows {
		if bad[r.ID] {
			rejected = append(rejected, map[string]string{"id": r.ID, "reason": "official restoration failure list"})
			continue
		}
		path := filepath.Join(alignments, r.Speaker, r.ID+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			rejected = append(rejected, map[string]string{"id": r.ID, "reason": err.Error()})
			continue
		}
		var alignment map[string]any
		if err = json.Unmarshal(raw, &alignment); err != nil {
			return nil, err
		}
		record, err := build(r, alignment)
		if err == nil {
			record["split"] = splits[r.Speaker]
			key := cleanup.ReplaceAllString(strings.ToLower(r.Text), "")
			if _, ok := seen[key]; ok {
				err = fmt.Errorf("duplicate text")
			} else {
				seen[key] = splits[r.Speaker]
			}
		}
		if err != nil {
			rejected = append(rejected, map[string]string{"id": r.ID, "reason": err.Error()})
		} else {
			accepted = append(accepted, record)
		}
	}
	if e = os.MkdirAll(filepath.Dir(out), 0755); e != nil {
		return nil, e
	}
	if _, e = os.Stat(out); e == nil {
		return nil, fmt.Errorf("refusing to overwrite %s", out)
	}
	f, e := toolutil.CreateExclusive(out)
	if e != nil {
		return nil, e
	}
	w := bufio.NewWriter(f)
	for _, r := range accepted {
		b, _ := json.Marshal(r)
		if _, e = w.Write(append(b, '\n')); e != nil {
			f.Close()
			return nil, e
		}
	}
	if e = w.Flush(); e != nil {
		f.Close()
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	counts := map[string]int{}
	for _, r := range accepted {
		counts[r["split"].(string)]++
	}
	report := map[string]any{"accepted": len(accepted), "splits": counts, "speaker_splits": splits, "rejected": rejected}
	audit := strings.TrimSuffix(out, filepath.Ext(out)) + ".audit.json"
	b, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return nil, e
	}
	if e = os.WriteFile(audit, append(b, '\n'), 0644); e != nil {
		return nil, e
	}
	return report, nil
}
func main() {
	manifest := flag.String("manifest", "", "input manifest JSON")
	alignments := flag.String("alignments", "", "MFA alignment directory")
	failed := flag.String("failed-list", "", "official failed restoration list")
	out := flag.String("out", "", "output JSONL under out/")
	flag.String("helper", "", "ignored: Go calls the same internal/frontend parser directly")
	flag.Parse()
	if *manifest == "" || *alignments == "" || *failed == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "--manifest, --alignments, --failed-list and --out are required")
		os.Exit(2)
	}
	report, e := run(*manifest, *alignments, *failed, *out)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("accepted=%d splits=%v\n", report["accepted"], report["splits"])
}
