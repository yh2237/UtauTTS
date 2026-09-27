package frontend

// 時間配分用のPinyin区分。IPA表記や原音aliasは変更しない。
func mandarinRhymeParts(final string) []Phone {
	parts := map[string][]Phone{
		"ai": {{"a", "nucleus"}, {"i", "offglide"}}, "ei": {{"e", "nucleus"}, {"i", "offglide"}},
		"ao": {{"a", "nucleus"}, {"u", "offglide"}}, "ou": {{"o", "nucleus"}, {"u", "offglide"}},
		"ia": {{"i", "medial"}, {"a", "nucleus"}}, "ie": {{"i", "medial"}, {"e", "nucleus"}},
		"ua": {{"u", "medial"}, {"a", "nucleus"}}, "uo": {{"u", "medial"}, {"o", "nucleus"}},
		"ve": {{"v", "medial"}, {"e", "nucleus"}}, "ue": {{"u", "medial"}, {"e", "nucleus"}},
		"iao": {{"i", "medial"}, {"a", "nucleus"}, {"u", "offglide"}},
		"iou": {{"i", "medial"}, {"o", "nucleus"}, {"u", "offglide"}},
		"uai": {{"u", "medial"}, {"a", "nucleus"}, {"i", "offglide"}},
		"uei": {{"u", "medial"}, {"e", "nucleus"}, {"i", "offglide"}},
	}
	if p := parts[final]; p != nil {
		return append([]Phone(nil), p...)
	}
	return []Phone{{final, "nucleus"}}
}

// MandarinRhymeSharesは韻母長を配分し、音節長と鼻音位置を保つ。
func MandarinRhymeShares(phones []Phone) []float64 {
	result := make([]float64, len(phones))
	medial, offglide := false, false
	for _, p := range phones {
		medial = medial || p.Role == "medial"
		offglide = offglide || p.Role == "offglide"
	}
	nucleus := 1.0
	if medial {
		nucleus -= .2
	}
	if offglide {
		nucleus -= .3
	}
	for i, p := range phones {
		switch p.Role {
		case "medial":
			result[i] = .2
		case "offglide":
			result[i] = .3
		case "nucleus":
			result[i] = nucleus
		}
	}
	return result
}
