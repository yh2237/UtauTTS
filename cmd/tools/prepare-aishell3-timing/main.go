// AISHELL-3のparquet音声とPaddleSpeechのtone TextGridから、
// 時間伸縮モデル学習用のcorpus.jsonl（トークン+音素区間、runtime記号）を作る。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/parquet-go/parquet-go"
	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
	"utautts/internal/frontend"
)

type parquetRecord struct {
	Audio struct {
		Bytes []byte `parquet:"bytes"`
		Path  string `parquet:"path"`
	} `parquet:"audio"`
	Pinyin string `parquet:"pinyin"`
}

type interval struct {
	start, end float64
	text       string
}

type corpusPhone struct {
	Symbol  string  `json:"symbol"`
	Role    string  `json:"role,omitempty"`
	StartMS float64 `json:"start_ms"`
	EndMS   float64 `json:"end_ms"`
}

type corpusToken struct {
	StartMS float64       `json:"start_ms"`
	EndMS   float64       `json:"end_ms"`
	Phones  []corpusPhone `json:"phones"`
}

type corpusLine struct {
	Version   int           `json:"version"`
	ID        string        `json:"id"`
	Language  string        `json:"language"`
	AudioPath string        `json:"audio_path"`
	Tokens    []corpusToken `json:"tokens"`
}

var intervalPattern = regexp.MustCompile(`intervals \[\d+\]:\s*xmin = ([\d.]+)\s*xmax = ([\d.]+)\s*text = "([^"]*)"`)

func tierIntervals(raw, name string) []interval {
	start := strings.Index(raw, `name = "`+name+`"`)
	if start < 0 {
		return nil
	}
	section := raw[start:]
	if index := strings.Index(section, "\n    item ["); index >= 0 {
		section = section[:index]
	}
	var result []interval
	for _, match := range intervalPattern.FindAllStringSubmatch(section, -1) {
		a, errA := strconv.ParseFloat(match[1], 64)
		b, errB := strconv.ParseFloat(match[2], 64)
		if errA != nil || errB != nil || strings.TrimSpace(match[3]) == "" {
			continue
		}
		result = append(result, interval{a, b, match[3]})
	}
	return result
}

func main() {
	parquetPath := flag.String("parquet", filepath.Join("data", "aishell3", "train-00000-of-00045.parquet"), "AISHELL-3 parquet shard")
	alignments := flag.String("alignments", filepath.Join("data", "aishell3", "aishell3_alignment_tone"), "tone TextGrid directory")
	outAudio := flag.String("out-audio", filepath.Join("out", "aishell3-zh-timing", "audio"), "extracted WAV directory")
	outCorpus := flag.String("out-corpus", filepath.Join("out", "aishell3-zh-timing", "corpus.jsonl"), "corpus JSONL path")
	speakers := flag.Int("speakers", 0, "limit speakers (0: all)")
	perSpeaker := flag.Int("per-speaker", 0, "limit utterances per speaker (0: all)")
	flag.Parse()
	if err := run(*parquetPath, *alignments, *outAudio, *outCorpus, *speakers, *perSpeaker); err != nil {
		fmt.Fprintln(os.Stderr, "aishell3-zh-corpus:", err)
		os.Exit(1)
	}
}

func run(parquetPath, alignments, outAudio, outCorpus string, speakerLimit, perSpeaker int) error {
	file, err := os.Open(parquetPath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := parquet.NewGenericReader[parquetRecord](file)
	defer reader.Close()
	if err := os.MkdirAll(outAudio, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outCorpus), 0o755); err != nil {
		return err
	}
	corpus, err := toolutil.CreateExclusive(outCorpus)
	if err != nil {
		return err
	}
	defer corpus.Close()
	encoder := json.NewEncoder(corpus)

	seen := map[string]int{}
	order := []string{}
	total, written, skipped := 0, 0, 0
	batch := make([]parquetRecord, 64)
	for {
		count, readErr := reader.Read(batch)
		for index := 0; index < count; index++ {
			record := batch[index]
			total++
			utterance := strings.TrimSuffix(filepath.Base(record.Audio.Path), filepath.Ext(record.Audio.Path))
			// AISHELL-3のIDは話者7文字+発話4文字。
			speaker := utterance
			if len(utterance) > 4 {
				speaker = utterance[:len(utterance)-4]
			}
			if speakerLimit > 0 {
				if _, ok := seen[speaker]; !ok {
					if len(order) >= speakerLimit {
						continue
					}
					order = append(order, speaker)
				}
				seen[speaker]++
				if perSpeaker > 0 && seen[speaker] > perSpeaker {
					continue
				}
			}
			gridPath := filepath.Join(alignments, speaker, utterance+".TextGrid")
			raw, err := os.ReadFile(gridPath)
			if err != nil {
				skipped++
				continue
			}
			text := string(raw)
			words := tierIntervals(text, "words")
			phones := tierIntervals(text, "phones")
			tokens, err := buildTokens(words, phones)
			if err != nil || len(tokens) == 0 {
				skipped++
				continue
			}
			pcm, err := audio.DecodeWav(bytes.NewReader(record.Audio.Bytes))
			if err != nil {
				skipped++
				continue
			}
			wavPath := filepath.Join(outAudio, utterance+".wav")
			if _, err := os.Stat(wavPath); err != nil {
				if err := audio.WriteWav(wavPath, pcm); err != nil {
					return err
				}
			}
			line := corpusLine{Version: 1, ID: utterance, Language: "zh", AudioPath: wavPath, Tokens: tokens}
			if err := encoder.Encode(line); err != nil {
				return err
			}
			written++
			if written%200 == 0 {
				fmt.Printf("corpus %d (scanned %d, skipped %d)\n", written, total, skipped)
			}
		}
		if readErr != nil {
			break
		}
		if count == 0 {
			break
		}
	}
	fmt.Printf("done: scanned=%d written=%d skipped=%d speakers=%d corpus=%s\n", total, written, skipped, len(order), outCorpus)
	return nil
}

