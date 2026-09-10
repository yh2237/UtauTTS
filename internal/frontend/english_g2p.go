package frontend

func englishWordPronunciation(word string) (string, error) {
	pronunciation, err := lookupEnglishDictionary(word)
	if err != nil {
		return "", err
	}
	if pronunciation != "" {
		return pronunciation, nil
	}
	if pronunciation, err = englishInflectedPronunciation(word); err != nil || pronunciation != "" {
		return pronunciation, err
	}
	if pronunciation, err = englishPrefixedPronunciation(word); err != nil || pronunciation != "" {
		return pronunciation, err
	}
	return englishRulePronunciation(word)
}
