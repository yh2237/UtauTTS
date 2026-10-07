package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"utautts/cmd/tools/internal/sourcephone"
)

type paths []string

func (p *paths) String() string         { return strings.Join(*p, ",") }
func (p *paths) Set(value string) error { *p = append(*p, value); return nil }
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected prepare or finish")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	var reports, libraries paths
	f.Var(&reports, "report", "source observation report (repeatable)")
	f.Var(&libraries, "library", "known source library (repeatable)")
	work := f.String("work", "", "")
	alignments := f.String("alignments", "", "")
	model := f.String("model", "english_us_arpa", "")
	out := f.String("out", "", "")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	switch args[0] {
	case "prepare":
		result, err := sourcephone.DiscoveryPrepare(reports, libraries, *out)
		if err == nil {
			fmt.Printf("prepared %d sources / %d hypotheses\n", result["sources"], result["candidates"])
		}
		return err
	case "finish":
		result, err := sourcephone.DiscoveryFinish(*work, *alignments, *model, *out)
		if err == nil {
			fmt.Printf("selected %d sources; rejected %d\n", len(sourcephone.List(result["chosen_candidates"])), len(sourcephone.List(result["rejected"])))
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
