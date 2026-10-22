package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

var catalogFlags = map[string]bool{
	"list": true, "json": true, "version": true, "voicebank": true,
	"renderer-dir": true, "model-dir": true,
	"worldline-bridge": true, "openjtalk-features": true, "openjtalk-dictionary": true,
}

type cliCatalogReport struct {
	Kind      string           `json:"kind"`
	DefaultID string           `json:"default_id,omitempty"`
	Items     []cliCatalogItem `json:"items"`
	Problems  []string         `json:"problems"`
}

type cliCatalogItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Path          string   `json:"path,omitempty"`
	Language      string   `json:"language,omitempty"`
	VoicebankKind string   `json:"voicebank_kind,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	Available     *bool    `json:"available,omitempty"`
	Problems      []string `json:"problems,omitempty"`
}

func loadCatalog(kind, voiceDirectory string, config synth.RuntimeConfig) (report *cliCatalogReport, runErr error) {
	report = &cliCatalogReport{Kind: kind, Items: make([]cliCatalogItem, 0), Problems: make([]string, 0)}
	switch kind {
	case "voicebanks":
		library := voicebank.NewLibrary(voiceDirectory)
		if err := library.Reload(); err != nil {
			return nil, cliFailure("input_error", err)
		}
		for _, item := range library.List() {
			report.Items = append(report.Items, cliCatalogItem{
				ID: item.ID, Name: item.Summary.Name, Path: item.Summary.Path, VoicebankKind: item.Summary.Kind,
			})
		}
		return report, nil
	case "models", "renderers":
	default:
		return nil, cliFailure("invalid_arguments", fmt.Errorf("unknown --list kind %q", kind))
	}
	runtime, err := synth.NewRuntime(config, nil)
	if err != nil {
		return nil, cliFailure("initialization_error", err)
	}
	defer func() { runErr = errors.Join(runErr, cliFailure("shutdown_error", runtime.Close())) }()
	report.Problems = append(report.Problems, runtime.Catalog.Problems...)
	switch kind {
	case "models":
		report.DefaultID = "none"
		for _, model := range runtime.Catalog.Models {
			report.Items = append(report.Items, cliCatalogItem{ID: model.ID, Name: model.DisplayName, Path: model.Path, Language: model.Language})
		}
	case "renderers":
		report.DefaultID = runtime.Catalog.DefaultRenderer()
		availability := runtime.Service.RendererAvailability()
		for _, renderer := range runtime.Catalog.Renderers {
			status := availability[renderer.ID]
			item := cliCatalogItem{ID: renderer.ID, Name: renderer.DisplayName, Path: renderer.Directory, Provider: renderer.Provider, Available: &status.Available}
			for _, issue := range status.Issues {
				item.Problems = append(item.Problems, issue.Message)
			}
			report.Items = append(report.Items, item)
		}
	}
	return report, nil
}

func writeCatalog(output io.Writer, catalog *cliCatalogReport) error {
	var text strings.Builder
	if catalog.DefaultID != "" {
		fmt.Fprintf(&text, "default: %s\n", catalog.DefaultID)
	}
	if len(catalog.Items) == 0 {
		fmt.Fprintf(&text, "no %s found\n", catalog.Kind)
	}
	for _, item := range catalog.Items {
		fmt.Fprintf(&text, "%s\t%s", item.ID, item.Name)
		if item.Available != nil {
			status := "unavailable"
			if *item.Available {
				status = "available"
			}
			fmt.Fprintf(&text, "\t%s", status)
		}
		if item.Path != "" {
			fmt.Fprintf(&text, "\t%s", item.Path)
		}
		text.WriteByte('\n')
		for _, problem := range item.Problems {
			fmt.Fprintf(&text, "  warning: %s\n", problem)
		}
	}
	for _, problem := range catalog.Problems {
		fmt.Fprintf(&text, "warning: %s\n", problem)
	}
	_, err := io.WriteString(output, text.String())
	return err
}
