package main

import (
	"flag"
	"fmt"
	"os"

	"utautts/cmd/tools/internal/sourcephone"
)

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected prepare or finish")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	report := f.String("report", "", "")
	work := f.String("work", "", "")
	alignments := f.String("alignments", "", "")
	out := f.String("out", "", "")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	var count int
	var err error
	switch args[0] {
	case "prepare":
		count, err = sourcephone.ChinesePrepare(*report, *out)
	case "finish":
		count, err = sourcephone.ChineseFinish(*work, *alignments, *out)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
	if err == nil {
		fmt.Printf("%s: %d supported Chinese source units\n", args[0], count)
	}
	return err
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
