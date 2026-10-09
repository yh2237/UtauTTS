package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
	"utautts/internal/frontend"
)

type utterance struct {
	ID                 string
	Frames, Continuous int
	IDs                []int
	Cont, Target       []float32
	F0Target           []float32
	EnergyTarget       []float32
	F0Extra            []float32
	// Positionは文内の位置の特徴（--f0-position）。キャッシュの保存後に付けるため、キャッシュには入らない。
	Position []float32
}

// featureVersionはキャッシュ形式の版。utteranceの目標を変えたら上げる。
const featureVersion = 7

// f0ContextFeaturesはF0ブランチの連続入力（音素内位置・対数長・アクセント12・POS12・pos_group1 29）。
const (
	f0AccentFeatures   = 12
	f0PosFeatures      = 12
	f0PosGroupFeatures = 29
	f0ContextFeatures  = 2 + f0AccentFeatures + f0PosFeatures + f0PosGroupFeatures
	f0ExtraFeatures    = f0AccentFeatures + f0PosFeatures + f0PosGroupFeatures
)

// posVocabはall-mfaのトークンから取った固定語彙（昇順）。未知は末尾のother。
var posVocab = []string{"フィラー", "副詞", "助動詞", "助詞", "動詞", "名詞", "形容詞", "感動詞", "接続詞", "接頭詞", "連体詞"}

// posGroup1Vocabは同様にpos_group1の固定語彙（昇順）。未知は末尾のother。
var posGroup1Vocab = []string{"*", "サ変接続", "ナイ形容詞語幹", "一般", "並立助詞", "代名詞", "係助詞", "副助詞", "副助詞／並立助詞／終助詞", "副詞化", "副詞可能", "助詞類接続", "動詞接続", "動詞非自立的", "名詞接続", "固有名詞", "形容動詞語幹", "接尾", "接続助詞", "接続詞的", "数", "数接続", "格助詞", "特殊", "終助詞", "自立", "連体化", "非自立"}

type datasetToken struct {
	Mora                 string  `json:"mora"`
	Vowel                string  `json:"vowel"`
	Pause                bool    `json:"pause"`
	AccentPhrasePosition int     `json:"accent_phrase_position"`
	AccentPhraseLength   int     `json:"accent_phrase_length"`
	AccentNucleus        int     `json:"accent_nucleus"`
	AccentHigh           bool    `json:"accent_high"`
	AccentPhraseStart    bool    `json:"accent_phrase_start"`
	AccentPhraseEnd      bool    `json:"accent_phrase_end"`
	WordStart            bool    `json:"word_start"`
	WordEnd              bool    `json:"word_end"`
	Pos                  string  `json:"pos"`
	PosGroup1            string  `json:"pos_group1"`
	StartMS              float64 `json:"start_ms"`
	EndMS                float64 `json:"end_ms"`
}

type record struct {
	ID         string         `json:"id"`
	AudioPath  string         `json:"audio_path"`
	Version    int            `json:"version"`
	PlanTiming bool           `json:"plan_timing"`
	Tokens     []datasetToken `json:"tokens"`
}

type alignment struct {
	Tiers struct {
		Phones struct {
			Entries [][]json.RawMessage `json:"entries"`
		} `json:"phones"`
	} `json:"tiers"`
}
type phone struct {
	start, end float64
	name, raw  string
}

var ipa = map[string]string{"a": "a", "i": "i", "ɯ": "u", "e": "e", "o": "o", "aː": "a", "iː": "i", "ɯː": "u", "eː": "e", "oː": "o", "i̥": "i", "ɯ̥": "u", "ɴ": "N", "ʔ": "cl", "k": "k", "ɡ": "g", "s": "s", "ɕ": "sh", "z": "z", "dʑ": "j", "t": "t", "tɕ": "ch", "ts": "ts", "d": "d", "n": "n", "h": "h", "ɸ": "f", "b": "b", "p": "p", "m": "m", "j": "y", "ɾ": "r", "w": "w", "v": "v", "dʲ": "dy", "tʲ": "ty", "ʑ": "j", "dz": "z", "ŋ": "n", "ɰ̃": "N"}
var palatal = map[string][2]string{"c": {"k", "ky"}, "ɟ": {"g", "gy"}, "ɲ": {"n", "ny"}, "ç": {"h", "hy"}, "mʲ": {"m", "my"}, "ɾʲ": {"r", "ry"}, "bʲ": {"b", "by"}, "pʲ": {"p", "py"}}

