package worldline

import (
	"fmt"
	"math"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/provider"
	"utautts/internal/render/base"
	"utautts/internal/speechwindow"
)

func multilingualScore(p *plan.Plan) bool {
	return p.PhoneTimingSource == "multilingual-speech-score-v1" && base.CorrectionAppliesTo("source_phone_library", p.Language)
}

// 原音のoto時刻を出力へ写す。鼻音境界は規則による推定値。
func placeSpeechUnit(p *plan.Plan, index int, item worldlineManifestUnit, sourceDuration, leading float64) (worldlineManifestUnit, error) {
	u := &p.Units[index]
	if sourceDuration < 2 || u.DurationMS <= 0 {
		return item, fmt.Errorf("invalid speech source or target for %q", u.Alias)
	}
	transient, transientDuration, hasTransient := speechCodaTransient(p, *u, sourceDuration)
	if hasTransient && u.SpeechProfile.ActivityConfidence >= .65 {
		// 破裂音の後ろに余白を残し、長い末尾無音を除く。
		activeEnd := math.Max(u.SpeechProfile.ActivityEndMS+8, transient+speechwindow.TransientTailMS(transientDuration, 10)+8)
		if activeEnd > u.PreutteranceMS+4 {
			sourceDuration = math.Min(sourceDuration, activeEnd)
		}
	}
	start, end := u.NoteStartMS, u.NoteStartMS+u.DurationMS
	if u.Role == "transition" {
		start -= math.Min(20, u.DurationMS*.5)
		for _, main := range p.Units {
			if main.Position == u.Position && main.Role == "mora" {
				end = u.NoteStartMS + main.TargetOnsetMS
				break
			}
		}
		if end <= u.NoteStartMS {
			end = u.NoteStartMS + math.Min(20, u.DurationMS)
		}
	} else if u.Role == "ending" {
		start -= math.Min(20, u.DurationMS*.5)
	} else {
		for _, next := range p.Units {
			if next.Position == u.Position && next.Role == "ending" && len(next.CodaPhones) > 0 {
				end = math.Min(end, next.NoteStartMS+3)
			}
		}
	}
	start = math.Max(-leading, start)
	length := end - start
	if length <= 2 {
		return item, fmt.Errorf("empty speech coverage for %q", u.Alias)
	}
	anchors := []provider.SpeechAnchor{}
	rhymeMapping := false
	add := func(source, target float64) {
		source = math.Max(0, math.Min(sourceDuration, source))
		target = math.Max(0, math.Min(length, target))
		if len(anchors) > 0 {
			last := anchors[len(anchors)-1]
			if source <= last.SourceMS+.01 || target <= last.TargetMS+.01 {
				return
			}
		}
		anchors = append(anchors, provider.SpeechAnchor{SourceMS: source, TargetMS: target})
	}
	pre := math.Max(0, math.Min(sourceDuration-1, u.PreutteranceMS))
	if u.Role == "transition" || u.Role == "ending" {
		add(math.Max(0, pre-math.Min(40, pre)), 0)
		add(pre, u.NoteStartMS-start)
	} else {
		onset := u.TargetOnsetMS
		if onset > 0 {
			add(0, 0)
			add(pre, onset)
		} else {
			isVowel := false
			for _, phone := range p.Morae[u.Position].Phones {
				if phone.Role == "nucleus" {
					isVowel = true
				}
			}
			if isVowel {
				add(pre, 0)
			} else {
				add(0, 0)
			}
		}
		var rhymeAnchors []provider.SpeechAnchor
		if p.Language == frontend.LanguageChinese {
			rhymeAnchors, rhymeMapping = mandarinRhymeAnchors(p, *u, sourceDuration, length)
		}
		if len(rhymeAnchors) > 0 {
			for _, anchor := range rhymeAnchors {
				add(anchor.SourceMS, anchor.TargetMS)
			}
		} else if onset > 0 && u.SourceFixedMS != nil {
			add(*u.SourceFixedMS, math.Min(length-10, onset+math.Min(20, length*.1)))
		}
	}
	for len(anchors) > 0 && (anchors[len(anchors)-1].SourceMS >= sourceDuration-.01 || anchors[len(anchors)-1].TargetMS >= length-.01) {
		anchors = anchors[:len(anchors)-1]
	}
	if len(anchors) == 0 {
		add(0, 0)
	}
	add(sourceDuration, length)
	if len(anchors) < 2 {
		return item, fmt.Errorf("invalid speech anchors for %q", u.Alias)
	}
	protected := false
	if hasTransient {
		anchors, protected = protectSpeechTransient(anchors, transient, transientDuration)
	}
	fade := math.Min(5, length*.2)
	item.PositionMS = start + leading
	item.SkipMS = 0
	item.LengthMS = length
	item.RequiredLengthMS = length
	item.FadeInMS, item.FadeOutMS = fade, fade
	item.Envelope = []base.WorldlineEnvelopePoint{{XMS: 0, Y: 0}, {XMS: fade, Y: 1}, {XMS: length * .5, Y: 1}, {XMS: length - fade, Y: 1}, {XMS: length, Y: 0}}
	item.Speech = &provider.WorldSpeechTiming{UnitIndex: index, Anchors: anchors}
	if protected {
		item.Speech.ProtectStop = true
		item.Speech.SourceTransientMS = transient
		item.Speech.SourceTransientDurationMS = transientDuration
		u.StopBurstReason = "transient-detected"
	}
	item.LegacyMix = false
	u.SpeechMapping = "oto-landmark-prior-v1"
	if rhymeMapping {
		u.SpeechMapping = "mandarin-rhyme-prior-v1"
	}
	u.CodaReleaseSeparated = false
	u.CodaClosureMS, u.CodaReleaseMS = 0, 0
	if protected {
		u.SpeechMapping = "oto-landmark-transient-v2"
		u.CodaReleaseSeparated = true
		u.CodaReleaseMS = speechwindow.TransientTailMS(transientDuration, 10) + speechwindow.TransientLeadMS
		u.CodaClosureMS = math.Max(0, u.DurationMS-u.CodaReleaseMS)
	}
	u.SpeechSourceAnchorsMS = nil
	u.SpeechTargetAnchorsMS = nil
	for _, anchor := range anchors {
		u.SpeechSourceAnchorsMS = append(u.SpeechSourceAnchorsMS, anchor.SourceMS)
		u.SpeechTargetAnchorsMS = append(u.SpeechTargetAnchorsMS, anchor.TargetMS)
	}
	if u.SourceFixedMS != nil {
		item.ConsonantMS = *u.SourceFixedMS
	}
	return item, nil
}
