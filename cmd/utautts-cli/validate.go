package main

import (
	flagpkg "flag"
	"fmt"
	"math"
	"strconv"

	"github.com/yh2237/UtauTTS/internal/entryinput"
	"github.com/yh2237/UtauTTS/internal/settings"
)

// rangeFlagsは設定表の範囲を適用するフラグ。範囲はinternal/settingsと共有する。
var rangeFlags = map[string]string{
	"mora-ms":                 "mora_duration_ms",
	"pause-ms":                "pause_duration_ms",
	"leading-preutterance-ms": "leading_preutterance_ms",
	"intonation-strength":     "intonation_strength",
}

// validateRangesは指定されたフラグの値を設定表の範囲で確かめる。
func validateRanges(flags *flagpkg.FlagSet) error {
	var failure error
	flags.Visit(func(option *flagpkg.Flag) {
		if failure != nil {
			return
		}
		id, ok := rangeFlags[option.Name]
		if !ok {
			return
		}
		setting, ok := settings.Lookup(id)
		if !ok {
			return
		}
		value, err := strconv.ParseFloat(option.Value.String(), 64)
		if err != nil {
			return
		}
		hint := flagHelp(flags, option.Name)
		switch {
		case math.IsNaN(value) || math.IsInf(value, 0):
			failure = cliFailureHint("invalid_arguments", hint, fmt.Errorf("--%s must be a finite number", option.Name))
		case setting.Min != nil && value < *setting.Min:
			failure = cliFailureHint("invalid_arguments", hint, fmt.Errorf("--%s must be at least %g", option.Name, *setting.Min))
		case setting.Max != nil && value > *setting.Max:
			failure = cliFailureHint("invalid_arguments", hint, fmt.Errorf("--%s must be at most %g", option.Name, *setting.Max))
		}
	})
	return failure
}

// validateUtteranceLengthはHTTPと同じ上限をCLIにも適用する。
func validateUtteranceLength(text, reading string) error {
	for _, input := range []struct{ name, value string }{{"text", text}, {"reading", reading}} {
		if len([]rune(input.value)) > entryinput.MaxTextRunes {
			return cliFailureHint("invalid_arguments", "split the input into shorter utterances",
				fmt.Errorf("%s is limited to %d characters", input.name, entryinput.MaxTextRunes))
		}
	}
	return nil
}
