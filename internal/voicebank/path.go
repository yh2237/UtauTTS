package voicebank

import (
	"math"

	"utautts/internal/connection"
	"utautts/internal/oto"
)

type pathState struct {
	score           float64
	previous        int
	joinScore       float64
	joinProbability float64
}

// selectBestPathsは候補コストと接続スコアを分離したラティス探索を行う。
func selectBestPaths(layers [][]Selection, mode SelectionMode, model *connection.LearnedModel, extractor *connection.Extractor) []Selection {
	return selectBestPathsWithAcoustic(layers, mode, model, extractor, "")
}

func selectBestPathsWithAcoustic(layers [][]Selection, mode SelectionMode, model *connection.LearnedModel, extractor *connection.Extractor, acousticMode string) []Selection {
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
		phrase := layers[start:end]
		switch mode {
		case SelectionGreedy:
			result = append(result, selectGreedyPathWithAcoustic(phrase, cache, model, true, acousticMode)...)
		case SelectionTargetOnly:
			result = append(result, selectGreedyPathWithAcoustic(phrase, cache, nil, false, acousticMode)...)
		default:
			result = append(result, selectPhrasePathWithAcoustic(phrase, cache, model, acousticMode)...)
		}
		start = end
	}
	return result
}

func selectPhrasePath(layers [][]Selection, cache *connection.Extractor, model *connection.LearnedModel) []Selection {
	return selectPhrasePathWithAcoustic(layers, cache, model, "")
}

