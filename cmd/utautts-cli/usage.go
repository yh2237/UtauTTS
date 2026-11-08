package main

import (
	flagpkg "flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// usageGroupsは--helpの表示順。説明はフラグ定義から引く。
var usageGroups = []struct {
	title string
	flags []string
}{
	{"common", []string{"json", "version", "timeout", "help"}},
	{"input", []string{"text", "reading", "kana", "text-file", "reading-file", "request", "language", "phonemizer", "dictionary", "method"}},
	{"output", []string{"out", "plan-out", "ustx-out", "write-text", "write-lab", "text-encoding"}},
	{"voicebank", []string{"voicebank", "tone", "color", "alias-policy", "openjtalk-engine", "openjtalk-dictionary"}},
	{"prosody", []string{
		"prosody", "model-dir", "pitch-contours", "pitch-case", "apply-pitch", "intonation-strength",
		"manual-pitch", "prosody-features", "prosody-feature-case", "prosody-pitch-only",
		"context-duration", "context-duration-strength", "boundary-tone", "boundary-tone-strength",
		"stretch-adapt", "stretch-adapt-strength", "pause-context", "pause-context-strength",
		"microprosody", "timing-warp",
	}},
	{"timing", []string{
		"mora-ms", "pause-ms", "leading-preutterance-ms", "release-ms",
		"mora-durations", "phone-weights", "phone-lengths",
		"cvvc-timing", "cvvc-transition-gain", "cvvc-pre-boundary-fade",
	}},
	{"diagnostics", []string{"list", "doctor"}},
}

var usageExamples = []string{
	"# synthesize text from a file",
	"utautts-cli --voicebank voice/example --method waveform-synthesis --text-file input.txt --out out.wav",
	"",
	"# apply a prosody model",
	"utautts-cli --voicebank voice/example --method waveform-synthesis --prosody frame-intonation-tcn-v12 --text-file input.txt --out out.wav",
	"",
	"# synthesize from a JSON request and print the result as JSON",
	"utautts-cli --voicebank voice/example --request request.json --out out.wav --json",
	"",
	"# list voicebanks or models",
	"utautts-cli --list voicebanks",
	"",
	"# check the environment without synthesizing",
	"utautts-cli --doctor --voicebank voice/example",
}

type usageLine struct {
	spec        string
	description string
}

// writeUsageは--helpの本文を書く。フラグの説明はflagの定義を唯一の出所にする。
func writeUsage(output io.Writer, flags *flagpkg.FlagSet) error {
	var text strings.Builder
	text.WriteString("utautts-cli - synthesize speech from a UTAU voicebank\n\n")
	text.WriteString("usage:\n")
	text.WriteString("  utautts-cli --voicebank <dir> --method <id> --out <file> (--text <text> | --reading <kana> | --text-file <path> | --request <path>)\n\n")
	text.WriteString("examples:\n")
	for _, example := range usageExamples {
		if example == "" {
			text.WriteByte('\n')
			continue
		}
		fmt.Fprintf(&text, "  %s\n", example)
	}
	for _, group := range usageGroups {
		fmt.Fprintf(&text, "\n%s:\n", group.title)
		lines := make([]usageLine, 0, len(group.flags))
		width := 0
		for _, name := range group.flags {
			line, ok := describeFlag(flags, name)
			if !ok {
				continue
			}
			lines = append(lines, line)
			width = max(width, len(line.spec))
		}
		for _, line := range lines {
			fmt.Fprintf(&text, "  %-*s  %s\n", width, line.spec, line.description)
		}
	}
	_, err := io.WriteString(output, text.String())
	return err
}

// describeFlagは1つのフラグの表記と説明を返す。
func describeFlag(flags *flagpkg.FlagSet, name string) (usageLine, bool) {
	if name == "help" {
		return usageLine{spec: "--help", description: "show this message"}, true
	}
	option := flags.Lookup(name)
	if option == nil {
		return usageLine{}, false
	}
	typeName, description := flagpkg.UnquoteUsage(option)
	spec := "--" + option.Name
	if typeName != "" {
		if typeName == "float" {
			typeName = "number"
		}
		spec += " <" + typeName + ">"
	}
	if option.DefValue != "" && option.DefValue != "0" && option.DefValue != "0s" && option.DefValue != "false" {
		description += fmt.Sprintf(" (default %s)", option.DefValue)
	}
	return usageLine{spec: spec, description: description}, true
}

// flagHelpは指定したフラグの説明を1行ずつ返す。必須引数の不足を伝えるときに使う。
func flagHelp(flags *flagpkg.FlagSet, names ...string) string {
	lines := make([]string, 0, len(names))
	for _, name := range names {
		line, ok := describeFlag(flags, name)
		if !ok {
			continue
		}
		lines = append(lines, line.spec+"  "+line.description)
	}
	return strings.Join(lines, "\n")
}

// jsonRequestedは--jsonを先読みする。フラグ解析に失敗してもJSONで返すため。
func jsonRequested(flags *flagpkg.FlagSet, args []string) bool {
	requested := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			break
		}
		name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		switch {
		case name == "json":
			requested = true
		case strings.HasPrefix(name, "json="):
			parsed, err := strconv.ParseBool(strings.TrimPrefix(name, "json="))
			requested = err == nil && parsed
		case takesValue(flags, name):
			index++ // 値の引数を読み飛ばす
		}
	}
	return requested
}

// takesValueはフラグが値を受け取るかを返す。真偽値フラグだけが値を持たない。
func takesValue(flags *flagpkg.FlagSet, name string) bool {
	option := flags.Lookup(name)
	if option == nil {
		return false
	}
	typeName, _ := flagpkg.UnquoteUsage(option)
	return typeName != ""
}
