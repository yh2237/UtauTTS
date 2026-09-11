package voicebank

import (
	"math"
	"path/filepath"
	"strings"
	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

// ApplyRepeatedContextは反復収録の3つのoto定義から2音目の範囲を復元する。
func (b *Bank) ApplyRepeatedContext(morae []frontend.Mora, selections []Selection) []Selection {
	result := append([]Selection(nil), selections...)
	for i := range result {
		s := &result[i]
		if s.Transition == nil || s.Mora.Language != frontend.LanguageChinese || s.Position <= 0 || s.Position >= len(morae) {
			continue
		}
		prev := morae[s.Position-1]
		if prev.Pause {
			continue
		}
		blocked := false
		for _, p := range prev.Phones {
			if p.Role == "coda" {
				blocked = true
			}
		}
		if blocked {
			continue
		}
		vc := s.Transition.Entry
		limit, ok := b.repeatedContextLimit(vc, s.Mora, prev)
		if !ok {
			continue
		}
		pcm, err := audio.ReadWav(vc.Filename)
		if err != nil {
			continue
		}
		wave := acoustic.Mono(pcm)
		// 別に定義された末尾CVへ探索範囲を広げない。
		frames := min(len(wave), int(math.Floor(limit*float64(pcm.SampleRate)/1000)))
		if frames <= 0 {
			continue
		}
		bounded := vc
		if vc.Blank >= 0 {
			bounded.Blank = -(float64(len(wave))*1000/float64(pcm.SampleRate) - vc.Blank - vc.Offset)
		}
		onset, end, ok := recoverVowelAfterVC(wave[:frames], pcm.SampleRate, bounded)
		if !ok {
			s.SourceContextReason = "repeated-no-clear-boundary"
			continue
		}
		entry := vc
		entry.Alias = prev.Vowel + " " + s.Mora.Text
		entry.Preutterance = onset - entry.Offset
		entry.Fixed = entry.Preutterance + 30
		entry.Blank = -(end - entry.Offset)
		s.Entry = entry
		s.Alias = entry.Alias
		s.Kind = AliasVCV
		s.Composite = false
		s.Transition = nil
		s.TransitionScore = 0
		s.TransitionJoinScore = 0
		s.TransitionJoinProbability = 0
		s.SourceContext = "recovered-repeat"
		s.SourceContextReason = "three-oto-anchors"
	}
	return result
}

func (b *Bank) repeatedContextLimit(vc oto.Entry, current, previous frontend.Mora) (float64, bool) {
	parts := strings.Split(strings.TrimSuffix(filepath.Base(vc.Filename), filepath.Ext(vc.Filename)), "_")
	if len(parts) != 3 || parts[0] != current.Text || parts[1] != current.Text || parts[2] != current.Text {
		return 0, false
	}
	probe := vc
	probe.Filename = parts[0] + "_" + parts[1] + ".wav"
	if !b.matchesContextRecording(probe, current, previous) {
		return 0, false
	}
	first := false
	limit := math.Inf(1)
	for _, e := range b.Entries["- "+current.Text] {
		if e.Filename == vc.Filename && e.Offset+e.Preutterance < vc.Offset && e.Offset >= 0 {
			first = true
		}
	}
	for _, e := range b.Entries[current.Text] {
		if e.Filename == vc.Filename && e.Offset > vc.Offset+math.Max(vc.Preutterance, vc.Fixed)+80 {
			limit = math.Min(limit, e.Offset)
		}
	}
	return limit, first && !math.IsInf(limit, 1)
}
