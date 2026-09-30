package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"

	"utautts/internal/render/worldline"
	"utautts/internal/tts"
)

type phaseMeasurement struct {
	ID         string   `json:"id"`
	Renderer   string   `json:"renderer"`
	Repetition int      `json:"repetition"`
	Source     string   `json:"source"`
	Message    string   `json:"message"`
	Phase      string   `json:"phase,omitempty"`
	ElapsedMS  *float64 `json:"elapsed_ms,omitempty"`
}

type evaluationProfile struct {
	mu               sync.Mutex
	current          phaseMeasurement
	encoder          *json.Encoder
	phases, cpu      *os.File
	dir              string
	writeErr         error
	oldTTS, oldWorld func(string)
	before           runtime.MemStats
}

// CPU・割り当ては親Goプロセスだけ。WORLD子プロセスはworld-profile.jsonlで別に観測する。
func startEvaluationProfile(dir string) (*evaluationProfile, error) {
	phases, err := os.Create(filepath.Join(dir, "phase-profile.jsonl"))
	if err != nil {
		return nil, err
	}
	cpu, err := os.Create(filepath.Join(dir, "cpu.pprof"))
	if err != nil {
		_ = phases.Close()
		return nil, err
	}
	if err := pprof.StartCPUProfile(cpu); err != nil {
		_ = cpu.Close()
		_ = phases.Close()
		return nil, err
	}
	p := &evaluationProfile{phases: phases, cpu: cpu, encoder: json.NewEncoder(phases), dir: dir,
		oldTTS: tts.Trace, oldWorld: worldline.Trace}
	runtime.ReadMemStats(&p.before)
	tts.Trace = p.trace("tts", p.oldTTS)
	worldline.Trace = p.trace("worldline", p.oldWorld)
	return p, nil
}

func (p *evaluationProfile) SetCase(id, renderer string, repetition int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = phaseMeasurement{ID: id, Renderer: renderer, Repetition: repetition}
}

func (p *evaluationProfile) trace(source string, previous func(string)) func(string) {
	return func(message string) {
		p.mu.Lock()
		row := p.current
		row.Source, row.Message = source, message
		if phase, value, ok := strings.Cut(message, ":"); ok {
			if ms, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "ms"), 64); err == nil {
				row.Phase, row.ElapsedMS = phase, &ms
			}
		}
		if p.writeErr == nil {
			p.writeErr = p.encoder.Encode(row)
		}
		p.mu.Unlock()
		if previous != nil {
			previous(message)
		}
	}
}

func (p *evaluationProfile) Close() error {
	tts.Trace, worldline.Trace = p.oldTTS, p.oldWorld
	pprof.StopCPUProfile()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocs, err := os.Create(filepath.Join(p.dir, "allocs.pprof"))
	if err == nil {
		err = errors.Join(pprof.Lookup("allocs").WriteTo(allocs, 0), allocs.Close())
	}
	memory, memoryErr := os.Create(filepath.Join(p.dir, "memory.json"))
	if memoryErr == nil {
		memoryErr = errors.Join(json.NewEncoder(memory).Encode(struct {
			Scope  string           `json:"scope"`
			Before runtime.MemStats `json:"before"`
			After  runtime.MemStats `json:"after"`
		}{"host Go process; snapshots, not peak RSS", p.before, after}), memory.Close())
	}
	return errors.Join(p.writeErr, p.phases.Close(), p.cpu.Close(), err, memoryErr)
}
