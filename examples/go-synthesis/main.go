package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/yh2237/UtauTTS"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	assets := flag.String("assets", "../..", "UtauTTS resource directory")
	voice := flag.String("voice-dir", "", "voicebank directory (default: assets/voice)")
	text := flag.String("text", "こんにちは。今日はいい天気ですね。", "text to synthesize")
	library := flag.String("openjtalk-engine", "", "Open JTalk engine library (default: assets/runtime)")
	dictionary := flag.String("openjtalk-dictionary", "", "Open JTalk dictionary (default: assets/runtime)")
	out := flag.String("out", "speech.wav", "output WAV path")
	flag.Parse()
	root, err := filepath.Abs(*assets)
	if err != nil {
		return err
	}
	if *voice == "" {
		*voice = filepath.Join(root, "voice")
	}
	libraryName := "utautts-openjtalk-engine.so"
	switch runtime.GOOS {
	case "windows":
		libraryName = "utautts-openjtalk-engine.dll"
	case "darwin":
		libraryName = "utautts-openjtalk-engine.dylib"
	}
	if *library == "" {
		*library = filepath.Join(root, "runtime", libraryName)
	}
	if *dictionary == "" {
		*dictionary = filepath.Join(root, "runtime", "open_jtalk_dic_utf_8-1.11")
	}
	engine, err := utautts.New(utautts.Config{
		VoiceDirectory: *voice, OpenJTalkPath: *library, OpenJTalkDictionary: *dictionary,
		ModelDirectories: []string{filepath.Join(root, "models")},
	})
	if err != nil {
		return err
	}
	defer func() { runErr = errors.Join(runErr, engine.Close()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	voices, err := engine.Voicebanks(ctx)
	if err != nil {
		return err
	}
	if len(voices) == 0 {
		return errors.New("no voicebanks found")
	}
	result, err := engine.Synthesize(ctx, utautts.Request{
		Text: *text, Language: "ja", Method: "waveform-synthesis", VoicebankID: voices[0].ID,
	})
	if err != nil {
		return err
	}
	file, err := os.Create(*out)
	if err != nil {
		return err
	}
	err = errors.Join(result.Audio.WriteWAV(file), file.Close())
	if err == nil {
		fmt.Printf("wrote %s: %s, %.2fs\n", *out, result.Reading, result.DurationMS/1000)
	}
	return err
}
