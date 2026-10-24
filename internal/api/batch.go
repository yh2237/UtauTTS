package api

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/sidecar"
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
		writeAPIError(w, jsonDecodeStatus(err), err)
		return
	}
	if len(request.Items) == 0 {
		writeAPIError(w, http.StatusBadRequest, errors.New("items are required"))
		return
	}
	if len(request.Items) > maxBatchItems {
		writeAPIError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("batch supports at most %d items", maxBatchItems))
		return
	}
	if request.WriteText {
		if _, err := sidecar.TextBytes("", request.TextEncoding); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
	}
	names := make([]string, len(request.Items))
	seenNames := make(map[string]struct{}, len(request.Items))
	for index, item := range request.Items {
		name := batchFileName(item.Name)
		if name == "" {
			name = fmt.Sprintf("utterance-%d.wav", index+1)
		}
		if !strings.EqualFold(filepath.Ext(name), ".wav") {
			name += ".wav"
		}
		if _, exists := seenNames[name]; exists {
			writeAPIError(w, http.StatusBadRequest, fmt.Errorf("duplicate batch filename %q", name))
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
			writeAPIError(w, http.StatusRequestTimeout, r.Context().Err())
			return
		}
	}
	output, err := os.CreateTemp("", "utautts-batch-*.zip")
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
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
			writeAPIError(w, status, fmt.Errorf("item %d: %w", index+1, err))
			return
		}
		name := names[index]
		wav := audio.PCMToWavBytes(result.Audio)
		totalWAVBytes += len(wav)
		if totalWAVBytes > maxBatchWAVBytes {
			_ = archive.Close()
			writeAPIError(w, http.StatusRequestEntityTooLarge, errors.New("batch audio exceeds 256 MiB"))
			return
		}
		entry, err := archive.Create(name)
		if err != nil {
			_ = archive.Close()
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		if _, err := entry.Write(wav); err != nil {
			_ = archive.Close()
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if request.WriteText {
			text := item.Request.Text
			if text == "" {
				text = item.Request.ReadingOrKana()
			}
			data, err := sidecar.TextBytes(text, request.TextEncoding)
			if err != nil {
				_ = archive.Close()
				writeAPIError(w, http.StatusBadRequest, err)
				return
			}
			entry, err := archive.Create(base + ".txt")
			if err != nil {
				_ = archive.Close()
				writeAPIError(w, http.StatusInternalServerError, err)
				return
			}
			if _, err := entry.Write(data); err != nil {
				_ = archive.Close()
				writeAPIError(w, http.StatusInternalServerError, err)
				return
			}
		}
		if request.WriteLab {
			data, err := sidecar.LabBytes(result.Lab)
			if err != nil {
				_ = archive.Close()
				writeAPIError(w, http.StatusUnprocessableEntity, err)
				return
			}
			entry, err := archive.Create(base + ".lab")
			if err != nil {
				_ = archive.Close()
				writeAPIError(w, http.StatusInternalServerError, err)
				return
			}
			if _, err := entry.Write(data); err != nil {
				_ = archive.Close()
				writeAPIError(w, http.StatusInternalServerError, err)
				return
			}
		}
	}
	if err := archive.Close(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if err := r.Context().Err(); err != nil {
		writeAPIError(w, http.StatusRequestTimeout, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="utautts-audio.zip"`)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, output); err != nil {
		log.Printf("write batch response: %v", err)
	}
}

// batchFileNameは、ZIPに入れる名前をフォルダを含まない1つのファイル名にする。
// どのOSで展開しても区切りとみなされる「/」「\」と、Windowsのドライブ指定の「:」を受け付けない。
func batchFileName(name string) string {
	name = name[strings.LastIndexAny(name, "/\\")+1:]
	if name == "." || name == ".." || strings.ContainsAny(name, ":\x00") {
		return ""
	}
	return name
}