// alignmentPathはカンマ区切りのalignmentsディレクトリからIDのJSONを探す。
func alignmentPath(alignDir, id string) (string, bool) {
	for _, dir := range strings.Split(alignDir, ",") {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, id+".json")
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func readPhones(path string) ([]phone, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var a alignment
	if e = json.Unmarshal(b, &a); e != nil {
		return nil, e
	}
	var ps []phone
	cursor := 0.0
	for _, entry := range a.Tiers.Phones.Entries {
		if len(entry) != 3 {
			return nil, fmt.Errorf("bad phones entry in %s", path)
		}
		var s, t float64
		var raw string
		if e = json.Unmarshal(entry[0], &s); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(entry[1], &t); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(entry[2], &raw); e != nil {
			return nil, e
		}
		if s > cursor+1e-4 {
			ps = append(ps, phone{cursor, s, "sil", ""})
		}
		ps = append(ps, phone{s, t, "", raw})
		cursor = t
	}
	for i := range ps {
		p := &ps[i]
		switch p.raw {
		case "", "<eps>", "sil", "sp", "spn":
			p.name = "sil"
		default:
			if pair, ok := palatal[p.raw]; ok {
				next := ""
				if i+1 < len(ps) {
					next = ps[i+1].raw
				}
				if next == "i" || next == "iː" || next == "i̥" {
					p.name = pair[0]
				} else {
					p.name = pair[1]
				}
			} else if v, ok := ipa[p.raw]; ok {
				p.name = v
			} else {
				p.name = "<unk>"
			}
		}
	}
	return ps, nil
}

func frameInputs(ps []phone, f0 []float64, vocab *vocabulary) ([]int, []float32) {
	frames := len(f0)
	ids := make([]int, frames*3)
	cont := make([]float32, frames*4)
	for i := range ids {
		ids[i] = vocab.silence
	}
	for i, p := range ps {
		a := int(math.Round(p.start * 100))
		b := int(math.Round(p.end * 100))
		if a < 0 {
			a = 0
		}
		if b < a+1 {
			b = a + 1
		}
		if b > frames {
			b = frames
		}
		if a >= frames {
			break
		}
		prev, next := "sil", "sil"
		if i > 0 {
			prev = ps[i-1].name
		}
		if i+1 < len(ps) {
			next = ps[i+1].name
		}
		for t := a; t < b; t++ {
			copy(ids[t*3:t*3+3], []int{vocab.id(p.name), vocab.id(prev), vocab.id(next)})
			cont[t*4] = float32(float64(t-a)+.5) / float32(max(1, b-a))
			cont[t*4+1] = float32(math.Log((p.end-p.start)*1000+1) / 6)
		}
	}
	var voiced []int
	mean := 0.0
	for i, v := range f0 {
		if v > 0 {
			voiced = append(voiced, i)
			mean += math.Log(v)
			cont[i*4+3] = 1
		}
	}
	if len(voiced) > 0 {
		mean /= float64(len(voiced))
		k := 0
		for t := range f0 {
			for k < len(voiced) && voiced[k] < t {
				k++
			}
			var v float64
			switch {
			case k == 0:
				v = math.Log(f0[voiced[0]])
			case k == len(voiced):
				v = math.Log(f0[voiced[len(voiced)-1]])
			default:
				l, r := voiced[k-1], voiced[k]
				w := float64(t-l) / float64(r-l)
				v = math.Log(f0[l])*(1-w) + math.Log(f0[r])*w
			}
			cont[t*4+2] = float32((v - mean) / .3)
		}
	}
	return ids, cont
}

func boolFeature(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

// f0ExtraFeatureFrameはアクセント12＋POS one-hot12＋pos_group1 one-hot29を返す。
func f0ExtraFeatureFrame(token datasetToken) [f0ExtraFeatures]float32 {
	var result [f0ExtraFeatures]float32
	posIndex := len(posVocab)
	groupIndex := len(posGroup1Vocab)
	if token.Pause {
		result[f0AccentFeatures+posIndex] = 1
		result[f0AccentFeatures+f0PosFeatures+groupIndex] = 1
		return result
	}
	length := token.AccentPhraseLength
	if length < 1 {
		length = 1
	}
	position := token.AccentPhrasePosition
	nucleus := token.AccentNucleus
	accent := [f0AccentFeatures]float32{
		float32(float64(position) / float64(length)),
		float32(float64(length-position) / float64(length)),
		float32(float64(nucleus) / float64(length)),
		boolFeature(token.AccentHigh),
		boolFeature(token.AccentPhraseStart),
		boolFeature(token.AccentPhraseEnd),
		boolFeature(token.WordStart),
		boolFeature(token.WordEnd),
	}
	switch {
	case nucleus == 0:
		accent[8] = 1
	case position < nucleus:
		accent[9] = 1
	case position == nucleus:
		accent[10] = 1
	default:
		accent[11] = 1
	}
	copy(result[:f0AccentFeatures], accent[:])
	for index, name := range posVocab {
		if token.Pos == name {
			posIndex = index
			break
		}
	}
	result[f0AccentFeatures+posIndex] = 1
	for index, name := range posGroup1Vocab {
		if token.PosGroup1 == name {
			groupIndex = index
			break
		}
	}
	result[f0AccentFeatures+f0PosFeatures+groupIndex] = 1
	return result
}

func logMel(sp []float64, frames, fft, rate int) []float64 {
	const mels = 80
	bins := fft/2 + 1
	mel := func(hz float64) float64 { return 2595 * math.Log10(1+hz/700) }
	points := make([]float64, mels+2)
	lo, hi := mel(40), mel(12000)
	for i := range points {
		points[i] = 700 * (math.Pow(10, (lo+(hi-lo)*float64(i)/81)/2595) - 1)
	}
	type weight struct {
		bin   int
		value float64
	}
	weights := make([][]weight, mels)
	for m := 0; m < mels; m++ {
		sum := 0.0
		for b := 0; b < bins; b++ {
			hz := float64(b*rate) / float64(fft)
			v := math.Max(0, math.Min((hz-points[m])/(points[m+1]-points[m]), (points[m+2]-hz)/(points[m+2]-points[m+1])))
			if v > 0 {
				weights[m] = append(weights[m], weight{b, v})
			}
			sum += v
		}
		for b := range weights[m] {
			weights[m][b].value /= math.Max(sum, 1e-12)
		}
	}
	out := make([]float64, frames*mels)
	for t := 0; t < frames; t++ {
		for m := 0; m < mels; m++ {
			sum := 0.0
			for _, w := range weights[m] {
				sum += sp[t*bins+w.bin] * w.value
			}
			out[t*mels+m] = 10 * math.Log10(sum+1e-12)
		}
	}
	return out
}

// trainingRecordは特徴量化の入力。Phonesがnilならalignmentsから読む。
type trainingRecord struct {
	ID         string
	AudioPath  string
	Phones     []phone
	Tokens     []datasetToken
	PlanTiming bool
}

// f0TeacherLineは蒸留教師の1行（v10のランタイム輪郭、cent）。
type f0TeacherLine struct {
	ID    string    `json:"id"`
	Cents []float64 `json:"cents"`
}

func loadF0Teacher(path string) (map[string][]float32, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result := map[string][]float32{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 16<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row f0TeacherLine
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, err
		}
		values := make([]float32, len(row.Cents))
		for i, value := range row.Cents {
			values[i] = float32(value / 100)
		}
		result[row.ID] = values
	}
	return result, scanner.Err()
}

func featurize(world *worldEngine, rec trainingRecord, alignDir string, vocab *vocabulary, teacher []float32) (utterance, error) {
	if rec.PlanTiming {
		return featurizePlan(rec, vocab, teacher)
	}
	pcm, e := audio.ReadWav(rec.AudioPath)
	if e != nil {
		return utterance{}, e
	}
	if pcm.Channels < 1 {
		return utterance{}, fmt.Errorf("no channels: %s", rec.AudioPath)
	}
	x := make([]float64, len(pcm.Data)/pcm.Channels)
	for i := range x {
		for ch := 0; ch < pcm.Channels; ch++ {
			x[i] += float64(pcm.Data[i*pcm.Channels+ch]) / 32768
		}
		x[i] /= float64(pcm.Channels)
	}
	f0, sp, fft, e := world.analyze(x, pcm.SampleRate)
	if e != nil {
		return utterance{}, e
	}
	ps := rec.Phones
	if ps == nil {
		path, ok := alignmentPath(alignDir, rec.ID)
		if !ok {
			return utterance{}, fmt.Errorf("no alignment for %s", rec.ID)
		}
		ps, e = readPhones(path)
		if e != nil {
			return utterance{}, e
		}
	}
	ids, cont := frameInputs(ps, f0, vocab)
	mel := logMel(sp, len(f0), fft, pcm.SampleRate)
	speech := make([]bool, len(f0))
	n := 0
	for t := range f0 {
		speech[t] = ids[t*3] != vocab.silence
		if speech[t] {
			n++
		}
	}
	if n < 2 {
		for t := range speech {
			speech[t] = true
		}
		n = len(speech)
	}
	target := make([]float32, len(mel))
	for m := 0; m < 80; m++ {
		mean := 0.0
		for t := range f0 {
			if speech[t] {
				mean += mel[t*80+m]
			}
		}
		mean /= float64(n)
		variance := 0.0
		for t := range f0 {
			if speech[t] {
				d := mel[t*80+m] - mean
				variance += d * d
			}
		}
		scale := 1 / (math.Sqrt(variance/float64(n)) + 1e-3)
		for t := range f0 {
			target[t*80+m] = float32((mel[t*80+m] - mean) * scale)
		}
	}
	// F0目標: 蒸留教師があればその輪郭（cent/100）、なければ内部自己相関F0（無声・休止はNaN）。
	f0Target := make([]float32, len(f0))
	if teacher != nil {
		for t := range f0 {
			if t < len(teacher) {
				f0Target[t] = teacher[t]
			}
		}
	} else {
		internal := internalF0Track(x, pcm.SampleRate, 10.0)
		pause := make([]bool, len(f0))
		for _, token := range rec.Tokens {
			if !token.Pause || token.EndMS <= token.StartMS {
				continue
			}
			a := int(math.Round(token.StartMS / 10))
			b := int(math.Round(token.EndMS / 10))
			a, b = max(0, a), min(len(f0), max(b, a+1))
			for t := a; t < b; t++ {
				pause[t] = true
			}
		}
		voiced := make([]bool, len(f0))
		mean := 0.0
		count := 0
		for t := range f0 {
			if t < len(internal) && internal[t] > 0 && !pause[t] {
				mean += math.Log(internal[t])
				voiced[t] = true
				count++
			}
		}
		if count > 0 {
			mean /= float64(count)
			for t := range f0 {
				if !voiced[t] {
					f0Target[t] = float32(math.NaN())
					continue
				}
				sum := 0.0
				neighbors := 0
				for k := -1; k <= 1; k++ {
					j := t + k
					if j >= 0 && j < len(f0) && voiced[j] {
						sum += math.Log(internal[j]) - mean
						neighbors++
					}
				}
				value := sum / float64(neighbors) / 0.3
				f0Target[t] = float32(math.Min(1.0, math.Max(-1.0, value)))
			}
		} else {
			for t := range f0 {
				f0Target[t] = float32(math.NaN())
			}
		}
	}
	// エネルギー目標はスペクトル合計のdBを発話内で中心化し/10した値。
	bins := fft/2 + 1
	energyTarget := make([]float32, len(f0))
	meanEnergy := 0.0
	for t := range f0 {
		sum := 0.0
		for b := 0; b < bins; b++ {
			sum += sp[t*bins+b]
		}
		energyTarget[t] = float32(10 * math.Log10(sum+1e-12))
		if speech[t] {
			meanEnergy += float64(energyTarget[t])
		}
	}
	meanEnergy /= float64(n)
	for t := range f0 {
		energyTarget[t] = float32((float64(energyTarget[t]) - meanEnergy) / 10)
	}
	// アクセント・POS特徴はトークンの時刻からフレームへ展開する（F0ブランチ専用）。
	extra := make([]float32, len(f0)*f0ExtraFeatures)
	for _, token := range rec.Tokens {
		if token.EndMS <= token.StartMS {
			continue
		}
		feature := f0ExtraFeatureFrame(token)
		a := int(math.Round(token.StartMS / 10))
		b := int(math.Round(token.EndMS / 10))
		a, b = max(0, a), min(len(f0), max(b, a+1))
		for t := a; t < b; t++ {
			copy(extra[t*f0ExtraFeatures:(t+1)*f0ExtraFeatures], feature[:])
		}
	}
	return utterance{rec.ID, len(f0), 4, ids, cont, target, f0Target, energyTarget, extra, nil}, nil
}

// featurizePlanはプラン時間風のレコードを音声なしで特徴量化する（F0蒸留用。メル・エネルギーはNaNで除外）。
func featurizePlan(rec trainingRecord, vocab *vocabulary, teacher []float32) (utterance, error) {
	durationMS := 0.0
	for _, token := range rec.Tokens {
		if token.EndMS > durationMS {
			durationMS = token.EndMS
		}
	}
	frames := int(math.Round(durationMS / 10))
	if frames < 2 {
		return utterance{}, fmt.Errorf("plan record %s is too short", rec.ID)
	}
	ps := planPhones(rec.Tokens)
	f0 := make([]float64, frames)
	ids, cont := frameInputs(ps, f0, vocab)
	target := make([]float32, frames*80)
	energyTarget := make([]float32, frames)
	for i := range target {
		target[i] = float32(math.NaN())
	}
	for i := range energyTarget {
		energyTarget[i] = float32(math.NaN())
	}
	f0Target := make([]float32, frames)
	for t := 0; t < frames && t < len(teacher); t++ {
		f0Target[t] = teacher[t]
	}
	extra := make([]float32, frames*f0ExtraFeatures)
	for _, token := range rec.Tokens {
		if token.EndMS <= token.StartMS {
			continue
		}
		feature := f0ExtraFeatureFrame(token)
		a := int(math.Round(token.StartMS / 10))
		b := int(math.Round(token.EndMS / 10))
		a, b = max(0, a), min(frames, max(b, a+1))
		for t := a; t < b; t++ {
			copy(extra[t*f0ExtraFeatures:(t+1)*f0ExtraFeatures], feature[:])
		}
	}
	return utterance{rec.ID, frames, 4, ids, cont, target, f0Target, energyTarget, extra, nil}, nil
}

// planPhonesはトークン（プラン時間）から音素区間を組む。子音は先頭40%（最大50ms）。
func planPhones(tokens []datasetToken) []phone {
	var result []phone
	for _, token := range tokens {
		if token.Pause || token.EndMS <= token.StartMS {
			continue
		}
		start := token.StartMS / 1000
		end := token.EndMS / 1000
		vowel := token.Vowel
		consonant := ""
		if parsed, err := frontend.ParseKana(token.Mora); err == nil && len(parsed) > 0 {
			consonant = parsed[0].Consonant
			if vowel == "" {
				vowel = parsed[0].Vowel
			}
		}
		switch token.Mora {
		case "っ", "ッ":
			consonant, vowel = "", "cl"
		case "ん", "ン":
			consonant, vowel = "", "N"
		case "ー":
			consonant = ""
		}
		if vowel == "" {
			vowel = "a"
		}
		if consonant != "" {
			length := end - start
			consonantEnd := start + math.Min(0.05, length*0.4)
			result = append(result, phone{start, consonantEnd, consonant, consonant})
			result = append(result, phone{consonantEnd, end, vowel, vowel})
		} else {
			result = append(result, phone{start, end, vowel, vowel})
		}
	}
	return result
}

// featureCacheは特徴量と語彙を保存する。旧形式（[]utterance、日本語語彙）も読める。
type featureCache struct {
	Version int
	Phones  string
	Data    []utterance
}

func loadOrBuildFeatures(cache, dataset, corpusPath, alignDir, engine, teacherPath string) ([]utterance, string, error) {
	if f, e := os.Open(cache); e == nil {
		defer f.Close()
		var cached featureCache
		decodeErr := gob.NewDecoder(f).Decode(&cached)
		if decodeErr != nil {
			if _, e := f.Seek(0, io.SeekStart); e != nil {
				return nil, "", e
			}
			var legacy []utterance
			if e := gob.NewDecoder(f).Decode(&legacy); e != nil {
				return nil, "", fmt.Errorf("cache %s: %w", cache, decodeErr)
			}
			cached = featureCache{Phones: phoneNames, Data: legacy}
		}
		if cached.Version != featureVersion {
			return nil, "", fmt.Errorf("feature cache %s is version %d, want %d; delete it to rebuild", cache, cached.Version, featureVersion)
		}
		if cached.Phones == "" {
			cached.Phones = phoneNames
		}
		return cached.Data, cached.Phones, nil
	}
	vocab := jaVocabulary()
	if corpusPath != "" {
		var e error
		if vocab, e = corpusVocabulary(corpusPath); e != nil {
			return nil, "", e
		}
	}
	teachers, e := loadF0Teacher(teacherPath)
	if e != nil {
		return nil, "", e
	}
	var records []trainingRecord
	if corpusPath != "" {
		err := toolutil.ScanJSONL(corpusPath, func(line []byte) error {
			var r corpusRecord
			if e := json.Unmarshal(line, &r); e != nil {
				return e
			}
			if r.ID == "" || r.AudioPath == "" {
				return nil
			}
			records = append(records, trainingRecord{ID: r.ID, AudioPath: r.AudioPath, Phones: phonesFromTokens(r.Tokens)})
			return nil
		})
		if err != nil {
			return nil, "", err
		}
	} else {
		err := toolutil.ScanJSONL(dataset, func(line []byte) error {
			var r record
			if e := json.Unmarshal(line, &r); e != nil {
				return e
			}
			if r.Version != 1 {
				return fmt.Errorf("%s: version %d", r.ID, r.Version)
			}
			if !r.PlanTiming {
				if _, ok := alignmentPath(alignDir, r.ID); !ok {
					return nil
				}
			}
			records = append(records, trainingRecord{ID: r.ID, AudioPath: r.AudioPath, Tokens: r.Tokens, PlanTiming: r.PlanTiming})
			return nil
		})
		if err != nil {
			return nil, "", err
		}
	}
	type result struct {
		index int
		item  utterance
		err   error
	}
	jobs := make(chan int)
	results := make(chan result, len(records))
	var wg sync.WaitGroup
	workers := min(8, runtime.GOMAXPROCS(0))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			world, err := openWorld(engine)
			if err == nil {
				defer world.close()
			}
			for index := range jobs {
				if err != nil {
					results <- result{index: index, err: err}
					continue
				}
				item, e := featurize(world, records[index], alignDir, vocab, teachers[records[index].ID])
				results <- result{index, item, e}
			}
		}()
	}
	go func() {
		for i := range records {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	data := make([]utterance, len(records))
	var firstErr error
	done := 0
	for r := range results {
		if r.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", records[r.index].ID, r.err)
		}
		data[r.index] = r.item
		done++
		if done%100 == 0 {
			fmt.Printf("features %d/%d\n", done, len(records))
		}
	}
	if firstErr != nil {
		return nil, "", firstErr
	}
	if e := os.MkdirAll(filepath.Dir(cache), 0755); e != nil {
		return nil, "", e
	}
	f, e := toolutil.CreateExclusive(cache)
	if e != nil {
		return nil, "", e
	}
	e = gob.NewEncoder(f).Encode(featureCache{Version: featureVersion, Phones: vocab.String(), Data: data})
	closeErr := f.Close()
	if e != nil {
		return nil, "", e
	}
	return data, vocab.String(), closeErr
}
