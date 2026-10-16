package refmatch

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/yh2237/UtauTTS/internal/atomicfile"
	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/gograd/tensor"
)

const (
	// FrameMSは特徴量1フレームの長さ。WORLDはその1/subFramesの周期で解析する。
	FrameMS        = 20.0
	subFrames      = 4
	worldFrameMS   = FrameMS / subFrames
	spectrumDims   = 60
	d4cThreshold   = 0.85
	quietMarginC0  = 9.0
	indexMagic     = "UTRM"
	indexVersion   = 1
	minimumFileSec = 0.1
)

// WorldはWORLDの解析と合成。worldrender.Worldが満たす。
type World interface {
	Analyze(samples []float64, sampleRate int, frameMS, d4cThreshold float64) (f0, spectrum, aperiodicity []float64, fftSize int, err error)
	Synthesize(f0, spectrum, aperiodicity []float64, fftSize, sampleRate int, frameMS float64) ([]float64, error)
}

// FeatureFuncは波形から20msごとの特徴量（frames×dims）を返す。複数のgoroutineから同時に呼ぶ。
type FeatureFunc func(samples []float64, sampleRate int) (values []float32, frames, dims int, err error)

// Indexは音源の録音から選べるフレームの一覧。
type Index struct {
	Header
	// Featuresは長さ1に正規化した特徴量（Frames×Dims）。
	Features []float32
	// SpectrumはWORLDの包絡の符号（Frames×subFrames×spectrumDims）。
	Spectrum []float32
	// AperiodicityはWORLDの非周期性の符号（Frames×subFrames×APBands）。
	Aperiodicity []float32
	F0           []float32
	File         []int32

	packOnce sync.Once
	packed   *tensor.PackedB
}

type Header struct {
	Version    int      `json:"version"`
	Model      string   `json:"model"`
	SampleRate int      `json:"sample_rate"`
	FFTSize    int      `json:"fft_size"`
	APBands    int      `json:"ap_bands"`
	Dims       int      `json:"dims"`
	Frames     int      `json:"frames"`
	F0Median   float64  `json:"f0_median"`
	Files      []string `json:"files"`
	Source     string   `json:"source"`
}

// BuildIndexは録音ファイルを解析して索引を作る。最初のファイルと標本化周波数が違うファイルと0.1秒未満のファイルは使わない。
// WORLDの解析はファイルごとに並列に行い、featuresは同時に1つだけ呼ぶ。
func BuildIndex(files []string, model string, world World, features FeatureFunc, progress func(done, total int)) (*Index, error) {
	results := make([]fileFrames, len(files))
	jobs := make(chan int)
	var mu sync.Mutex
	var firstErr error
	done := 0
	var wg sync.WaitGroup
	for range max(1, min(runtime.GOMAXPROCS(0), len(files))) {
		wg.Go(func() {
			for number := range jobs {
				result, err := analyzeFile(files[number], world, features)
				mu.Lock()
				results[number] = result
				if err != nil && firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", files[number], err)
				}
				done++
				if progress != nil {
					progress(done, len(files))
				}
				mu.Unlock()
			}
		})
	}
	for number := range files {
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed {
			break
		}
		jobs <- number
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	index := &Index{Header: Header{Version: indexVersion, Model: model}}
	var voicedF0 []float64
	for number, result := range results {
		if index.SampleRate == 0 {
			index.SampleRate = result.rate
		}
		if result.rate != index.SampleRate || !result.usable {
			continue
		}
		if index.FFTSize == 0 {
			index.FFTSize, index.Dims, index.APBands = result.fftSize, result.dims, result.apBands
		}
		fileID := int32(len(index.Files))
		index.Features = append(index.Features, result.features...)
		index.Spectrum = append(index.Spectrum, result.spectrum...)
		index.Aperiodicity = append(index.Aperiodicity, result.aperiodicity...)
		index.F0 = append(index.F0, result.f0...)
		for range result.frames {
			index.File = append(index.File, fileID)
		}
		index.Frames += result.frames
		for _, v := range result.f0 {
			if v > 0 {
				voicedF0 = append(voicedF0, float64(v))
			}
		}
		index.Files = append(index.Files, files[number])
	}
	if index.Frames == 0 {
		return nil, errors.New("no usable frames in the voicebank recordings")
	}
	index.F0Median = percentile(voicedF0, 50)
	return index, nil
}

type fileFrames struct {
	rate, fftSize, dims, apBands, frames int
	usable                               bool
	features, spectrum, aperiodicity, f0 []float32
}

// analyzeFileは、95パーセンタイルの音量から約45dB以上小さいフレームを除いて、1ファイル分のフレームを返す。
func analyzeFile(path string, world World, features FeatureFunc) (fileFrames, error) {
	samples, rate, err := readMono(path)
	if err != nil {
		return fileFrames{}, err
	}
	result := fileFrames{rate: rate}
	if float64(len(samples)) < float64(rate)*minimumFileSec {
		return result, nil
	}
	values, frames, dims, err := features(samples, rate)
	if err != nil {
		return result, err
	}
	f0, spectrum, aperiodicity, fftSize, err := world.Analyze(samples, rate, worldFrameMS, d4cThreshold)
	if err != nil {
		return result, err
	}
	c := newCodec(rate, fftSize, spectrumDims)
	result.usable, result.fftSize, result.dims, result.apBands = true, fftSize, dims, c.apBands
	bins := fftSize/2 + 1
	usable := min(frames, len(f0)/subFrames)
	coded := make([]float64, usable*subFrames*spectrumDims)
	for i := range usable * subFrames {
		c.codeSpectrum(spectrum[i*bins:(i+1)*bins], coded[i*spectrumDims:(i+1)*spectrumDims])
	}
	loud := percentile(levels(coded), 95)
	codedAP := make([]float64, c.apBands)
	for frame := range usable {
		quiet := true
		for sub := range subFrames {
			if coded[(frame*subFrames+sub)*spectrumDims] >= loud-quietMarginC0 {
				quiet = false
			}
		}
		if quiet {
			continue
		}
		result.features = append(result.features, normalized(values[frame*dims:(frame+1)*dims])...)
		for sub := range subFrames {
			row := frame*subFrames + sub
			for _, v := range coded[row*spectrumDims : (row+1)*spectrumDims] {
				result.spectrum = append(result.spectrum, float32(v))
			}
			c.codeAperiodicity(aperiodicity[row*bins:(row+1)*bins], codedAP)
			for _, v := range codedAP {
				result.aperiodicity = append(result.aperiodicity, float32(v))
			}
			result.f0 = append(result.f0, float32(f0[row]))
		}
		result.frames++
	}
	return result, nil
}

