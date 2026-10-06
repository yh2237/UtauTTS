package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
)

type utterance struct {
	ID                 string
	Frames, Continuous int
	IDs                []int
	Cont, Target       []float32
	F0Target           []float32
	EnergyTarget       []float32
}

// featureVersionはキャッシュ形式の版。utteranceの目標を変えたら上げる。
const featureVersion = 2

type record struct {
	ID        string `json:"id"`
	AudioPath string `json:"audio_path"`
	Version   int    `json:"version"`
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
	ID        string
	AudioPath string
	Phones    []phone
}

func featurize(world *worldEngine, rec trainingRecord, alignDir string, vocab *vocabulary) (utterance, error) {
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
		ps, e = readPhones(filepath.Join(alignDir, rec.ID+".json"))
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
	// F0目標は入力と同じ補間済みの相対log F0（発話中央値基準、/0.3）。
	f0Target := make([]float32, len(f0))
	for t := range f0 {
		f0Target[t] = cont[t*4+2]
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
	return utterance{rec.ID, len(f0), 4, ids, cont, target, f0Target, energyTarget}, nil
}

// featureCacheは特徴量と語彙を保存する。旧形式（[]utterance、日本語語彙）も読める。
type featureCache struct {
	Version int
	Phones  string
	Data    []utterance
}

func loadOrBuildFeatures(cache, dataset, corpusPath, alignDir, engine string) ([]utterance, string, error) {
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
			if _, e := os.Stat(filepath.Join(alignDir, r.ID+".json")); e != nil {
				return nil
			}
			records = append(records, trainingRecord{ID: r.ID, AudioPath: r.AudioPath})
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
				item, e := featurize(world, records[index], alignDir, vocab)
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
