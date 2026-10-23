package main

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/yh2237/UtauTTS/internal/appinfo"
	"github.com/yh2237/UtauTTS/internal/openjtalk"
	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

var doctorFlags = map[string]bool{
	"doctor": true, "json": true, "version": true, "voicebank": true,
	"renderer": true, "prosody": true,
	"renderer-dir": true, "model-dir": true,
	"openjtalk-features": true, "openjtalk-dictionary": true,
}

type cliDoctorReport struct {
	Version string           `json:"version"`
	OS      string           `json:"os"`
	Arch    string           `json:"arch"`
	Checks  []cliDoctorCheck `json:"checks"`
}

type cliDoctorCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

func inspectEnvironment(voicePath, modelID string, config synth.RuntimeConfig) (report *cliDoctorReport, runErr error) {
	report = &cliDoctorReport{Version: appinfo.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, Checks: make([]cliDoctorCheck, 0)}
	add := func(id, path, message string, err error, required bool) {
		status := "ok"
		if err != nil {
			status, message = "warning", err.Error()
			if required {
				status = "error"
				runErr = errors.Join(runErr, cliFailure("unavailable", fmt.Errorf("%s: %w", id, err)))
			}
		}
		report.Checks = append(report.Checks, cliDoctorCheck{ID: id, Status: status, Path: path, Message: message})
	}
	rt, err := synth.NewRuntime(config, nil)
	if err != nil {
		return nil, cliFailure("initialization_error", err)
	}
	defer func() { runErr = errors.Join(runErr, cliFailure("shutdown_error", rt.Close())) }()
	for _, problem := range rt.Catalog.Problems {
		add("catalog", "", "", errors.New(problem), false)
	}
	selected, err := rt.Service.ResolveRenderer(config.Renderer)
	if err != nil {
		add("renderer", "", "", err, true)
	} else {
		definition, _ := rt.Catalog.Renderer(string(selected.PublicID()))
		add("renderer", definition.Directory, string(selected.PublicID()), selected.RequireAvailable(), true)
	}
	modelPath, err := rt.Service.ResolveModel(modelID)
	modelMessage := modelID
	if modelPath == "" && err == nil {
		modelMessage = "no prosody model selected"
	}
	add("model", modelPath, modelMessage, err, true)
	if voicePath == "" {
		add("voicebank", "", "", errors.New("no voicebank selected; use --voicebank to inspect one"), false)
	} else {
		bank, err := voicebank.InspectSinger(voicePath)
		add("voicebank", voicePath, bank.Name, err, true)
	}
	helper, err := openjtalk.ResolveHelperPath(config.OpenJTalkPath)
	if err != nil {
		helper = config.OpenJTalkPath
	}
	add("openjtalk_helper", helper, "helper file found (not executed)", err, config.OpenJTalkPath != "")
	dictionary, err := openjtalk.ResolveDictionaryPath(config.OpenJTalkDictionary)
	if err != nil {
		dictionary = config.OpenJTalkDictionary
	}
	add("openjtalk_dictionary", dictionary, "dictionary directory found (contents not validated)", err, config.OpenJTalkDictionary != "")
	return report, runErr
}

func writeDoctor(output io.Writer, report *cliDoctorReport) error {
	var text strings.Builder
	fmt.Fprintf(&text, "%s %s (%s/%s)\n", appinfo.Name(), report.Version, report.OS, report.Arch)
	for _, check := range report.Checks {
		fmt.Fprintf(&text, "%s\t%s\t%s", check.Status, check.ID, check.Message)
		if check.Path != "" {
			fmt.Fprintf(&text, "\t%s", check.Path)
		}
		text.WriteByte('\n')
	}
	_, err := io.WriteString(output, text.String())
	return err
}
