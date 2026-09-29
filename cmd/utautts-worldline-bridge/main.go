package main

import (
	"fmt"
	"os"
	"strings"

	"utautts/internal/worldrender"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "--provider" {
		if strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("provider id must not be empty")
		}
		return worldrender.ServeProvider(os.Stdin, os.Stdout, args[1])
	}
	return fmt.Errorf("usage: utautts-worldline-bridge --provider PROVIDER_ID")
}
