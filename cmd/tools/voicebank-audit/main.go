package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"utautts/internal/voicebank"
)

func main() {
	voicebankPath := flag.String("voicebank", "", "UTAUボイスバンクのディレクトリまたはoto.ini")
	outPath := flag.String("out", "", "標準出力の代わりにJSONを書き出すパス")
	flag.Parse()
	if *voicebankPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	bank, err := voicebank.Load(*voicebankPath)
	if err != nil {
		fail(err)
	}
	audit, err := bank.AuditSingleCV()
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if *outPath == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fail(err)
		}
		return
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
