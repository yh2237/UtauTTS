package label

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

const htkTicksPerMillisecond = 10000.0

type anchor struct {
	startMS float64
	phone   string
	order   int
}

// HTSは合成済みWAVの時間軸に合わせたmonophone labelを返す。
func HTS(synthesisPlan *plan.Plan, moraDurationsMS []float64, audioDurationMS float64) (string, error) {
	if synthesisPlan == nil {
		return "", fmt.Errorf("synthesis plan is nil")
	}
	if audioDurationMS <= 0 || math.IsNaN(audioDurationMS) || math.IsInf(audioDurationMS, 0) {
		return "", fmt.Errorf("audio duration must be positive and finite")
	}
	morae := synthesisPlan.Morae
	if len(morae) == 0 {
		var err error
		morae, err = frontend.ParseKana(synthesisPlan.Reading)
		if err != nil {
			return "", fmt.Errorf("parse reading: %w", err)
		}
	}
	if len(moraDurationsMS) != len(morae) {
		return "", fmt.Errorf("mora durations: got %d values for %d morae", len(moraDurationsMS), len(morae))
	}
	mainUnits := make(map[int]plan.Unit, len(synthesisPlan.Units))
	for _, unit := range synthesisPlan.Units {
		if unit.Role == "mora" || unit.Role == "" {
			mainUnits[unit.Position] = unit
		}
	}

	anchors := make([]anchor, 0, len(morae)*2+1)
	cursor := 0.0
	order := 0
	for position, mora := range morae {
		duration := math.Max(0, moraDurationsMS[position])
		noteStart := cursor + synthesisPlan.LeadingMarginMS
		if mora.Pause {
			anchors = append(anchors, anchor{startMS: noteStart, phone: "pau", order: order})
			order++
			cursor += duration
			continue
		}
		unit, ok := mainUnits[position]
		if !ok {
			return "", fmt.Errorf("mora unit missing at position %d", position)
		}
		if mora.Vowel == "n" || mora.Vowel == "cl" {
			phone := mora.Vowel
			if phone == "n" {
				phone = "N"
			}
			anchors = append(anchors, anchor{startMS: noteStart, phone: phone, order: order})
			order++
		} else {
			if mora.Consonant != "" {
				preutterance := math.Max(0, unit.EffectivePreutteranceMS)
				anchors = append(anchors, anchor{
					startMS: math.Max(0, noteStart-preutterance),
					phone:   labPhone(mora.Consonant), order: order,
				})
				order++
			}
			anchors = append(anchors, anchor{startMS: noteStart, phone: labPhone(mora.Vowel), order: order})
			order++
		}
		cursor += duration
	}
	if len(anchors) == 0 {
		return "", fmt.Errorf("reading contains no label phones")
	}

	nominalEnd := cursor + synthesisPlan.LeadingMarginMS
	if nominalEnd < audioDurationMS-0.01 {
		anchors = append(anchors, anchor{startMS: nominalEnd, phone: "sil", order: order})
	}
	sort.SliceStable(anchors, func(i, j int) bool {
		if anchors[i].startMS == anchors[j].startMS {
			return anchors[i].order < anchors[j].order
		}
		return anchors[i].startMS < anchors[j].startMS
	})
	if anchors[0].startMS > 0.01 {
		anchors = append([]anchor{{startMS: 0, phone: "sil", order: -1}}, anchors...)
	} else {
		anchors[0].startMS = 0
	}

	var builder strings.Builder
	for index, current := range anchors {
		start := clamp(current.startMS, 0, audioDurationMS)
		end := audioDurationMS
		if index+1 < len(anchors) {
			end = clamp(anchors[index+1].startMS, start, audioDurationMS)
		}
		if end-start < 0.0001 || current.phone == "" {
			continue
		}
		fmt.Fprintf(&builder, "%d %d %s\n",
			int64(math.Round(start*htkTicksPerMillisecond)),
			int64(math.Round(end*htkTicksPerMillisecond)), current.phone)
	}
	if builder.Len() == 0 {
		return "", fmt.Errorf("label contains no positive-duration phones")
	}
	return builder.String(), nil
}

func labPhone(phone string) string { return phone }

func clamp(value, minimum, maximum float64) float64 {
	return math.Max(minimum, math.Min(maximum, value))
}
