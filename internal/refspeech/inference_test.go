package refspeech

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func externalTestEngine(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	data := `{"vocab":{"a":1,"b":2," ":3},"hidden_dim":512,"style_dim":128,"max_dur":50,"n_layer":3,"text_encoder_kernel_size":5,"plbert":{"max_position_embeddings":512}}`
	if err := os.WriteFile(config, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, err := openExternal(Models{KokoroWeights: filepath.Join(dir, "kokoro-v1_0.pth"), KokoroConfig: config,
		KokoroVoice: filepath.Join(dir, "af_heart.pt"), SharedVoices: map[string]string{kokoroSpanish: filepath.Join(dir, "ef_dora.pt")}})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestExternalReadAsksThenUsesProvidedResult(t *testing.T) {
	engine := externalTestEngine(t)
	pieces := []Piece{{Symbols: "ab"}, {Symbols: " a"}}
	_, err := engine.ReadProsody(kokoroSpanish, pieces, 1, nil)
	var need *NeedInference
	if !errors.As(err, &need) || len(need.Jobs) != 1 {
		t.Fatalf("err=%v", err)
	}
	job := need.Jobs[0]
	if job.Kind != "kokoro" || job.Model != "kokoro-v1_0" || job.Voice != "ef_dora" || job.Style != 3 || job.Audio ||
		!reflect.DeepEqual(job.Tokens, []int{0, 1, 2, 3, 1, 0}) {
		t.Fatalf("job=%+v", job)
	}
	durations := []int{1, 2, 3, 2, 2, 1}
	if err := engine.ProvideRead(job.Key, durations, []float32{100, 110}, nil); err != nil {
		t.Fatal(err)
	}
	prosody, err := engine.ReadProsody(kokoroSpanish, pieces, 1, nil)
	if err != nil || !reflect.DeepEqual(prosody.Durations, durations) || !reflect.DeepEqual(prosody.PieceTokens, [][2]int{{1, 3}, {3, 5}}) {
		t.Fatalf("prosody=%+v err=%v", prosody, err)
	}
	// 波形が要るときは、同じ長さで読むよう頼む。
	_, err = engine.Read(kokoroSpanish, pieces, 1, nil)
	if !errors.As(err, &need) || !need.Jobs[0].Audio || !reflect.DeepEqual(need.Jobs[0].Durations, durations) || need.Jobs[0].Key != job.Key {
		t.Fatalf("err=%v", err)
	}
	if err := engine.ProvideRead(job.Key, durations, []float32{100, 110}, []float32{0.1, 0.2}); err != nil {
		t.Fatal(err)
	}
	reference, err := engine.Read(kokoroSpanish, pieces, 1, nil)
	if err != nil || !reflect.DeepEqual(reference.Audio, []float32{0.1, 0.2}) {
		t.Fatalf("reference=%+v err=%v", reference, err)
	}
}

func TestExternalFeaturesAskCollectAndProvide(t *testing.T) {
	engine := externalTestEngine(t)
	samples := make([]float64, 16000)
	for i := range samples {
		samples[i] = float64(i%50) / 50
	}
	_, _, _, err := engine.Features(samples, 16000)
	var need *NeedInference
	if !errors.As(err, &need) || need.Jobs[0].Kind != "features" || len(need.Jobs[0].Samples) != 16000 {
		t.Fatalf("err=%v", err)
	}
	jobs := engine.collectFeatures(func() {
		values, frames, dims, err := engine.Features(samples, 16000)
		if err != nil || frames != 49 || dims != wavlmDims || len(values) != frames*dims {
			t.Errorf("collect: frames=%d dims=%d err=%v", frames, dims, err)
		}
	})
	if len(jobs) != 1 || jobs[0].Key != need.Jobs[0].Key {
		t.Fatalf("jobs=%v", jobs)
	}
	values := make([]float32, 49*wavlmDims)
	values[0] = 1
	if err := engine.ProvideFeatures(jobs[0].Key, values, 49, wavlmDims); err != nil {
		t.Fatal(err)
	}
	got, frames, _, err := engine.Features(samples, 16000)
	if err != nil || frames != 49 || got[0] != 1 {
		t.Fatalf("frames=%d err=%v", frames, err)
	}
}

// 波形を後から渡したときも、待ち合わせの記録を残さない。
func TestExternalProvideReadClearsPendingAfterAudioUpgrade(t *testing.T) {
	engine := externalTestEngine(t)
	pieces := []Piece{{Symbols: "ab"}}
	_, err := engine.ReadProsody(kokoroSpanish, pieces, 1, nil)
	var need *NeedInference
	if !errors.As(err, &need) || len(need.Jobs) != 1 {
		t.Fatalf("err=%v", err)
	}
	job := need.Jobs[0]
	durations := []int{1, 2, 3, 2}
	if err := engine.ProvideRead(job.Key, durations, []float32{100, 110}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Read(kokoroSpanish, pieces, 1, nil); !errors.As(err, &need) {
		t.Fatalf("expected an audio request, got %v", err)
	}
	if err := engine.ProvideRead(job.Key, durations, []float32{100, 110}, []float32{0.1, 0.2}); err != nil {
		t.Fatal(err)
	}
	engine.readsMu.Lock()
	pending := len(engine.pending)
	engine.readsMu.Unlock()
	if pending != 0 {
		t.Fatalf("pending reads = %d, want 0", pending)
	}
}

// 桁あふれする形の特徴量は受け取らない。
func TestProvideFeaturesRejectsOverflowingShape(t *testing.T) {
	engine := externalTestEngine(t)
	if err := engine.ProvideFeatures("key", nil, 1<<32, 1<<32); err == nil {
		t.Fatal("accepted an overflowing feature shape")
	}
}
