package voicebank

import (
	"math"

	"utautts/internal/connection"
	"utautts/internal/oto"
)

type pathState struct {
	score     float64
	previous  int
	joinScore float64
}

// selectBestPaths finds the highest scoring path for each phrase.
func selectBestPaths(layers [][]Selection, extractor *connection.Extractor) []Selection {
	result := make([]Selection, 0, len(layers))
	cache := extractor
	if cache == nil {
		cache = connection.NewExtractor()
	}
	for start := 0; start < len(layers); {
		for start < len(layers) && len(layers[start]) == 0 {
			start++
		}
		if start == len(layers) {
			break
		}
		end := start + 1
		for end < len(layers) && len(layers[end]) > 0 {
			end++
		}
		result = append(result, selectPhrasePath(layers[start:end], cache)...)
		start = end
	}
	return result
}

func selectPhrasePath(layers [][]Selection, cache *connection.Extractor) []Selection {
	if len(layers) == 0 {
		return nil
	}
	states := make([][]pathState, len(layers))
	states[0] = make([]pathState, len(layers[0]))
	for candidateIndex, candidate := range layers[0] {
		local, transitionJoin := candidateScores(candidate, cache, true)
		states[0][candidateIndex] = pathState{score: local, previous: -1}
		layers[0][candidateIndex].TransitionJoinScore = transitionJoin
	}
	for layerIndex := 1; layerIndex < len(layers); layerIndex++ {
		states[layerIndex] = make([]pathState, len(layers[layerIndex]))
		for currentIndex, current := range layers[layerIndex] {
			best := pathState{score: math.Inf(-1), previous: -1}
			local, transitionJoin := candidateScores(current, cache, true)
			for previousIndex, previous := range layers[layerIndex-1] {
				join := joinScore(currentEndEntry(previous), currentStartEntry(current), cache)
				score := states[layerIndex-1][previousIndex].score + local + join
				if score > best.score {
					best = pathState{score: score, previous: previousIndex, joinScore: join}
				}
			}
			layers[layerIndex][currentIndex].TransitionJoinScore = transitionJoin
			states[layerIndex][currentIndex] = best
		}
	}

	last := 0
	for candidateIndex := 1; candidateIndex < len(states[len(states)-1]); candidateIndex++ {
		if states[len(states)-1][candidateIndex].score > states[len(states)-1][last].score {
			last = candidateIndex
		}
	}
	path := make([]Selection, len(layers))
	for layerIndex := len(layers) - 1; layerIndex >= 0; layerIndex-- {
		path[layerIndex] = layers[layerIndex][last]
		path[layerIndex].JoinScore = states[layerIndex][last].joinScore
		path[layerIndex].PathScore = states[layerIndex][last].score
		last = states[layerIndex][last].previous
	}
	return path
}

func currentStartEntry(selection Selection) oto.Entry {
	if selection.Transition != nil {
		return selection.Transition.Entry
	}
	return selection.Entry
}

func currentEndEntry(selection Selection) oto.Entry {
	if len(selection.Endings) > 0 {
		return selection.Endings[len(selection.Endings)-1].Entry
	}
	return selection.Entry
}

func candidateScores(selection Selection, cache *connection.Extractor, includeJoin bool) (local, transitionJoin float64) {
	local = selection.TargetScore + selection.PreferenceScore
	previous := selection.Entry
	for index := range selection.Endings {
		ending := &selection.Endings[index]
		local += ending.TargetScore - 114 + englishEndingReleasePreference(*ending)
		if includeJoin {
			local += joinScore(previous, ending.Entry, cache)
		}
		previous = ending.Entry
	}
	if selection.Transition == nil {
		return local, 0
	}
	if !includeJoin {
		return local + selection.TransitionScore - 114, 0
	}
	transitionJoin = joinScore(selection.Transition.Entry, selection.Entry, cache)
	return local + selection.TransitionScore - 114 + transitionJoin, transitionJoin
}

func joinScore(previous, current oto.Entry, cache *connection.Extractor) float64 {
	return cache.ScoreEntries(previous, current) + sourceGroupContinuityScore(previous, current)
}

func sourceGroupContinuityScore(previous, current oto.Entry) float64 {
	if previous.SourceGroup == "" || current.SourceGroup == "" || previous.SourceGroup == current.SourceGroup {
		return 0
	}
	return -6
}
