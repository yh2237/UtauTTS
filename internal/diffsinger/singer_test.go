package diffsinger

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"utautts/internal/audio"
)

func TestMain(m *testing.M) {
	if os.Getenv("UTAUTTS_DIFFSINGER_PROVIDER_HELPER") == "1" {
		runDiffSingerProviderHelper()
		os.Exit(0)
	}
	if os.Getenv("UTAUTTS_DIFFSINGER_TEST_BRIDGE") == "1" {
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			os.Exit(2)
		}
		var request Request
		if json.Unmarshal(data, &request) != nil {
			os.Exit(2)
		}
		if audio.WriteWav(request.OutputPath, &audio.PCM{SampleRate: request.SampleRate, Channels: 1, Data: []int16{1, -1}}) != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestLoadOpenUtauStyleSinger(t *testing.T) {
	root := makeSinger(t)
	singer, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if singer.Tokens["SP"] != 0 || singer.Tokens["a"] != 2 {
		t.Fatalf("tokens = %#v", singer.Tokens)
	}
	if singer.FrameMS() != 10 {
		t.Fatalf("frame ms = %v", singer.FrameMS())
	}
}

func TestLoadJapaneseDictionary(t *testing.T) {
	root := makeSinger(t)
	mustWrite(t, filepath.Join(root, "dsdur", "dsdict-ja.yaml"), "entries:\n- {grapheme: きょ, phonemes: [k, a]}\n")
	singer, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(singer.JapaneseDictionary["きょ"], []string{"k", "a"}) {
		t.Fatalf("dictionary = %#v", singer.JapaneseDictionary)
	}
}

func TestLoadSingerWithSharedCoreAndSpeaker(t *testing.T) {
	bundle := t.TempDir()
	root := filepath.Join(bundle, "Singer")
	core := filepath.Join(bundle, "Core")
	mustWrite(t, filepath.Join(root, "dsconfig.yaml"), "acoustic: ../Core/acoustic.onnx\nphonemes: ../Core/phonemes.json\nlanguages: ../Core/languages.json\nvocoder: dsvocoder\nspeakers: [dsacoustic/voice]\nhidden_size: 2\nuse_lang_id: true\n")
	mustWrite(t, filepath.Join(core, "acoustic.onnx"), "model")
	mustWrite(t, filepath.Join(core, "phonemes.json"), `{"SP":0,"ja/a":1}`)
	mustWrite(t, filepath.Join(core, "languages.json"), `{"ja":7}`)
	mustWrite(t, filepath.Join(root, "dsvocoder", "vocoder.yaml"), "model: ../../Core/vocoder.onnx\n")
	mustWrite(t, filepath.Join(core, "vocoder.onnx"), "model")
	mustFloat32(t, filepath.Join(root, "dsacoustic", "voice.emb"), []float32{1.25, -2.5})
	mustWrite(t, filepath.Join(root, "dsdur", "dsconfig.yaml"), "linguistic: ../../Core/linguistic.onnx\ndur: ../../Core/dur.onnx\nphonemes: ../../Core/phonemes.json\nlanguages: ../../Core/languages.json\nspeakers: [embs/voice]\nhidden_size: 2\nuse_lang_id: true\n")
	mustWrite(t, filepath.Join(core, "linguistic.onnx"), "model")
	mustWrite(t, filepath.Join(core, "dur.onnx"), "model")
	mustFloat32(t, filepath.Join(root, "dsdur", "embs", "voice.emb"), []float32{3.5, -4.5})
	mustWrite(t, filepath.Join(root, "dspitch", "dsconfig.yaml"), "linguistic: ../../Core/pitch-linguistic.onnx\npitch: ../../Core/pitch.onnx\nphonemes: ../../Core/phonemes.json\nlanguages: ../../Core/languages.json\nspeakers: [embs/voice]\nhidden_size: 2\nuse_lang_id: true\n")
	mustWrite(t, filepath.Join(core, "pitch-linguistic.onnx"), "model")
	mustWrite(t, filepath.Join(core, "pitch.onnx"), "model")
	mustFloat32(t, filepath.Join(root, "dspitch", "embs", "voice.emb"), []float32{5.5, -6.5})
	mustWrite(t, filepath.Join(root, "dsvariance", "dsconfig.yaml"), "linguistic: ../../Core/variance-linguistic.onnx\nvariance: ../../Core/variance.onnx\nphonemes: ../../Core/phonemes.json\nlanguages: ../../Core/languages.json\nspeakers: [embs/voice]\nhidden_size: 2\nuse_lang_id: true\npredict_breathiness: true\n")
	mustWrite(t, filepath.Join(core, "variance-linguistic.onnx"), "model")
	mustWrite(t, filepath.Join(core, "variance.onnx"), "model")
	mustFloat32(t, filepath.Join(root, "dsvariance", "embs", "voice.emb"), []float32{7.5, -8.5})

	singer, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(singer.SpeakerEmbed, []float32{1.25, -2.5}) || singer.LanguageIDs["ja"] != 7 {
		t.Fatalf("singer = %#v", singer)
	}
	if singer.Duration == nil || !reflect.DeepEqual(singer.Duration.SpeakerEmbed, []float32{3.5, -4.5}) || singer.Duration.LanguageIDs["ja"] != 7 {
		t.Fatalf("duration = %#v", singer.Duration)
	}
	if singer.Pitch == nil || !reflect.DeepEqual(singer.Pitch.SpeakerEmbed, []float32{5.5, -6.5}) || singer.Pitch.LanguageIDs["ja"] != 7 {
		t.Fatalf("pitch = %#v", singer.Pitch)
	}
	if singer.Variance == nil || !reflect.DeepEqual(singer.Variance.SpeakerEmbed, []float32{7.5, -8.5}) || singer.Variance.LanguageIDs["ja"] != 7 || !singer.Variance.Config.PredictBreathiness {
		t.Fatalf("variance = %#v", singer.Variance)
	}
}

func TestLoadSingerUsesNamedDependency(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Singer")
	mustWrite(t, filepath.Join(root, "dsconfig.yaml"), "acoustic: acoustic.onnx\nphonemes: phonemes.txt\nvocoder: nsf_hifigan\n")
	mustWrite(t, filepath.Join(root, "acoustic.onnx"), "model")
	mustWrite(t, filepath.Join(root, "phonemes.txt"), "SP\na\n")
	mustWrite(t, filepath.Join(base, "Dependencies", "nsf_hifigan", "vocoder.yaml"), "model: model.onnx\n")
	mustWrite(t, filepath.Join(base, "Dependencies", "nsf_hifigan", "model.onnx"), "model")

	singer, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(singer.VocoderPath)) != "nsf_hifigan" {
		t.Fatalf("vocoder = %q", singer.VocoderPath)
	}
}

