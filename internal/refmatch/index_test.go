package refmatch

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/audio"
)

func TestIndexSaveLoad(t *testing.T) {
	index := &Index{
		Header: Header{Version: indexVersion, Model: "test", SampleRate: 44100, FFTSize: 2048, APBands: 2, Dims: 3,
			Frames: 2, F0Median: 220, Files: []string{"a.wav"}},
		Features:     []float32{0.6, 0.8, 0, -1, 0, 0},
		Spectrum:     make([]float32, 2*subFrames*spectrumDims),
		Aperiodicity: make([]float32, 2*subFrames*2),
		F0:           []float32{0, 100, 200, 300, 400, 500, 600, 700},
		File:         []int32{0, 0},
	}
	for i := range index.Spectrum {
		index.Spectrum[i] = float32(i)
	}
	path := filepath.Join(t.TempDir(), "index.bin")
	if err := index.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Header, index.Header) || !reflect.DeepEqual(loaded.Spectrum, index.Spectrum) ||
		!reflect.DeepEqual(loaded.F0, index.F0) || !reflect.DeepEqual(loaded.File, index.File) {
		t.Fatal("index changed after save and load")
	}
	for i, v := range loaded.Features {
		// 特徴量はint8で保存するため、フレームごとのスケール分の誤差が出る。
		if math.Abs(float64(v-index.Features[i])) > 1e-2 {
			t.Fatalf("feature %d = %g, want %g", i, v, index.Features[i])
		}
	}
}

func TestHalfFloat(t *testing.T) {
	for _, v := range []float32{0, 1, -1, 0.5, 65504, 1e-5, -3.14159, 1e-8} {
		got := halfToFloat32(float32ToHalf(v))
		if math.Abs(float64(got-v)) > math.Max(1e-3*math.Abs(float64(v)), 6e-8) {
			t.Errorf("half(%g) = %g", v, got)
		}
	}
}

type fakeWorld struct{}

func (fakeWorld) Analyze(samples []float64, rate int, frameMS, threshold float64) ([]float64, []float64, []float64, int, error) {
	const fft = 1024
	frames := int(1000*float64(len(samples))/float64(rate)/frameMS) + 1
	f0 := make([]float64, frames)
	spectrum := make([]float64, frames*(fft/2+1))
	aperiodicity := make([]float64, frames*(fft/2+1))
	for i := range spectrum {
		spectrum[i], aperiodicity[i] = 1+float64(i%7), 0.1
	}
	return f0, spectrum, aperiodicity, fft, nil
}

func (fakeWorld) Synthesize([]float64, []float64, []float64, int, int, float64) ([]float64, error) {
	return nil, nil
}

func TestBuildIndexSkipsUnreadableWAV(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i := range pcm.Data {
		pcm.Data[i] = int16(8000 * math.Sin(float64(i)*0.1))
	}
	if err := audio.WriteWav(good, pcm); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.wav")
	if err := os.WriteFile(bad, []byte("RIFF\x00\x00\x00\x00WAVEnot a pcm file"), 0o644); err != nil {
		t.Fatal(err)
	}
	features := func(samples []float64, rate int) ([]float32, int, int, error) {
		frames := len(samples) * 50 / rate
		values := make([]float32, frames*3)
		for i := range values {
			values[i] = float32(i%3 + 1)
		}
		return values, frames, 3, nil
	}
	index, err := BuildIndex([]string{bad, good}, "test", fakeWorld{}, features, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index.Files, []string{good}) || index.Frames == 0 {
		t.Fatalf("files %v, frames %d", index.Files, index.Frames)
	}
}

func TestBuildIndexReads24BitWAV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "voice24.wav")
	if err := write24BitWav(path, 16000, 1, 16000); err != nil {
		t.Fatal(err)
	}
	features := func(samples []float64, rate int) ([]float32, int, int, error) {
		frames := len(samples) * 50 / rate
		return make([]float32, frames*3), frames, 3, nil
	}
	index, err := BuildIndex([]string{path}, "test", fakeWorld{}, features, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index.Files, []string{path}) || index.SampleRate != 16000 || index.Frames == 0 {
		t.Fatalf("files %v, rate %d, frames %d", index.Files, index.SampleRate, index.Frames)
	}
}