// buildTokensは単語tierと音素tierから、runtime記号の音素区間を作る。
func buildTokens(words, phones []interval) ([]corpusToken, error) {
	phoneIndex := 0
	var tokens []corpusToken
	for _, word := range words {
		if strings.TrimSpace(word.text) == "" {
			continue
		}
		_, morae, err := frontend.ParseChineseCVVC("", word.text, nil)
		if err != nil || len(morae) == 0 || len(morae[0].Phones) == 0 {
			return nil, fmt.Errorf("invalid syllable %q", word.text)
		}
		runtime := morae[0].Phones
		// 音素tierからこの音節のinitialとfinalを取る。
		var initialSpans []interval
		var finalSpan interval
		hasFinal := false
		if len(runtime) > 0 && runtime[0].Role == "onset" {
			if phoneIndex < len(phones) && !strings.ContainsAny(phones[phoneIndex].text, "12345") {
				initialSpans = append(initialSpans, phones[phoneIndex])
				phoneIndex++
			}
		}
		if phoneIndex < len(phones) {
			finalSpan = phones[phoneIndex]
			hasFinal = true
			phoneIndex++
		}
		spans := allocateSpans(runtime, word, initialSpans, finalSpan, hasFinal)
		token := corpusToken{StartMS: word.start * 1000, EndMS: word.end * 1000}
		for index, phone := range runtime {
			token.Phones = append(token.Phones, corpusPhone{
				Symbol: phone.Symbol, Role: phone.Role,
				StartMS: spans[index][0] * 1000, EndMS: spans[index][1] * 1000,
			})
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// allocateSpansはinitialの実測区間とfinalの実測区間へ音素を配分する。
func allocateSpans(runtime []frontend.Phone, word interval, initials []interval, final interval, hasFinal bool) [][2]float64 {
	spans := make([][2]float64, len(runtime))
	weights := make([]float64, len(runtime))
	for i, phone := range runtime {
		weights[i] = frontend.PhoneWeight(phone.Symbol, phone.Role)
	}
	onsetCount := 0
	for _, phone := range runtime {
		if phone.Role == "onset" {
			onsetCount++
		}
	}
	if onsetCount == 0 || len(initials) == 0 {
		// 実測のinitialが無い場合は音節全体（finalがあればその区間）へ配る。
		start, end := word.start, word.end
		if hasFinal {
			start, end = final.start, final.end
		}
		shares := frontend.PhoneSpansFromWeights(weights, end-start)
		cursor := start
		for i := range runtime {
			spans[i] = [2]float64{cursor, cursor + shares[i]}
			cursor += shares[i]
		}
		return spans
	}
	// initialの実測区間をonset音素へ配る。
	start, end := initials[0].start, initials[0].end
	shares := frontend.PhoneSpansFromWeights(weights[:onsetCount], end-start)
	cursor := start
	for i := 0; i < onsetCount; i++ {
		spans[i] = [2]float64{cursor, cursor + shares[i]}
		cursor += shares[i]
	}
	// 残り（韻母・coda）をfinal区間へ配る。
	if onsetCount < len(runtime) {
		start, end := word.start, word.end
		if hasFinal {
			start, end = final.start, final.end
		}
		shares := frontend.PhoneSpansFromWeights(weights[onsetCount:], end-start)
		cursor := start
		for i := onsetCount; i < len(runtime); i++ {
			spans[i] = [2]float64{cursor, cursor + shares[i-onsetCount]}
			cursor += shares[i-onsetCount]
		}
	}
	return spans
}
