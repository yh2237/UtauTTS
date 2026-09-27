package worldline

import (
	"fmt"
	"math"
	"utautts/internal/oto"
	"utautts/internal/plan"
	"utautts/internal/provider"
	"utautts/internal/render/base"
	"utautts/internal/voicebank"
)

// 音素区間を検証し、時間写像と過渡音保護を確定する。
func placeSourceSpan(p *plan.Plan, index int, item worldlineManifestUnit, span base.SourceSpan, duration, leading float64) (worldlineManifestUnit, error) {
	u := &p.Units[index]
	fail := func() (worldlineManifestUnit, error) {
		return item, fmt.Errorf("invalid experimental source span: unit %d %q", index, u.Alias)
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	wanted := u.CodaPhones
	chineseRhyme := p.Language == "zh" && u.Role == "mora" && len(u.CodaPhones) > 0
	if chineseRhyme {
		wanted = nil
		for _, t := range p.PhoneTimings {
			if t.Position == u.Position {
				wanted = append(wanted, t.Symbol)
			}
		}
	}
	if (u.Role != "ending" && !chineseRhyme) || span.Alias != u.Alias || len(span.Mappings) != len(wanted) || len(span.Mappings) == 0 {
		return fail()
	}
	if !finite(span.CoreStartMS) || !finite(span.CoreEndMS) || !finite(span.ContextStartMS) || span.ContextStartMS < 0 || span.ContextStartMS > span.CoreStartMS || span.CoreStartMS < 0 || span.CoreEndMS > duration+.1 || span.CoreEndMS <= span.CoreStartMS {
		return fail()
	}
	start := item.PositionMS - leading
	anchors := []provider.SpeechAnchor{}
	add := func(s, t float64) bool {
		if !finite(s) || !finite(t) || s < 0 || s > duration+.1 || t < -.01 || t > item.LengthMS+.01 {
			return false
		}
		if len(anchors) > 0 {
			a := anchors[len(anchors)-1]
			if s <= a.SourceMS+.01 || t <= a.TargetMS+.01 {
				return false
			}
		}
		anchors = append(anchors, provider.SpeechAnchor{SourceMS: s, TargetMS: math.Max(0, t)})
		return true
	}
	first := span.Mappings[0]
	if math.Abs(first.SourceStartMS-span.CoreStartMS) > .1 || math.Abs(first.RequestedStartMS-u.NoteStartMS) > .1 {
		return fail()
	}
	if first.RequestedStartMS-start > 0 {
		if !add(math.Max(span.ContextStartMS, span.CoreStartMS-40), 0) {
			return fail()
		}
	}
	if !add(first.SourceStartMS, first.RequestedStartMS-start) {
		return fail()
	}
	for j, m := range span.Mappings {
		if !finite(m.SourceStartMS) || !finite(m.SourceEndMS) || !finite(m.RequestedStartMS) || !finite(m.RequestedEndMS) {
			return fail()
		}
		if m.Symbol != wanted[j] || m.SourceEndMS <= m.SourceStartMS || m.RequestedEndMS <= m.RequestedStartMS {
			return fail()
		}
		if j > 0 {
			prev := span.Mappings[j-1]
			if math.Abs(prev.SourceEndMS-m.SourceStartMS) > .1 || math.Abs(prev.RequestedEndMS-m.RequestedStartMS) > .1 {
				return fail()
			}
		}
		if !add(m.SourceEndMS, m.RequestedEndMS-start) {
			return fail()
		}
	}
	last := anchors[len(anchors)-1]
	if math.Abs(last.SourceMS-span.CoreEndMS) > .1 || math.Abs(last.TargetMS-item.LengthMS) > .1 {
		return fail()
	}
	bank := &voicebank.Bank{}
	entry := oto.Entry{Alias: u.Alias, Filename: u.Source, Offset: u.OffsetMS, Blank: u.CutoffMS, Fixed: u.ConsonantMS, Preutterance: u.PreutteranceMS, Overlap: u.OverlapMS}
	analysis, err := bank.AnalyzeSpeechSource(entry)
	if err != nil {
		return item, err
	}
	if span.SourceSHA256 == "" || analysis.SourceSHA256 != span.SourceSHA256 {
		return fail()
	}
	item.Speech = &provider.WorldSpeechTiming{UnitIndex: index, Anchors: anchors}
	u.CodaReleaseSeparated = false
	u.CodaClosureMS = 0
	u.CodaReleaseMS = 0
	u.SpeechMapping = "experimental-aligned-source-span-v1"
	// エネルギー上昇は探索の手掛かりとし、過渡音の長さを再計測する。
	bestScore := 0.0
	var transient, transientDuration float64
	for _, hint := range span.Landmarks {
		if chineseRhyme {
			break
		}
		if !finite(hint.SourceMS) || !finite(hint.DurationMS) || !finite(hint.Score) || !finite(hint.RelativeDB) || hint.DurationMS <= 0 {
			continue
		}
		if hint.Score < .4 || hint.RelativeDB < -25 || hint.SourceMS <= span.CoreStartMS+4 || hint.SourceMS >= span.CoreEndMS-24 {
			continue
		}
		pos, dur, score := hint.SourceMS, hint.DurationMS, hint.Score
		if hint.Kind == "energy-rise" {
			e := entry
			e.Preutterance = hint.SourceMS + 20
			e.Fixed = 0
			profile := bank.CalibrateSpeech(e)
			pos, dur, score = profile.TransientMS, profile.TransientDurationMS, profile.TransientConfidence
		}
		if score >= .65 && score > bestScore && pos-4 > span.CoreStartMS && pos+math.Max(10, math.Min(24, dur+6)) < span.CoreEndMS {
			bestScore = score
			transient = pos
			transientDuration = dur
		}
	}
	if bestScore > 0 {
		updated, protected := protectSpeechTransient(anchors, transient, transientDuration)
		if protected {
			anchors = updated
			item.Speech.Anchors = anchors
			item.Speech.ProtectStop = true
			item.Speech.SourceTransientMS = transient
			item.Speech.SourceTransientDurationMS = transientDuration
			u.CodaReleaseSeparated = true
			u.CodaReleaseMS = math.Max(10, math.Min(24, transientDuration+6)) + 4
			u.CodaClosureMS = math.Max(0, u.DurationMS-u.CodaReleaseMS)
		}
	}
	u.SpeechSourceAnchorsMS = nil
	u.SpeechTargetAnchorsMS = nil
	for _, a := range anchors {
		u.SpeechSourceAnchorsMS = append(u.SpeechSourceAnchorsMS, a.SourceMS)
		u.SpeechTargetAnchorsMS = append(u.SpeechTargetAnchorsMS, a.TargetMS)
	}
	return item, nil
}

// oto推定、手動指定、ライブラリの順に処理し、手動指定を優先する。
func mapSpeechSource(p *plan.Plan, index int, item worldlineManifestUnit, options base.WorldlineProviderOptions, libraries []*voicebank.SourcePhoneLibrary, duration, leading float64) (worldlineManifestUnit, error) {
	mapped, err := placeSpeechUnit(p, index, item, duration, leading)
	if err != nil {
		return item, err
	}
	if span, ok := options.ExperimentalSourceSpans[index]; ok {
		return placeSourceSpan(p, index, mapped, span, duration, leading)
	}
	if span, ok := librarySpeechSpan(p, index, libraries); ok {
		if candidate, err := placeSourceSpan(p, index, mapped, span, duration, leading); err == nil {
			p.Units[index].SpeechMapping = "source-phone-library-v1"
			return candidate, nil
		}
	}
	return mapped, nil
}
