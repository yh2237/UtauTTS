// Package aishell3 は AISHELL-3 の parquet レコードと PaddleSpeech tone TextGrid を読む。
package aishell3

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Record は AISHELL-3 parquet の1行。音声はWAVバイト列で入っている。
type Record struct {
	Audio struct {
		Bytes []byte `parquet:"bytes"`
		Path  string `parquet:"path"`
	} `parquet:"audio"`
	Pinyin string `parquet:"pinyin"`
}

// Interval は TextGrid の1区間。空テキストの区間は含めない。
type Interval struct {
	Start, End float64
	Text       string
}

var intervalPattern = regexp.MustCompile(`intervals \[\d+\]:\s*xmin = ([\d.]+)\s*xmax = ([\d.]+)\s*text = "([^"]*)"`)

func Utterance(audioPath string) string {
	return strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
}

// Speaker は発話IDの先頭7文字から話者IDを返す。
func Speaker(utterance string) string {
	if len(utterance) < 7 {
		return utterance
	}
	return utterance[:7]
}

// TextGridIntervals は raw から name ティアの空でない区間を返す。
func TextGridIntervals(raw, name string) []Interval {
	start := strings.Index(raw, `name = "`+name+`"`)
	if start < 0 {
		return nil
	}
	section := raw[start:]
	if index := strings.Index(section, "item ["); index >= 0 {
		section = section[:index]
	}
	var result []Interval
	for _, match := range intervalPattern.FindAllStringSubmatch(section, -1) {
		a, errA := strconv.ParseFloat(match[1], 64)
		b, errB := strconv.ParseFloat(match[2], 64)
		if errA != nil || errB != nil || strings.TrimSpace(match[3]) == "" {
			continue
		}
		result = append(result, Interval{Start: a, End: b, Text: match[3]})
	}
	return result
}

func ReadTextGrid(path, name string) ([]Interval, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return TextGridIntervals(string(raw), name), nil
}
