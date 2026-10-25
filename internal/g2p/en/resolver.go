package en

import "github.com/yh2237/UtauTTS/internal/g2p"

var englishReadingCache = g2p.NewReadingCache(8192)

// englishCandidateは1つの読み候補を返す。okがfalseなら次の候補へ進む。
type englishCandidate func(word string) (g2p.Reading, bool, error)

// englishCandidateListは優先順位の高い順。先に読めたものを採用する。
func englishCandidateList() []englishCandidate {
	return []englishCandidate{
		englishAcronymCandidate,
		englishEntityCandidate,
		englishLexiconCandidate,
		englishInflectionCandidate,
		englishPrefixCandidate,
		englishCompoundCandidate,
		englishNeuralCandidate,
		englishRulesCandidate,
	}
}

// ContextLexiconは、文脈つきの読み（misakiの辞書など）を返す。
// 返す文字列はKokoroの音素記号として扱う。
type ContextLexicon func(word string) (string, bool)

// Resolveは英語の1語を読み、出所つきで返す。
func Resolve(word string) (g2p.Reading, error) {
	return englishResolve(word)
}

// ResolveInContextは、前後の語を使って同綴異音語の読みを選ぶ。
func ResolveInContext(word, previous, next string) (g2p.Reading, error) {
	return ResolveInContextWithLexicon(word, previous, next, nil)
}

// ResolveWithContextは、文脈つきの辞書を引く。前後の語は使わない。
func ResolveWithContext(word string, context ContextLexicon) (g2p.Reading, error) {
	return ResolveInContextWithLexicon(word, "", "", context)
}

// ResolveInContextWithLexiconは、頭字語・固有名詞・同綴異音語・文脈辞書・通常の解決器の順に読む。
func ResolveInContextWithLexicon(word, previous, next string, context ContextLexicon) (g2p.Reading, error) {
	// 頭字語と登録済みの固有名詞は、文脈辞書より先に読む（AI、PostgreSQLなど）。
	if reading, ok, err := englishAcronymCandidate(word); err != nil {
		return g2p.Reading{}, err
	} else if ok {
		return reading, nil
	}
	if reading, ok, err := englishEntityCandidate(word); err != nil {
		return g2p.Reading{}, err
	} else if ok {
		return reading, nil
	}
	if pronunciation, ok := heteronymReading(word, previous, next); ok {
		return g2p.Reading{Phones: pronunciation, Notation: g2p.NotationARPAbet, Source: g2p.SourceHeteronym, Confidence: 0.9}, nil
	}
	if context != nil {
		if symbols, ok := context(word); ok && symbols != "" {
			return g2p.Reading{
				Phones:     symbols,
				Notation:   g2p.NotationKokoro,
				Source:     g2p.SourceMisaki,
				Confidence: 0.99,
			}, nil
		}
	}
	return englishResolve(word)
}

// englishWordPronunciationは音素列だけを返す。頭字語や複合語の内部から使う。
func englishWordPronunciation(word string) (string, error) {
	reading, err := englishResolve(word)
	return reading.Phones, err
}

func englishResolve(word string) (g2p.Reading, error) {
	if reading, ok := englishReadingCache.Get(word); ok {
		return reading, nil
	}
	for _, candidate := range englishCandidateList() {
		reading, ok, err := candidate(word)
		if err != nil {
			return g2p.Reading{}, err
		}
		if !ok || reading.Phones == "" {
			continue
		}
		englishReadingCache.Put(word, reading)
		return reading, nil
	}
	return g2p.Reading{}, nil
}

func englishAcronymCandidate(word string) (g2p.Reading, bool, error) {
	pronunciation, ok := englishAcronymPronunciation(word)
	if !ok {
		return g2p.Reading{}, false, nil
	}
	return g2p.Reading{Phones: pronunciation, Source: g2p.SourceAcronym, Confidence: 0.95}, true, nil
}

func englishLexiconCandidate(word string) (g2p.Reading, bool, error) {
	entry, err := g2p.LookupEnglish(word)
	if err != nil {
		return g2p.Reading{}, false, err
	}
	if entry.Phones == "" {
		return g2p.Reading{}, false, nil
	}
	return g2p.Reading{Phones: entry.Phones, Source: entry.Source, Confidence: 0.99}, true, nil
}

func englishEntityCandidate(word string) (g2p.Reading, bool, error) {
	pronunciation, ok := g2p.ForeignReading(g2p.English, word)
	if !ok || pronunciation == "" {
		return g2p.Reading{}, false, nil
	}
	return g2p.Reading{Phones: pronunciation, Notation: g2p.NotationARPAbet, Source: g2p.SourceEntity, Confidence: 0.98}, true, nil
}

func englishInflectionCandidate(word string) (g2p.Reading, bool, error) {
	pronunciation, err := englishInflectedPronunciation(word)
	if err != nil || pronunciation == "" {
		return g2p.Reading{}, false, err
	}
	return g2p.Reading{Phones: pronunciation, Source: g2p.SourceMorphology, Confidence: 0.85}, true, nil
}

func englishPrefixCandidate(word string) (g2p.Reading, bool, error) {
	pronunciation, err := englishPrefixedPronunciation(word)
	if err != nil || pronunciation == "" {
		return g2p.Reading{}, false, err
	}
	return g2p.Reading{Phones: pronunciation, Source: g2p.SourceMorphology, Confidence: 0.8}, true, nil
}

func englishCompoundCandidate(word string) (g2p.Reading, bool, error) {
	pronunciation, err := englishCompoundPronunciation(word)
	if err != nil || pronunciation == "" {
		return g2p.Reading{}, false, err
	}
	return g2p.Reading{Phones: pronunciation, Source: g2p.SourceCompound, Confidence: 0.75}, true, nil
}

func englishNeuralCandidate(word string) (g2p.Reading, bool, error) {
	pronunciation, ok := englishNeuralFallback(word)
	if !ok {
		return g2p.Reading{}, false, nil
	}
	return g2p.Reading{Phones: pronunciation, Source: g2p.SourceNeural, Confidence: 0.6}, true, nil
}

func englishRulesCandidate(word string) (g2p.Reading, bool, error) {
	pronunciation, err := englishRulePronunciation(word)
	if err != nil {
		return g2p.Reading{}, false, err
	}
	if pronunciation == "" {
		return g2p.Reading{}, false, nil
	}
	return g2p.Reading{Phones: pronunciation, Source: g2p.SourceRules, Confidence: 0.35}, true, nil
}
