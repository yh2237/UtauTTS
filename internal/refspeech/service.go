package refspeech

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/refmatch"
	"github.com/yh2237/UtauTTS/internal/worldrender"
)

// IndexStateは音源の索引の状態。
type IndexState string

const (
	IndexMissing  IndexState = ""
	IndexBuilding IndexState = "building"
	IndexReady    IndexState = "ready"
	IndexFailed   IndexState = "failed"
)

type IndexStatus struct {
	State    IndexState `json:"state"`
	Progress float64    `json:"progress,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// Serviceはモデルと音源ごとの索引を持ち、索引を裏で作る。
type Service struct {
	models   Models
	cacheDir string

	mu      sync.Mutex
	buildMu sync.Mutex
	// foregroundは読み上げの間に持つ。索引の作成は、ファイルごとに読み上げが終わるのを待つ。
	foreground sync.RWMutex
	engine     *Engine
	worlds     map[string]*worldrender.World
	indexes    map[string]*indexEntry
}

type indexEntry struct {
	status IndexStatus
	index  *refmatch.Index
	done   chan struct{}
}

var (
	sharedMu       sync.Mutex
	sharedServices = map[string]*Service{}

	batchMu sync.Mutex
	batch   indexBatch
)

// indexBatchは、作成中の索引がなくなるまでに作り始めた索引の数と、作り終えた数。
type indexBatch struct {
	total, done int
	current     float64
	failed      []string
}

// Progressは索引の作成の進み具合。
type Progress struct {
	Building bool     `json:"building"`
	Progress float64  `json:"progress"`
	Failed   []string `json:"failed,omitempty"`
}

// CurrentProgressは、作成中の索引がなくなるまでにまとめて作った索引全体の進み具合を返す。
// 作成中の索引がなくなると、それまでに失敗した索引を返して数え直す。
func CurrentProgress() Progress {
	batchMu.Lock()
	defer batchMu.Unlock()
	if batch.total == 0 || batch.done >= batch.total {
		progress := Progress{Failed: batch.failed}
		batch = indexBatch{}
		return progress
	}
	return Progress{Building: true, Progress: (float64(batch.done) + batch.current) / float64(batch.total)}
}

// Sharedはモデルの置き場所ごとに1つのServiceを返す。
func Shared(models Models, cacheDir string) *Service {
	key := strings.Join([]string{models.KokoroWeights, models.HuBERT, cacheDir}, "\x00")
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if service, ok := sharedServices[key]; ok {
		return service
	}
	service := &Service{models: models, cacheDir: cacheDir, worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	sharedServices[key] = service
	return service
}

// DefaultCacheDirは利用者ごとのキャッシュに置く索引のディレクトリ。
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "UtauTTS", "english-reference"), nil
}

func (s *Service) openEngine() (*Engine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.engine == nil {
		engine, err := Open(s.models)
		if err != nil {
			return nil, err
		}
		s.engine = engine
	}
	return s.engine, nil
}

func (s *Service) open(worldEnginePath string) (*Engine, *worldrender.World, error) {
	engine, err := s.openEngine()
	if err != nil {
		return nil, nil, err
	}
	if worldEnginePath == "" {
		return nil, nil, errors.New("English synthesis needs the UtauTTS WORLD engine")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	world := s.worlds[worldEnginePath]
	if world == nil {
		world, err = worldrender.OpenWorld(worldEnginePath)
		if err != nil {
			return nil, nil, err
		}
		s.worlds[worldEnginePath] = world
	}
	return engine, world, nil
}

// Prepareは音源の索引がないか、録音が変わっていれば、裏で作り始める。
func (s *Service) Prepare(bankRoot string, recordings []string, worldEnginePath string) {
	s.mu.Lock()
	entry := s.indexes[bankRoot]
	var index *refmatch.Index
	if entry != nil {
		index = entry.index
	}
	s.mu.Unlock()
	if index != nil {
		if _, key, err := bankRecordings(bankRoot, recordings); err == nil && key != index.Source {
			s.mu.Lock()
			if s.indexes[bankRoot] == entry {
				delete(s.indexes, bankRoot)
			}
			s.mu.Unlock()
		}
	}
	s.entry(bankRoot, recordings, worldEnginePath)
}

// Indexは音源の索引を返す。作成中なら終わるまで待つ。
func (s *Service) Index(ctx context.Context, bankRoot string, recordings []string, worldEnginePath string) (*refmatch.Index, error) {
	entry := s.entry(bankRoot, recordings, worldEnginePath)
	select {
	case <-entry.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry.index == nil {
		return nil, fmt.Errorf("voicebank index: %s", entry.status.Error)
	}
	return entry.index, nil
}

func (s *Service) entry(bankRoot string, recordings []string, worldEnginePath string) *indexEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.indexes[bankRoot]; ok && entry.status.State != IndexFailed {
		return entry
	}
	entry := &indexEntry{status: IndexStatus{State: IndexBuilding}, done: make(chan struct{})}
	s.indexes[bankRoot] = entry
	batchMu.Lock()
	batch.total++
	batchMu.Unlock()
	go s.build(bankRoot, recordings, worldEnginePath, entry)
	return entry
}

// buildは索引を1つずつ作る。どの索引の作成も全コアを使うため。
func (s *Service) build(bankRoot string, recordings []string, worldEnginePath string, entry *indexEntry) {
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	index, err := s.loadOrBuild(bankRoot, recordings, worldEnginePath, func(done, total int) {
		progress := float64(done) / float64(max(total, 1))
		s.mu.Lock()
		entry.status.Progress = progress
		s.mu.Unlock()
		batchMu.Lock()
		batch.current = progress
		batchMu.Unlock()
	})
	batchMu.Lock()
	batch.done++
	batch.current = 0
	if err != nil {
		batch.failed = append(batch.failed, fmt.Sprintf("%s: %v", filepath.Base(bankRoot), err))
	}
	batchMu.Unlock()
	s.mu.Lock()
	if err != nil {
		entry.status = IndexStatus{State: IndexFailed, Error: err.Error()}
	} else {
		entry.status = IndexStatus{State: IndexReady, Progress: 1}
		entry.index = index
	}
	s.mu.Unlock()
	close(entry.done)
}

func (s *Service) loadOrBuild(bankRoot string, recordings []string, worldEnginePath string, progress func(done, total int)) (*refmatch.Index, error) {
	files, key, err := bankRecordings(bankRoot, recordings)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(s.cacheDir, key+".idx")
	if index, err := refmatch.LoadIndex(path); err == nil && index.Model == FeatureModel && index.Source == key {
		return index, nil
	}
	engine, world, err := s.open(worldEnginePath)
	if err != nil {
		return nil, err
	}
	features := func(samples []float64, rate int) ([]float32, int, int, error) {
		s.foreground.Lock()
		s.foreground.Unlock()
		return engine.Features(samples, rate)
	}
	index, err := refmatch.BuildIndex(files, FeatureModel, world, features, progress, refmatch.OpenPartCache(filepath.Join(s.cacheDir, "parts")))
	if err != nil {
		return nil, err
	}
	index.Source = key
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return nil, err
	}
	if err := index.Save(path); err != nil {
		return nil, err
	}
	return index, nil
}

// bankRecordingsは索引に使う録音と、その一覧・大きさ・更新時刻から作るキーを返す。
func bankRecordings(bankRoot string, recordings []string) ([]string, string, error) {
	sorted := append([]string(nil), recordings...)
	sort.Strings(sorted)
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\x00%s\x00", FeatureModel, filepath.Clean(bankRoot))
	var files []string
	for _, path := range sorted {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		files = append(files, path)
		fmt.Fprintf(hash, "%s\x00%d\x00%d\x00", path, info.Size(), info.ModTime().UnixNano())
	}
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no WAV files in %s", bankRoot)
	}
	return files, hex.EncodeToString(hash.Sum(nil))[:32], nil
}

// Speakは音源の索引を使って音素列を合成する。
func (s *Service) Speak(ctx context.Context, bankRoot string, recordings []string, worldEnginePath string, pieces []Piece, options Options) (Speech, error) {
	index, err := s.Index(ctx, bankRoot, recordings, worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	engine, world, err := s.open(worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.Speak(index, world, pieces, options)
}

// SpeakReferenceは、参照の読みから音源のフレームで合成し直す。参照TTSの種類に依存しない入口。
func (s *Service) SpeakReference(ctx context.Context, bankRoot string, recordings []string, worldEnginePath string, reference Reference, options Options) (Speech, error) {
	index, err := s.Index(ctx, bankRoot, recordings, worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	engine, world, err := s.open(worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.SpeakReference(index, world, reference, options)
}

// SpeakAudioは参照の音声から、音源のフレームで合成し直す。言語に依存しない入口。
func (s *Service) SpeakAudio(ctx context.Context, bankRoot string, recordings []string, worldEnginePath string, audio []float64, sampleRate int) (Speech, error) {
	index, err := s.Index(ctx, bankRoot, recordings, worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	engine, world, err := s.open(worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.SpeakAudio(index, world, audio, sampleRate)
}

// ReadProsodyは音素列をKokoroで読み、記号ごとの長さとF0を返す。音源と索引は使わない。
func (s *Service) ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error) {
	engine, err := s.openEngine()
	if err != nil {
		return Reference{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.ReadProsody(pieces, speed, durations)
}
