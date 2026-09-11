package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"utautts/internal/atomicfile"
	"utautts/internal/audio"
	"utautts/internal/synth"
)

type sourceAudit struct {
	Unit                                      int
	Alias, Source, Context, Reason            string
	SourceStartMS, SourceEndMS, VowelAnchorMS float64
	ExcerptStartMS, ExcerptEndMS              float64
	MixedStartMS, MixedEndMS                  float64
}

// 出力断片には前後の音との重なりも含める。
func writeSourceAudit(dir string, result *synth.Result) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := result.RenderedPlan()
	var rows []sourceAudit
	for i, u := range p.Units {
		if u.Source == "" || u.Silent {
			continue
		}
		pcm, err := audio.ReadWav(u.Source)
		if err != nil {
			return err
		}
		length := float64(len(pcm.Data)/pcm.Channels) * 1000 / float64(pcm.SampleRate)
		end := length - u.CutoffMS
		if u.CutoffMS < 0 {
			end = u.OffsetMS - u.CutoffMS
		}
		selected, start, end := auditClip(pcm, u.OffsetMS, end)
		excerpt, exStart, exEnd := auditClip(pcm, start-150, end+150)
		mixed, mixStart, mixEnd := auditClip(result.Audio, p.LeadingMarginMS+u.NoteStartMS-u.PreutteranceMS, p.LeadingMarginMS+u.NoteStartMS+u.DurationMS)
		for _, clip := range []struct {
			name string
			pcm  *audio.PCM
		}{{"selected", selected}, {"original-context", excerpt}, {"mixed-output", mixed}} {
			if err := audio.WriteWav(filepath.Join(dir, fmt.Sprintf("%03d-%s.wav", i, clip.name)), clip.pcm); err != nil {
				return err
			}
		}
		rows = append(rows, sourceAudit{i, u.Alias, u.Source, u.SourceContext, u.SourceContextReason, start, end, u.OffsetMS + u.PreutteranceMS, exStart, exEnd, mixStart, mixEnd})
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(dir, "sources.json"), data)
}

func auditClip(pcm *audio.PCM, start, end float64) (*audio.PCM, float64, float64) {
	frames := len(pcm.Data) / pcm.Channels
	first := int(math.Round(start * float64(pcm.SampleRate) / 1000))
	last := int(math.Round(end * float64(pcm.SampleRate) / 1000))
	first = max(0, min(frames, first))
	last = max(first, min(frames, last))
	return &audio.PCM{SampleRate: pcm.SampleRate, Channels: pcm.Channels, Data: append([]int16(nil), pcm.Data[first*pcm.Channels:last*pcm.Channels]...)}, float64(first) * 1000 / float64(pcm.SampleRate), float64(last) * 1000 / float64(pcm.SampleRate)
}
