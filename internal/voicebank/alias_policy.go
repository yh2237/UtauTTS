package voicebank

import (
	"strings"
)

func aliasCandidatesWithPolicy(mora, previousVowel string, phraseStart bool, policy AliasPolicy) []aliasCandidate {
	forms := make([]aliasForm, 0, 4)
	if mora == "ー" {
		if vowelKana := map[string]string{"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お"}[previousVowel]; vowelKana != "" {
			forms = append(forms, aliasForm{text: vowelKana}, aliasForm{text: toKatakana(vowelKana)})
		}
	}
	// 同音候補より元の仮名を優先する。
	base := []aliasForm{{text: mora}}
	for _, equivalent := range equivalentKanaForms(mora) {
		base = append(base, aliasForm{text: equivalent, fallback: 1, equivalent: true})
	}
	for _, form := range base {
		forms = append(forms, form)
		if katakana := toKatakana(form.text); katakana != form.text {
			forms = append(forms, aliasForm{text: katakana, fallback: form.fallback, equivalent: form.equivalent})
		}
		// ヴ行は「ヴぁ」のように、ヴだけ片仮名で小書きは平仮名の表記の音源が多い。
		if rest, ok := strings.CutPrefix(form.text, "ゔ"); ok && rest != "" {
			forms = append(forms, aliasForm{text: "ヴ" + rest, fallback: form.fallback, equivalent: form.equivalent})
		}
	}

	var candidates []aliasCandidate
	allowVCVTarget := mora != "っ"
	if policy != AliasPolicyCVOnly && allowVCVTarget && phraseStart {
		for _, form := range forms {
			candidates = append(candidates, aliasCandidate{name: "- " + form.text, tier: form.fallback, kind: AliasVCV, equivalent: form.equivalent})
		}
	} else if policy != AliasPolicyCVOnly && allowVCVTarget && previousVowel != "" && previousVowel != "cl" {
		for _, form := range forms {
			candidates = append(candidates, aliasCandidate{name: previousVowel + " " + form.text, tier: form.fallback, kind: AliasVCV, equivalent: form.equivalent})
		}
	}
	for _, form := range forms {
		candidates = append(candidates, aliasCandidate{name: form.text, tier: policyTier(policy, 1, AliasCV) + form.fallback, kind: AliasCV, equivalent: form.equivalent})
	}
	if policy != AliasPolicyCVOnly && !phraseStart {
		for _, form := range forms {
			candidates = append(candidates, aliasCandidate{name: "* " + form.text, tier: policyTier(policy, 2, AliasCV) + form.fallback, kind: AliasCV, equivalent: form.equivalent})
		}
	}
	return uniqueCandidates(candidates)
}

func vcAliasCandidates(previousVowel, consonant string, policy AliasPolicy) []aliasCandidate {
	if policy == AliasPolicyCVOnly || previousVowel == "" || previousVowel == "cl" || consonant == "" || consonant == "cl" {
		return nil
	}
	contexts := vowelContextForms(previousVowel)
	result := make([]aliasCandidate, 0, len(contexts))
	for _, context := range contexts {
		result = append(result, aliasCandidate{name: context + " " + consonant, tier: vcPolicyTier(policy), kind: AliasVC})
	}
	return uniqueCandidates(result)
}

func vowelContextForms(vowel string) []string {
	forms := []string{vowel}
	if kana := map[string]string{"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お", "n": "ん"}[vowel]; kana != "" {
		forms = append(forms, kana)
	}
	return forms
}

func vcPolicyTier(policy AliasPolicy) int {
	if policy == AliasPolicyVCVPrefer {
		return 2
	}
	return 0
}
