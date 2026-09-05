package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/openutau"
)

// utautts-ustxはUtauTTSプロジェクト(.utautts)をOpenUtauのUSTXへ変換する。
//
// 使い方:
//
//	utautts-ustx <project.utautts> [output.ustx]
//
// 出力先を省略するとプロジェクトと同じ場所へ保存する。
func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Fprintln(os.Stderr, "usage: utautts-ustx <project.utautts> [output.ustx]")
		os.Exit(2)
	}
	projectPath := os.Args[1]
	outputPath := ""
	if len(os.Args) >= 3 {
		outputPath = os.Args[2]
	} else {
		base := strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath))
		outputPath = filepath.Join(filepath.Dir(projectPath), base+".ustx")
	}

	project, err := openutau.LoadUtauTTSProject(projectPath)
	if err != nil {
		log.Fatal(err)
	}
	data, err := openutau.ExportUSTX(project, openutau.ExportOptions{})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d utterances, %d bytes)", outputPath, len(project.Utterances), len(data))
}
