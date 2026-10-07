package main

import (
	"flag"
	"fmt"
	"os"

	"utautts/cmd/tools/internal/sourcephone"
)

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected propose or select")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	report := f.String("report", "", "")
	requests := f.String("requests", "", "")
	out := f.String("out", "", "")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out required")
	}
	switch args[0] {
	case "propose":
		result, err := sourcephone.Propose(*report, *out)
		if err == nil {
			fmt.Printf("proposed %d spans; %d units remain unmapped\n", len(sourcephone.List(result["units"])), len(sourcephone.List(result["rejected"])))
		}
		return err
	case "select":
		result, err := sourcephone.Select(*report, *requests, *out)
		if err == nil {
			fmt.Printf("exported %d unverified spans: %s\n", len(sourcephone.List(result["units"])), *out)
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
