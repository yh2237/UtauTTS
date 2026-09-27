package worldline

import (
	"math"
	"path/filepath"
	"utautts/internal/oto"
	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/render/base"
	"utautts/internal/voicebank"
)

func sourceLibraries(p *plan.Plan, cfg base.Config) []*voicebank.SourcePhoneLibrary {
	if !multilingualScore(p) {
		return nil
	}
	if cfg.ProviderOptions.Worldline.SourcePhoneMapping != nil && !*cfg.ProviderOptions.Worldline.SourcePhoneMapping {
		return nil
	}
	paths := []string{filepath.Join(p.Voicebank, "source-phone-library.json")}
	_, directories := plugin.DefaultDirectories()
	for _, dir := range directories {
		paths = append(paths, filepath.Join(dir, "source-phones", p.Language+".json"))
	}
	var result []*voicebank.SourcePhoneLibrary
	for _, path := range paths {
		library, err := voicebank.LoadSourcePhoneLibrary(path)
		if err == nil && library.Language == p.Language {
			result = append(result, library)
		}
	}
	return result
}

func librarySpeechSpan(p *plan.Plan, index int, libraries []*voicebank.SourcePhoneLibrary) (base.SourceSpan, bool) {
	u := p.Units[index]
	if (u.Role != "ending" && !(p.Language == "zh" && u.Role == "mora")) || len(u.CodaPhones) == 0 || len(libraries) == 0 {
		return base.SourceSpan{}, false
	}
	bank := &voicebank.Bank{}
	a, err := bank.AnalyzeSpeechSource(oto.Entry{Filename: u.Source, Offset: u.OffsetMS, Blank: u.CutoffMS, Fixed: u.ConsonantMS, Preutterance: u.PreutteranceMS, Overlap: u.OverlapMS})
	if err != nil {
		return base.SourceSpan{}, false
	}
	for _, library := range libraries {
		for _, record := range library.Entries {
			if record.SourceSHA256 != a.SourceSHA256 || math.Abs(record.DurationMS-a.DurationMS) > .1 {
				continue
			}
			return sourceRecordSpan(p, index, record, a)
		}
	}
	return base.SourceSpan{}, false
}

func sourceRecordSpan(p *plan.Plan, index int, record voicebank.SourcePhoneRecord, a voicebank.SourceAnalysis) (base.SourceSpan, bool) {
	u := p.Units[index]
	var targets []plan.PhoneTiming
	for _, t := range p.PhoneTimings {
		if t.Position == u.Position && (t.Role == "coda" || (p.Language == "zh" && u.Role == "mora")) {
			targets = append(targets, t)
		}
	}
	find := func(symbols []string, wanted []string) int {
		match := -1
		for i := 0; i+len(wanted) <= len(symbols); i++ {
			same := true
			for j, s := range wanted {
				if symbols[i+j] != s {
					same = false
					break
				}
			}
			if same {
				if match >= 0 {
					return -1
				}
				match = i
			}
		}
		return match
	}
	var sourceSymbols, targetSymbols []string
	for _, t := range record.Phones {
		sourceSymbols = append(sourceSymbols, t.Symbol)
	}
	for _, t := range targets {
		targetSymbols = append(targetSymbols, t.Symbol)
	}
	wanted := u.CodaPhones
	if p.Language == "zh" && u.Role == "mora" {
		wanted = targetSymbols
	}
	if len(wanted) == 0 || len(record.Phones) == 0 {
		return base.SourceSpan{}, false
	}
	sourceStart, targetStart := find(sourceSymbols, wanted), find(targetSymbols, wanted)
	if sourceStart < 0 || targetStart < 0 {
		return base.SourceSpan{}, false
	}
	span := base.SourceSpan{Alias: u.Alias, SourceSHA256: record.SourceSHA256, CoreStartMS: record.Phones[sourceStart].StartMS, CoreEndMS: record.Phones[sourceStart+len(wanted)-1].EndMS}
	span.ContextStartMS = span.CoreStartMS
	if sourceStart > 0 {
		span.ContextStartMS = record.Phones[sourceStart-1].StartMS
	}
	for j, s := range wanted {
		src, target := record.Phones[sourceStart+j], targets[targetStart+j]
		span.Mappings = append(span.Mappings, base.SourcePhoneMapping{Symbol: s, SourceStartMS: src.StartMS, SourceEndMS: src.EndMS, RequestedStartMS: target.StartMS, RequestedEndMS: target.StartMS + target.DurationMS})
	}
	for _, l := range a.Landmarks {
		span.Landmarks = append(span.Landmarks, base.SourceLandmark{Kind: l.Kind, SourceMS: l.SourceMS, DurationMS: l.DurationMS, Score: l.HeuristicScore, RelativeDB: l.RelativeToPeakDB})
	}
	return span, true
}