func readMono(path string) ([]float64, int, error) {
	pcm, err := audio.ReadWav(path)
	if err != nil {
		return nil, 0, err
	}
	samples := make([]float64, len(pcm.Data)/pcm.Channels)
	for i := range samples {
		for c := range pcm.Channels {
			samples[i] += float64(pcm.Data[i*pcm.Channels+c]) / 32768
		}
		samples[i] /= float64(pcm.Channels)
	}
	return samples, pcm.SampleRate, nil
}

func levels(coded []float64) []float64 {
	out := make([]float64, len(coded)/spectrumDims)
	for i := range out {
		out[i] = coded[i*spectrumDims]
	}
	return out
}

// percentileはnumpy.percentileと同じ線形補間で値を返す。
func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	position := q / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := min(lower+1, len(sorted)-1)
	return sorted[lower] + (sorted[upper]-sorted[lower])*(position-float64(lower))
}

func normalized(values []float32) []float32 {
	var sum float64
	for _, v := range values {
		sum += float64(v) * float64(v)
	}
	scale := float32(1 / math.Max(math.Sqrt(sum), 1e-12))
	out := make([]float32, len(values))
	for i, v := range values {
		out[i] = v * scale
	}
	return out
}

// Saveは索引を書き出す。特徴量は半精度で保存する。
func (index *Index) Save(path string) error {
	header, err := json.Marshal(index.Header)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, func(w io.Writer) error {
		buffered := bufio.NewWriter(w)
		buffered.WriteString(indexMagic)
		binary.Write(buffered, binary.LittleEndian, uint32(len(header)))
		buffered.Write(header)
		half := make([]uint16, len(index.Features))
		for i, v := range index.Features {
			half[i] = float32ToHalf(v)
		}
		for _, data := range []any{half, index.Spectrum, index.Aperiodicity, index.F0, index.File} {
			if err := binary.Write(buffered, binary.LittleEndian, data); err != nil {
				return err
			}
		}
		return buffered.Flush()
	})
}

func LoadIndex(path string) (*Index, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	magic := make([]byte, len(indexMagic))
	if _, err := io.ReadFull(reader, magic); err != nil || string(magic) != indexMagic {
		return nil, fmt.Errorf("%s is not a reference matching index", path)
	}
	var length uint32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil || length > 1<<26 {
		return nil, fmt.Errorf("%s has an invalid header", path)
	}
	header := make([]byte, length)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	index := &Index{}
	if err := json.Unmarshal(header, &index.Header); err != nil {
		return nil, err
	}
	if index.Version != indexVersion || index.Frames <= 0 || index.Dims <= 0 || index.APBands <= 0 {
		return nil, fmt.Errorf("%s has an unsupported index version", path)
	}
	n := index.Frames
	half := make([]uint16, n*index.Dims)
	index.Spectrum = make([]float32, n*subFrames*spectrumDims)
	index.Aperiodicity = make([]float32, n*subFrames*index.APBands)
	index.F0 = make([]float32, n*subFrames)
	index.File = make([]int32, n)
	for _, data := range []any{half, index.Spectrum, index.Aperiodicity, index.F0, index.File} {
		if err := binary.Read(reader, binary.LittleEndian, data); err != nil {
			return nil, fmt.Errorf("%s is truncated: %w", path, err)
		}
	}
	index.Features = make([]float32, len(half))
	for i, v := range half {
		index.Features[i] = halfToFloat32(v)
	}
	return index, nil
}

func float32ToHalf(value float32) uint16 {
	bits := math.Float32bits(value)
	sign := uint16(bits>>16) & 0x8000
	exponent := int((bits>>23)&0xff) - 127 + 15
	mantissa := bits & 0x7fffff
	switch {
	case exponent <= 0:
		if exponent < -10 {
			return sign
		}
		mantissa |= 0x800000
		shift := uint(14 - exponent)
		half := uint16(mantissa >> shift)
		if (mantissa>>(shift-1))&1 != 0 {
			half++
		}
		return sign | half
	case exponent >= 31:
		return sign | 0x7c00
	}
	half := sign | uint16(exponent)<<10 | uint16(mantissa>>13)
	if mantissa&0x1000 != 0 {
		half++
	}
	return half
}

func halfToFloat32(value uint16) float32 {
	sign := uint32(value&0x8000) << 16
	exponent := uint32(value>>10) & 0x1f
	mantissa := uint32(value & 0x3ff)
	switch {
	case exponent == 0:
		subnormal := float32(mantissa) / 1024 / 16384
		if sign != 0 {
			subnormal = -subnormal
		}
		return subnormal
	case exponent == 31:
		return math.Float32frombits(sign | 0x7f800000 | mantissa<<13)
	}
	return math.Float32frombits(sign | (exponent+112)<<23 | mantissa<<13)
}
