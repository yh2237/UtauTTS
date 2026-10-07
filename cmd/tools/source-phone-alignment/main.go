package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"utautts/cmd/tools/internal/sourcephone"
)

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected prepare, import, evaluate, or audit")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	report := f.String("report", "", "")
	requests := f.String("requests", "", "")
	manifest := f.String("manifest", "", "")
	alignments := f.String("alignments", "", "")
	model := f.String("model", "", "")
	references := f.String("references", "", "")
	out := f.String("out", "", "")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out required")
	}
	var result sourcephone.Object
	var err error
	switch args[0] {
	case "prepare":
		result, err = sourcephone.Prepare(*report, *requests, *out)
		if err == nil {
			fmt.Printf("prepared %d source clips\n", len(sourcephone.List(result["units"])))
		}
	case "import":
		result, err = sourcephone.Import(*manifest, *alignments, *model, *out)
		if err == nil {
			data, _ := json.Marshal(result["alignment_audit"])
			fmt.Println(string(data))
		}
	case "evaluate":
		result, err = sourcephone.Evaluate(*report, *references, *out)
		if err == nil {
			data, _ := json.Marshal(result)
			fmt.Println(string(data))
		}
	case "audit":
		result, err = sourcephone.Audit(*report, *out)
		if err == nil {
			delete(result, "units")
			data, _ := json.Marshal(result)
			fmt.Println(string(data))
		}
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
	return err
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
