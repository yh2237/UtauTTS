package refmatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/yh2237/UtauTTS/internal/atomicfile"
)

const (
	partMagic   = "UTTPART1"
	partVersion = 1
)

var errBadPart = errors.New("part cache is not usable")

// PartCacheは録音1本ぶんの解析結果をファイルに保存する。
// 索引を作り直すとき、内容が変わっていない録音を解析し直さないために使う。
type PartCache struct {
	dir string
}

// OpenPartCacheは保存先のフォルダを用意する。作れなければnilを返し、キャッシュを使わない。
func OpenPartCache(dir string) *PartCache {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	return &PartCache{dir: dir}
}

// partKeyは解析条件と録音の中身から決まる。音源の場所が変わっても同じ録音なら使い回せる。
func partKey(model string, data []byte) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\x00%d\x00", model, partVersion)
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func (c *PartCache) path(key string) string {
	return filepath.Join(c.dir, key[:2], key+".part")
}

// Loadは保存した解析結果を返す。ないか読めなければfalse。
func (c *PartCache) Load(key string) (fileFrames, bool) {
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return fileFrames{}, false
	}
	frames, err := decodePart(data)
	if err != nil {
		return fileFrames{}, false
	}
	return frames, true
}

// Saveは解析結果を保存する。失敗しても索引の作成は続ける。
func (c *PartCache) Save(key string, frames fileFrames) {
	path := c.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = atomicfile.WriteFile(path, encodePart(frames))
}

func encodePart(frames fileFrames) []byte {
	var buffer bytes.Buffer
	buffer.WriteString(partMagic)
	usable := uint32(0)
	if frames.usable {
		usable = 1
	}
	for _, value := range []uint32{
		partVersion, usable, uint32(frames.rate), uint32(frames.fftSize),
		uint32(frames.dims), uint32(frames.apBands), uint32(frames.frames),
	} {
		binary.Write(&buffer, binary.LittleEndian, value)
	}
	for _, values := range [][]float32{frames.features, frames.spectrum, frames.aperiodicity, frames.f0} {
		binary.Write(&buffer, binary.LittleEndian, values)
	}
	return buffer.Bytes()
}

func decodePart(data []byte) (fileFrames, error) {
	reader := bytes.NewReader(data)
	magic := make([]byte, len(partMagic))
	if _, err := io.ReadFull(reader, magic); err != nil || string(magic) != partMagic {
		return fileFrames{}, errBadPart
	}
	var header [7]uint32
	for i := range header {
		if err := binary.Read(reader, binary.LittleEndian, &header[i]); err != nil {
			return fileFrames{}, errBadPart
		}
	}
	version, usable, rate, fftSize, dims, apBands, frames := header[0], header[1], header[2], header[3], header[4], header[5], header[6]
	if version != partVersion || frames > 1<<24 || dims > 1<<12 || apBands > 1<<10 {
		return fileFrames{}, errBadPart
	}
	result := fileFrames{
		rate: int(rate), fftSize: int(fftSize), dims: int(dims), apBands: int(apBands),
		frames: int(frames), usable: usable == 1,
	}
	sizes := []int{
		int(frames) * int(dims),
		int(frames) * subFrames * spectrumDims,
		int(frames) * subFrames * int(apBands),
		int(frames) * subFrames,
	}
	targets := []*[]float32{&result.features, &result.spectrum, &result.aperiodicity, &result.f0}
	for i, size := range sizes {
		values := make([]float32, size)
		if err := binary.Read(reader, binary.LittleEndian, values); err != nil {
			return fileFrames{}, errBadPart
		}
		*targets[i] = values
	}
	return result, nil
}
