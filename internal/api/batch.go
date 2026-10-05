package api

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/audio"
	"utautts/internal/sidecar"
)

func (s *Server) handleSynthesizeBatch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		WriteText    bool   `json:"write_text"`
		WriteLab     bool   `json:"write_lab"`
		TextEncoding string `json:"text_encoding"`
		Items        []struct {
			Name    string           `json:"name"`
			Request SynthesisRequest `json:"request"`
		} `json:"items"`
	}
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeJSON(w, jsonDecodeStatus(err), map[string]string{"error": err.Error()})
		return
	}
	if len(request.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items are required"})
		return
	}
	if len(request.Items) > maxBatchItems {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": fmt.Sprintf("batch supports at most %d items", maxBatchItems)})
		return
	}
	if request.WriteText {
		if _, err := sidecar.TextBytes("", request.TextEncoding); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	names := make([]string, len(request.Items))
	seenNames := make(map[string]struct{}, len(request.Items))
	for index, item := range request.Items {
		name := filepath.Base(item.Name)
		if name == "." || name == "" || name == ".." {
			name = fmt.Sprintf("utterance-%d.wav", index+1)
		}
		if !strings.EqualFold(filepath.Ext(name), ".wav") {
			name += ".wav"
		}
		if _, exists := seenNames[name]; exists {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("duplicate batch filename %q", name)})
			return
		}
		seenNames[name] = struct{}{}
		names[index] = name
	}
	if s.batchSem != nil {
		select {
		case s.batchSem <- struct{}{}:
			defer func() { <-s.batchSem }()
		case <-r.Context().Done():
			writeJSON(w, http.StatusRequestTimeout, map[string]string{"error": r.Context().Err().Error()})
			return
		}
	}
	output, err := os.CreateTemp("", "utautts-batch-*.zip")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	outputPath := output.Name()
	defer func() {
		_ = output.Close()
		_ = os.Remove(outputPath)
	}()
	archive := zip.NewWriter(output)
	totalWAVBytes := 0
	for index, item := range request.Items {
		result, status, err := s.synthesize(r.Context(), item.Request)
		if err != nil {
			_ = archive.Close()
			writeJSON(w, status, map[string]string{"error": fmt.Sprintf("item %d: %v", index+1, err)})
			return
		}
		name := names[index]
		wav := audio.PCMToWavBytes(result.Audio)
		totalWAVBytes += len(wav)
		if totalWAVBytes > maxBatchWAVBytes {
			_ = archive.Close()
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "batch audio exceeds 256 MiB"})
			return
		}
		entry, err := archive.Create(name)
		if err != nil {
			_ = archive.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if _, err := entry.Write(wav); err != nil {
			_ = archive.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if request.WriteText {
			text := item.Request.Text
			if text == "" {
				text = synthesisReading(item.Request)
			}
			data, err := sidecar.TextBytes(text, request.TextEncoding)
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			entry, err := archive.Create(base + ".txt")
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if _, err := entry.Write(data); err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		if request.WriteLab {
			data, err := sidecar.LabBytes(result.Lab)
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
				return
			}
			entry, err := archive.Create(base + ".lab")
			if err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if _, err := entry.Write(data); err != nil {
				_ = archive.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
	}
	if err := archive.Close(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="utautts-audio.zip"`)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, output); err != nil {
		log.Printf("write batch response: %v", err)
	}
}
