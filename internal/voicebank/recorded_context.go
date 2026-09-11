package voicebank

import (
	"math"
	"path/filepath"
	"strings"
	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/oto"
	"utautts/internal/pitch"
)

// ApplyRecordedContextは音源を変更せず収録済みの前後関係から原音範囲を復元する。
func (b *Bank) ApplyRecordedContext(morae []frontend.Mora, selections []Selection, recover bool) []Selection {
	result := append([]Selection(nil), selections...)
	for i := range result {
		s := &result[i]
		pos := s.Position
		if pos <= 0 || pos >= len(morae) || morae[pos-1].Pause {
			continue
		}
		blocked := false
		for _, p := range morae[pos-1].Phones {
			if p.Role == "coda" {
				blocked = true
			}
		}
		if blocked {
			s.SourceContextReason = "previous-coda"
			continue
		}
		if s.Kind == AliasVCV {
			s.SourceContext = "existing"
			continue
		}
		if !recover {
			s.SourceContextReason = "no-existing-context"
			continue
		}
		if s.Transition == nil || s.Mora.Language != frontend.LanguageChinese {
			s.SourceContextReason = "no-supported-vc"
			continue
		}
		vc := s.Transition.Entry
		if !b.matchesContextRecording(vc, s.Mora, morae[pos-1]) {
			s.SourceContextReason = "recording-context-mismatch"
			continue
		}
		pcm, err := audio.ReadWav(vc.Filename)
		if err != nil {
			s.SourceContextReason = "source-unavailable"
			continue
		}
		onset, end, ok := recoverVowelAfterVC(acoustic.Mono(pcm), pcm.SampleRate, vc)
		if !ok {
			s.SourceContextReason = "no-clear-vowel-boundary"
			continue
		}
		entry := vc
		entry.Alias = morae[pos-1].Vowel + " " + s.Mora.Text
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
		s.SourceContext = "recovered-vc"
		s.SourceContextReason = "matched-two-syllable-recording"
	}
	return result
}

func (b *Bank) matchesContextRecording(vc oto.Entry, current, previous frontend.Mora) bool {
	// 収録内容を特定できる2音節名と無声子音だけを復元する。
	parts := strings.Split(strings.TrimSuffix(filepath.Base(vc.Filename), filepath.Ext(vc.Filename)), "_")
	if len(parts) != 2 || parts[1] != current.Text {
		return false
	}
	match := parts[0] == previous.Text || parts[0] == previous.Vowel
	if b.Presamp != nil {
		match = match || b.Presamp.Vowels[parts[0]] == previous.Vowel && previous.Vowel != ""
	}
	if !match {
		return false
	}
	for _, p := range current.Phones {
		if p.Role == "onset" {
			return strings.Contains(" b p d t g k j q x zh ch sh z c s f h ", " "+p.Symbol+" ")
		}
	}
	return false
}

// 無声区間の後に有声区間が続く場合だけCVを復元する。
func recoverVowelAfterVC(wave []float64, rate int, vc oto.Entry) (float64, float64, bool) {
	if rate <= 0 {
		return 0, 0, false
	}
	length := float64(len(wave)) * 1000 / float64(rate)
	oldEnd := length - vc.Blank
	if vc.Blank < 0 {
		oldEnd = vc.Offset - vc.Blank
	}
	start := math.Max(vc.Offset+vc.Preutterance, vc.Offset+vc.Fixed-30)
	window := int(math.Round(.03 * float64(rate)))
	noise := false
	runStart := -1.0
	lastVoiced := 0.0
	reference := 0.0
	for ms := start; ms <= math.Min(length-30, start+400); ms += 5 {
		index := int(math.Round(ms * float64(rate) / 1000))
		if index < 0 || index+window > len(wave) {
			break
		}
		f0 := pitch.Estimate(wave[index:index+window], rate)
		voiced := f0 >= 80 && (reference == 0 || math.Abs(math.Log2(f0/reference)) < .4)
		if !voiced {
			if runStart >= 0 {
				break
			}
			noise = true
			continue
		}
		if !noise {
			continue
		}
		if runStart < 0 {
			runStart = ms
			reference = f0
		}
		lastVoiced = ms + 30
		if lastVoiced-runStart >= 250 {
			break
		}
	}
	if runStart < 0 || lastVoiced-runStart < 100 || runStart > oldEnd+150 || lastVoiced < oldEnd+40 {
		return 0, 0, false
	}
	onset := runStart + 15
	end := math.Min(lastVoiced, length)
	if onset-vc.Offset < 30 || end-onset < 80 {
		return 0, 0, false
	}
	return onset, end, true
}
