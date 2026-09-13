package tts

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/jsut"
)

const defaultTargetPriorMinContextCount = 5

// resolveTargetPriorはJSUT由来の音素時間事前分布を読み込む。
// このモデルは日本語の実験用で、指定しない限り合成経路へ入らない。
func resolveTargetPrior(cfg Config) (*jsut.Prior, error) {
	if cfg.TargetPrior != nil {
		if err := validateTargetPrior(cfg.TargetPrior); err != nil {
			return nil, err
		}
		return cfg.TargetPrior, nil
	}
	path := strings.TrimSpace(cfg.TargetPriorPath)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read target prior %s: %w", path, err)
	}
	var prior jsut.Prior
	if err := json.Unmarshal(data, &prior); err != nil {
		return nil, fmt.Errorf("decode target prior %s: %w", path, err)
	}
	if err := validateTargetPrior(&prior); err != nil {
		return nil, fmt.Errorf("validate target prior %s: %w", path, err)
	}
	return &prior, nil
}

func validateTargetPrior(prior *jsut.Prior) error {
	if prior == nil {
		return fmt.Errorf("target prior is nil")
	}
	if prior.Version != jsut.PriorSchemaVersion || prior.Kind != "jsut_target_prior" {
		return fmt.Errorf("unsupported target prior %d/%q", prior.Version, prior.Kind)
	}
	if len(prior.Phones) == 0 {
		return fmt.Errorf("target prior has no phone statistics")
	}
	return nil
}

// targetPriorPhoneWeightsは現在のモーラ長を変えず、各音素の相対時間だけを返す。
// 前後音素コンテキストを優先し、未観測時は音素統計、固定重みの順に戻す。
func targetPriorPhoneWeights(prior *jsut.Prior, morae []frontend.Mora, strength float64, minContextCount int) [][]float64 {
	if prior == nil || len(morae) == 0 || strength <= 0 {
		return nil
	}
	if strength > 1 {
		strength = 1
	}
	if minContextCount < 1 {
		minContextCount = defaultTargetPriorMinContextCount
	}
	refs := targetPriorPhoneRefs(morae)
	weights := make([][]float64, len(morae))
	for _, ref := range refs {
		if ref.moraIndex < 0 || ref.moraIndex >= len(morae) || ref.phoneIndex < 0 || ref.phoneIndex >= len(morae[ref.moraIndex].Phones) {
			continue
		}
		if weights[ref.moraIndex] == nil {
			weights[ref.moraIndex] = make([]float64, len(morae[ref.moraIndex].Phones))
		}
		phone := morae[ref.moraIndex].Phones[ref.phoneIndex]
		base := frontend.PhoneWeight(phone.Symbol, phone.Role)
		priorValue := targetPriorPhoneMean(prior, ref, minContextCount)
		if priorValue <= 0 || !finiteTargetPrior(priorValue) {
			priorValue = base
		}
		weights[ref.moraIndex][ref.phoneIndex] = priorValue
	}
	for moraIndex, mora := range morae {
		if mora.Pause || len(mora.Phones) == 0 || len(weights[moraIndex]) != len(mora.Phones) {
			continue
		}
		var currentSum, priorSum float64
		for phoneIndex, phone := range mora.Phones {
			current := frontend.PhoneWeight(phone.Symbol, phone.Role)
			if current <= 0 || !finiteTargetPrior(current) {
				current = 1
			}
			if weights[moraIndex][phoneIndex] <= 0 || !finiteTargetPrior(weights[moraIndex][phoneIndex]) {
				weights[moraIndex][phoneIndex] = current
			}
			currentSum += current
			priorSum += weights[moraIndex][phoneIndex]
		}
		if currentSum <= 0 || priorSum <= 0 {
			weights[moraIndex] = nil
			continue
		}
		for phoneIndex, phone := range mora.Phones {
			current := frontend.PhoneWeight(phone.Symbol, phone.Role)
			if current <= 0 || !finiteTargetPrior(current) {
				current = 1
			}
			currentShare := current / currentSum
			priorShare := weights[moraIndex][phoneIndex] / priorSum
			weights[moraIndex][phoneIndex] = (1-strength)*currentShare + strength*priorShare
		}
	}
	return weights
}

type targetPriorPhoneRef struct {
	moraIndex  int
	phoneIndex int
	symbol     string
	previous   string
	next       string
}

func targetPriorPhoneRefs(morae []frontend.Mora) []targetPriorPhoneRef {
	refs := make([]targetPriorPhoneRef, 0)
	lastIndex := -1
	boundaryPrevious := "sil"
	for moraIndex, mora := range morae {
		if mora.Pause {
			if lastIndex >= 0 {
				refs[lastIndex].next = "pau"
			}
			lastIndex = -1
			boundaryPrevious = "pau"
			continue
		}
		for phoneIndex, phone := range mora.Phones {
			symbol := targetPriorSymbol(mora, phoneIndex, phone)
			previous := boundaryPrevious
			if lastIndex >= 0 {
				previous = refs[lastIndex].symbol
				refs[lastIndex].next = symbol
			}
			refs = append(refs, targetPriorPhoneRef{
				moraIndex: moraIndex, phoneIndex: phoneIndex, symbol: symbol,
				previous: previous,
			})
			lastIndex = len(refs) - 1
			boundaryPrevious = "sil"
		}
	}
	if lastIndex >= 0 {
		refs[lastIndex].next = "sil"
	}
	return refs
}

func targetPriorSymbol(mora frontend.Mora, phoneIndex int, phone frontend.Phone) string {
	// Open JTalkのんはn、jsut-labelの鼻音はNなので合わせる
	// 通常の子音nは変えない
	if phone.Role == "nucleus" && mora.Vowel == "n" && phoneIndex == len(mora.Phones)-1 {
		return "N"
	}
	return phone.Symbol
}

func targetPriorPhoneMean(prior *jsut.Prior, ref targetPriorPhoneRef, minContextCount int) float64 {
	key := ref.previous + "|" + ref.symbol + "|" + ref.next
	if stats, ok := prior.Contexts[key]; ok && stats.Count >= minContextCount && stats.DurationMS.Count > 0 {
		return stats.DurationMS.Mean
	}
	if stats, ok := prior.Phones[ref.symbol]; ok && stats.DurationMS.Count > 0 {
		return stats.DurationMS.Mean
	}
	return 0
}

func finiteTargetPrior(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