func selectPhrasePathWithAcoustic(layers [][]Selection, cache *connection.Extractor, model *connection.LearnedModel, acousticMode string) []Selection {
	states := make([][]pathState, len(layers))
	states[0] = make([]pathState, len(layers[0]))
	for candidateIndex, candidate := range layers[0] {
		local, _, _ := candidateScoresWithAcoustic(candidate, cache, model, true, acousticMode)
		states[0][candidateIndex] = pathState{score: local, previous: -1}
	}
	for layerIndex := 1; layerIndex < len(layers); layerIndex++ {
		states[layerIndex] = make([]pathState, len(layers[layerIndex]))
		for currentIndex, current := range layers[layerIndex] {
			best := pathState{score: math.Inf(-1), previous: -1}
			for previousIndex, previous := range layers[layerIndex-1] {
				join, probability := pairScoreWithAcoustic(currentEndEntry(previous), currentStartEntry(current), cache, model, acousticMode)
				local, transitionJoin, transitionProbability := candidateScoresWithAcoustic(current, cache, model, true, acousticMode)
				score := states[layerIndex-1][previousIndex].score + local + join
				if score > best.score {
					best = pathState{score: score, previous: previousIndex, joinScore: join, joinProbability: probability}
					current.TransitionJoinScore = transitionJoin
					current.TransitionJoinProbability = transitionProbability
				}
			}
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
		_, transitionJoin, transitionProbability := candidateScoresWithAcoustic(path[layerIndex], cache, model, true, acousticMode)
		path[layerIndex].TransitionJoinScore = transitionJoin
		path[layerIndex].TransitionJoinProbability = transitionProbability
		path[layerIndex].JoinScore = states[layerIndex][last].joinScore
		path[layerIndex].JoinProbability = states[layerIndex][last].joinProbability
		path[layerIndex].PathScore = states[layerIndex][last].score
		last = states[layerIndex][last].previous
	}
	if acousticMode != "" {
		setPathAcousticJoinScores(path, cache)
	}
	return path
}

func selectGreedyPath(layers [][]Selection, cache *connection.Extractor, model *connection.LearnedModel, useJoin bool) []Selection {
	return selectGreedyPathWithAcoustic(layers, cache, model, useJoin, "")
}

func selectGreedyPathWithAcoustic(layers [][]Selection, cache *connection.Extractor, model *connection.LearnedModel, useJoin bool, acousticMode string) []Selection {
	path := make([]Selection, 0, len(layers))
	pathScore := 0.0
	for layerIndex, layer := range layers {
		bestIndex := 0
		bestJoin := 0.0
		bestProbability := 0.0
		bestLocal := math.Inf(-1)
		for candidateIndex, candidate := range layer {
			join, probability := 0.0, 0.0
			if useJoin && layerIndex > 0 {
				join, probability = pairScoreWithAcoustic(currentEndEntry(path[layerIndex-1]), currentStartEntry(candidate), cache, model, acousticMode)
			}
			local, transitionJoin, transitionProbability := candidateScoresWithAcoustic(candidate, cache, model, useJoin, acousticMode)
			local += join
			if local > bestLocal {
				bestIndex, bestJoin, bestProbability, bestLocal = candidateIndex, join, probability, local
				layer[candidateIndex].TransitionJoinScore = transitionJoin
				layer[candidateIndex].TransitionJoinProbability = transitionProbability
			}
		}
		selected := layer[bestIndex]
		_, selectedTransitionJoin, selectedTransitionProbability := candidateScoresWithAcoustic(selected, cache, model, useJoin, acousticMode)
		selected.TransitionJoinScore = selectedTransitionJoin
		selected.TransitionJoinProbability = selectedTransitionProbability
		selected.JoinScore = bestJoin
		selected.JoinProbability = bestProbability
		pathScore += bestLocal
		selected.PathScore = pathScore
		path = append(path, selected)
	}
	if acousticMode != "" {
		setPathAcousticJoinScores(path, cache)
	}
	return path
}

func setPathAcousticJoinScores(path []Selection, cache *connection.Extractor) {
	for index := 1; index < len(path); index++ {
		path[index].AcousticJoinScore = acousticPairAdjustment(
			cache.Pair(currentEndEntry(path[index-1]), currentStartEntry(path[index])),
		)
	}
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

func candidateScores(selection Selection, cache *connection.Extractor, model *connection.LearnedModel, includeJoin bool) (local, transitionJoin, transitionProbability float64) {
	return candidateScoresWithAcoustic(selection, cache, model, includeJoin, "")
}

func candidateScoresWithAcoustic(selection Selection, cache *connection.Extractor, model *connection.LearnedModel, includeJoin bool, acousticMode string) (local, transitionJoin, transitionProbability float64) {
	local = selection.TargetScore + selection.PreferenceScore
	if acousticMode == AcousticModeApply {
		local += selection.AcousticTargetScore
	}
	previous := selection.Entry
	for _, ending := range selection.Endings {
		local += ending.TargetScore - 114
		if includeJoin {
			endingJoin, _ := pairScoreWithAcoustic(previous, ending.Entry, cache, model, acousticMode)
			local += endingJoin
		}
		previous = ending.Entry
	}
	if selection.Transition == nil {
		return local, 0, 0
	}
	if !includeJoin {
		return local + selection.TransitionScore - 114, 0, 0
	}
	transitionJoin, transitionProbability = pairScoreWithAcoustic(selection.Transition.Entry, selection.Entry, cache, model, acousticMode)

	local += selection.TransitionScore - 114 + transitionJoin
	return local, transitionJoin, transitionProbability
}

func joinScore(previous, current oto.Entry, cache *connection.Extractor) float64 {
	return connection.HandcraftedScore(cache.Pair(previous, current))
}

func pairScore(previous, current oto.Entry, cache *connection.Extractor, model *connection.LearnedModel) (float64, float64) {
	return pairScoreWithAcoustic(previous, current, cache, model, "")
}

func pairScoreWithAcoustic(previous, current oto.Entry, cache *connection.Extractor, model *connection.LearnedModel, acousticMode string) (float64, float64) {
	features := cache.Pair(previous, current)
	groupScore := sourceGroupContinuityScore(previous, current)
	acousticScore := 0.0
	if acousticMode == AcousticModeApply {
		acousticScore = acousticPairAdjustment(features)
	}
	if model != nil {
		score, probability := connection.LearnedScore(features, model)
		return score + groupScore + acousticScore, probability
	}
	return connection.HandcraftedScore(features) + groupScore + acousticScore, 0
}

func sourceGroupContinuityScore(previous, current oto.Entry) float64 {
	if previous.SourceGroup == "" || current.SourceGroup == "" || previous.SourceGroup == current.SourceGroup {
		return 0
	}
	return -6
}
