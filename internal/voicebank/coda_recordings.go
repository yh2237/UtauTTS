package voicebank

import (
	"math"
	"utautts/internal/connection"
)

func contiguousCodas(left, right Selection) bool {
	return len(left.CodaPhones) > 0 && len(right.CodaPhones) > 0 && left.CodaStart+len(left.CodaPhones) == right.CodaStart
}

// 選択済みの語末経路で録音候補を最適化する。不足音素は接合しない。
func selectEndingRecordings(layers [][]Selection, cache *connection.Extractor) []Selection {
	return selectEndingRecordingsFrom(nil, layers, cache)
}

func selectEndingRecordingsFrom(main *Selection, layers [][]Selection, cache *connection.Extractor) []Selection {
	paths, best := endingRecordingPathsFrom(main, layers, cache)
	if len(paths) == 0 {
		return nil
	}
	return paths[best]
}

// 最後の録音ごとに最良経路を残し、後続との接合も評価する。
func endingRecordingPathsFrom(main *Selection, layers [][]Selection, cache *connection.Extractor) ([][]Selection, int) {
	if len(layers) == 0 {
		return nil, 0
	}
	states := make([][]pathState, len(layers))
	for i, choices := range layers {
		states[i] = make([]pathState, len(choices))
		for j, current := range choices {
			local := current.TargetScore + englishEndingReleasePreference(current)
			if i == 0 && main != nil && current.CodaStart == 0 && len(current.CodaPhones) > 0 {
				local += cache.ScoreSpeechContext(main.Entry, current.Entry) + sourceGroupContinuityScore(main.Entry, current.Entry)
			}
			best := pathState{score: local, previous: -1}
			if i > 0 {
				best.score = math.Inf(-1)
				for k, previous := range layers[i-1] {
					join := 0.0
					if contiguousCodas(previous, current) {
						join = cache.ScoreSpeechTailContext(previous.Entry, current.Entry) + sourceGroupContinuityScore(previous.Entry, current.Entry)
					}
					score := states[i-1][k].score + local + join
					if score > best.score {
						best = pathState{score: score, previous: k}
					}
				}
			}
			states[i][j] = best
		}
	}
	last := 0
	for j := range states[len(states)-1] {
		if states[len(states)-1][j].score > states[len(states)-1][last].score {
			last = j
		}
	}
	paths := make([][]Selection, len(states[len(states)-1]))
	for terminal := range paths {
		paths[terminal] = make([]Selection, len(layers))
		cursor := terminal
		for i := len(layers) - 1; i >= 0; i-- {
			paths[terminal][i] = layers[i][cursor]
			cursor = states[i][cursor].previous
		}
	}
	return paths, last
}