// write24BitWavは、24bit PCMのWAVを書き出す。8/24/32bitの録音を読めるかの確認に使う。
func write24BitWav(path string, rate, channels, frames int) error {
	data := make([]byte, 0, frames*channels*3)
	for i := 0; i < frames*channels; i++ {
		value := int32(8000) << 8
		data = append(data, byte(value), byte(value>>8), byte(value>>16))
	}
	var buffer bytes.Buffer
	buffer.WriteString("RIFF")
	binary.Write(&buffer, binary.LittleEndian, uint32(0))
	buffer.WriteString("WAVE")
	buffer.WriteString("fmt ")
	binary.Write(&buffer, binary.LittleEndian, uint32(16))
	binary.Write(&buffer, binary.LittleEndian, uint16(1))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels))
	binary.Write(&buffer, binary.LittleEndian, uint32(rate))
	binary.Write(&buffer, binary.LittleEndian, uint32(rate*channels*3))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels*3))
	binary.Write(&buffer, binary.LittleEndian, uint16(24))
	buffer.WriteString("data")
	binary.Write(&buffer, binary.LittleEndian, uint32(len(data)))
	buffer.Write(data)
	out := buffer.Bytes()
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return os.WriteFile(path, out, 0o644)
}

func TestBuildIndexReusesUnchangedRecordings(t *testing.T) {
	dir := t.TempDir()
	makeWav := func(name string, frequency float64) string {
		path := filepath.Join(dir, name)
		pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
		for i := range pcm.Data {
			pcm.Data[i] = int16(8000 * math.Sin(float64(i)*frequency))
		}
		if err := audio.WriteWav(path, pcm); err != nil {
			t.Fatal(err)
		}
		return path
	}
	files := []string{makeWav("a.wav", 0.1), makeWav("b.wav", 0.2)}
	cache := OpenPartCache(filepath.Join(t.TempDir(), "parts"))
	calls := 0
	features := func(samples []float64, rate int) ([]float32, int, int, error) {
		calls++
		frames := len(samples) * 50 / rate
		values := make([]float32, frames*3)
		for i := range values {
			values[i] = float32(i%3 + 1)
		}
		return values, frames, 3, nil
	}
	first, err := BuildIndex(files, "test", fakeWorld{}, features, nil, cache)
	if err != nil {
		t.Fatal(err)
	}
	if calls != len(files) {
		t.Fatalf("first build analyzed %d files", calls)
	}
	calls = 0
	second, err := BuildIndex(files, "test", fakeWorld{}, features, nil, cache)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("second build analyzed %d files, want 0", calls)
	}
	if first.Frames != second.Frames || !reflect.DeepEqual(first.Features, second.Features) ||
		!reflect.DeepEqual(first.Spectrum, second.Spectrum) || !reflect.DeepEqual(first.Aperiodicity, second.Aperiodicity) ||
		!reflect.DeepEqual(first.F0, second.F0) || !reflect.DeepEqual(first.File, second.File) {
		t.Fatal("index built from cached parts differs")
	}
	if err := audio.WriteWav(files[0], &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 9000)}); err != nil {
		t.Fatal(err)
	}
	calls = 0
	if _, err := BuildIndex(files, "test", fakeWorld{}, features, nil, cache); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("build after one change analyzed %d files, want 1", calls)
	}
}

func TestBuildIndexTakesRateFromUsableWAV(t *testing.T) {
	dir := t.TempDir()
	short := filepath.Join(dir, "short.wav")
	if err := audio.WriteWav(short, &audio.PCM{SampleRate: 44100, Channels: 1, Data: make([]int16, 441)}); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "good.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i := range pcm.Data {
		pcm.Data[i] = int16(8000 * math.Sin(float64(i)*0.1))
	}
	if err := audio.WriteWav(good, pcm); err != nil {
		t.Fatal(err)
	}
	features := func(samples []float64, rate int) ([]float32, int, int, error) {
		frames := len(samples) * 50 / rate
		return make([]float32, frames*3), frames, 3, nil
	}
	index, err := BuildIndex([]string{short, good}, "test", fakeWorld{}, features, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if index.SampleRate != 16000 || !reflect.DeepEqual(index.Files, []string{good}) {
		t.Fatalf("rate %d, files %v", index.SampleRate, index.Files)
	}
}
