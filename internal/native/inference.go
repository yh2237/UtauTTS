package native

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	pathpkg "path"
	"strings"

	"github.com/yh2237/UtauTTS/internal/tts"
)

// InferenceDirは、Web版の推論の結果（float32）を置く仮想FSのフォルダ。
const InferenceDir = "/tmp/utautts-inference"

// provideInferenceは、Web版でONNX Runtime Webが求めた推論の結果を受け取る。
// 大きい配列（音声と特徴量）は仮想FSのfloat32のファイルで受け取り、読んだら消す。
func (e *Engine) provideInference(requestJSON []byte) error {
	var request struct {
		Reads []struct {
			Key       string    `json:"key"`
			Durations []int     `json:"durations"`
			F0        []float32 `json:"f0"`
			AudioPath string    `json:"audio_path"`
		} `json:"reads"`
		Features []struct {
			Key    string `json:"key"`
			Path   string `json:"path"`
			Frames int    `json:"frames"`
			Dims   int    `json:"dims"`
		} `json:"features"`
		Prefetched []string `json:"prefetched"`
	}
	if err := json.Unmarshal(requestJSON, &request); err != nil {
		return fmt.Errorf("decode inference results: %w", err)
	}
	reads := make([]tts.ReferenceRead, len(request.Reads))
	for i, read := range request.Reads {
		reads[i] = tts.ReferenceRead{Key: read.Key, Durations: read.Durations, F0: read.F0}
		if read.AudioPath != "" {
			audio, err := takeFloat32File(read.AudioPath)
			if err != nil {
				return err
			}
			reads[i].Audio = audio
		}
	}
	features := make([]tts.ReferenceFeatures, len(request.Features))
	for i, item := range request.Features {
		values, err := takeFloat32File(item.Path)
		if err != nil {
			return err
		}
		features[i] = tts.ReferenceFeatures{Key: item.Key, Values: values, Frames: item.Frames, Dims: item.Dims}
	}
	return e.synth.ProvideReferenceInference(reads, features, request.Prefetched)
}

func takeFloat32File(path string) ([]float32, error) {
	return takeFloat32FileIn(InferenceDir, path)
}

// takeFloat32FileInは、dirの下にある推論の結果だけを読んで消す。
// 仮想FSのパスはスラッシュで区切るため、Windowsでも同じ規則で比べる。
func takeFloat32FileIn(dir, path string) ([]float32, error) {
	root := pathpkg.Clean(strings.ReplaceAll(dir, "\\", "/"))
	clean := pathpkg.Clean(strings.ReplaceAll(path, "\\", "/"))
	if !strings.HasPrefix(clean, root+"/") {
		return nil, fmt.Errorf("%s: not an inference result", path)
	}
	// Read and delete the path that was checked. On Unix a backslash is a
	// literal filename character, so the original spelling can resolve
	// somewhere else than the slash-normalized path.
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(clean)
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("%s: %d bytes is not float32", path, len(data))
	}
	values := make([]float32, len(data)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return values, nil
}
