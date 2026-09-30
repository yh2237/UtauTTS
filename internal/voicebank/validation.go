package voicebank

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/oto"
	"utautts/internal/sourceaudio"
)

// EntryValidationは原音候補へ追加する前の検査結果。
type EntryValidation struct {
	Status string
	Checks []string
	Reason string
}

// メモリ上のテスト音源は出典を確認できないためunknownのまま扱う。
func (b *Bank) validateEntry(entry oto.Entry) EntryValidation {
	if b.Root == "" {
		return EntryValidation{Status: "unknown"}
	}
	if entry.OtoPath == "" {
		if _, err := os.Stat(entry.Filename); err != nil {
			return EntryValidation{Status: "unknown"}
		}
	}
	info, statErr := os.Stat(entry.Filename)
	var size, modTime int64
	if statErr == nil {
		size, modTime = info.Size(), info.ModTime().UnixNano()
	}

	b.validationMu.Lock()
	if b.validationCache == nil {
		b.validationCache = make(map[oto.Entry]cachedEntryValidation)
	}
	if cached, ok := b.validationCache[entry]; ok && cached.Size == size && cached.ModTime == modTime {
		b.validationMu.Unlock()
		return cloneEntryValidation(cached.Result)
	}
	b.validationMu.Unlock()

	result := inspectEntryWAV(entry)
	b.validationMu.Lock()
	if cached, ok := b.validationCache[entry]; ok && cached.Size == size && cached.ModTime == modTime {
		result = cached.Result
	} else {
		b.validationCache[entry] = cachedEntryValidation{Result: result, Size: size, ModTime: modTime}
	}
	b.validationMu.Unlock()
	return cloneEntryValidation(result)
}

type cachedEntryValidation struct {
	Result  EntryValidation
	Size    int64
	ModTime int64
}

func inspectEntryWAV(entry oto.Entry) EntryValidation {
	if strings.TrimSpace(entry.Filename) == "" {
		return EntryValidation{Status: "unusable", Reason: "missing-source"}
	}
	for name, value := range map[string]float64{
		"offset": entry.Offset, "fixed": entry.Fixed, "blank": entry.Blank,
		"preutterance": entry.Preutterance, "overlap": entry.Overlap,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return EntryValidation{Status: "unusable", Reason: "oto-non-finite-" + name}
		}
	}
	pcm, mono, err := sourceaudio.ReadMono(filepath.Clean(entry.Filename))
	if err != nil {
		return EntryValidation{Status: "unusable", Reason: fmt.Sprintf("wav-read: %v", err)}
	}
	start, end, err := audio.TrimFrames(pcm, entry.Offset, entry.Blank)
	if err != nil {
		return EntryValidation{Status: "unusable", Reason: fmt.Sprintf("trim-range: %v", err)}
	}
	frames := end - start
	if frames < 32 {
		return EntryValidation{Status: "unusable", Reason: fmt.Sprintf("trim-too-short: %d frames", frames)}
	}
	mono = mono[start:end]
	rms := acoustic.RMS(mono)
	if rms < 1e-5 {
		return EntryValidation{Status: "unusable", Reason: "trim-silent"}
	}
	checks := []string{"wav-readable", "trim-valid", "trim-non-silent"}
	status := "usable"
	mean, peak, clipped := 0.0, 0.0, 0
	for _, sample := range mono {
		mean += sample
		peak = math.Max(peak, math.Abs(sample))
		if math.Abs(sample) >= 0.999 {
			clipped++
		}
	}
	mean /= float64(len(mono))
	if clipped > len(mono)/100 || math.Abs(mean) > math.Max(0.08, rms*0.35) {
		status = "degraded"
		if clipped > len(mono)/100 {
			checks = append(checks, "clipping-detected")
		}
		if math.Abs(mean) > math.Max(0.08, rms*0.35) {
			checks = append(checks, "dc-offset-detected")
		}
	}
	return EntryValidation{
		Status: status,
		Checks: checks,
	}
}

func cloneEntryValidation(result EntryValidation) EntryValidation {
	result.Checks = append([]string(nil), result.Checks...)
	return result
}