func TestLoadSingerUsesExactDependencyName(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Singer")
	mustWrite(t, filepath.Join(root, "dsconfig.yaml"), "acoustic: acoustic.onnx\nphonemes: phonemes.txt\nvocoder: pc_nsf_hifigan_44.1k_hop512_128bin_2025.02\n")
	mustWrite(t, filepath.Join(root, "acoustic.onnx"), "model")
	mustWrite(t, filepath.Join(root, "phonemes.txt"), "SP\na\n")
	mustWrite(t, filepath.Join(base, "Dependencies", "nsf_hifigan", "vocoder.yaml"), "model: model.onnx\n")
	mustWrite(t, filepath.Join(base, "Dependencies", "nsf_hifigan", "model.onnx"), "model")

	if _, err := Load(root); err == nil {
		t.Fatal("別名のvocoderを使用した")
	}
}

func TestLoadTokensAcceptsTrailingComma(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phonemes.json")
	mustWrite(t, path, "{\"SP\": 0, \"a\": 1,}\n")
	tokens, err := loadTokens(path)
	if err != nil || tokens["a"] != 1 {
		t.Fatalf("tokens = %#v, err = %v", tokens, err)
	}
}

func TestRenderUsesManifestBridge(t *testing.T) {
	t.Setenv("UTAUTTS_DIFFSINGER_TEST_BRIDGE", "1")
	pcm, err := Render(nil, os.Args[0], Request{
		Tokens: []int64{0}, Durations: []int64{2}, F0: []float32{261, 261}, SampleRate: 44100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pcm.SampleRate != 44100 || len(pcm.Data) != 2 {
		t.Fatalf("pcm = %#v", pcm)
	}
}

func makeSinger(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "dsconfig.yaml"), "acoustic: acoustic.onnx\nphonemes: phonemes.txt\nsample_rate: 44100\nhop_size: 441\nnum_mel_bins: 128\n")
	mustWrite(t, filepath.Join(root, "acoustic.onnx"), "model")
	mustWrite(t, filepath.Join(root, "phonemes.txt"), "SP\n\na\nk\n")
	mustWrite(t, filepath.Join(root, "dsvocoder", "vocoder.yaml"), "model: model.onnx\nsample_rate: 44100\nhop_size: 441\nnum_mel_bins: 128\n")
	mustWrite(t, filepath.Join(root, "dsvocoder", "model.onnx"), "model")
	return root
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustFloat32(t *testing.T, path string, values []float32) {
	t.Helper()
	data := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(data[index*4:], math.Float32bits(value))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
