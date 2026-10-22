package refmatch

import (
	"bufio"
	"bytes"
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
	"github.com/yh2237/gograd/dsp"
	"github.com/yh2237/gograd/tensor"
)

const (
	// FrameMSは特徴量1フレームの長さ。WORLDはその1/subFramesの周期で解析する。
	FrameMS      = 20.0
	subFrames    = 4
	worldFrameMS = FrameMS / subFrames
	spectrumDims = 60
	// d4cThresholdは索引の解析に使うD4Cのしきい値。原音接続（worldrenderのunitD4CThreshold）の0と違い、
	// WORLDの既定値を使う。0では参照照合の出力の高音に雑音が乗った。
	d4cThreshold   = 0.85
	quietMarginC0  = 9.0
	indexMagic     = "UTRM"
	indexVersion   = 4
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

// Recordingは索引に使う録音と、その録音を使う原音のoto.iniの範囲。
type Recording struct {
	Path    string
	Regions []Region
}

// Regionはoto.iniのoffsetとblank（ミリ秒）。blankが負ならoffsetからの長さ、正なら録音の末尾から除く長さ。
type Region struct {
	Offset, Blank float64
}

// spanは、最初の原音の始まりから最後の原音の終わりまで（ミリ秒）を返す。範囲が決まらなければ録音の全体を返す。
// 範囲の外には、収録の前後の息や物音が入っていることがある。
func (r Recording) span(durationMS float64) (start, end float64) {
	start, end = math.Inf(1), math.Inf(-1)
	for _, region := range r.Regions {
		last := durationMS - region.Blank
		if region.Blank < 0 {
			last = region.Offset - region.Blank
		}
		if last <= region.Offset {
			continue
		}
		start, end = min(start, region.Offset), max(end, last)
	}
	if start >= end {
		return 0, durationMS
	}
	return max(start, 0), min(end, durationMS)
}

// BuildIndexは録音を解析して索引を作る。最初のファイルと標本化周波数が違うファイルと0.1秒未満のファイルは使わない。
// WORLDの解析はファイルごとに並列に行う。cacheを渡すと、内容と範囲が同じ録音の解析結果を再利用する。
func BuildIndex(recordings []Recording, model string, world World, features FeatureFunc, progress func(done, total int), cache *PartCache) (*Index, error) {
	files := make([]string, len(recordings))
	for i, recording := range recordings {
		files[i] = recording.Path
	}
	// 録音ごとに標本化周波数が違う音源があるため、索引の周波数を先に決めて揃える。
	targetRate := chooseIndexRate(files)
	results := make([]fileFrames, len(files))
	jobs := make(chan int)
	var mu sync.Mutex
	var firstErr error
	done := 0
	var wg sync.WaitGroup
	for range max(1, min(runtime.GOMAXPROCS(0), len(files))) {
		wg.Go(func() {
			for number := range jobs {
				result, err := analyzeFile(recordings[number], model, world, features, cache, targetRate)
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
		if !result.usable {
			continue
		}
		if index.SampleRate == 0 {
			index.SampleRate = result.rate
		}
		if result.rate != index.SampleRate {
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
	// 索引はint8とfloat16で保存する。作った直後と読み直した後で結果が変わらないよう、ここでも丸める。
	index.Features = quantizeRoundTrip(index.Features, index.Dims)
	index.Spectrum = roundToHalf(index.Spectrum)
	index.Aperiodicity = roundToHalf(index.Aperiodicity)
	index.F0 = roundToHalf(index.F0)
	return index, nil
}

// quantizeFeaturesは、フレームごとの最大値でint8に量子化し、スケールを返す。
func quantizeFeatures(values []float32, frames, dims int) ([]int8, []float32) {
	quantized := make([]int8, len(values))
	scales := make([]float32, frames)
	for frame := 0; frame < frames; frame++ {
		row := values[frame*dims : (frame+1)*dims]
		var peak float32
		for _, value := range row {
			if absolute := float32(math.Abs(float64(value))); absolute > peak {
				peak = absolute
			}
		}
		if peak == 0 {
			scales[frame] = 1
			continue
		}
		scale := peak / 127
		scales[frame] = scale
		for i, value := range row {
			quantized[frame*dims+i] = int8(math.Round(float64(value / scale)))
		}
	}
	return quantized, scales
}

// dequantizeFeaturesは、int8とスケールから元の値に戻す。
func dequantizeFeatures(quantized []int8, scales []float32, dims int) []float32 {
	values := make([]float32, len(quantized))
	for i, value := range quantized {
		values[i] = float32(value) * scales[i/dims]
	}
	return values
}

// quantizeRoundTripは、保存するint8に丸めてから戻す。
func quantizeRoundTrip(values []float32, dims int) []float32 {
	if dims <= 0 {
		return values
	}
	frames := len(values) / dims
	quantized, scales := quantizeFeatures(values, frames, dims)
	return dequantizeFeatures(quantized, scales, dims)
}

// roundToHalfは、値をfloat16に丸めてfloat32へ戻す。保存した索引と同じ値にするために使う。
func roundToHalf(values []float32) []float32 {
	out := make([]float32, len(values))
	for i, value := range values {
		out[i] = halfToFloat32(float32ToHalf(value))
	}
	return out
}

type fileFrames struct {
	rate, fftSize, dims, apBands, frames int
	usable                               bool
	features, spectrum, aperiodicity, f0 []float32
}

// analyzeFileは、oto.iniの範囲の外と、95パーセンタイルの音量から約45dB以上小さいフレームを除いて、1ファイル分のフレームを返す。
// 内容と範囲が同じ録音の解析結果がcacheにあれば、解析せずにそれを返す。標本化周波数はtargetRateに揃える。
func analyzeFile(recording Recording, model string, world World, features FeatureFunc, cache *PartCache, targetRate int) (fileFrames, error) {
	data, err := os.ReadFile(recording.Path)
	if err != nil {
		// 読めない形式のWAV（PCM以外など）は、索引に入れずに飛ばす。
		return fileFrames{}, nil
	}
	start, end := 0.0, math.Inf(1)
	if _, duration, err := wavInfo(recording.Path); err == nil {
		start, end = recording.span(duration * 1000)
	}
	key := ""
	if cache != nil {
		key = partKey(model, data, targetRate, start, end)
		if frames, ok := cache.Load(key); ok {
			return frames, nil
		}
	}
	result, err := analyzeBytes(data, world, features, targetRate, start, end)
	if err != nil {
		return result, err
	}
	if key != "" {
		cache.Save(key, result)
	}
	return result, nil
}

// chooseIndexRateは、索引の標本化周波数を決める。短すぎない録音でいちばん多い周波数を使い、48kHzを上限にする。
func chooseIndexRate(files []string) int {
	counts := map[int]int{}
	for _, path := range files {
		if rate, duration, err := wavInfo(path); err == nil && rate > 0 && duration >= minimumFileSec {
			counts[rate]++
		}
	}
	best, bestCount := 0, 0
	for rate, count := range counts {
		if count > bestCount || (count == bestCount && rate > best) {
			best, bestCount = rate, count
		}
	}
	if best > 48000 {
		best = 48000
	}
	return best
}

// wavInfoは、WAVのヘッダだけを読んで標本化周波数と長さ（秒）を返す。
func wavInfo(path string) (int, float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	header := make([]byte, 12)
	if _, err := io.ReadFull(file, header); err != nil {
		return 0, 0, err
	}
	if string(header[:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, 0, errors.New("not a WAV file")
	}
	rate, channels, bits := 0, 0, 0
	for {
		chunk := make([]byte, 8)
		if _, err := io.ReadFull(file, chunk); err != nil {
			return 0, 0, err
		}
		size := int(binary.LittleEndian.Uint32(chunk[4:]))
		switch string(chunk[:4]) {
		case "fmt ":
			format := make([]byte, size)
			if _, err := io.ReadFull(file, format); err != nil {
				return 0, 0, err
			}
			if len(format) < 16 {
				return 0, 0, errors.New("bad fmt chunk")
			}
			channels = int(binary.LittleEndian.Uint16(format[2:]))
			rate = int(binary.LittleEndian.Uint32(format[4:]))
			bits = int(binary.LittleEndian.Uint16(format[14:]))
		case "data":
			bytesPerFrame := channels * (bits / 8)
			if rate == 0 || bytesPerFrame == 0 {
				return 0, 0, errors.New("data before fmt")
			}
			return rate, float64(size) / float64(bytesPerFrame*rate), nil
		default:
			if _, err := file.Seek(int64(size+size%2), io.SeekCurrent); err != nil {
				return 0, 0, err
			}
		}
	}
}

// analyzeBytesは、中心がstartからendまで（ミリ秒）のフレームを使う。
func analyzeBytes(data []byte, world World, features FeatureFunc, targetRate int, start, end float64) (fileFrames, error) {
	samples, rate, err := decodeMono(data)
	if err != nil {
		// 読めない形式のWAV（PCM以外など）は、索引に入れずに飛ばす。
		return fileFrames{}, nil
	}
	if targetRate > 0 && rate != targetRate {
		converted := dsp.Resample(float32Values(samples), rate, targetRate)
		samples = make([]float64, len(converted))
		for i, value := range converted {
			samples[i] = float64(value)
		}
		rate = targetRate
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
		if center := (float64(frame) + 0.5) * FrameMS; center < start || center >= end {
			continue
		}
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

// float32Valuesは、標本化周波数を変えるためにfloat64の波形をfloat32へ移す。
func float32Values(samples []float64) []float32 {
	values := make([]float32, len(samples))
	for i, sample := range samples {
		values[i] = float32(sample)
	}
	return values
}

func decodeMono(data []byte) ([]float64, int, error) {
	pcm, err := audio.DecodeWav(bytes.NewReader(data))
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
		quantized, scales := quantizeFeatures(index.Features, index.Frames, index.Dims)
		spectrum := make([]uint16, len(index.Spectrum))
		for i, v := range index.Spectrum {
			spectrum[i] = float32ToHalf(v)
		}
		aperiodicity := make([]uint16, len(index.Aperiodicity))
		for i, v := range index.Aperiodicity {
			aperiodicity[i] = float32ToHalf(v)
		}
		f0 := make([]uint16, len(index.F0))
		for i, v := range index.F0 {
			f0[i] = float32ToHalf(v)
		}
		for _, data := range []any{quantized, scales, spectrum, aperiodicity, f0, index.File} {
			if err := binary.Write(buffered, binary.LittleEndian, data); err != nil {
				return err
			}
		}
		return buffered.Flush()
	})
}

// ReadHeaderは索引ファイルのヘッダだけを読む。中身を読まずに使えるかを確かめるときに使う。
func ReadHeader(path string) (Header, error) {
	file, err := os.Open(path)
	if err != nil {
		return Header{}, err
	}
	defer file.Close()
	return readHeader(bufio.NewReader(file))
}

// readHeaderは、マジックとヘッダを読む。
func readHeader(reader *bufio.Reader) (Header, error) {
	magic := make([]byte, len(indexMagic))
	if _, err := io.ReadFull(reader, magic); err != nil || string(magic) != indexMagic {
		return Header{}, fmt.Errorf("not a reference matching index")
	}
	var length uint32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil || length == 0 || length > 1<<26 {
		return Header{}, fmt.Errorf("invalid header")
	}
	header := make([]byte, length)
	if _, err := io.ReadFull(reader, header); err != nil {
		return Header{}, err
	}
	var parsed Header
	if err := json.Unmarshal(header, &parsed); err != nil {
		return Header{}, err
	}
	return parsed, nil
}

// Versionは索引の版を返す。版が変わった索引は使えない。
func Version() int { return indexVersion }

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
	quantized := make([]int8, n*index.Dims)
	scales := make([]float32, n)
	spectrum := make([]uint16, n*subFrames*spectrumDims)
	aperiodicity := make([]uint16, n*subFrames*index.APBands)
	f0 := make([]uint16, n*subFrames)
	index.File = make([]int32, n)
	for _, data := range []any{quantized, scales, spectrum, aperiodicity, f0, index.File} {
		if err := binary.Read(reader, binary.LittleEndian, data); err != nil {
			return nil, fmt.Errorf("%s is truncated: %w", path, err)
		}
	}
	index.Features = dequantizeFeatures(quantized, scales, index.Dims)
	index.Spectrum = make([]float32, len(spectrum))
	for i, v := range spectrum {
		index.Spectrum[i] = halfToFloat32(v)
	}
	index.Aperiodicity = make([]float32, len(aperiodicity))
	for i, v := range aperiodicity {
		index.Aperiodicity[i] = halfToFloat32(v)
	}
	index.F0 = make([]float32, len(f0))
	for i, v := range f0 {
		index.F0[i] = halfToFloat32(v)
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
