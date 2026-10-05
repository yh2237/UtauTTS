package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ikawaha/kagome-dict/ipa"

	"utautts/internal/frontend/ipapron"
)

func main() {
	out := flag.String("out", "pron.bin", "output file")
	flag.Parse()
	data, err := ipapron.Build(ipa.Dict())
	if err == nil {
		err = os.WriteFile(*out, data, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
