package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"
	"utautts/cmd/tools/internal/sourcephone"
	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/openjtalk"
)

var onsetPhones = map[string]bool{"a": true, "i": true, "ɯ": true, "e": true, "o": true, "aː": true, "iː": true, "ɯː": true, "eː": true, "oː": true, "i̥": true, "ɯ̥": true, "ɴ": true, "ʔ": true}

func safeID(value string) bool {
	return value != "" && value != "." && value != ".." && filepath.Base(value) == value && !strings.ContainsAny(value, "/\\:")
}
func moraPhones(mora, previous string) []string {
	switch mora {
	case "ー":
		if previous != "" {
			return []string{previous}
		}
		return nil
	case "っ":
		return []string{"ʔ"}
	case "ん":
		return []string{"ɴ"}
	}
	return openjtalk.MoraPhones(norm.NFC.String(mora))
}
func wordName(id string, index int) string { return fmt.Sprintf("%s_%03d", strings.ToLower(id), index) }
func eachRecord(paths []string, handle func(sourcephone.Object) error) error {
	for _, file := range paths {
		err := toolutil.ScanJSONL(file, func(line []byte) error {
			var record sourcephone.Object
			if err := json.Unmarshal(line, &record); err != nil {
				return err
			}
			return handle(record)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func prepare(paths []string, out string) (int, int, int, error) {
	if err := sourcephone.FreshDir(out); err != nil {
		return 0, 0, 0, err
	}
	corpus := filepath.Join(out, "corpus")
	if err := os.MkdirAll(corpus, 0755); err != nil {
		return 0, 0, 0, err
	}
	dictionary := []string{}
	prepared, skipped := 0, 0
	err := eachRecord(paths, func(record sourcephone.Object) error {
		id := sourcephone.String(record["id"])
		if !safeID(id) {
			return fmt.Errorf("invalid record id: %q", id)
		}
		words, entries := []string{}, []string{}
		previous := ""
		for index, raw := range sourcephone.List(record["tokens"]) {
			token := sourcephone.Map(raw)
			if sourcephone.Bool(token["pause"]) {
				previous = ""
				continue
			}
			phones := moraPhones(sourcephone.String(token["mora"]), previous)
			if len(phones) == 0 {
				words = nil
				break
			}
			for _, p := range phones {
				if p == "a" || p == "i" || p == "ɯ" || p == "e" || p == "o" {
					previous = p
				}
			}
			name := wordName(id, index)
			words = append(words, name)
			entries = append(entries, name+"\t"+strings.Join(phones, " "))
		}
		if len(words) == 0 {
			skipped++
			return nil
		}
		src := sourcephone.String(record["audio_path"])
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		wav := filepath.Join(corpus, id+".wav")
		f, err := toolutil.CreateExclusive(wav)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(corpus, id+".lab"), []byte(strings.Join(words, " ")), 0644); err != nil {
			return err
		}
		dictionary = append(dictionary, entries...)
		prepared++
		return nil
	})
	if err != nil {
		return 0, 0, 0, err
	}
	if err := os.WriteFile(filepath.Join(out, "dictionary.dict"), []byte(strings.Join(dictionary, "\n")+"\n"), 0644); err != nil {
		return 0, 0, 0, err
	}
	if err := os.WriteFile(filepath.Join(out, "config.yaml"), []byte("tokenization: simple\n"), 0644); err != nil {
		return 0, 0, 0, err
	}
	return prepared, skipped, len(dictionary), nil
}

type interval struct {
	start, end float64
	label      string
}

func alignmentEntries(raw any) ([]interval, error) {
	result := []interval{}
	for _, entry := range sourcephone.List(raw) {
		values := sourcephone.List(entry)
		if len(values) != 3 {
			return nil, fmt.Errorf("invalid MFA interval")
		}
		start, end := sourcephone.Number(values[0]), sourcephone.Number(values[1])
		if !sourcephone.Finite(start, end) {
			return nil, fmt.Errorf("invalid MFA interval")
		}
		result = append(result, interval{start, end, sourcephone.String(values[2])})
	}
	return result, nil
}
func loadAlignment(file string) ([]interval, []interval, error) {
	value, err := sourcephone.Read(file)
	if err != nil {
		return nil, nil, err
	}
	tiers := sourcephone.Map(value["tiers"])
	words, err := alignmentEntries(sourcephone.Map(tiers["words"])["entries"])
	if err != nil {
		return nil, nil, err
	}
	phones, err := alignmentEntries(sourcephone.Map(tiers["phones"])["entries"])
	if err != nil {
		return nil, nil, err
	}
	filteredWords, filteredPhones := []interval{}, []interval{}
	for _, v := range words {
		if v.label != "" && v.label != "<eps>" {
			filteredWords = append(filteredWords, v)
		}
	}
	for _, v := range phones {
		if v.label != "" {
			filteredPhones = append(filteredPhones, v)
		}
	}
	return filteredWords, filteredPhones, nil
}

type span struct{ start, end, onset float64 }

func importAlignments(paths []string, alignments, out string) (int, int, error) {
	if err := toolutil.RequireUnderOut(out, "output", false); err != nil {
		return 0, 0, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return 0, 0, err
	}
	file, err := toolutil.CreateExclusive(out)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	written, skipped := 0, 0
	err = eachRecord(paths, func(record sourcephone.Object) error {
		path := filepath.Join(alignments, sourcephone.String(record["id"])+".json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			skipped++
			return nil
		}
		words, phones, err := loadAlignment(path)
		if err != nil {
			return err
		}
		spans := map[int]span{}
		for _, word := range words {
			inside := []interval{}
			for _, p := range phones {
				if p.start >= word.start-1e-4 && p.end <= word.end+1e-4 {
					inside = append(inside, p)
				}
			}
			onset := word.start
			for i := len(inside) - 1; i >= 0; i-- {
				if onsetPhones[inside[i].label] {
					onset = inside[i].start
					break
				}
			}
			last := strings.LastIndexByte(word.label, '_')
			if last < 0 {
				return fmt.Errorf("invalid MFA word label: %s", word.label)
			}
			index, err := strconv.Atoi(word.label[last+1:])
			if err != nil {
				return err
			}
			spans[index] = span{word.start * 1000, word.end * 1000, onset * 1000}
		}
		tokens := sourcephone.List(record["tokens"])
		speech := []int{}
		for index, raw := range tokens {
			if !sourcephone.Bool(sourcephone.Map(raw)["pause"]) {
				speech = append(speech, index)
			}
		}
		for _, index := range speech {
			if _, ok := spans[index]; !ok {
				skipped++
				return nil
			}
		}
		for order, index := range speech {
			current := spans[index]
			end := current.end
			if order+1 < len(speech) && speech[order+1] == index+1 {
				end = spans[speech[order+1]].onset
			}
			token := sourcephone.Map(tokens[index])
			token["start_ms"] = current.onset
			token["end_ms"] = end
			token["vowel_onset_ms"] = current.onset
			token["consonant_onset_ms"] = current.start
		}
		for index, raw := range tokens {
			token := sourcephone.Map(raw)
			if sourcephone.Bool(token["pause"]) {
				previous := 0.0
				if index > 0 {
					previous = sourcephone.Number(sourcephone.Map(tokens[index-1])["end_ms"])
				}
				following := previous + 1
				if index+1 < len(tokens) {
					following = sourcephone.Number(sourcephone.Map(tokens[index+1])["start_ms"])
				}
				token["start_ms"] = previous
				token["end_ms"] = math.Max(previous+1, following)
			}
		}
		for _, raw := range tokens {
			token := sourcephone.Map(raw)
			duration := sourcephone.Number(token["end_ms"]) - sourcephone.Number(token["start_ms"])
			token["duration_ms"] = duration
			if duration <= 0 {
				skipped++
				return nil
			}
		}
		record["alignment_source"] = "mfa_japanese_note"
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return err
		}
		written++
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	if err := writer.Flush(); err != nil {
		return 0, 0, err
	}
	return written, skipped, nil
}
func mfaArgs(out, model string) []string {
	return []string{"align", filepath.Join(out, "corpus"), filepath.Join(out, "dictionary.dict"), model, filepath.Join(out, "alignments"), "--output_format", "json", "--single_speaker", "--config_path", filepath.Join(out, "config.yaml")}
}
func runWorkflow(paths []string, out, mfa, model string) error {
	prepared, skipped, words, err := prepare(paths, out)
	if err != nil {
		return err
	}
	fmt.Printf("prepared %d utterances (%d skipped), %d mora words: %s\n", prepared, skipped, words, out)
	command := exec.Command(mfa, mfaArgs(out, model)...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("MFA alignment failed: %w", err)
	}
	aligned := filepath.Join(out, "aligned.jsonl")
	written, missing, err := importAlignments(paths, filepath.Join(out, "alignments"), aligned)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %d records (%d skipped): %s\n", written, missing, aligned)
	return nil
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected prepare, import, or run")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	out := f.String("out", "", "")
	alignments := f.String("alignments", "", "")
	mfa := f.String("mfa", "mfa", "external MFA executable")
	model := f.String("model", "japanese_mfa", "MFA acoustic model")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if *out == "" || len(f.Args()) == 0 {
		return fmt.Errorf("--out and dataset required")
	}
	switch args[0] {
	case "run":
		return runWorkflow(f.Args(), *out, *mfa, *model)
	case "prepare":
		prepared, skipped, words, err := prepare(f.Args(), *out)
		if err == nil {
			fmt.Printf("prepared %d utterances (%d skipped), %d mora words: %s\n", prepared, skipped, words, *out)
		}
		return err
	case "import":
		written, skipped, err := importAlignments(f.Args(), *alignments, *out)
		if err == nil {
			fmt.Printf("wrote %d records (%d skipped): %s\n", written, skipped, *out)
		}
		return err
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
